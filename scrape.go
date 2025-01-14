package main

import (
	"context"
	"fmt"
	"net/http"

	bencode "github.com/jackpal/bencode-go"
)

type Scrape struct {
	Files map[string]File `bencode:"files"`
}

type File struct {
	Complete   int    `bencode:"complete"`
	Downloaded int    `bencode:"downloaded"`
	Incomplete int    `bencode:"incomplete"`
	Name       string `bencode:"name"`
}

// abortScrape is a helper function to write a failure reason to the peer. This
// is an unofficial extension to the scraping protocol. Errors do not need to
// be logged.
func abortScrape(w http.ResponseWriter, reason string) {
	_, _ = w.Write(FailureReason(reason))
}

// ScrapeHandler implements the scrape convention to return information on
// currently available torrents. For more information, see
// https://wiki.theory.org/BitTorrentSpecification#Tracker_.27scrape.27_Convention
func ScrapeHandler(config Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// info_hashes, ok := r.URL.Query()["info_hash"]
		query := fmt.Sprintf(
			`WITH recent_announces AS (
			SELECT DISTINCT ON (peer_id) amount_left, info_hash FROM peers
			WHERE last_announce >= NOW() - INTERVAL '%s' AND event <> $1
			ORDER BY peer_id, last_announce DESC
			)
			SELECT infohashes.info_hash, name, downloaded, COUNT(*) FILTER(WHERE recent_announces.amount_left > 0) AS leechers, COUNT(*) FILTER(WHERE recent_announces.amount_left = 0) as seeders FROM infohashes
			LEFT JOIN recent_announces
			ON infohashes.info_hash = recent_announces.info_hash
			GROUP BY infohashes.info_hash`, "60 minutes")
		rows, err := config.dbpool.Query(context.Background(), query, stopped)
		if err != nil {
			abortScrape(w, "error fetching information for scrape")
			return
		}

		defer rows.Close()

		var scrape Scrape

		scrape.Files = make(map[string]File)

		for rows.Next() {
			var info_hash []byte
			var name string
			var downloaded int
			var incomplete int
			var complete int

			err = rows.Scan(&info_hash, &name, &downloaded, &incomplete, &complete)
			if err != nil {
				abortScrape(w, "error constructing scrape response")
				return
			}
			scrape.Files[string(info_hash)] = File{complete, downloaded, incomplete, name}
		}

		if rows.Err() != nil {
			abortScrape(w, "error parsing db rows for scrape")
			return
		}

		err = bencode.Marshal(w, scrape)
		if err != nil {
			abortScrape(w, "error sending bencoded result")
			return
		}
	}
}
