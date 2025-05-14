package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/dmoerner/etracker/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type Event int

const (
	_ Event = iota
	Started
	Stopped
	Completed
)

const (
	Interval      = 2700 // 45 minutes
	StaleInterval = 2 * Interval
	MinInterval   = 30 // 30 seconds

	DefaultBackendPort      = 3000
	DefaultFrontendHostname = "localhost"
)

type Announce struct {
	Announce_key string
	Ip_port      []byte
	Info_hash    []byte
	Numwant      int
	Amount_left  int
	Downloaded   int
	Uploaded     int
	Event        Event
}

type PeeringAlgorithm func(ctx context.Context, config Config, a *Announce) (int, error)

type Config struct {
	Algorithm        PeeringAlgorithm
	Authorization    string
	Dbpool           *pgxpool.Pool
	Rdb              *redis.Client
	BackendPort      int
	DisableAllowlist bool
	FrontendHostname string
}

type TLSConfig struct {
	CertFile    string
	KeyFile     string
	TlsHostname string
}

const AnnounceKeyLength = 30

// GenerateAnnounceKey creates random, AnnounceKeyLength-character hex announce
// keys. This has AnnounceKeyLength / 2 bytes of entropy. With adequate
// AnnounceKeyLength we do not need to check for collisions. We also write the
// new key to the database.
func GenerateAnnounceKey(ctx context.Context, conf Config) (string, error) {
	randomBytes := make([]byte, AnnounceKeyLength/2)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("unable to generate new announce key: %w", err)
	}
	key := hex.EncodeToString(randomBytes)

	_, err := conf.Dbpool.Exec(ctx, `
			INSERT INTO peers (announce_key)
			    VALUES ($1)
			`,
		key)
	if err != nil {
		return "", fmt.Errorf("createNSeeders: Unable to insert announce key: %w", err)
	}

	return key, nil
}

func BuildConfig(ctx context.Context, algorithm PeeringAlgorithm) Config {
	err := godotenv.Load()
	if err != nil {
		log.Print("Unable to load .env file, will use existing environment for configuration variables.")
	}
	if _, ok := os.LookupEnv("PGHOST"); !ok {
		log.Fatal("PGHOST not set in environment.")
	}
	if _, ok := os.LookupEnv("PGDATABASE"); !ok {
		log.Fatal("PGDATABASE not set in environment.")
	}
	if _, ok := os.LookupEnv("PGUSER"); !ok {
		log.Fatal("PGUSER not set in environment.")
	}
	if _, ok := os.LookupEnv("PGPASSWORD"); !ok {
		log.Fatal("PGPASSWORD not set in environment.")
	}

	redis_password, ok := os.LookupEnv("ETRACKER_REDIS")
	if !ok {
		log.Fatal("ETRACKER_REDIS not set in environment.")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: redis_password,
		DB:       0, // Production DB
	})

	// An empty authorization string in the config means the API is forbidden.
	// It is the responsibility of functions who use this struct key to forbid this.
	authorization, ok := os.LookupEnv("ETRACKER_AUTHORIZATION")
	if !ok {
		log.Print("ETRACKER_AUTHORIZATION not set in environment.")
	}

	disableAllowlist := false
	if envDisableAllowlist, ok := os.LookupEnv("ETRACKER_DISABLE_ALLOWLIST"); ok && envDisableAllowlist == "true" {
		disableAllowlist = true
	}

	backendPort := DefaultBackendPort
	if envBackendPort, ok := os.LookupEnv("ETRACKER_BACKEND_PORT"); ok {
		if intBackendPort, err := strconv.Atoi(envBackendPort); err != nil {
			backendPort = intBackendPort
		}
	}

	frontendHostname := DefaultFrontendHostname
	if envFrontendHostname, ok := os.LookupEnv("ETRACKER_FRONTEND_HOSTNAME"); ok {
		frontendHostname = envFrontendHostname
	}

	dbpool, err := db.DbConnect(ctx, "")
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	err = db.DbInitialize(ctx, dbpool)
	if err != nil {
		log.Fatalf("Unable to initialize DB: %v", err)
	}

	config := Config{
		Algorithm:        algorithm,
		Authorization:    authorization,
		Dbpool:           dbpool,
		Rdb:              rdb,
		BackendPort:      backendPort,
		DisableAllowlist: disableAllowlist,
		FrontendHostname: frontendHostname,
	}

	return config
}
