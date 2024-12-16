package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func buildTestConfig(algorithm PeeringAlgorithm) Config {
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

	authorization := "testauthorizationkey"

	for _, v := range allowedInfoHashes {
		_, err = dbpool.Exec(context.Background(), `INSERT INTO infohash_allowlist (info_hash, note) VALUES ($1, $2);`, v, "test allowed infohash")
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
