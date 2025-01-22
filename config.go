package main

import (
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type PeeringAlgorithm func(config Config, a *Announce) (int, error)

type Config struct {
	algorithm     PeeringAlgorithm
	authorization string
	dbpool        *pgxpool.Pool
	tls           tlsConfig
}

type tlsConfig struct {
	certFile string
	keyFile  string
}

func BuildConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	if _, ok := os.LookupEnv("DATABASE_URL"); !ok {
		log.Fatal("DATABASE_URL not set in environment")
	}
	if _, ok := os.LookupEnv("PGDATABASE"); !ok {
		log.Fatal("PGDATABASE not set in environment")
	}

	// An empty authorization string in the config means the API is forbidden.
	// It is the responsibility of clients who use this struct key to forbid this.
	var authorization string
	authorization, ok := os.LookupEnv("ETRACKER_AUTHORIZATION")
	if !ok {
		log.Print("ETRACKER_AUTHORIZATION not set in environment")
	}

	algorithm := PeersForGoodSeeds

	var tls tlsConfig
	certFile, ok1 := os.LookupEnv("ETRACKER_CERTFILE")
	keyFile, ok2 := os.LookupEnv("ETRACKER_KEYFILE")
	if ok1 && ok2 {
		tls.certFile = certFile
		tls.keyFile = keyFile
	}

	dbpool, err := DbConnect(os.Getenv("PGDATABASE"))
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	err = DbInitialize(dbpool)
	if err != nil {
		log.Fatalf("Unable to initialize DB: %v", err)
	}

	config := Config{
		algorithm:     algorithm,
		authorization: authorization,
		dbpool:        dbpool,
		tls:           tls,
	}

	return config
}
