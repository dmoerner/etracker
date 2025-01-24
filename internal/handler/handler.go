package handler

import (
	"context"
	"encoding/binary"
	"errors"
	"etracker/internal/bencode"
	"etracker/internal/config"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const interval = "60 minutes"

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

	peer_id := query.Get("peer_id")
	if peer_id == "" {
		return nil, fmt.Errorf("no peer_id in request")
	}

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

	announce.Peer_id = []byte(peer_id)
	announce.Info_hash = []byte(info_hash)
	announce.Ip_port = ip_port
	announce.Numwant = numwant
	announce.Amount_left = amount_left
	announce.Downloaded = downloaded
	announce.Uploaded = uploaded
	announce.Event = event

	return &announce, nil
}

var ErrInfoHashNotAllowed = errors.New("info_hash not in infohashes")

// checkInfoHash verifies that an announce is on the allowlist.
func checkInfoHash(conf config.Config, announce *config.Announce) error {
	var b bool
	err := conf.Dbpool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT FROM infohashes WHERE info_hash = $1);
		`,
		announce.Info_hash).Scan(&b)
	if err != nil {
		return fmt.Errorf("error checking infohashes: %w", err)
	}
	if !b {
		return ErrInfoHashNotAllowed
	}
	return nil
}

// writeAnnounce updates the peers table with an announce.
func writeAnnounce(conf config.Config, announce *config.Announce) error {
	// Calculate most recent upload change.
	var last_uploaded int
	var upload_change int
	err := conf.Dbpool.QueryRow(context.Background(), `
		SELECT uploaded
		FROM peers
		LEFT JOIN infohashes ON peers.info_hash_id = infohashes.id
		LEFT JOIN peerids ON peers.peer_id_id = peerids.id
		WHERE info_hash = $1 AND peer_id <> $2 AND event <> $3
		ORDER BY last_announce DESC LIMIT 1;
		`,
		announce.Info_hash, announce.Peer_id, config.Stopped).Scan(&last_uploaded)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			upload_change = 0
		} else {
			return fmt.Errorf("error fetching recent announces: %w", err)
		}
	}
	upload_change = announce.Uploaded - last_uploaded

	// Update peerids table, setting a new peer_max_upload. At the moment this key
	// is only written, but not read. It is an example of the kind of information
	// which I hope will be useful in the future to detect cheating.
	_, err = conf.Dbpool.Exec(context.Background(), `
		INSERT INTO peerids (peer_id, peer_max_upload)
		VALUES ($1, $2)
		ON CONFLICT (peer_id)
		DO UPDATE SET peer_max_upload = GREATEST(peerids.peer_max_upload, $2);
		`,
		announce.Peer_id, upload_change)
	if err != nil {
		return fmt.Errorf("error inserting peer_id: %w", err)
	}

	// Update infohashes table on completed event.
	if announce.Event == config.Completed {
		_, err = conf.Dbpool.Exec(context.Background(), `
			UPDATE infohashes SET downloaded = downloaded + 1 WHERE info_hash = $1
			`,
			announce.Info_hash)
		if err != nil {
			return fmt.Errorf("error updating infohashes on downloaded event: %w", err)
		}
	}

	// Update peers table
	_, err = conf.Dbpool.Exec(context.Background(), `
		INSERT INTO peers (peer_id_id, info_hash_id, ip_port, amount_left, uploaded, downloaded, event)
		SELECT peerids.id, infohashes.id, $3, $4, $5, $6, $7
		FROM infohashes
		JOIN peerids on peerids.peer_id = $1
		WHERE infohashes.info_hash = $2
		ON CONFLICT (peer_id_id, info_hash_id)
		DO UPDATE SET ip_port = $3, amount_left = $4, uploaded = $5, downloaded = $6, event = $7;
		`,
		announce.Peer_id, announce.Info_hash, announce.Ip_port, announce.Amount_left, announce.Uploaded, announce.Downloaded, announce.Event)
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
		SELECT DISTINCT ON (peer_id) ip_port
		FROM peers
		JOIN peerids ON peers.peer_id_id = peerids.id
		JOIN infohashes ON peers.info_hash_id = infohashes.id
		WHERE info_hash = $1 AND peer_id <> $2 AND last_announce >= NOW() - INTERVAL '%s' AND event <> $3
		ORDER BY peer_id, last_announce DESC;
		`,
		interval)
	rows, err := conf.Dbpool.Query(context.Background(), query, a.Info_hash, a.Peer_id, config.Stopped)
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

		err = checkInfoHash(conf, announce)
		if errors.Is(err, ErrInfoHashNotAllowed) {
			_, err = w.Write(bencode.FailureReason("info_hash not in the allowed list"))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return
		}

		err = sendReply(conf, w, announce)
		if err != nil {
			log.Printf("Error responding to peer: %v", err)
		}

		err = writeAnnounce(conf, announce)
		if err != nil {
			reason := "tracker error"
			log.Printf("Error writing announce to db: %v", err)
			_, err = w.Write(bencode.FailureReason(reason))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return

		}
	}
}
