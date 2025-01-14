// A roadmap for the algorithms developed here can be found here:
// https://moerner.com/posts/brainstorming-peer-distribution-algorithms/
package main

import (
	"context"
	"fmt"
)

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
//
// TODO: Implement better normalization than this simple linear algorithm which
// gives at least one peer.
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
	query := fmt.Sprintf(`SELECT amount_left, uploaded, downloaded FROM peers JOIN peerids ON peers.peer_id_id = peerids.id WHERE peer_id = $1 AND last_announce >= NOW() - INTERVAL '%s' and event <> $2;`, interval)
	rows, err := config.dbpool.Query(context.Background(), query, a.peer_id, stopped)
	if err != nil {
		return 0, fmt.Errorf("error querying for rows: %w", err)
	}
	defer rows.Close()

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

	// New peers always get at least one other peer. This totalCount test
	// is required to prevent division by zero.
	if totalCount == 1 || seededCount == 0 {
		return 1, nil
	}

	// Current algorithm: Number of seeds, increased in scale by the number of torrents
	// with a positive ratio. Remove the current torrent from the
	// totalCount; previous test will ensure we do not divide by zero.
	//
	// This still does not reward clients which are mostly
	// partial seeding.
	numToGive := seededCount * (1 + posRatio/totalCount)

	if numToGive >= a.numwant {
		numToGive = a.numwant
	}

	return numToGive, nil
}
