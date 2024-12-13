package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackpal/bencode-go"
)

type Request struct {
	peer_id    string
	info_hash  string
	ip         *string
	port       int
	numwant    int
	uploaded   int
	downloaded int
	left       int
	event      *string
}

type DummyRequest struct {
	request  *http.Request
	recorder *httptest.ResponseRecorder
}

var peerids = map[int]string{
	1: "-TR4060-111111111111",
	2: "-TR4060-111111111112",
	3: "-TR4060-111111111113",
	4: "-TR4060-111111111114",
}

var allowedInfoHashes = map[string]string{
	"a": "aaaaaaaaaaaaaaaaaaaa",
	"b": "bbbbbbbbbbbbbbbbbbbb",
	"c": "cccccccccccccccccccc",
	"d": "dddddddddddddddddddd",
}

const (
	deniedInfoHash = "denydenydenydenydeny"
)

func formatRequest(request Request) string {
	return fmt.Sprintf(
		"http://example.com/?peer_id=%s&info_hash=%s&port=%d&numwant=%d&uploaded=%d&downloaded=%d&left=%d",
		request.peer_id,
		request.info_hash,
		request.port,
		request.numwant,
		request.uploaded,
		request.downloaded,
		request.left)
}

var defaultAlgorithm = PeersForAnnounces

func teardownTest(config Config) {
	_, err := config.dbpool.Exec(context.Background(), "DROP TABLE peers;")
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = config.dbpool.Exec(context.Background(), "DROP TABLE infohash_allowlist;")
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}

	config.dbpool.Close()
}

func TestPoorSeeder(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm)

	requests := []Request{
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["a"],
			port:      6881,
			numwant:   1,
		},
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["b"],
			port:      6881,
			numwant:   1,
		},
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["c"],
			port:      6881,
			numwant:   1,
		},
		{
			peer_id:   peerids[2],
			info_hash: allowedInfoHashes["a"],
			port:      6881,
			numwant:   1,
		},
		{
			peer_id:   peerids[3],
			info_hash: allowedInfoHashes["a"],
			port:      6883,
			numwant:   1,
		},
		{
			peer_id:   peerids[4],
			info_hash: allowedInfoHashes["a"],
			port:      6883,
			numwant:   3,
		},
	}

	var dummyRequests []DummyRequest

	handler := PeerHandler(config)

	for _, r := range requests {
		req := httptest.NewRequest("GET", formatRequest(r), nil)
		w := httptest.NewRecorder()
		dummyRequests = append(dummyRequests, DummyRequest{request: req, recorder: w})
		handler(w, req)
	}

	lastIndex := len(dummyRequests) - 1

	resp := dummyRequests[lastIndex].recorder.Result()
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		t.Errorf("failure decoding tracker response: %v", err)
	}

	// Use type assertions to extract the compacted peerlist, which
	// uses 6 bytes per peer.
	peersReceived := []byte(data.(map[string]any)["peers"].(string))
	numRec := len(peersReceived) / 6

	// Hardcoded for test: We expect that we should receive 1 peer because
	// we have made announces for 1 torrent, although there are 3 peers
	// and the peer wanted 3.
	numToGive := 1
	if numRec != numToGive {
		t.Errorf("expected %d peers, received %d", numToGive, numRec)
	}

	teardownTest(config)
}

func TestPeerList(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm)

	requests := []Request{
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["a"],
			port:      6881,
			numwant:   1,
		},
		{
			peer_id:   peerids[2],
			info_hash: allowedInfoHashes["a"],
			port:      6882,
			numwant:   1,
		},
		{
			peer_id:   peerids[3],
			info_hash: allowedInfoHashes["a"],
			port:      6883,
			numwant:   1,
		},
	}

	var dummyRequests []DummyRequest

	handler := PeerHandler(config)

	for _, r := range requests {
		req := httptest.NewRequest("GET", formatRequest(r), nil)
		w := httptest.NewRecorder()
		dummyRequests = append(dummyRequests, DummyRequest{request: req, recorder: w})
		handler(w, req)
	}

	lastIndex := len(dummyRequests) - 1

	resp := dummyRequests[lastIndex].recorder.Result()
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		t.Errorf("failure decoding tracker response: %v", err)
	}

	// Use type assertions to extract the compacted peerlist, which
	// uses 6 bytes per peer.
	peersReceived := []byte(data.(map[string]any)["peers"].(string))
	numRec := len(peersReceived) / 6
	if numRec != requests[lastIndex].numwant {
		t.Errorf("expected %d peers, received %d", requests[lastIndex].numwant, numRec)
	}

	teardownTest(config)
}

func TestDenylistInfoHash(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm)

	request := Request{
		peer_id:   peerids[1],
		info_hash: deniedInfoHash,
		port:      6881,
	}

	req := httptest.NewRequest("GET", formatRequest(request), nil)
	w := httptest.NewRecorder()

	handler := PeerHandler(config)

	handler(w, req)

	resp := w.Result()
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		t.Errorf("failure decoding tracker response: %v", err)
	}

	if data.(map[string]any)["failure reason"].(string) != "info_hash not in the allowed list" {
		t.Errorf("did not error properly with non-allowlisted announce")
	}

	teardownTest(config)
}

func TestAnnounceWrite(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm)

	request := Request{
		peer_id:   peerids[1],
		info_hash: allowedInfoHashes["a"],
		port:      6881,
	}

	req := httptest.NewRequest("GET", formatRequest(request), nil)
	w := httptest.NewRecorder()

	handler := PeerHandler(config)

	handler(w, req)

	var peer_id []byte
	var ip_port []byte
	var info_hash []byte
	var last_announce time.Time

	err := config.dbpool.QueryRow(context.Background(), "SELECT peer_id, ip_port, info_hash, last_announce FROM peers LIMIT 1;").Scan(&peer_id, &ip_port, &info_hash, &last_announce)
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
	binary.Write(&expectedIpPort, binary.BigEndian, uint16(request.port))
	if !bytes.Equal(ip_port, expectedIpPort.Bytes()) {
		t.Errorf("ip_port: expected %v, got %v", expectedIpPort.Bytes(), ip_port)
	}

	if !last_announce.Before(time.Now()) || !last_announce.After(time.Now().Add(-time.Second)) {
		t.Error("last_announce outside one second delta from present")
	}

	teardownTest(config)
}
