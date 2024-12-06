package main

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var dbpool *pgxpool.Pool

func TestMain(m *testing.M) {
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
	dbpool, err = DbConnect(testdb)

	if err != nil {
		log.Fatalf("%v", err)
	}
	exitCode := m.Run()

	dbpool.Close()

	os.Exit(exitCode)
}

func tableExists(tablename string) (bool, error) {
	var tableExists bool
	err := dbpool.QueryRow(context.Background(),
		"select exists (select from pg_tables where tablename = $1);", tablename).Scan(&tableExists)

	return tableExists, err

}

func TestTables(t *testing.T) {
	ok, err := tableExists("peers")

	if err != nil {
		t.Fatalf("%v", err)
	}

	if !ok {
		t.Fatalf("peers table does not exist")
	}

	_, err = dbpool.Exec(context.Background(), "drop table peers;")
	if err != nil {
		t.Fatalf("%v", err)
	}

	ok, err = tableExists("peers")

	if err != nil {
		t.Fatalf("%v", err)
	}

	if ok {
		t.Fatalf("peers table exists after drop")
	}
}
