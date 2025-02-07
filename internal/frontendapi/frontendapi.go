package frontendapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/jackc/pgx/v5"
)

type Stats struct {
	Hashcount int `json:"hashcount"`
	Seeders   int `json:"seeders"`
	Leechers  int `json:"leechers"`
}

type Key struct {
	Announce_key string `json:"announce_key"`
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%d", code)
	log.Printf("Error: %v", err)
}

func enableCors(conf config.Config, w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", fmt.Sprintf("http://localhost:%d", conf.Port))
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// StatsHandler presents a REST API on /frontendapi/stats which returns an object
// including the total tracked infohashes, seeders, and leechers.
func StatsHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(conf, &w)
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

		rows, err := conf.Dbpool.Query(context.Background(), query, config.Stopped)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		stats, err := pgx.CollectRows(rows, pgx.RowToStructByName[Stats])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		result, err := json.Marshal(stats[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		fmt.Fprintf(w, "%s", result)
	}
}

func GenerateHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(conf, &w)
		announce_key, err := config.GenerateAnnounceKey(conf)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		key := Key{Announce_key: announce_key}

		result, err := json.Marshal(key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		fmt.Fprintf(w, "%s", result)
	}
}
