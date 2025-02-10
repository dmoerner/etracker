package frontendapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/jackc/pgx/v5"
)

type StatsJSON struct {
	Hashcount int `json:"hashcount"`
	Seeders   int `json:"seeders"`
	Leechers  int `json:"leechers"`
}

type InfohashesJSON struct {
	Name       string `json:"name"`
	Downloaded int    `json:"downloaded"`
	Seeders    int    `json:"seeders"`
	Leechers   int    `json:"leechers"`
	Info_hash  []byte `json:"infohash (base64)"`
}

type KeyJSON struct {
	Announce_key string `json:"announce_key"`
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	fmt.Fprintf(w, "%d", code)
	log.Printf("Error: %v", err)
}

func enableCors(conf config.Config, w *http.ResponseWriter, r *http.Request) {
	allowed := []string{fmt.Sprintf("http://localhost:%d", conf.Port)}
	if conf.Tls != (config.TLSConfig{}) {
		allowed = append(allowed, fmt.Sprintf("https://localhost:%d", conf.Tls.TlsPort))
	}

	origin := r.Header.Get("Origin")
	if slices.Contains(allowed, origin) {
		(*w).Header().Set("Access-Control-Allow-Origin", origin)
		(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST")
		(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	}
}

// Return a pointer to a new http.Server object which will serve the frontendapi.
func NewFrontendAPIServer(conf config.Config) *http.Server {
	frontendMux := http.NewServeMux()
	frontendMux.HandleFunc("/frontendapi/stats", StatsHandler(conf))
	frontendMux.HandleFunc("/frontendapi/generate", GenerateHandler(conf))
	frontendMux.HandleFunc("/frontendapi/infohashes", InfohashesHandler(conf))

	f := &http.Server{
		Addr:              "localhost:9000",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(frontendMux, time.Second, "Timeout"),
	}
	return f
}

// ServeFrontend provides the basic routing logic for the SPA.
func ServeFrontend(frontendPath string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fs := http.Dir(frontendPath)
		path := filepath.Join(r.URL.Path)

		// Serve static assets, if they exist.
		if _, err := fs.Open(path); err == nil {
			http.FileServer(fs).ServeHTTP(w, r)
			return
		}

		// Route everything else through index.html.
		http.ServeFile(w, r, filepath.Join(frontendPath, "index.html"))
	}
}

// InfohashesHandler presets a REST API on /frontend/infohashes which returns
// an object including information on each tracked infohash.
func InfohashesHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(conf, &w, r)

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
			    name,
			    downloaded,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left = 0) AS seeders,
			    COUNT(*) FILTER (WHERE recent_announces.amount_left > 0) AS leechers,
			    info_hash
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
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		infohashes, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[InfohashesJSON])
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		result, err := json.Marshal(infohashes)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		fmt.Fprintf(w, "%s", result)
	}
}

// StatsHandler presents a REST API on /frontendapi/stats which returns an object
// including the total tracked infohashes, seeders, and leechers.
func StatsHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(conf, &w, r)
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
		stats, err := pgx.CollectRows(rows, pgx.RowToStructByName[StatsJSON])
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
		enableCors(conf, &w, r)
		announce_key, err := config.GenerateAnnounceKey(conf)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		key := KeyJSON{Announce_key: announce_key}

		result, err := json.Marshal(key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		fmt.Fprintf(w, "%s", result)
	}
}
