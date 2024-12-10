package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Announce struct {
	peer_id   []byte
	ip_port   []byte
	info_hash []byte
}

// encodeAddr converts a request RemoteAddr in the format x.x.x.x:port into
// 6-byte compact format expected by BEP 23. The port used is extracted from
// the client announce; the RemoteAddr port is ignored.
func encodeAddr(remoteAddr string, port []byte) ([]byte, error) {
	splitAddr := strings.Split(remoteAddr, ":")

	if len(splitAddr) != 2 {
		return nil, fmt.Errorf("invalid address format: %s", remoteAddr)
	}

	ipString := splitAddr[0]

	portInt, err := strconv.Atoi(string(port))
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

// queryHead confirms we have a singleton list of values and returns the head.
func queryHead(query []string) ([]byte, error) {
	if len(query) != 1 {
		return []byte(""), fmt.Errorf("only one key-value pair allowed in request: %v", query)
	}
	return []byte(query[0]), nil
}

// parseAnnounce parses a request to construct an announce struct, and returns
// a pointer to the struct and any error.
func parseAnnounce(r *http.Request) (*Announce, error) {
	query := r.URL.Query()

	peer_id, err := queryHead(query["peer_id"])
	if err != nil {
		return nil, err
	}

	info_hash, err := queryHead(query["info_hash"])
	if err != nil {
		return nil, err
	}

	port, err := queryHead(query["port"])
	if err != nil {
		return nil, err
	}

	ip_port, err := encodeAddr(r.RemoteAddr, port)
	if err != nil {
		return nil, fmt.Errorf("error encoding remote address: %w", err)
	}

	var announce Announce

	announce.peer_id = peer_id
	announce.info_hash = info_hash
	announce.ip_port = ip_port

	return &announce, nil
}

// writeAnnounce updates the peers table with an announce.
func writeAnnounce(dbpool *pgxpool.Pool, announce *Announce) error {
	_, err := dbpool.Exec(context.Background(), `INSERT INTO peers (peer_id, ip_port, info_hash) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (peer_id, info_hash) 
		DO UPDATE SET ip_port = $2;`,
		announce.peer_id, announce.ip_port, announce.info_hash)
	if err != nil {
		return fmt.Errorf("error upserting peer row: %w", err)
	}

	return nil
}

// sendReply writes a bencoded reply to the client consisting of an appropriate
// peer list. Tracker error messages will generally be sent by the parent
// PeerHandler due to earlier failures.
func sendReply(dbpool *pgxpool.Pool, w http.ResponseWriter, a *Announce) error {
	rows, err := dbpool.Query(context.Background(),
		`SELECT ip_port FROM peers 
		WHERE info_hash = $1 AND peer_id <> $2;`,
		a.info_hash,
		a.peer_id)
	if err != nil {
		return fmt.Errorf("error selecting peer rows: %w", err)
	}
	defer rows.Close()

	peers, err := pgx.CollectRows(rows, pgx.RowTo[[]byte])
	if err != nil {
		return fmt.Errorf("error collecting rows: %w", err)
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
func PeerHandler(dbpool *pgxpool.Pool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip favicon requests and anything else.
		if r.URL.Path != "/" {
			return
		}

		announce, err := parseAnnounce(r)
		if err != nil {
			log.Printf("Error parsing announce: %v", err)

			_, err = w.Write(FailureReason("tracker error"))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return
		}

		err = writeAnnounce(dbpool, announce)
		if err != nil {
			log.Printf("Error writing announce to db: %v", err)

			_, err = w.Write(FailureReason("tracker error"))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return

		}

		err = sendReply(dbpool, w, announce)
		if err != nil {
			log.Printf("Error responding to peer: %v", err)
		}
	}
}
