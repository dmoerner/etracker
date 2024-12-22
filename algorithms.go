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
	query := fmt.Sprintf(`SELECT COUNT(*) FROM peers WHERE peer_id = $1 AND last_announce >= NOW() - INTERVAL '%s' and event <> $2;`, interval)
	var torrentCount int
	err := config.dbpool.QueryRow(context.Background(), query, a.peer_id, stopped).Scan(&torrentCount)
	if err != nil {
		return 0, fmt.Errorf("error determining announce count: %w", err)
	}

	var numToGive int

	if torrentCount >= a.numwant {
		numToGive = a.numwant
	} else {
		// Since this algorithm counts the present announce by this client, every client
		// is guaranteed to get at least one peer.
		numToGive = torrentCount
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
	query := fmt.Sprintf(`SELECT COUNT(*) FROM peers WHERE peer_id = $1 AND amount_left = 0 AND last_announce >= NOW() - INTERVAL '%s' and event <> $2;`, interval)
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
