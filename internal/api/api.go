package api

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/dmoerner/etracker/internal/config"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	bencode "github.com/jackpal/bencode-go"
)

type GlobalStats struct {
	Hashcount int `json:"hashcount"`
	Seeders   int `json:"seeders"`
	Leechers  int `json:"leechers"`
}

type Key struct {
	Announce_key string `json:"announce_key"`
}

type Infohash struct {
	Info_hash []byte `json:"info_hash"`
}

type InfohashPost struct {
	Info_hash []byte `json:"info_hash"`
	Name      string `json:"name"`
}

type InfohashStats struct {
	Name       string `json:"name"`
	Downloaded int    `json:"downloaded"`
	Seeders    int    `json:"seeders"`
	Leechers   int    `json:"leechers"`
	Info_hash  []byte `json:"info_hash"`
}

type MessageJSON struct {
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, code int, msg MessageJSON) {
	w.WriteHeader(code)
	response, _ := json.Marshal(msg)
	fmt.Fprintf(w, "%s", response)
	log.Printf("API Error: %s", msg.Message)
}

func enableCors(conf config.Config, w *http.ResponseWriter, _ *http.Request) {
	// allowed := []string{conf.FrontendHostname}
	// origin := r.Header.Get("Origin")
	(*w).Header().Set("Access-Control-Allow-Origin", conf.FrontendHostname)
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// validateAPIKey is a helper function which should be used at the start of any restricted
// API paths.
func validateAPIKey(conf config.Config, w http.ResponseWriter, r *http.Request) bool {
	// The API key must be set in the configuration.
	if conf.Authorization == "" {
		writeError(w, http.StatusForbidden, MessageJSON{"error: restricted API access disabled"})
		return false
	}

	//
	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		writeError(w, http.StatusBadRequest, MessageJSON{"error: restricted API request with empty authorization header"})
		return false
	}

	if conf.Authorization == "" || authorization != conf.Authorization {
		writeError(w, http.StatusForbidden, MessageJSON{"restricted API request from non-https source"})
		return false
	}

	return true
}

// MuxAPIRoutes adds all the REST API routes to a mux.
func MuxAPIRoutes(ctx context.Context, conf config.Config, mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stats", StatsHandler(ctx, conf))
	mux.HandleFunc("GET /api/generate", GenerateHandler(ctx, conf))
	mux.HandleFunc("GET /api/infohashes", InfohashesHandler(ctx, conf))
	mux.HandleFunc("POST /api/infohash", PostInfohashHandler(ctx, conf))
	mux.HandleFunc("POST /api/torrentfile", PostTorrentFileHandler(ctx, conf))
	mux.HandleFunc("GET /api/torrentfile", GetTorrentFileHandler(ctx, conf))
	mux.HandleFunc("DELETE /api/infohash", DeleteInfohashHandler(ctx, conf))
}

// PostInfohashHandler takes a POST request to the /api/infohash endpoint, with
// the body as a JSON object with a base64-encoded infohash and a name for the
// infohash. It inserts it into the database and returns an appropriate JSON
// message on success or failure.
//
// This is an authorization-only endpoint.
func PostInfohashHandler(ctx context.Context, conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateAPIKey(conf, w, r) {
			return
		}

		var infohash InfohashPost
		err := json.NewDecoder(r.Body).Decode(&infohash)
		if err != nil || len(infohash.Info_hash) != 20 {
			writeError(w, http.StatusBadRequest, MessageJSON{"error: did not receive valid infohash"})
			return
		}

		_, err = conf.Dbpool.Exec(ctx, `
		INSERT INTO infohashes (info_hash, name)
		    VALUES ($1, $2)
		`,
			infohash.Info_hash, infohash.Name)
		if err != nil {
			var pgErr *pgconn.PgError
			// 23505: duplicate key insertion error code
			if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
				writeError(w, http.StatusBadRequest, MessageJSON{"error: infohash already inserted"})
				return
			}
			writeError(w, http.StatusInternalServerError, MessageJSON{"error inserting infohash"})
			return
		}

		response, err := json.Marshal(MessageJSON{"success"})
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"success posting, but error making response"})
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "%s", response)
	}
}

