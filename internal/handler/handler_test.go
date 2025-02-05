package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/testutils"

	bencode "github.com/jackpal/bencode-go"
)

type RequestResponseWrapper struct {
	request  *http.Request
	recorder *httptest.ResponseRecorder
}

const (
	deniedInfoHash = "denydenydenydenydeny"
)

// createNSeeders is a helper function which creates n request structs for a
// specified info_hash. Used to populate the handler with many existing
// seeders.
func createNSeeders(conf config.Config, n int, info_hash string) []testutils.Request {
	var requests []testutils.Request

	for range n {
		announce_key, err := config.GenerateAnnounceKey()
		if err != nil {
			log.Fatalf("createNSeeders: Unable to generate announce keys: %v", err)
		}
		_, err = conf.Dbpool.Exec(context.Background(), `
			INSERT INTO peerids (announce_key)
			    VALUES ($1)
			`,
			announce_key)
		if err != nil {
			log.Fatalf("createNSeeders: Unable to insert announce keys: %v", err)
		}
		requests = append(requests, testutils.Request{
			AnnounceKey: announce_key,
			Info_hash:   info_hash,
		})
	}

	return requests
}

// seedNTorrents is a helper function which adds n torrents with random
// info_hashes to a particular announce_key. This also requires inserting these
// info_hashes into the allowlist in the test db. Used to mimic good or bad
// seeding behavior.
func seedNTorrents(conf config.Config, n int, announce_key string) []testutils.Request {
	var requests []testutils.Request

	for i := range n {
		info_hash := make([]byte, 20)
		_, _ = rand.Read(info_hash)
		_, err := conf.Dbpool.Exec(context.Background(), `
			INSERT INTO infohashes (info_hash, name)
			    VALUES ($1, $2)
			`, info_hash, fmt.Sprintf("test infohash %d", i))
		if err != nil {
			log.Fatalf("Unable to insert test allowed infohashes: %v", err)
		}
		requests = append(requests, testutils.Request{
			AnnounceKey: announce_key,
			Info_hash:   string(info_hash),
		})
	}
	return requests
}

// countPeersReceived is a helper function which reads in a compact HTTP
// tracker response and returns the number of peers.
func countPeersReceived(recorder *httptest.ResponseRecorder) int {
	resp := recorder.Result()
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		return 0
	}

	// Use type assertions to extract the compacted peerlist, which
	// uses 6 bytes per peer.
	peersReceived := []byte(data.(map[string]any)["peers"].(string))
	numRec := len(peersReceived) / 6

	return numRec
}

func TestDownloadedIncrement(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
	}

	handler := PeerHandler(conf)

	req := httptest.NewRequest("GET", testutils.FormatRequest(request), nil)
	w := httptest.NewRecorder()
	handler(w, req)

	var downloaded int

	err := conf.Dbpool.QueryRow(context.Background(), `
		SELECT
		    downloaded
		FROM
		    infohashes
		WHERE
		    info_hash = $1
		`,
		request.Info_hash).Scan(&downloaded)
	if err != nil {
		t.Fatalf("error querying test db: %v", err)
	}

	if downloaded != 1 {
		t.Errorf("expected %d downloads for info_hash %v, got %d", 1, request.Info_hash, downloaded)
	}
}

