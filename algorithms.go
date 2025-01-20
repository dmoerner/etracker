// A roadmap for the algorithms developed here can be found here:
// https://moerner.com/posts/brainstorming-peer-distribution-algorithms/
package main

import (
	"context"
	"fmt"
	"math"
)

// The minimumPeers to return to a peer, and the minimum target goodSeedCount.
// Must be greater than zero.
const minimumPeers int = 5

// NumwantPeers is the non-intelligent algorithm which distributes peers up to
// the number requested by the client, not including themselves.
func NumwantPeers(config Config, a *Announce) (int, error) {
	return a.numwant, nil
}

// PeersForAnnounces, aka "Algorithm 1", gives peers to each client as a
// function of the number of torrents they have in their client.
//
// A problem with this algorithm is that freeriders can get around limits by always
// snatching more torrents. An improvement would count only torrents you are seeding,
// not torrents you are leeching as well.
func PeersForAnnounces(config Config, a *Announce) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM peers JOIN peerids ON peers.peer_id_id = peerids.id WHERE peer_id = $1 AND last_announce >= NOW() - INTERVAL '%s' and event <> $2;`, interval)
	var torrentCount int
	err := config.dbpool.QueryRow(context.Background(), query, a.peer_id, stopped).Scan(&torrentCount)
	if err != nil {
		return 0, fmt.Errorf("error determining announce count: %w", err)
	}

	var numToGive int

	if torrentCount >= a.numwant {
		numToGive = a.numwant
	} else {
		// Make sure even new peers get at least one peer.
		numToGive = torrentCount + 1
	}

	return numToGive, nil
}

// PeersForSeeds, aka "Algorithm 2", gives peers to each client as a function
// of the number of torrents they are seeding.
//
// A problem with this algorithm is that it does not count partial seeders.
func PeersForSeeds(config Config, a *Announce) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM peers JOIN peerids ON peers.peer_id_id = peerids.id WHERE peer_id = $1 AND amount_left = 0 AND last_announce >= NOW() - INTERVAL '%s' and event <> $2;`, interval)
	var torrentCount int
	err := config.dbpool.QueryRow(context.Background(), query, a.peer_id, stopped).Scan(&torrentCount)
	if err != nil {
		return 0, fmt.Errorf("error determining seed count: %w", err)
	}

	var numToGive int

	if torrentCount >= a.numwant {
		numToGive = a.numwant
	} else {
		// Make sure peers seeding nothing receive at least one peer.
		numToGive = torrentCount + 1
	}

	return numToGive, nil
}

// PeersForGoodSeeds, aka "Algorithm 3", gives peers to each client as a
// function of how many torrents they are seeding, and how much data they are
// uploading. This is intended to both reward fast uplinks and partial seeders
// who upload.
//
// This algorithm still does not reward partial seeders who do not upload, but
// this is intentional: If no one is uploading, the content is likely either
// unpopular or very well-seeded. In the former case we should incentivize only
// full seeders, and in the latter case there is nothing to reward.
//
// As I understand it, this calculation will only account for upload and download
// amounts in the current session. Therefore, we are indirectly rewarding only
// clients with long uptime or clients with recent activity. However, this is a
// necessary limitation of a public tracker algorithm which relies on peer_id's
// which reset on restart, rather than an unchanging, unique announce URL.
func PeersForGoodSeeds(config Config, a *Announce) (int, error) {
	if a.numwant == 0 {
		return 0, nil
	}

	query := fmt.Sprintf(`SELECT DISTINCT ON (info_hash_id) amount_left, uploaded, downloaded FROM peers JOIN peerids ON peers.peer_id_id = peerids.id WHERE peer_id = $1 AND last_announce >= NOW() - INTERVAL '%s' and event <> $2 ORDER BY info_hash_id, last_announce DESC;`, interval)
	rows, err := config.dbpool.Query(context.Background(), query, a.peer_id, stopped)
	if err != nil {
		return 0, fmt.Errorf("error querying for rows: %w", err)
	}
	defer rows.Close()

	// Calculate client score. TODO: Do this in postgres.
	var totalCount int
	var seededCount int
	var posRatio int
	for rows.Next() {
		var amount_left int
		var uploaded int
		var downloaded int

		err = rows.Scan(&amount_left, &uploaded, &downloaded)
		if err != nil {
			return 0, fmt.Errorf("scan error: %w", err)
		}

		totalCount += 1
		if amount_left == 0 {
			seededCount += 1
		}
		if downloaded == 0 {
			// The original uploader or a cross-seeder can report upload
			// while reporting no download.
			if uploaded > 0 {
				posRatio += 1
			}
		} else if uploaded/downloaded >= 1 {
			posRatio += 1
		}
	}
	// The peerScore is a function of seeded torrents, with a bonus for the
	// percentage of torrents with a positive ratio. Positive ratio data
	// will be noisy due to being reset on client restarts, so it is
	// treated only as a bonus.
	var peerScore int
	if totalCount == 0 {
		peerScore = 0
	} else {
		peerScore = seededCount * (1 + posRatio/totalCount)
	}

	// Calculate goodSeedCount, which is defined as seeding more torrents
	// than 1 standard deviation above the mean. The minimum for small swarms
	// is the constant minimumPeers.
	query = fmt.Sprintf(`
		WITH seed_counts AS
			(SELECT COUNT(*) as seed_count FROM peers JOIN peerids ON peers.peer_id_id = peerids.id WHERE amount_left = 0 AND last_announce >= NOW() - INTERVAL '%s' AND event <> $1 GROUP BY peerids.id)
		SELECT COALESCE((STDDEV_POP(seed_count) + AVG(seed_count))::INTEGER, $2) FROM seed_counts;`, interval)
	var goodSeedCount int
	err = config.dbpool.QueryRow(context.Background(), query, stopped, minimumPeers).Scan(&goodSeedCount)
	if err != nil {
		return 0, fmt.Errorf("error calculating current swarm seeder counts: %w", err)
	}

	numToGive := smoothFunction(peerScore, a.numwant, goodSeedCount)

	return numToGive, nil
}

// smoothFunction is a mathematical function from x to y which calculates how
// many peers to return (y) for a requesting client of score (x). It takes two
// additional parameters, numWanted, the number of peers requested by the
// client (an upper bound on y), and goodSeedCount, which is the target value
// of x at which numWanted peers should be returned.
//
// Written out without types, the function is:
//
//	y = minimumPeers + (numWanted - minimumPeers)*tanh(kx)
//
// where the steepness k is calculated as a function of goodSeedCount.
func smoothFunction(x, numWanted, goodSeedCount int) int {
	y_int := float64(minimumPeers)
	// delta must be non-zero
	delta := 0.1

	// Calculate the steepness k, for x = goodSeedCount, y = numWanted-delta.
	// Add the delta in the denominator to avoid division by zero.
	k := math.Atanh((float64(numWanted)-y_int-delta)/(float64(numWanted)-y_int+delta)) / float64(goodSeedCount)

	return int(y_int + (float64(numWanted)-y_int)*(math.Tanh(k*float64(x))))
}
