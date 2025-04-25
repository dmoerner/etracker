package handler

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/dmoerner/etracker/internal/bencode"
	"github.com/dmoerner/etracker/internal/config"
	"github.com/redis/go-redis/v9"

	"github.com/jackc/pgx/v5"
)

const DefaultTrackerError = "tracker error"

var (
	ErrInfoHashNotAllowed = errors.New("info_hash not in infohashes")
	ErrUntrackedAnnounce  = errors.New("untracked announce key")
)

// encodeAddr converts a request RemoteAddr in the format x.x.x.x:port into
// 6-byte compact format expected by BEP 23. The port used is extracted from
// the client announce; the RemoteAddr port is ignored.
func encodeAddr(remoteAddr string, port string) ([]byte, error) {
	splitAddr := strings.Split(remoteAddr, ":")

	if len(splitAddr) != 2 {
		return nil, fmt.Errorf("invalid address format: %s", remoteAddr)
	}

	ipString := splitAddr[0]

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("error converting port to int: %w", err)
	}

	bytesPort := make([]byte, 2)
	binary.BigEndian.PutUint16(bytesPort, uint16(portInt))

	parsedIP := []byte(net.ParseIP(ipString).To4())
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipString)
	}

	ip_port := append(parsedIP, bytesPort...)

	return ip_port, nil
}

// parseAnnounce parses a request to construct an announce struct, and returns
// a pointer to the struct and any error.
func parseAnnounce(r *http.Request) (*config.Announce, error) {
	query := r.URL.Query()

	announce_key := r.PathValue("id")

	info_hash := query.Get("info_hash")
	if info_hash == "" {
		return nil, fmt.Errorf("no info_hash in request")
	}

	port := query.Get("port")
	if port == "" {
		return nil, fmt.Errorf("no port in request")
	}
	ip_port, err := encodeAddr(r.RemoteAddr, port)
	if err != nil {
		return nil, fmt.Errorf("error encoding remote address: %w", err)
	}

	// "left" is the key in the announce, but it's a reserved word in
	// PostgreSQL, so we will store the integer as amount_left.
	left := query.Get("left")
	if left == "" {
		return nil, fmt.Errorf("no left in request")
	}
	amount_left, err := strconv.Atoi(left)
	if err != nil {
		return nil, err
	}

	uploadedASCII := query.Get("uploaded")
	if uploadedASCII == "" {
		return nil, fmt.Errorf("no uploaded in request")
	}
	uploaded, err := strconv.Atoi(uploadedASCII)
	if err != nil {
		return nil, err
	}

	downloadedASCII := query.Get("downloaded")
	if downloadedASCII == "" {
		return nil, fmt.Errorf("no downloaded in request")
	}
	downloaded, err := strconv.Atoi(downloadedASCII)
	if err != nil {
		return nil, err
	}

	// numwant is optional
	numwantString := query.Get("numwant")
	numwant, err := strconv.Atoi(numwantString)
	if err != nil || numwant < 0 || numwant > 100 {
		numwant = 50
	}

	// event is optional, but if present must be "started", "stopped", or "completed"
	var event config.Event
	eventString := query.Get("event")
	switch eventString {
	case "started":
		event = config.Started
	case "stopped":
		event = config.Stopped
	case "completed":
		event = config.Completed
	}

	var announce config.Announce

	announce.Announce_key = announce_key
	announce.Info_hash = []byte(info_hash)
	announce.Ip_port = ip_port
	announce.Numwant = numwant
	announce.Amount_left = amount_left
	announce.Downloaded = downloaded
	announce.Uploaded = uploaded
	announce.Event = event

	return &announce, nil
}

