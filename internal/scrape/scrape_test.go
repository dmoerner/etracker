package scrape

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/testutils"
)

// This test tests both single and multiple query scrapes.
func TestSpecificScrape(t *testing.T) {
	ctx := context.Background()
	tc, conf := testutils.BuildTestConfig(ctx, handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, tc, conf)

	// The database is pre-populated with allowed infohashes.

	scrapeHandler := ScrapeHandler(ctx, conf)

	request := testutils.CreateTestAnnounce(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	})
	w := httptest.NewRecorder()

	peerHandler := handler.PeerHandler(ctx, conf)
	peerHandler(w, request)

	request = httptest.NewRequest("GET",
		fmt.Sprintf("http://example.com/scrape?info_hash=%s", testutils.AllowedInfoHashes["a"]),
		nil)
	w = httptest.NewRecorder()
	scrapeHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := "d5:filesd20:aaaaaaaaaaaaaaaaaaaad8:completei1e10:downloadedi1e10:incompletei0e4:name20:aaaaaaaaaaaaaaaaaaaaeee"

	if string(body) != expected {
		t.Errorf("expected non-empty swarm scrape %s, got %s", expected, body)
	}

	request = httptest.NewRequest("GET",
		fmt.Sprintf("http://example.com/scrape?info_hash=%s&info_hash=%s", testutils.AllowedInfoHashes["a"], testutils.AllowedInfoHashes["b"]),
		nil)
	w = httptest.NewRecorder()
	scrapeHandler(w, request)

	body, _ = io.ReadAll(w.Result().Body)

	expected = "d5:filesd20:aaaaaaaaaaaaaaaaaaaad8:completei1e10:downloadedi1e10:incompletei0e4:name20:aaaaaaaaaaaaaaaaaaaae20:bbbbbbbbbbbbbbbbbbbbd8:completei0e10:downloadedi0e10:incompletei0e4:name20:bbbbbbbbbbbbbbbbbbbbeee"

	if string(body) != expected {
		t.Errorf("expected non-empty swarm scrape %s, got %s", expected, body)
	}
}

func TestAllScrape(t *testing.T) {
	ctx := context.Background()
	tc, conf := testutils.BuildTestConfig(ctx, handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(ctx, tc, conf)

	scrapeHandler := ScrapeHandler(ctx, conf)

	request := httptest.NewRequest("GET", "http://example.com/scrape", nil)
	w := httptest.NewRecorder()
	scrapeHandler(w, request)

	body, _ := io.ReadAll(w.Result().Body)

	expected := "d5:filesd20:aaaaaaaaaaaaaaaaaaaad8:completei0e10:downloadedi0e10:incompletei0e4:name20:aaaaaaaaaaaaaaaaaaaae20:bbbbbbbbbbbbbbbbbbbbd8:completei0e10:downloadedi0e10:incompletei0e4:name20:bbbbbbbbbbbbbbbbbbbbe20:ccccccccccccccccccccd8:completei0e10:downloadedi0e10:incompletei0e4:name20:cccccccccccccccccccce20:ddddddddddddddddddddd8:completei0e10:downloadedi0e10:incompletei0e4:name20:ddddddddddddddddddddeee"

	if string(body) != expected {
		t.Errorf("expected empty swarm scrape %s, got %s", expected, body)
	}

	request = testutils.CreateTestAnnounce(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
		Event:       config.Completed,
		Left:        0,
	})
	w = httptest.NewRecorder()

	peerHandler := handler.PeerHandler(ctx, conf)
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
