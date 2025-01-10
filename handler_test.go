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
	event      Event
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
	5: "-TR4060-111111111115",
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
	announce := fmt.Sprintf(
		"http://example.com/announce/?peer_id=%s&info_hash=%s&port=%d&numwant=%d&uploaded=%d&downloaded=%d&left=%d",
		request.peer_id,
		request.info_hash,
		request.port,
		request.numwant,
		request.uploaded,
		request.downloaded,
		request.left)

	var event string
	switch request.event {
	case stopped:
		event = "stopped"
	case started:
		event = "started"
	case completed:
		event = "completed"
	}

	if event != "" {
		announce += fmt.Sprintf("&event=%s", event)
	}

	return announce
}

func teardownTest(config Config) {
	_, err := config.dbpool.Exec(context.Background(), "DROP TABLE peers;")
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = config.dbpool.Exec(context.Background(), "DROP TABLE infohashes;")
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}

	config.dbpool.Close()
}

func TestDownloadedIncrement(t *testing.T) {
	config := buildTestConfig(PeersForSeeds, defaultAPIKey)
	defer teardownTest(config)

	request := Request{
		peer_id:   peerids[1],
		info_hash: allowedInfoHashes["a"],
		event:     completed,
	}

	handler := PeerHandler(config)

	req := httptest.NewRequest("GET", formatRequest(request), nil)
	w := httptest.NewRecorder()
	handler(w, req)

	var downloaded int

	err := config.dbpool.QueryRow(context.Background(), "SELECT downloaded FROM infohashes where info_hash = $1;", request.info_hash).Scan(&downloaded)
	if err != nil {
		t.Fatalf("error querying test db: %v", err)
	}

	if downloaded != 1 {
		t.Errorf("expected %d downloads for info_hash %v, got %d", 1, request.info_hash, downloaded)
	}
}

// TODO: Refactor these tests to not rely on fragile indexing into a slice.
func TestPeersForSeeds(t *testing.T) {
	config := buildTestConfig(PeersForSeeds, defaultAPIKey)
	defer teardownTest(config)

	// Setup: A client with three seeds requesting three peers gets three.
	// A client with no seeds requesting three peers gets one.
	requests := []Request{
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["a"],
		},
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["b"],
		},
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["c"],
		},
		{
			peer_id:   peerids[3],
			info_hash: allowedInfoHashes["d"],
		},
		{
			peer_id:   peerids[4],
			info_hash: allowedInfoHashes["d"],
		},
		{
			peer_id:   peerids[5],
			info_hash: allowedInfoHashes["d"],
		},
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["d"],
			left:      100,
			numwant:   3,
		},
		{
			peer_id:   peerids[2],
			info_hash: allowedInfoHashes["d"],
			left:      100,
			numwant:   3,
		},
		{
			peer_id:   peerids[2],
			info_hash: allowedInfoHashes["b"],
			numwant:   1,
			left:      100,
		},
		{
			peer_id:   peerids[2],
			info_hash: allowedInfoHashes["c"],
			numwant:   1,
			left:      100,
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

	expected := []struct {
		name     string
		index    int
		expected int
	}{
		{"good seeder", 6, 3},
		{"poor seeder", 7, 1},
	}

	for i := range expected {
		resp := dummyRequests[expected[i].index].recorder.Result()
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
		if numRec != expected[i].expected {
			t.Errorf("%s expected %d peers, received %d", expected[i].name, expected[i].expected, numRec)
		}
	}
}

func TestStopped(t *testing.T) {
	config := buildTestConfig(PeersForAnnounces, defaultAPIKey)
	defer teardownTest(config)

	requests := []Request{
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["a"],
			port:      6881,
			numwant:   1,
		},
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["a"],
			port:      6881,
			numwant:   1,
			event:     stopped,
		},
		{
			peer_id:   peerids[2],
			info_hash: allowedInfoHashes["a"],
			port:      6881,
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

	// Hardcoded for test: We expect that we should receive 0 peers despite
	// wanting one, because the only peer in the swarm announces that
	// they have stopped.
	numToGive := 0
	if numRec != numToGive {
		t.Errorf("expected %d peers, received %d", numToGive, numRec)
	}
}

func TestPeersForGoodSeeds(t *testing.T) {
	config := buildTestConfig(PeersForGoodSeeds, defaultAPIKey)
	defer teardownTest(config)

	// Peer 1 is seeding two torrents, but both with ratio > 1,
	// so can receive all 4 peers.
	requests := []Request{
		{
			peer_id:    peerids[1],
			info_hash:  allowedInfoHashes["a"],
			port:       6881,
			uploaded:   2,
			downloaded: 1,
		},
		{
			peer_id:    peerids[1],
			info_hash:  allowedInfoHashes["b"],
			port:       6881,
			uploaded:   2,
			downloaded: 1,
		},
		{
			peer_id:   peerids[2],
			info_hash: allowedInfoHashes["d"],
			port:      6881,
		},
		{
			peer_id:   peerids[3],
			info_hash: allowedInfoHashes["d"],
			port:      6883,
		},
		{
			peer_id:   peerids[4],
			info_hash: allowedInfoHashes["d"],
			port:      6883,
		},
		{
			peer_id:   peerids[5],
			info_hash: allowedInfoHashes["d"],
			port:      6883,
		},
		{
			peer_id:   peerids[1],
			info_hash: allowedInfoHashes["d"],
			port:      6881,
			numwant:   10,
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

	// Hardcoded for test: We expect that we should receive 4 peers because
	// we are seeding 2 torrents and both have a positive ratio.
	numToGive := 4
	if numRec != numToGive {
		t.Errorf("expected %d peers, received %d", numToGive, numRec)
	}
}

func TestPeersForAnnounces(t *testing.T) {
	config := buildTestConfig(PeersForAnnounces, defaultAPIKey)
	defer teardownTest(config)

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
}

func TestPeerList(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm, defaultAPIKey)
	defer teardownTest(config)

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
}

func TestDenylistInfoHash(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm, defaultAPIKey)
	defer teardownTest(config)

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
}

func TestAnnounceWrite(t *testing.T) {
	config := buildTestConfig(defaultAlgorithm, defaultAPIKey)
	defer teardownTest(config)

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
}
