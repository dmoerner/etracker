package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/dmoerner/etracker/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
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

	DefaultHostname = "localhost"
	DefaultPort     = "8080"
	DefaultTlsPort  = "8443"
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

type PeeringAlgorithm func(config Config, a *Announce) (int, error)

type Config struct {
	Algorithm        PeeringAlgorithm
	Authorization    string
	Dbpool           *pgxpool.Pool
	Hostname         string
	Tls              TLSConfig
	DisableAllowlist bool
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
func GenerateAnnounceKey(conf Config) (string, error) {
	randomBytes := make([]byte, AnnounceKeyLength/2)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("unable to generate new announce key: %w", err)
	}
	key := hex.EncodeToString(randomBytes)

	_, err := conf.Dbpool.Exec(context.Background(), `
			INSERT INTO peerids (announce_key)
			    VALUES ($1)
			`,
		key)
	if err != nil {
		return "", fmt.Errorf("createNSeeders: Unable to insert announce key: %w", err)
	}

	return key, nil
}

func BuildConfig(algorithm PeeringAlgorithm) Config {
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

	// An empty authorization string in the config means the API is forbidden.
	// It is the responsibility of clients who use this struct key to forbid this.
	var authorization string
	authorization, ok := os.LookupEnv("ETRACKER_AUTHORIZATION")
	if !ok {
		log.Print("ETRACKER_AUTHORIZATION not set in environment.")
	}

	disableAllowlist := false
	if envDisableAllowlist, ok := os.LookupEnv("ETRACKER_DISABLE_ALLOWLIST"); ok && envDisableAllowlist == "true" {
		disableAllowlist = true
	}

	hostname := DefaultHostname
	if envHostname, ok := os.LookupEnv("ETRACKER_HOSTNAME"); ok {
		hostname = envHostname
	}

	port := DefaultPort
	if envPort, ok := os.LookupEnv("ETRACKER_PORT"); ok {
		port = envPort
	}

	tlsPort := DefaultTlsPort
	if envTlsPort, ok := os.LookupEnv("ETRACKER_TLS_PORT"); ok {
		tlsPort = envTlsPort
	}

	var tls TLSConfig
	certFile, ok1 := os.LookupEnv("ETRACKER_CERTFILE")
	keyFile, ok2 := os.LookupEnv("ETRACKER_KEYFILE")
	if ok1 && ok2 {
		tls.TlsHostname = fmt.Sprintf("%s:%s", hostname, tlsPort)
		tls.CertFile = certFile
		tls.KeyFile = keyFile
		log.Print("TLS tracker enabled.")
	}

	dbpool, err := db.DbConnect()
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	err = db.DbInitialize(dbpool)
	if err != nil {
		log.Fatalf("Unable to initialize DB: %v", err)
	}

	config := Config{
		Algorithm:        algorithm,
		Authorization:    authorization,
		Dbpool:           dbpool,
		Hostname:         fmt.Sprintf("%s:%s", hostname, port),
		Tls:              tls,
		DisableAllowlist: disableAllowlist,
	}

	return config
}
