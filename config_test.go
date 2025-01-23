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
	if _, ok := os.LookupEnv("PGUSER"); !ok {
		log.Fatal("DATABASE_URL not set in environment")
	}
	if _, ok := os.LookupEnv("PGPASSWORD"); !ok {
		log.Fatal("DATABASE_URL not set in environment")
	}
	os.Setenv("PGDATABASE", "etracker_test")
	os.Setenv("PGPORT", "5432")
	os.Setenv("PGHOST", "localhost")

	dbpool, err := DbConnect()
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

	// // Postgres does not support create database if not exists, so we use a subquery.
	// _, err = dbpool.Exec(context.Background(),
	// 	`
	// 	SELECT 'CREATE DATABASE etracker_test'
	// 	WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'etracker_test')\gexec
	// 	`)
	// if err != nil {
	// 	log.Fatalf("unable to create test database: %v", err)
	// }
	//
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

func teardownTest(config Config) {
	_, err := config.dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS peers;
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = config.dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS infohashes;
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}
	_, err = config.dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS peerids;
		`)
	if err != nil {
		log.Fatalf("error dropping table on db cleanup: %v", err)
	}

	config.dbpool.Close()
}