// TODO: Refactor these tests to not rely on fragile indexing into a slice.
func TestPeersForSeeds(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	// Setup: A client with three seeds requesting three peers gets three.
	// A client with no seeds requesting three peers gets one.
	requests := []testutils.Request{
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["a"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["b"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["c"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[3],
			Info_hash:   testutils.AllowedInfoHashes["d"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[4],
			Info_hash:   testutils.AllowedInfoHashes["d"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[5],
			Info_hash:   testutils.AllowedInfoHashes["d"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["d"],
			Left:        100,
			Numwant:     3,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[2],
			Info_hash:   testutils.AllowedInfoHashes["d"],
			Left:        100,
			Numwant:     3,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[2],
			Info_hash:   testutils.AllowedInfoHashes["b"],
			Numwant:     1,
			Left:        100,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[2],
			Info_hash:   testutils.AllowedInfoHashes["c"],
			Numwant:     1,
			Left:        100,
		},
	}

	var dummyRequests []RequestResponseWrapper

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	for _, r := range requests {
		req := httptest.NewRequest("GET", testutils.FormatRequest(r), nil)
		w := httptest.NewRecorder()
		dummyRequests = append(dummyRequests, RequestResponseWrapper{request: req, recorder: w})
		mux.ServeHTTP(w, req)
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
		resp := dummyRequests[expected[i].index].recorder
		numRec := countPeersReceived(resp)

		if numRec != expected[i].expected {
			t.Errorf("%s expected %d peers, received %d", expected[i].name, expected[i].expected, numRec)
		}
	}
}

func TestStopped(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForAnnounces, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	requests := []testutils.Request{
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Numwant:     1,
			Event:       config.Stopped,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[2],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Numwant:     1,
		},
	}

	var dummyRequests []RequestResponseWrapper

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	for _, r := range requests {
		req := httptest.NewRequest("GET", testutils.FormatRequest(r), nil)
		w := httptest.NewRecorder()
		dummyRequests = append(dummyRequests, RequestResponseWrapper{request: req, recorder: w})
		mux.ServeHTTP(w, req)
	}

	lastIndex := len(dummyRequests) - 1

	resp := dummyRequests[lastIndex].recorder
	numRec := countPeersReceived(resp)

	// Hardcoded for test: We expect that we should receive 0 peers despite
	// wanting one, because the only peer in the swarm announces that
	// they have stopped.
	numToGive := 0
	if numRec != numToGive {
		t.Errorf("expected %d peers, received %d", numToGive, numRec)
	}
}

// TestPeersForGoodSeedsBigSwarm builds a swarm of 50 seeders, and then tests
// two new leechers: One with zero torrents seeding, and a second with six
// total torrents seeding. The expectations for this test are set by the values
// encoded in algorithms.go.
func TestPeersForGoodSeedsBigSwarm(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForGoodSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	// Populate 50 seeders
	seeders := createNSeeders(conf, 50, testutils.AllowedInfoHashes["a"])

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	for _, r := range seeders {
		req := httptest.NewRequest("GET", testutils.FormatRequest(r), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}

	// Test bad seeder, they are not currently in the swarm.
	badSeederRequest := testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Numwant:     50,
	}
	badSeederExpected := minimumPeers

	badSeederRecorder := httptest.NewRecorder()
	mux.ServeHTTP(badSeederRecorder,
		httptest.NewRequest("GET", testutils.FormatRequest(badSeederRequest), nil))

	badSeederReceived := countPeersReceived(badSeederRecorder)
	if badSeederReceived != badSeederExpected {
		t.Errorf("bad seeder: expected %d peers, got %d", badSeederExpected, badSeederReceived)
	}

	// Test good seeder, they are the first infohash in seeders.
	goodSeederRequest := testutils.Request{
		AnnounceKey: seeders[0].AnnounceKey,
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Numwant:     50,
	}
	goodSeederExpected := goodSeederRequest.Numwant

	goodSeederSeeds := seedNTorrents(conf, 5, goodSeederRequest.AnnounceKey)
	for _, r := range goodSeederSeeds {
		req := httptest.NewRequest("GET", testutils.FormatRequest(r), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}

	goodSeederRecorder := httptest.NewRecorder()
	mux.ServeHTTP(goodSeederRecorder,
		httptest.NewRequest("GET", testutils.FormatRequest(goodSeederRequest), nil))

	goodSeederReceived := countPeersReceived(goodSeederRecorder)
	if goodSeederReceived != goodSeederExpected {
		t.Errorf("good seeder: expected %d peers, got %d", goodSeederExpected, goodSeederReceived)
	}
}

// TestPeersForGoodSeedsSmallSwarm is an older test from before the
// introduction of the smoothing algorithm. With the new smoothing function,
// all it verifies is that PeersForGoodSeeds works properly when the swarm
// size is below minimumPeers.
func TestPeersForGoodSeedsSmallSwarm(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForGoodSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	// Peer 1 is seeding two torrents, but both with ratio > 1,
	// so can receive all 4 peers.
	requests := []testutils.Request{
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Uploaded:    2,
			Downloaded:  1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["b"],
			Uploaded:    2,
			Downloaded:  1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[2],
			Info_hash:   testutils.AllowedInfoHashes["d"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[3],
			Info_hash:   testutils.AllowedInfoHashes["d"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[4],
			Info_hash:   testutils.AllowedInfoHashes["d"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[5],
			Info_hash:   testutils.AllowedInfoHashes["d"],
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["d"],
			Numwant:     10,
		},
	}

	var dummyRequests []RequestResponseWrapper

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	for _, r := range requests {
		req := httptest.NewRequest("GET", testutils.FormatRequest(r), nil)
		w := httptest.NewRecorder()
		dummyRequests = append(dummyRequests, RequestResponseWrapper{request: req, recorder: w})
		mux.ServeHTTP(w, req)
	}

	lastIndex := len(dummyRequests) - 1

	resp := dummyRequests[lastIndex].recorder
	numRec := countPeersReceived(resp)

	// Hardcoded for test: We expect that we should receive 4 peers because
	// we are seeding 2 torrents and both have a positive ratio.
	numToGive := 4
	if numRec != numToGive {
		t.Errorf("expected %d peers, received %d", numToGive, numRec)
	}
}

func TestPeersForAnnounces(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForAnnounces, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	requests := []testutils.Request{
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["b"],
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["c"],
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[2],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[3],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[4],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Numwant:     3,
		},
	}

	var dummyRequests []RequestResponseWrapper

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	for _, r := range requests {
		req := httptest.NewRequest("GET", testutils.FormatRequest(r), nil)
		w := httptest.NewRecorder()
		dummyRequests = append(dummyRequests, RequestResponseWrapper{request: req, recorder: w})
		mux.ServeHTTP(w, req)
	}

	lastIndex := len(dummyRequests) - 1

	resp := dummyRequests[lastIndex].recorder
	numRec := countPeersReceived(resp)

	// Hardcoded for test: We expect that we should receive 1 peer because
	// we have made announces for 1 torrent, although there are 3 peers
	// and the peer wanted 3.
	numToGive := 1
	if numRec != numToGive {
		t.Errorf("expected %d peers, received %d", numToGive, numRec)
	}
}

func TestPeerList(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	requests := []testutils.Request{
		{
			AnnounceKey: testutils.AnnounceKeys[1],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Port:        6881,
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[2],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Port:        6882,
			Numwant:     1,
		},
		{
			AnnounceKey: testutils.AnnounceKeys[3],
			Info_hash:   testutils.AllowedInfoHashes["a"],
			Port:        6883,
			Numwant:     1,
		},
	}

	var dummyRequests []RequestResponseWrapper

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	for _, r := range requests {
		req := httptest.NewRequest("GET", testutils.FormatRequest(r), nil)
		w := httptest.NewRecorder()
		dummyRequests = append(dummyRequests, RequestResponseWrapper{request: req, recorder: w})
		mux.ServeHTTP(w, req)
	}

	lastIndex := len(dummyRequests) - 1

	resp := dummyRequests[lastIndex].recorder
	numRec := countPeersReceived(resp)

	if numRec != requests[lastIndex].Numwant {
		t.Errorf("expected %d peers, received %d", requests[lastIndex].Numwant, numRec)
	}
}

func TestDenylistInfoHash(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   deniedInfoHash,
		Port:        6881,
	}

	req := httptest.NewRequest("GET", testutils.FormatRequest(request), nil)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	mux.ServeHTTP(w, req)

	resp := w.Result()
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		t.Errorf("failure decoding tracker response: %v", err)
	}

	if data.(map[string]any)["failure reason"].(string) != "info_hash not in the allowed list" {
		t.Errorf("did not error properly with non-allowlisted announce")
	}
}

func TestDisableAllowlist(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	conf.DisableAllowlist = true

	request := testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   deniedInfoHash,
		Port:        6881,
	}

	req := httptest.NewRequest("GET", testutils.FormatRequest(request), nil)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	mux.ServeHTTP(w, req)

	resp := w.Result()
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		t.Errorf("failure decoding tracker response: %v", err)
	}

	if _, ok := data.(map[string]any)["failure reason"]; ok {
		t.Errorf("received failure reason for unlisted infohash despite disabling allowlist")
	}

	var found bool
	err = conf.Dbpool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT FROM infohashes WHERE info_hash = $1)
		`, request.Info_hash).Scan(&found)
	if err != nil {
		t.Fatalf("error querying test db: %v", err)
	}

	if !found {
		t.Errorf("did not find info_hash in infohashes table")
	}
}

func TestAnnounceWrite(t *testing.T) {
	conf := testutils.BuildTestConfig(PeersForSeeds, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Port:        6881,
	}

	req := httptest.NewRequest("GET", testutils.FormatRequest(request), nil)
	w := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.HandleFunc("/{id}/announce", PeerHandler(conf))

	mux.ServeHTTP(w, req)

	var ip_port []byte
	var info_hash []byte
	var last_announce time.Time

	err := conf.Dbpool.QueryRow(context.Background(), `
		SELECT
		    ip_port,
		    info_hash,
		    last_announce
		FROM
		    peers
		    JOIN peerids ON peers.announce_id = peerids.id
		    JOIN infohashes ON peers.info_hash_id = infohashes.id
		LIMIT 1
		`).Scan(&ip_port, &info_hash, &last_announce)
	if err != nil {
		t.Fatalf("error querying test db: %v", err)
	}

	if !bytes.Equal(info_hash, []byte(request.Info_hash)) {
		t.Errorf("info_hash: expected %s, got %s", request.Info_hash, info_hash)
	}

	var expectedIpPort bytes.Buffer

	// For reasons that are unclear to me, httptest.NewRequest ignores httptest.DefaultNewRequest
	// and hard-codes this IP instead, following RFC 5737.
	expectedIpPort.Write([]byte(net.ParseIP("192.0.2.1").To4()))
	binary.Write(&expectedIpPort, binary.BigEndian, uint16(request.Port))
	if !bytes.Equal(ip_port, expectedIpPort.Bytes()) {
		t.Errorf("ip_port: expected %v, got %v", expectedIpPort.Bytes(), ip_port)
	}

	if !last_announce.Before(time.Now()) || !last_announce.After(time.Now().Add(-time.Second)) {
		t.Error("last_announce outside one second delta from present")
	}
}