// PostTorrentFileHandler takes a POST request to the /api/torrentfile endpoint, with
// the body as a torrent file. It strips out any current announce url and
// inserts it into the database and returns an appropriate JSON message on
// success or failure.
//
// This is an authorization-only endpoint.
//
// Both the PostInfohashHandler and PostTorrentFileHandler endpoints are supported because
// the former makes testing easier, and may sometimes be convenient for public torrents.
func PostTorrentFileHandler(ctx context.Context, conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateAPIKey(conf, w, r) {
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, MessageJSON{"error: could not process posted file"})
			return
		}
		defer file.Close()

		data, err := bencode.Decode(file)
		if err != nil {
			writeError(w, http.StatusBadRequest, MessageJSON{"error: could not decode posted file"})
			return
		}

		// Strip out announce url.
		data.(map[string]any)["announce"] = ""

		// Ensure private flag is set.
		data.(map[string]any)["info"].(map[string]any)["private"] = int64(1)

		// Extract name and length.
		name := data.(map[string]any)["info"].(map[string]any)["name"].(string)

		var length int64
		if l, ok := data.(map[string]any)["info"].(map[string]any)["length"]; ok {
			length = l.(int64)
		} else {
			for _, f := range data.(map[string]any)["info"].(map[string]any)["files"].([]any) {
				length += f.(map[string]any)["length"].(int64)
			}
		}

		// Calculate info_hash.
		var b bytes.Buffer
		err = bencode.Marshal(&b, data.(map[string]any)["info"])
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not calculate infohash"})
			return
		}
		info_hash := sha1.Sum(b.Bytes())

		// Re-encode stripped torrent file.
		var torrentFile bytes.Buffer

		err = bencode.Marshal(&torrentFile, data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not construct new torrent file"})
			return
		}

		// Write to db.
		_, err = conf.Dbpool.Exec(ctx, `
		INSERT INTO infohashes (info_hash, name, file, length)
		    VALUES ($1, $2, $3, $4)
		`,
			info_hash[:], name, torrentFile.Bytes(), length)
		if err != nil {
			var pgErr *pgconn.PgError
			// 23505: duplicate key insertion error code
			if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
				writeError(w, http.StatusBadRequest, MessageJSON{"error: infohash already inserted"})
				return
			}
			writeError(w, http.StatusInternalServerError, MessageJSON{"error inserting infohash"})
			return
		}

		response, err := json.Marshal(MessageJSON{"success"})
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"success posting, but error making response"})
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "%s", response)
	}
}

// DeleteInfohashHandler takes a DELETE request to the /api/infohash endpoint, with
// the body as a JSON object with a base64-encoded infohash and a name for the
// infohash. It removes it from the database and returns an appropriate JSON
// message on success or failure.
//
// This is an authorization-only endpoint.
func DeleteInfohashHandler(ctx context.Context, conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validateAPIKey(conf, w, r) {
			return
		}

		var infohash Infohash
		err := json.NewDecoder(r.Body).Decode(&infohash)
		if err != nil || len(infohash.Info_hash) != 20 {
			writeError(w, http.StatusBadRequest, MessageJSON{"did not receive valid infohash"})
			return
		}

		_, err = conf.Dbpool.Exec(ctx, `
		DELETE FROM infohashes
		WHERE info_hash = $1
		`,
			infohash.Info_hash)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error deleting infohash"})
			return
		}

		response, err := json.Marshal(MessageJSON{"success"})
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"success deleting, but error making response"})
		}

		fmt.Fprintf(w, "%s", response)
	}
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
func InfohashesHandler(ctx context.Context, conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(conf, &w, r)

		query := fmt.Sprintf(`
			WITH recent_announces AS (
			    SELECT DISTINCT ON (peers_id, info_hash_id)
				amount_left,
				info_hash_id
			    FROM
				announces
			    WHERE
				last_announce >= NOW() - INTERVAL '%d seconds'
				AND event <> $1
			    ORDER BY
				peers_id,
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

		rows, err := conf.Dbpool.Query(ctx, query, config.Stopped)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not query database"})
			return
		}

		infohashes, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[InfohashStats])
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not parse response from database"})
			return
		}

		result, err := json.Marshal(infohashes)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: unable to construct response"})
			return
		}
		fmt.Fprintf(w, "%s", result)
	}
}

// StatsHandler presents a REST API on /frontendapi/stats which returns an object
// including the total tracked infohashes, seeders, and leechers.
func StatsHandler(ctx context.Context, conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(conf, &w, r)
		query := fmt.Sprintf(`
			WITH recent_announces AS (
			    SELECT DISTINCT ON (info_hash_id, peers_id)
				amount_left,
				info_hash_id
			    FROM
				announces
			    WHERE
				last_announce >= NOW() - INTERVAL '%d seconds'
				AND event <> $1
			    ORDER BY
				peers_id,
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

		rows, err := conf.Dbpool.Query(ctx, query, config.Stopped)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not query database"})
			return
		}
		stats, err := pgx.CollectRows(rows, pgx.RowToStructByName[GlobalStats])
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not parse response from database"})
			return
		}

		result, err := json.Marshal(stats[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: unable to construct response"})
			return
		}
		fmt.Fprintf(w, "%s", result)
	}
}

