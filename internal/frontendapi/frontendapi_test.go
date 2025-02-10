package frontendapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/testutils"
	"github.com/google/go-cmp/cmp"
)

func TestInfohashes(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := httptest.NewRequest("GET", testutils.FormatRequest(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	}), nil)
	w := httptest.NewRecorder()

	peerHandler := handler.PeerHandler(conf)
	peerHandler(w, request)

	request = httptest.NewRequest("GET", "http://example.com/frontendapi/infohashes", nil)
	w = httptest.NewRecorder()

	infohashesHandler := InfohashesHandler(conf)
	infohashesHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := []InfohashesJSON{
		{
			Name:       testutils.AllowedInfoHashes["a"],
			Downloaded: 1,
			Seeders:    1,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["a"]),
		},
		{
			Name:       testutils.AllowedInfoHashes["b"],
			Downloaded: 0,
			Seeders:    0,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["b"]),
		},
		{
			Name:       testutils.AllowedInfoHashes["c"],
			Downloaded: 0,
			Seeders:    0,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["c"]),
		},
		{
			Name:       testutils.AllowedInfoHashes["d"],
			Downloaded: 0,
			Seeders:    0,
			Leechers:   0,
			Info_hash:  []byte(testutils.AllowedInfoHashes["d"]),
		},
	}

	var received []InfohashesJSON

	err := json.Unmarshal(body, &received)
	if err != nil {
		t.Errorf("error unmarshalling json response: %v", err)
	}

	// Use cmp.Diff for deep comparison of slices.
	if cmp.Diff(expected, received) != "" {
		t.Errorf("error in infohashes json, expected %v, got %v", expected, received)
	}
}

func TestStats(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := httptest.NewRequest("GET", testutils.FormatRequest(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	}), nil)
	w := httptest.NewRecorder()

	peerHandler := handler.PeerHandler(conf)
	peerHandler(w, request)

	request = httptest.NewRequest("GET", "http://example.com/frontendapi/stats", nil)
	w = httptest.NewRecorder()

	statsHandler := StatsHandler(conf)
	statsHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := StatsJSON{
		Hashcount: len(testutils.AllowedInfoHashes),
		Seeders:   1,
		Leechers:  0,
	}

	var received StatsJSON

	err := json.Unmarshal(body, &received)
	if err != nil {
		t.Errorf("error unmarshalling json response: %v", err)
	}

	if received != expected {
		t.Errorf("error in stats json, expected %v, got %v", expected, received)
	}
}

func TestGenerate(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	request := httptest.NewRequest("GET", "http://example.com/frontendapi/generate", nil)
	w := httptest.NewRecorder()

	generateHandler := GenerateHandler(conf)
	generateHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	var received KeyJSON

	err := json.Unmarshal(body, &received)
	if err != nil {
		t.Errorf("error: did not receive key from generate endpoint: %v", err)
	}

	// Verify that the key was written to the db.
	var written bool
	err = conf.Dbpool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT FROM peerids WHERE announce_key = $1)
		`,
		received.Announce_key).Scan(&written)
	if err != nil {
		t.Errorf("error: could not check database for written key: %v", err)
	}

	if !written {
		t.Errorf("key %s not written to database", received.Announce_key)
	}
}
