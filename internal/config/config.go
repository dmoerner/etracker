package config

import (
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

type Announce struct {
	Peer_id     []byte
	Ip_port     []byte
	Info_hash   []byte
	Numwant     int
	Amount_left int
	Downloaded  int
	Uploaded    int
	Event       Event
}

type PeeringAlgorithm func(config Config, a *Announce) (int, error)

type Config struct {
	Algorithm     PeeringAlgorithm
	Authorization string
	Dbpool        *pgxpool.Pool
	Tls           TLSConfig
}

type TLSConfig struct {
	CertFile string
	KeyFile  string
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

	var tls TLSConfig
	certFile, ok1 := os.LookupEnv("ETRACKER_CERTFILE")
	keyFile, ok2 := os.LookupEnv("ETRACKER_KEYFILE")
	if ok1 && ok2 {
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
		Tls:           tls,
	}

	return config
}
