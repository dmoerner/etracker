package testutils

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/db"
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

func FormatRequest(request Request) string {
	announce := fmt.Sprintf(
		"http://example.com/announce/%s?peer_id=%s&info_hash=%s&port=%d&numwant=%d&uploaded=%d&downloaded=%d&left=%d",
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

	return announce
}

func BuildTestConfig(algorithm config.PeeringAlgorithm, authorization string) config.Config {
	if _, ok := os.LookupEnv("PGUSER"); !ok {
		log.Fatal("PGUSER not set in environment")
	}
	if _, ok := os.LookupEnv("PGPASSWORD"); !ok {
		log.Fatal("PGPASSWORD not set in environment")
	}
	os.Setenv("PGDATABASE", "etracker_test")
	os.Setenv("PGPORT", "5431")
	os.Setenv("PGHOST", "localhost")

	dbpool, err := db.DbConnect()
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	// Although infohashes table normally persists, for testing it should be
	// recreated each time.
	_, err = dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS infohashes CASCADE
		`)
	if err != nil {
		log.Fatalf("Unable to clean up old infohashes table")
	}

	err = db.DbInitialize(dbpool)
	if err != nil {
		log.Fatalf("Unable to initialize DB: %v", err)
	}

	for _, v := range AnnounceKeys {
		_, err = dbpool.Exec(context.Background(), `
			INSERT INTO peerids (announce_key)
			    VALUES ($1)
			`,
			v)
		if err != nil {
			log.Fatalf("Unable to insert test allowed announce URLs: %v", err)
		}
	}

	for _, v := range AllowedInfoHashes {
		_, err = dbpool.Exec(context.Background(), `
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
	}

	return conf
}

func TeardownTest(conf config.Config) {
	_, err := conf.Dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS peers
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = conf.Dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS infohashes
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = conf.Dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS peerids
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}

	conf.Dbpool.Close()
}
