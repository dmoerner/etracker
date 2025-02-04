package web

import (
	"context"
	_ "embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/jackc/pgx/v5"
)

//go:embed template/index.html
var indexTemplate string

//go:embed template/allowlist.html
var allowlistTemplate string

type IndexStats struct {
	HashCount int
	Seeders   int
	Leechers  int
	Tls       bool
	Port      int
	TlsPort   int
}

type AllowlistRow struct {
	Info_hash  []byte
	Name       string
	Downloaded int
	Seeders    int
	Leechers   int
}

type AllowlistEntry struct {
	Info_hash  string
	Name       string
	Downloaded int
	Seeders    int
	Leechers   int
}

func WebHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		tmpl, err := template.New("index").Parse(indexTemplate)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500: Internal server error")
			log.Printf("Error parsing template: %v", err)
			return
		}

		// Populate instance of IndexStats struct.
		var stats IndexStats

		stats.Port = conf.Port

		if conf.Tls != (config.TLSConfig{}) {
			stats.Tls = true
			stats.TlsPort = conf.Tls.TlsPort
		}

		query := fmt.Sprintf(`
			WITH recent_announces AS (
			    SELECT DISTINCT ON (info_hash_id, announce_id)
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
			    COUNT(DISTINCT info_hash) AS hashcount,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left = 0) AS seeders,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left > 0) AS leechers
			FROM
			    infohashes
			    LEFT JOIN recent_announces ON infohashes.id = recent_announces.info_hash_id
			`,
			config.StaleInterval)

		err = conf.Dbpool.QueryRow(context.Background(), query, config.Stopped).Scan(&(stats.HashCount), &(stats.Seeders), &(stats.Leechers))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500: Internal server error")
			log.Printf("Error executing template: %v", err)
			return
		}

		err = tmpl.Execute(w, stats)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500: Internal server error")
			log.Printf("Error executing template: %v", err)
			return
		}
	}
}

// AllowlistHandler builds a table of allowlist entries with basic statistics
// for each entry. Since the info_hash is stored as a byte array in the
// database, we have to first extract the row into an AllowlistRow struct, then
// for templating we need to iterate and convert them into AllowlistEntry
// structs which represent the info_hash as hex.
func AllowlistHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("index").Parse(allowlistTemplate)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500: Internal server error")
			log.Printf("Error parsing template: %v", err)
			return
		}

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
			    COUNT(*) FILTER (WHERE recent_announces.amount_left = 0) AS seeders,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left > 0) AS leechers
			FROM
			    infohashes
			    LEFT JOIN recent_announces ON infohashes.id = recent_announces.info_hash_id
			GROUP BY
			    info_hash,
			    name,
			    downloaded
			ORDER BY
			    name
			`,
			config.StaleInterval)

		rows, err := conf.Dbpool.Query(context.Background(), query, config.Stopped)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500: Internal server error")
			log.Printf("Error executing template: %v", err)
			return
		}

		allowlistRows, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[AllowlistRow])
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500: Internal server error")
			log.Printf("Error executing template: %v", err)
			return
		}

		var allowlistEntries []AllowlistEntry

		for _, r := range allowlistRows {
			allowlistEntries = append(allowlistEntries,
				AllowlistEntry{
					Name:       r.Name,
					Info_hash:  hex.EncodeToString(r.Info_hash),
					Downloaded: r.Downloaded,
					Seeders:    r.Seeders,
					Leechers:   r.Leechers,
				})
		}

		err = tmpl.Execute(w, allowlistEntries)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "500: Internal server error")
			log.Printf("Error executing template: %v", err)
			return
		}
	}
}
