package web

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/dmoerner/etracker/internal/config"
)

//go:embed template/index.html
var indexTemplate string

type IndexStats struct {
	HashCount int
	Seeders   int
	Leechers  int
	Tls       bool
	Port      int
	TlsPort   int
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
			    COUNT(DISTINCT info_hash) AS hashcount,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left = 0) AS seeders,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left > 0) AS leechers
			FROM
			    infohashes
			    LEFT JOIN recent_announces ON infohashes.id = recent_announces.info_hash_id
			`,
			"60 minutes")

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
