package api

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/dmoerner/etracker/internal/config"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// Some APIActions may not need to return anything to the hanlder other than
// report a lack of error. In that case, they return ("", nil).
type APIAction func(config.Config, *http.Request) (string, error)

var ActionsMap map[string]APIAction = map[string]APIAction{
	"insert_infohash": InsertInfoHash,
	"remove_infohash": RemoveInfoHash,
}

// APIHandler handles requests to the /api endpoint. It requires an appropriate
// authorization header, which is currently a single secret string managed by
// an environment variable.
func APIHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify authorization. An empty authorization value or no key
		// in the config means API access is forbidden.
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if conf.Authorization == "" || authorization != conf.Authorization {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		action := r.URL.Query().Get("action")
		if action == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		actionFunc, ok := ActionsMap[action]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		reply, err := actionFunc(conf, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
		_, err = io.WriteString(w, reply)
		if err != nil {
			// Log an error if we are unable to respond to client.
			log.Printf("Error responding to API request: %v", err)
		}
	}
}

func undigestHex(h string) ([]byte, error) {
	info_hash, err := hex.DecodeString(h)
	if err != nil {
		return []byte(""), fmt.Errorf("could not decode infohash, not a hex digest?")
	}

	if len(info_hash) != 20 {
		return []byte(""), fmt.Errorf("missing info_hash key")
	}
	return info_hash, nil
}

// InsertInfoHash is a function which takes the info_hash from a query and
// inserts it into the database. It always returns the empty string, and also
// returns either an error or nil.
func InsertInfoHash(conf config.Config, r *http.Request) (string, error) {
	// info_hash_hex is required parameter, must be a hex digest.
	info_hash_hex := r.URL.Query().Get("info_hash")

	info_hash, err := undigestHex(info_hash_hex)
	if err != nil {
		return "", err
	}

	// license is optional parameter
	name := r.URL.Query().Get("name")
	license := r.URL.Query().Get("license")

	_, err = conf.Dbpool.Exec(context.Background(), `
		INSERT INTO infohashes (info_hash, name, license)
		    VALUES ($1, $2, $3)
		`,
		[]byte(info_hash), name, license)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23505: duplicate key insertion error code
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Sprintf("info_hash %s already inserted", hex.EncodeToString(info_hash)), fmt.Errorf("unable to insert info_hash: %w", err)
		}
		return "", fmt.Errorf("unable to insert info_hash: %w", err)
	}

	return "", nil
}

// RemoveInfoHash is a function which takes the info_hash from a query and
// removes it from the database. It always returns the empty string, and also
// returns either an error or nil.
func RemoveInfoHash(conf config.Config, r *http.Request) (string, error) {
	// info_hash_hex is required parameter, must be a hex digest
	info_hash_hex := r.URL.Query().Get("info_hash")

	info_hash, err := undigestHex(info_hash_hex)
	if err != nil {
		return "", err
	}

	_, err = conf.Dbpool.Exec(context.Background(), `
		DELETE FROM infohashes
		WHERE info_hash = $1
		`,
		[]byte(info_hash))
	if err != nil {
		return "", fmt.Errorf("unable to remove info_hash: %w", err)
	}

	return "", nil
}
