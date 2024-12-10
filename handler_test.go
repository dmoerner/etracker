package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/joho/godotenv"
)

type Request struct {
	peer_id    string
	info_hash  string
	ip         *string
	port       string
	uploaded   string
	downloaded string
	left       string
	event      *string
}

func formatRequest(request Request) string {
	return fmt.Sprintf(
		"http://example.com/?peer_id=%s&info_hash=%s&port=%s&uploaded=%s&downloaded=%s&left=%s",
		request.peer_id,
		request.info_hash,
		request.port,
		request.uploaded,
		request.downloaded,
		request.left)
}

func TestAnnounceWrite(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	if _, ok := os.LookupEnv("DATABASE_URL"); !ok {
		log.Fatal("DATABASE_URL not set in environment")
	}
	if _, ok := os.LookupEnv("PGDATABASE"); !ok {
		log.Fatal("PGDATABASE not set in environment")
	}

	testdb := os.Getenv("PGDATABASE") + "_test"
	dbpool, err := DbConnect(testdb)

	request := Request{
		peer_id:    "-TR4060-7ltqlx8z3ch4",
		info_hash:  "aaaaaaaaaaaaaaaaaaaa",
		port:       "6881",
		uploaded:   "0",
		downloaded: "0",
		left:       "0",
	}

	req := httptest.NewRequest("GET", formatRequest(request), nil)
	w := httptest.NewRecorder()

	handler := PeerHandler(dbpool)

	handler(w, req)

	var peer_id []byte
	var ip_port []byte
	var info_hash []byte

	err = dbpool.QueryRow(context.Background(), "SELECT peer_id, ip_port, info_hash FROM peers LIMIT 1;").Scan(&peer_id, &ip_port, &info_hash)
	if err != nil {
		t.Fatalf("error querying test db: %v", err)
	}

	if !bytes.Equal(peer_id, []byte(request.peer_id)) {
		t.Errorf("peerid: expected %s, got %s", request.peer_id, peer_id)
	}
	if !bytes.Equal(info_hash, []byte(request.info_hash)) {
		t.Errorf("info_hash: expected %s, got %s", request.info_hash, info_hash)
	}

	var expectedIpPort bytes.Buffer

	// For reasons that are unclear to me, httptest.NewRequest ignores httptest.DefaultNewRequest
	// and hard-codes this IP instead, following RFC 5737.
	expectedIpPort.Write([]byte(net.ParseIP("192.0.2.1").To4()))
	port, err := strconv.Atoi(request.port)
	if err != nil {
		t.Fatalf("bad test data: port could not be converted to int: %s", request.port)
	}
	binary.Write(&expectedIpPort, binary.BigEndian, uint16(port))
	if !bytes.Equal(ip_port, expectedIpPort.Bytes()) {
		t.Errorf("ip_port: expected %v, got %v", expectedIpPort.Bytes(), ip_port)
	}

	_, err = dbpool.Exec(context.Background(), "DROP TABLE peers;")
	if err != nil {
		t.Fatalf("error dropping table on db cleanup: %v", err)
	}

	dbpool.Close()
}
