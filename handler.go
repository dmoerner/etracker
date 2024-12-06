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

	"github.com/jackc/pgx/v5/pgxpool"
)

// encodeAddr converts a request RemoteAddr in the format x.x.x.x:port into
// 6-byte compact format expected by BEP 23.
func encodeAddr(remoteAddr string) ([]byte, error) {
	splitAddr := strings.Split(remoteAddr, ":")

	if len(splitAddr) != 2 {
		return nil, fmt.Errorf("invalid address format: %s", remoteAddr)
	}

	ipString, portString := splitAddr[0], splitAddr[1]

	portInt, err := strconv.Atoi(portString)
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

// handleAnnounce extracts the relevant information from a client's http request
// and inserts it into the peers table.
func handleAnnounce(dbpool *pgxpool.Pool, r *http.Request) error {
	query := r.URL.Query()

	peer_id, err := queryHead(query["peer_id"])
	if err != nil {
		return err
	}

	info_hash, err := queryHead(query["info_hash"])
	if err != nil {
		return err
	}

	ip_port, err := encodeAddr(r.RemoteAddr)
	if err != nil {
		return fmt.Errorf("error encoding remote address: %w", err)
	}

	_, err = dbpool.Exec(context.Background(), `INSERT INTO peers (peer_id, ip_port, info_hash) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (peer_id, info_hash) 
		DO UPDATE SET ip_port = $2;`,
		peer_id, ip_port, info_hash)

	if err != nil {
		return fmt.Errorf("error upserting peer row: %w", err)
	}

	return nil
}

// sendReply writes a bencoded reply to the client consisting of an appropriate
// peer list. Tracker error messages will generally be sent by the parent
// PeerHandler due to earlier failures.
func sendReply(dbpool *pgxpool.Pool, w http.ResponseWriter, r *http.Request) error {
	_, err := w.Write(FailureReason("not implemented"))
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

		err := handleAnnounce(dbpool, r)
		if err != nil {
			log.Printf("Error handling announce: %v", err)

			_, err = w.Write(FailureReason("tracker error"))
			if err != nil {
				log.Printf("Error responding to peer: %v", err)
			}
			return

		}

		err = sendReply(dbpool, w, r)

		if err != nil {
			log.Printf("Error responding to peer: %v", err)
		}
	}
}
