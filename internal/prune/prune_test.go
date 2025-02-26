package prune

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/testutils"
)

func TestOldCreationOldAnnounces(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	query := fmt.Sprintf(`
		UPDATE
		    peers
		SET
		    created_time = created_time - INTERVAL '%d months'
		WHERE
		    announce_key = $1
		`, PruneIntervalMonths+1)

	_, err := conf.Dbpool.Exec(context.Background(), query, testutils.AnnounceKeys[1])
	if err != nil {
		t.Errorf("error setting fake key created time: %v", err)
	}

	handler := handler.PeerHandler(conf)
	req := testutils.CreateTestAnnounce(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
	})
	w := httptest.NewRecorder()

	handler(w, req)

	// Since we only have one announce, we can UPDATE on all rows. We need to disable
	// the trigger first.
	query = fmt.Sprintf(`
		ALTER TABLE announces DISABLE TRIGGER ALL;

		UPDATE
		    announces
		SET
		    last_announce = last_announce - INTERVAL '%d months';
		`, PruneIntervalMonths+1)

	_, err = conf.Dbpool.Exec(context.Background(), query)
	if err != nil {
		t.Errorf("error setting fake key created time: %v", err)
	}

	err = PruneAnnounceKeys(conf)
	if err != nil {
		t.Errorf("error pruning announce keys: %v", err)
	}

	var tracked_keys int
	err = conf.Dbpool.QueryRow(context.Background(), `
		SELECT COUNT(announce_key) FROM peers
		`).Scan(&tracked_keys)
	if err != nil {
		t.Errorf("error querying db: %v", err)
	}

	expected := len(testutils.AnnounceKeys) - 1

	if tracked_keys != expected {
		t.Errorf("expected %d keys in db, found %d", expected, tracked_keys)
	}
}

func TestOldCreationRecentAnnounces(t *testing.T) {
	conf := testutils.BuildTestConfig(handler.DefaultAlgorithm, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	query := fmt.Sprintf(`
		UPDATE
		    peers
		SET
		    created_time = NOW() - INTERVAL '%d months'
		WHERE
		    announce_key = $1
		`, PruneIntervalMonths+1)

	_, err := conf.Dbpool.Exec(context.Background(), query, testutils.AnnounceKeys[1])
	if err != nil {
		t.Errorf("error setting fake key created time: %v", err)
	}

	handler := handler.PeerHandler(conf)
	req := testutils.CreateTestAnnounce(testutils.Request{
		AnnounceKey: testutils.AnnounceKeys[1],
		Info_hash:   testutils.AllowedInfoHashes["a"],
	})
	w := httptest.NewRecorder()

	handler(w, req)

	err = PruneAnnounceKeys(conf)
	if err != nil {
		t.Errorf("error pruning announce keys: %v", err)
	}

	var tracked_keys int
	err = conf.Dbpool.QueryRow(context.Background(), `
		SELECT COUNT(announce_key) FROM peers
		`).Scan(&tracked_keys)
	if err != nil {
		t.Errorf("error querying db: %v", err)
	}

	expected := len(testutils.AnnounceKeys)

	if tracked_keys != expected {
		t.Errorf("expected %d keys in db, found %d", expected, tracked_keys)
	}
}

func TestOldCreationNoAnnounces(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	query := fmt.Sprintf(`
		UPDATE
		    peers
		SET
		    created_time = NOW() - INTERVAL '%d months'
		WHERE
		    announce_key = $1
		`, PruneIntervalMonths+1)

	_, err := conf.Dbpool.Exec(context.Background(), query, testutils.AnnounceKeys[1])
	if err != nil {
		t.Errorf("error setting fake key created time: %v", err)
	}

	err = PruneAnnounceKeys(conf)
	if err != nil {
		t.Errorf("error pruning announce keys: %v", err)
	}

	var tracked_keys int
	err = conf.Dbpool.QueryRow(context.Background(), `
		SELECT COUNT(announce_key) FROM peers
		`).Scan(&tracked_keys)
	if err != nil {
		t.Errorf("error querying db: %v", err)
	}

	expected := len(testutils.AnnounceKeys) - 1

	if tracked_keys != expected {
		t.Errorf("expected %d keys in db, found %d", expected, tracked_keys)
	}
}

func TestRecentCreationNoAnnounces(t *testing.T) {
	conf := testutils.BuildTestConfig(nil, testutils.DefaultAPIKey)
	defer testutils.TeardownTest(conf)

	err := PruneAnnounceKeys(conf)
	if err != nil {
		t.Errorf("error pruning announce keys: %v", err)
	}

	var tracked_keys int
	err = conf.Dbpool.QueryRow(context.Background(), `
		SELECT COUNT(announce_key) FROM peers
		`).Scan(&tracked_keys)
	if err != nil {
		t.Errorf("error querying db: %v", err)
	}

	expected := len(testutils.AnnounceKeys)

	if tracked_keys != expected {
		t.Errorf("expected %d keys in db, found %d", expected, tracked_keys)
	}
}
