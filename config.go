package main

import (
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type PeeringAlgorithm func(config Config, a *Announce) (int, error)

type Config struct {
	dbpool    *pgxpool.Pool
	algorithm PeeringAlgorithm
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

	algorithm := peersToGive

	dbpool, err := DbConnect(os.Getenv("PGDATABASE"))
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
	}

	config := Config{
		algorithm: algorithm,
		dbpool:    dbpool,
	}

	return config
}
