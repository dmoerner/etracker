package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"

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

	DefaultPort    = 8080
	DefaultTlsPort = 8443
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
	Algorithm     PeeringAlgorithm
	Authorization string
	Dbpool        *pgxpool.Pool
	Port          int
	Tls           TLSConfig
}

type TLSConfig struct {
	CertFile string
	KeyFile  string
	TlsPort  int
}

const AnnounceKeyLength = 30

// GenerateAnnounceKey creates random, AnnounceKeyLength-character hex announce
// keys. This has AnnounceKeyLength / 2 bytes of entropy.
func GenerateAnnounceKey() (string, error) {
	randomBytes := make([]byte, AnnounceKeyLength/2)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("unable to generate new announce key: %w", err)
	}

	return hex.EncodeToString(randomBytes), nil
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

	port := DefaultPort
	if envPort, ok := os.LookupEnv("ETRACKER_PORT"); ok {
		port, err = strconv.Atoi(envPort)
		if err != nil {
			log.Print("Unable to parse ETRACKER_PORT")
			port = DefaultPort
		}
	}

	tlsPort := DefaultTlsPort
	if envTlsPort, ok := os.LookupEnv("ETRACKER_TLS_PORT"); ok {
		tlsPort, err = strconv.Atoi(envTlsPort)
		if err != nil {
			log.Print("Unable to parse ETRACKER_TLS_PORT")
			tlsPort = DefaultTlsPort
		}
	}

	var tls TLSConfig
	certFile, ok1 := os.LookupEnv("ETRACKER_CERTFILE")
	keyFile, ok2 := os.LookupEnv("ETRACKER_KEYFILE")
	if ok1 && ok2 {
		tls.TlsPort = tlsPort
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
		Algorithm:     algorithm,
		Authorization: authorization,
		Dbpool:        dbpool,
		Port:          port,
		Tls:           tls,
	}

	return config
}
