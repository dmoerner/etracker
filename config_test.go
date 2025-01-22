package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
)

const defaultAPIKey = "testauthorizationkey"

var defaultAlgorithm = PeersForAnnounces

func buildTestConfig(algorithm PeeringAlgorithm, authorization string) Config {
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

	dbpool, err := DbConnect(os.Getenv("PGDATABASE") + "_test")
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	// Although infohashes table normally persists, for testing it should be
	// recreated each time.
	_, err = dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS infohashes CASCADE;
		`)
	if err != nil {
		log.Fatalf("Unable to clean up old infohashes table")
	}

	err = DbInitialize(dbpool)
	if err != nil {
		log.Fatalf("Unable to initialize DB: %v", err)
	}

	for _, v := range allowedInfoHashes {
		_, err = dbpool.Exec(context.Background(), `
			INSERT INTO infohashes (info_hash, name) VALUES ($1, $2);
			`,
			v,
			string(v))
		if err != nil {
			log.Fatalf("Unable to insert test allowed infohashes: %v", err)
		}
	}

	config := Config{
		algorithm:     algorithm,
		authorization: authorization,
		dbpool:        dbpool,
	}

	return config
}
