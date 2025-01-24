package scrape

import (
	"context"
	"etracker/internal/bencode"
	"etracker/internal/config"
	"fmt"
	"log"
	"net/http"

	bencode_go "github.com/jackpal/bencode-go"
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
	_, _ = w.Write(bencode.FailureReason(reason))
}

// ScrapeHandler implements the scrape convention to return information on
// currently available torrents. For more information, see
// https://wiki.theory.org/BitTorrentSpecification#Tracker_.27scrape.27_Convention
func ScrapeHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// info_hashes, ok := r.URL.Query()["info_hash"]
		query := fmt.Sprintf(`
			WITH recent_announces AS (
			    SELECT DISTINCT ON (peer_id_id)
				amount_left,
				info_hash_id
			    FROM
				peers
			    WHERE
				last_announce >= NOW() - INTERVAL '%s'
				AND event <> $1
			    ORDER BY
				peer_id_id,
				last_announce DESC
			)
			SELECT
			    info_hash,
			    name,
			    downloaded,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left > 0) AS leechers,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left = 0) AS seeders
			FROM
			    infohashes
			    LEFT JOIN recent_announces ON infohashes.id = recent_announces.info_hash_id
			GROUP BY
			    info_hash,
			    name,
			    downloaded
			`,
			"60 minutes")
		rows, err := conf.Dbpool.Query(context.Background(), query, config.Stopped)
		if err != nil {
			log.Printf("Error fetching data for scrape: %v", err)
			abortScrape(w, "error fetching data for scrape")
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
				// This error will be handled when rows.Err() is checked.
				break
			}
			scrape.Files[string(info_hash)] = File{complete, downloaded, incomplete, name}
		}

		if rows.Err() != nil {
			log.Printf("Error parsing data for scrape: %v", rows.Err())
			abortScrape(w, "error parsing data for scrape")
			return
		}

		err = bencode_go.Marshal(w, scrape)
		if err != nil {
			// Log an error if we are unable to respond to client.
			log.Printf("Error sending scrape response to client: %v", err)
		}
	}
}
