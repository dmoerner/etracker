package main

import (
	"io"
	"net/http/httptest"
	"testing"
)

func TestScrape(t *testing.T) {
	config := buildTestConfig(PeersForGoodSeeds, defaultAPIKey)
	defer teardownTest(config)

	scrapeHandler := ScrapeHandler(config)

	request := httptest.NewRequest("GET", "http://example.com/scrape", nil)
	w := httptest.NewRecorder()
	scrapeHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := "d5:filesd20:aaaaaaaaaaaaaaaaaaaad8:completei0e10:downloadedi0e10:incompletei0e4:name20:aaaaaaaaaaaaaaaaaaaae20:bbbbbbbbbbbbbbbbbbbbd8:completei0e10:downloadedi0e10:incompletei0e4:name20:bbbbbbbbbbbbbbbbbbbbe20:ccccccccccccccccccccd8:completei0e10:downloadedi0e10:incompletei0e4:name20:cccccccccccccccccccce20:ddddddddddddddddddddd8:completei0e10:downloadedi0e10:incompletei0e4:name20:ddddddddddddddddddddeee"

	if string(body) != expected {
		t.Errorf("expected empty swarm scrape %s, got %s", expected, body)
	}

	request = httptest.NewRequest("GET", formatRequest(Request{
		peer_id:   peerids[1],
		info_hash: allowedInfoHashes["a"],
		event:     completed,
		left:      0,
	}), nil)
	w = httptest.NewRecorder()

	peerHandler := PeerHandler(config)
	peerHandler(w, request)

	request = httptest.NewRequest("GET", "http://example.com/scrape", nil)
	w = httptest.NewRecorder()
	scrapeHandler(w, request)

	body, _ = io.ReadAll(w.Result().Body)

	expected = "d5:filesd20:aaaaaaaaaaaaaaaaaaaad8:completei1e10:downloadedi1e10:incompletei0e4:name20:aaaaaaaaaaaaaaaaaaaae20:bbbbbbbbbbbbbbbbbbbbd8:completei0e10:downloadedi0e10:incompletei0e4:name20:bbbbbbbbbbbbbbbbbbbbe20:ccccccccccccccccccccd8:completei0e10:downloadedi0e10:incompletei0e4:name20:cccccccccccccccccccce20:ddddddddddddddddddddd8:completei0e10:downloadedi0e10:incompletei0e4:name20:ddddddddddddddddddddeee"

	if string(body) != expected {
		t.Errorf("expected non-empty swarm scrape %s, got %s", expected, body)
	}
}
