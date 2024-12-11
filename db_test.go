package main

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func tableExists(dbpool *pgxpool.Pool, tablename string) (bool, error) {
	var tableExists bool
	err := dbpool.QueryRow(context.Background(),
		"select exists (select from pg_tables where tablename = $1);", tablename).Scan(&tableExists)

	return tableExists, err
}

func TestTables(t *testing.T) {
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

	testdb := os.Getenv("PGDATABASE") + "_test"
	dbpool, err := DbConnect(testdb)
	if err != nil {
		log.Fatalf("%v", err)
	}

	tables := []string{"peers", "infohash_whitelist"}

	for _, table := range tables {
		ok, err := tableExists(dbpool, table)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if !ok {
			t.Fatalf("%s table does not exist", table)
		}

		// Postgres doesn't support parameter placeholders for DROP.
		_, err = dbpool.Exec(context.Background(), "drop table "+table)
		if err != nil {
			t.Fatalf("%v", err)
		}

		ok, err = tableExists(dbpool, table)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if ok {
			t.Fatalf("%s table exists after drop", table)
		}

	}
}
