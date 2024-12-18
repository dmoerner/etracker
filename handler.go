package main

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

	"github.com/jackc/pgx/v5"
)

const interval = "60 minutes"

type Announce struct {
	peer_id     []byte
	ip_port     []byte
	info_hash   []byte
	numwant     int
	amount_left int
}

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
func parseAnnounce(r *http.Request) (*Announce, error) {
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

	// numwant is optional
	numwantString := query.Get("numwant")
	numwant, err := strconv.Atoi(numwantString)
	if err != nil || numwant < 0 || numwant > 100 {
		numwant = 50
	}

	var announce Announce

	announce.peer_id = []byte(peer_id)
	announce.info_hash = []byte(info_hash)
	announce.ip_port = ip_port
	announce.numwant = numwant
	announce.amount_left = amount_left

	return &announce, nil
}

var ErrInfoHashNotAllowed = errors.New("info_hash not in infohash_allowlist")

// writeAnnounce updates the peers table with an announce.
func writeAnnounce(config Config, announce *Announce) error {
	var b bool
	err := config.dbpool.QueryRow(context.Background(),
		"select exists (select from infohash_allowlist where info_hash = $1);", announce.info_hash).Scan(&b)
	if err != nil {
		return fmt.Errorf("error checking infohash_allowlist: %w", err)
	}
	if !b {
		return ErrInfoHashNotAllowed
	}

	_, err = config.dbpool.Exec(context.Background(), `INSERT INTO peers (peer_id, ip_port, info_hash, amount_left) 
		VALUES ($1, $2, $3, $4) 
		ON CONFLICT (peer_id, info_hash) 
		DO UPDATE SET ip_port = $2,amount_left = $4;`,
		announce.peer_id, announce.ip_port, announce.info_hash, announce.amount_left)
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
func sendReply(config Config, w http.ResponseWriter, a *Announce) error {
	query := fmt.Sprintf(`SELECT ip_port FROM peers WHERE info_hash = $1 AND peer_id <> $2 AND last_announce >= NOW() - INTERVAL '%s';`, interval)
	rows, err := config.dbpool.Query(context.Background(), query, a.info_hash, a.peer_id)
	if err != nil {
		return fmt.Errorf("error selecting peer rows: %w", err)
	}
	defer rows.Close()

	peers, err := pgx.CollectRows(rows, pgx.RowTo[[]byte])
	if err != nil {
		return fmt.Errorf("error collecting rows: %w", err)
	}

	numToGive, err := config.algorithm(config, a)
	if err != nil {
		return fmt.Errorf("error calculating number of peers to give: %w", err)
	}

	if len(peers) > numToGive {
		start := rand.Intn(len(peers) - numToGive)
		peers = peers[start : start+numToGive]
	}

	_, err = w.Write(PeerList(peers))
	if err != nil {
		return fmt.Errorf("error replying to peer: %w", err)
	}
	return nil
}

// PeerHandler encapsulates the handling of each peer request. The first step
// is to update the peers table with the information in the announce. The
// second step is to send a bencoded reply.
func PeerHandler(config Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip favicon requests and anything else.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		announce, err := parseAnnounce(r)
		if err != nil {
			log.Printf("Error parsing anounce: %v", err)
			_, err = w.Write(FailureReason("error parsing announce"))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return
		}

		err = writeAnnounce(config, announce)
		if err != nil {

			reason := "tracker error"
			if errors.Is(err, ErrInfoHashNotAllowed) {
				log.Printf("Not allowed info_hash: %s", announce.info_hash)
				reason = "info_hash not in the allowed list"
			} else {
				log.Printf("Error writing announce to db: %v", err)
			}
			_, err = w.Write(FailureReason(reason))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return

		}

		err = sendReply(config, w, announce)
		if err != nil {
			log.Printf("Error responding to peer: %v", err)
		}
	}
}
