// A roadmap for the algorithms developed here can be found here:
// https://moerner.com/posts/brainstorming-peer-distribution-algorithms/
package main

import (
	"context"
	"fmt"
)

// PeersForAnnounces, aka "Algorithm 1", takes the config and announce, and returns the number of peers
// to give and any error. The maximum return value is a.numwant.
//
// For distributing peers, we follow a simple algorithm: The more torrents you have
// in the swarm, the more peers you receive, up to numwant you request.
//
// A problem with this algorithm is that freeriders can get around limits by always
// snatching more torrents. An improvement would count only torrents you are seeding,
// not torrents you are leeching as well.
func PeersForAnnounces(config Config, a *Announce) (int, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM peers WHERE peer_id = $1 AND last_announce >= NOW() - INTERVAL '%s';`, interval)
	var torrentCount int
	err := config.dbpool.QueryRow(context.Background(), query, a.peer_id).Scan(&torrentCount)
	if err != nil {
		return 0, fmt.Errorf("error determining seed count: %w", err)
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
