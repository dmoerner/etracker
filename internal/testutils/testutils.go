package testutils

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/db"
	"github.com/redis/go-redis/v9"
)

const DefaultAPIKey = "testauthorizationkey"

var AllowedInfoHashes = map[string]string{
	"a": "aaaaaaaaaaaaaaaaaaaa",
	"b": "bbbbbbbbbbbbbbbbbbbb",
	"c": "cccccccccccccccccccc",
	"d": "dddddddddddddddddddd",
}

var AnnounceKeys = map[int]string{
	1: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	2: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	3: "cccccccccccccccccccccccccccccc",
	4: "dddddddddddddddddddddddddddddd",
	5: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
}

const UntrackedAnnounceKey = "000000000000000000000000000000"

type Request struct {
	AnnounceKey string
	Info_hash   string
	Ip          *string
	Port        int
	Numwant     int
	Uploaded    int
	Downloaded  int
	Left        int
	Event       config.Event
}

func GeneratePeerID() string {
	peer_id := make([]byte, 20)
	_, _ = rand.Read(peer_id)
	return string(peer_id)
}

func CreateTestAnnounce(request Request) *http.Request {
	announce := fmt.Sprintf(
		"http://example.com/%s/announce?peer_id=%s&info_hash=%s&port=%d&numwant=%d&uploaded=%d&downloaded=%d&left=%d",
		request.AnnounceKey,
		url.QueryEscape(GeneratePeerID()),
		url.QueryEscape(request.Info_hash),
		request.Port,
		request.Numwant,
		request.Uploaded,
		request.Downloaded,
		request.Left)

	var event string
	switch request.Event {
	case config.Stopped:
		event = "stopped"
	case config.Started:
		event = "started"
	case config.Completed:
		event = "completed"
	}

	if event != "" {
		announce += fmt.Sprintf("&event=%s", event)
	}

	newRequest := httptest.NewRequest("GET", announce, nil)
	newRequest.SetPathValue("id", request.AnnounceKey)

	return newRequest
}

func BuildTestConfig(ctx context.Context, algorithm config.PeeringAlgorithm, authorization string) config.Config {
	if _, ok := os.LookupEnv("PGUSER"); !ok {
		log.Fatal("PGUSER not set in environment")
	}
	if _, ok := os.LookupEnv("PGPASSWORD"); !ok {
		log.Fatal("PGPASSWORD not set in environment")
	}
	os.Setenv("PGDATABASE", "etracker_test")
	os.Setenv("PGPORT", "5431")
	os.Setenv("PGHOST", "localhost")

	redis_password, ok := os.LookupEnv("ETRACKER_REDIS")
	if !ok {
		log.Fatal("ETRACKER_REDIS not set in environment.")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: redis_password,
		DB:       1, // testing DB
	})

	// Always flush testing database before each run.
	rdb.FlushDB(ctx)

	dbpool, err := db.DbConnect(ctx)
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	// Although infohashes table normally persists, for testing it should be
	// recreated each time.
	_, err = dbpool.Exec(ctx, `
		DROP TABLE IF EXISTS infohashes CASCADE
		`)
	if err != nil {
		log.Fatalf("Unable to clean up old infohashes table")
	}

	err = db.DbInitialize(ctx, dbpool)
	if err != nil {
		log.Fatalf("Unable to initialize DB: %v", err)
	}

	for _, v := range AnnounceKeys {
		_, err = dbpool.Exec(ctx, `
			INSERT INTO peers (announce_key)
			    VALUES ($1)
			`,
			v)
		if err != nil {
			log.Fatalf("Unable to insert test allowed announce URLs: %v", err)
		}
	}

	for _, v := range AllowedInfoHashes {
		_, err = dbpool.Exec(ctx, `
			INSERT INTO infohashes (info_hash, name)
			    VALUES ($1, $2)
			`,
			v,
			string(v))
		if err != nil {
			log.Fatalf("Unable to insert test allowed infohashes: %v", err)
		}
	}

	conf := config.Config{
		Algorithm:     algorithm,
		Authorization: authorization,
		Dbpool:        dbpool,
		Rdb:           rdb,
	}

	return conf
}

func TeardownTest(ctx context.Context, conf config.Config) {
	_, err := conf.Dbpool.Exec(ctx, `
		DROP TABLE IF EXISTS announces
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = conf.Dbpool.Exec(ctx, `
		DROP TABLE IF EXISTS infohashes
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = conf.Dbpool.Exec(ctx, `
		DROP TABLE IF EXISTS peers
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}

	conf.Dbpool.Close()
}