// GenerateHandler returns a new announce key.
func GenerateHandler(ctx context.Context, conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(conf, &w, r)
		announce_key, err := config.GenerateAnnounceKey(ctx, conf)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not generate announce key"})
			return
		}
		key := Key{Announce_key: announce_key}

		result, err := json.Marshal(key)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: unable to construct response"})
			return
		}
		fmt.Fprintf(w, "%s", result)
	}
}

// GetTorrentFileHandler takes a GET request with an announce_key and info_hash query fields.
// If the announce_key is registered and the info_hash is present in the database,
// it returns a new torrent file with the appropriate announce URL.
//
// The info_hash is expected to be hex-encoded.
func GetTorrentFileHandler(ctx context.Context, conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		// Validate announce_key
		announce_key := query.Get("announce_key")
		if announce_key == "" {
			writeError(w, http.StatusBadRequest, MessageJSON{"error: no announce key provided in query"})
			return
		}

		var ok bool
		err := conf.Dbpool.QueryRow(ctx, `
			SELECT EXISTS (SELECT FROM peers WHERE announce_key = $1)
			`,
			announce_key).Scan(&ok)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: unable to validate announce key"})
			return
		}

		if !ok {
			writeError(w, http.StatusBadRequest, MessageJSON{"error: invalid announce key"})
			return
		}

		// Process info_hash
		info_hash_hex := query.Get("info_hash")

		if info_hash_hex == "" {
			writeError(w, http.StatusBadRequest, MessageJSON{"error: no infohash provided in query"})
			return
		}

		info_hash, err := hex.DecodeString(info_hash_hex)
		if err != nil {
			writeError(w, http.StatusBadRequest, MessageJSON{"error: could not decode hex info_hash"})
		}

		var stripped_torrent_file []byte

		err = conf.Dbpool.QueryRow(ctx, `
			SELECT file FROM infohashes WHERE info_hash = $1 AND file IS NOT NULL
			`,
			info_hash).Scan(&stripped_torrent_file)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusInternalServerError, MessageJSON{"error: unable to fetch torrent file from db"})
				return
			}
			writeError(w, http.StatusBadRequest, MessageJSON{"error: no matching infohash with stored torrent file"})
			return
		}

		data, err := bencode.Decode(bytes.NewReader(stripped_torrent_file))
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: unable to decode torrent file in db"})
			return
		}

		// Build a clean and complete announce URL.
		u := &url.URL{
			Scheme: "http",
			Host:   r.Host,
		}

		if r.TLS != nil {
			u.Scheme = "https"
		}

		announce_url := u.JoinPath(announce_key, "announce")

		data.(map[string]any)["announce"] = announce_url.String()

		var torrent_file bytes.Buffer
		err = bencode.Marshal(&torrent_file, data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not construct new torrent file"})
			log.Print(err)
			return
		}

		_, err = w.Write(torrent_file.Bytes())
		if err != nil {
			writeError(w, http.StatusInternalServerError, MessageJSON{"error: could not send torrent file"})
			return
		}
	}
}