// checkAnnounce checks announces for two conditions. First, is the announce
// key being tracked? Second, if the infohash allowlist is enabled, is the infohash
// allowed (otherwise it is tracked as well).
//
// Everything in checkAnnounce is stored in the Redis cache as a persistent key, since
// these values change at most once during the runtime of the tracker.
func checkAnnounce(conf config.Config, announce *config.Announce) error {
	tracked := true
	tracked_cache, err := conf.Rdb.Get(context.Background(), "announce:"+announce.Announce_key).Result()
	if err != nil {
		// Cache miss or failure
		if err != redis.Nil {
			// An issue with the cache must be logged but is not fatal.
			log.Printf("Error fetching announce keys from cache: %v", err)
		}
		err = conf.Dbpool.QueryRow(context.Background(), `
			SELECT EXISTS (SELECT FROM peers WHERE announce_key = $1);
			`,
			announce.Announce_key).Scan(&tracked)
		if err != nil {
			return fmt.Errorf("error checking peers for announce: %w", err)
		}
		if tracked {
			tracked_cache = "true"
		} else {
			tracked_cache = "false"
		}
		err = conf.Rdb.Set(context.Background(), "announce:"+announce.Announce_key, tracked_cache, 0).Err()
		if err != nil {
			// An issue with the cache must be logged but is not fatal.
			log.Printf("Error setting announce keys in cache: %v", err)
		}
	} else {
		if tracked_cache == "false" {
			tracked = false
		}
	}
	if !tracked {
		return ErrUntrackedAnnounce
	}

	if conf.DisableAllowlist {
		err = conf.Rdb.Get(context.Background(), "info_hash:"+string(announce.Info_hash)).Err()
		if err != nil {
			// Cache miss or failure
			if err != redis.Nil {
				// An issue with the cache must be logged but is not fatal.
				log.Printf("Error fetching info_hash from cache: %v", err)
			}
			err = conf.Rdb.Set(context.Background(), "info_hash:"+string(announce.Info_hash), "true", 0).Err()
			if err != nil {
				// An issue with the cache must be logged but is not fatal.
				log.Printf("Error setting info_hash in cache: %v", err)
			}
			_, err = conf.Dbpool.Exec(context.Background(), `
			INSERT INTO infohashes (info_hash, name)
			    VALUES ($1, $2)
			ON CONFLICT (info_hash)
			    DO NOTHING
			`,
				announce.Info_hash, "client added")
			if err != nil {
				fmt.Println(err)
				return fmt.Errorf("error inserting announce_key: %w", err)
			}
		}
		return nil
	}

	allowed := true
	allowed_cache, err := conf.Rdb.Get(context.Background(), "info_hash:"+string(announce.Info_hash)).Result()
	if err != nil {
		if err != redis.Nil {
			// An issue with the cache must be logged but is not fatal.
			log.Printf("Error fetching info_hash keys from cache: %v", err)
		}
		err = conf.Dbpool.QueryRow(context.Background(), `
			SELECT EXISTS (SELECT FROM infohashes WHERE info_hash = $1);
			`,
			announce.Info_hash).Scan(&allowed)
		if err != nil {
			return fmt.Errorf("error checking infohashes for info_hash: %w", err)
		}
		if allowed {
			allowed_cache = "true"
		} else {
			allowed_cache = "false"
		}
		err = conf.Rdb.Set(context.Background(), "info_hash:"+string(announce.Info_hash), allowed_cache, 0).Err()
		if err != nil {
			// An issue with the cache must be logged but is not fatal.
			log.Printf("Error setting info_hash keys in cache: %v", err)
		}
	} else {
		if allowed_cache == "false" {
			allowed = false
		}
	}
	if !allowed {
		return ErrInfoHashNotAllowed
	}

	// var allowed bool
	// err = conf.Dbpool.QueryRow(context.Background(), `
	// 	SELECT EXISTS (SELECT FROM infohashes WHERE info_hash = $1);
	// 	`,
	// 	announce.Info_hash).Scan(&allowed)
	// if err != nil {
	// 	return fmt.Errorf("error checking infohashes: %w", err)
	// }
	// if !allowed {
	// 	return ErrInfoHashNotAllowed
	// }
	return nil
}

// writeAnnounce updates the peers table with an announce.
func writeAnnounce(conf config.Config, announce *config.Announce) error {
	// Calculate most recent upload change.
	var last_uploaded int
	var last_downloaded int
	err := conf.Dbpool.QueryRow(context.Background(), `
		SELECT
		    announces.uploaded, announces.downloaded
		FROM
		    announces
		    LEFT JOIN infohashes ON announces.info_hash_id = infohashes.id
		    LEFT JOIN peers ON announces.peers_id = peers.id
		WHERE
		    info_hash = $1
		    AND announce_key = $2
		    AND event <> $3
		ORDER BY
		    last_announce DESC
		LIMIT 1
		`,
		announce.Info_hash, announce.Announce_key, config.Stopped).Scan(&last_uploaded, &last_downloaded)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("error fetching recent announces: %w", err)
		}
		// If the select returns no rows, this is the peer's first announce.
		last_uploaded = 0
		last_downloaded = 0
	}
	upload_change := announce.Uploaded - last_uploaded
	download_change := announce.Downloaded - last_downloaded

	// Upload and download only go up. If they are negative, an announce was
	// not sent or the client reset its session.
	if upload_change < 0 {
		upload_change = 0
	}
	if download_change < 0 {
		download_change = 0
	}

	completed_snatch := 0
	if announce.Event == config.Completed {
		completed_snatch = 1
	}

	// Update peers table.
	_, err = conf.Dbpool.Exec(context.Background(), `
		UPDATE
		    peers
		SET
		    snatched = snatched + $1,
		    uploaded = uploaded + $2,
		    downloaded = downloaded + $3
		WHERE
		    announce_key = $4
		`,
		completed_snatch,
		upload_change,
		download_change,
		announce.Announce_key)
	if err != nil {
		return fmt.Errorf("error updating peers table: %w", err)
	}

	// Update infohashes table on completed event.
	if announce.Event == config.Completed {
		_, err = conf.Dbpool.Exec(context.Background(), `
			UPDATE
			    infohashes
			SET
			    downloaded = downloaded + 1
			WHERE
			    info_hash = $1
			`,
			announce.Info_hash)
		if err != nil {
			return fmt.Errorf("error updating infohashes on downloaded event: %w", err)
		}
	}

	// Update announces table
	_, err = conf.Dbpool.Exec(context.Background(), `
		INSERT INTO announces (peers_id, info_hash_id, ip_port, amount_left, uploaded, downloaded, event)
		SELECT
		    peers.id,
		    infohashes.id,
		    $3,
		    $4,
		    $5,
		    $6,
		    $7
		FROM
		    infohashes
		    JOIN peers ON peers.announce_key = $1
		WHERE
		    infohashes.info_hash = $2
		ON CONFLICT (peers_id,
		    info_hash_id)
		    DO UPDATE SET
			ip_port = $3,
			amount_left = $4,
			uploaded = $5,
			downloaded = $6,
			event = $7
		`,
		announce.Announce_key, announce.Info_hash, announce.Ip_port, announce.Amount_left, announce.Uploaded, announce.Downloaded, announce.Event)
	if err != nil {
		return fmt.Errorf("error upserting peer row: %w", err)
	}

	return nil
}

