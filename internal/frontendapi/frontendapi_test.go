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
)

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

	expected := Stats{
		Hashcount: len(testutils.AllowedInfoHashes),
		Seeders:   1,
		Leechers:  0,
	}

	var received Stats

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

	var received Key

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
