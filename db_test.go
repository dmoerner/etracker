package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func tableExists(dbpool *pgxpool.Pool, tablename string) (bool, error) {
	var tableExists bool
	err := dbpool.QueryRow(context.Background(),
		"select exists (select from pg_tables where tablename = $1);", tablename).Scan(&tableExists)

	return tableExists, err

}

func TestTables(t *testing.T) {
	testdb := os.Getenv("PGDATABASE") + "_test"
	dbpool, err := DbConnect(testdb)
	defer func() {
		// pgxpool.Pool.Close() returns nothing, but some linters seem to think it does
		// and warn when deferring without a function.
		dbpool.Close()
	}()

	if err != nil {
		t.Fatalf("%v", err)
	}

	ok, err := tableExists(dbpool, "peers")

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

	ok, err = tableExists(dbpool, "peers")

	if err != nil {
		t.Fatalf("%v", err)
	}

	if ok {
		t.Fatalf("peers table exists after drop")
	}
}
