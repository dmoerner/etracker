package scrape

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/dmoerner/etracker/internal/bencode"
	"github.com/dmoerner/etracker/internal/config"

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
//
// Query is constructed in three stages, since SQL requires inserting the
// optional WHERE specification for specific infohashes in the middle of the
// query.
func ScrapeHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Start constructing query.
		query := fmt.Sprintf(`
			WITH recent_announces AS (
			    SELECT DISTINCT ON (announce_id, info_hash_id)
				amount_left,
				info_hash_id
			    FROM
				peers
			    WHERE
				last_announce >= NOW() - INTERVAL '%d seconds'
				AND event <> $1
			    ORDER BY
				announce_id,
				info_hash_id,
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
			`,
			config.StaleInterval)

		// This must be type []any to match the signature of pgxpool.Query(), and because
		// it takes multiple types.
		var paramsSlice []any
		paramsSlice = append(paramsSlice, config.Stopped)

		if infoHashes, ok := r.URL.Query()["info_hash"]; ok {
			query += `WHERE `
			for idx, info_hash := range infoHashes {
				if idx > 0 {
					query += " OR "
				}
				unescaped, err := url.QueryUnescape(info_hash)
				if err != nil {
					// Errors are skipped, clients have the responsibility to send
					// proper infohashes.
					paramsSlice = append(paramsSlice, []byte(""))
				} else {
					paramsSlice = append(paramsSlice, []byte(unescaped))
				}
				// Slice is zero-indexed, but SQL parameters are one-indexed, and
				// the first parameter is already taken.
				query += fmt.Sprintf("info_hash = $%d", idx+2)
			}
		}

		query += `
			GROUP BY
			    info_hash,
			    name,
			    downloaded
			`
		// Finished constructing query.

		rows, err := conf.Dbpool.Query(context.Background(), query, paramsSlice...)
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
