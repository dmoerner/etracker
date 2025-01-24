package scrape

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/testutils"
)

func TestScrape(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	scrapeHandler := ScrapeHandler(conf)

	request := httptest.NewRequest("GET", "http://example.com/scrape", nil)
	w := httptest.NewRecorder()
	scrapeHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := "d5:filesd20:aaaaaaaaaaaaaaaaaaaad8:completei0e10:downloadedi0e10:incompletei0e4:name20:aaaaaaaaaaaaaaaaaaaae20:bbbbbbbbbbbbbbbbbbbbd8:completei0e10:downloadedi0e10:incompletei0e4:name20:bbbbbbbbbbbbbbbbbbbbe20:ccccccccccccccccccccd8:completei0e10:downloadedi0e10:incompletei0e4:name20:cccccccccccccccccccce20:ddddddddddddddddddddd8:completei0e10:downloadedi0e10:incompletei0e4:name20:ddddddddddddddddddddeee"

	if string(body) != expected {
		t.Errorf("expected empty swarm scrape %s, got %s", expected, body)
	}

	request = httptest.NewRequest("GET", testutils.FormatRequest(testutils.Request{
		Peer_id:   testutils.Peerids[1],
		Info_hash: testutils.AllowedInfoHashes["a"],
		Event:     config.Completed,
		Left:      0,
	}), nil)
	w = httptest.NewRecorder()

	peerHandler := handler.PeerHandler(conf)
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
