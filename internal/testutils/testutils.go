package testutils

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/db"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
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

type TestContainer struct {
	pgs *postgres.PostgresContainer
	rdb *tcredis.RedisContainer
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

func BuildTestConfig(ctx context.Context, algorithm config.PeeringAlgorithm, authorization string) (*TestContainer, config.Config) {
	dbName := "users"
	dbUser := "testuser"
	dbPassword := "testpassword"

	pgsctr, err := postgres.Run(
		ctx,
		"postgres:17",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.BasicWaitStrategies(),
		postgres.WithSQLDriver("pgx"),
	)
	if err != nil {
		log.Fatal(err)
	}

	address, err := pgsctr.ConnectionString(ctx)
	if err != nil {
		log.Fatal(err)
	}

	dbpool, err := db.DbConnect(ctx, address)
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	rdbctr, err := tcredis.Run(ctx, "redis:7.2")
	if err != nil {
		log.Fatal(err)
	}

	address, err = rdbctr.Endpoint(ctx, "")
	if err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: address})

	tc := &TestContainer{pgs: pgsctr, rdb: rdbctr}

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

	return tc, conf
}

func TeardownTest(ctx context.Context, tc *TestContainer, conf config.Config) {
	conf.Dbpool.Close()
	if err := testcontainers.TerminateContainer(tc.pgs); err != nil {
		log.Printf("failed to terminate container: %s", err)
	}

	if err := testcontainers.TerminateContainer(tc.rdb); err != nil {
		log.Printf("failed to terminate container: %s", err)
	}
}