// sendReply writes a bencoded reply to the client consisting of an appropriate
// peer list. Tracker error messages will generally be sent by the parent
// PeerHandler due to earlier failures.
//
// If a client requests fewer than the number of available peers, a
// pseudorandom contiguous subset of the peers of the appropriate size will be
// sent. Given different client announce intervals, this should provide enough
// randomness, but it may be something revisit.
//
// PostgreSQL doesn't substitute inside of string literals, so to use a variable
// for the interval, we need to use fmt.Sprintf in an intermediate step. See further:
// https://github.com/jackc/pgx/issues/1043
func sendReply(conf config.Config, w http.ResponseWriter, a *config.Announce) error {
	query := fmt.Sprintf(`
		SELECT DISTINCT ON (announce_key)
		    ip_port
		FROM
		    announces
		    JOIN peers ON announces.peers_id = peers.id
		    JOIN infohashes ON announces.info_hash_id = infohashes.id
		WHERE
		    info_hash = $1
		    AND announce_key <> $2
		    AND last_announce >= NOW() - INTERVAL '%d seconds'
		    AND event <> $3
		ORDER BY
		    announce_key,
		    last_announce DESC
		`,
		config.StaleInterval)
	rows, err := conf.Dbpool.Query(context.Background(), query, a.Info_hash, a.Announce_key, config.Stopped)
	if err != nil {
		return fmt.Errorf("error selecting peer rows: %w", err)
	}
	defer rows.Close()

	peers, err := pgx.CollectRows(rows, pgx.RowTo[[]byte])
	if err != nil {
		return fmt.Errorf("error collecting rows: %w", err)
	}

	numToGive, err := conf.Algorithm(conf, a)
	if err != nil {
		return fmt.Errorf("error calculating number of peers to give: %w", err)
	}

	// Give a pseudo-random subset of peers.
	if len(peers) > numToGive {
		rand.Shuffle(len(peers), func(i, j int) {
			peers[i], peers[j] = peers[j], peers[i]
		})
		peers = peers[:numToGive]
	}

	_, err = w.Write(bencode.PeerList(peers))
	if err != nil {
		return fmt.Errorf("error replying to peer: %w", err)
	}
	return nil
}

// writeTrackerError is a helper function which writes a tracker error message
// to a peer. If there is a failure on right, we log an error.
func writeTrackerError(msg string, w http.ResponseWriter) {
	_, err := w.Write(bencode.FailureReason(msg))
	if err != nil {
		log.Printf("Error responding to peer: %v", err)
	}
}

// PeerHandler encapsulates the handling of each peer request. The first step
// is to update the peers table with the information in the announce. The
// second step is to send a bencoded reply.
func PeerHandler(conf config.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		announce, err := parseAnnounce(r)
		if err != nil {
			log.Printf("Error parsing announce: %v", err)
			_, err = w.Write(bencode.FailureReason("error parsing announce"))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return
		}

		err = checkAnnounce(conf, announce)
		if err != nil {
			msg := DefaultTrackerError
			if errors.Is(err, ErrInfoHashNotAllowed) {
				msg = "info_hash not in the allowed list"
			} else if errors.Is(err, ErrUntrackedAnnounce) {
				msg = "untracked announce key, generate new announce url"
			}
			writeTrackerError(msg, w)
			return
		}

		err = sendReply(conf, w, announce)
		if err != nil {
			log.Printf("Error responding to peer: %v", err)
		}

		err = writeAnnounce(conf, announce)
		if err != nil {
			writeTrackerError(DefaultTrackerError, w)
			return

		}
	}
}
