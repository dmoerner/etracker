package testutils

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func tableExists(dbpool *pgxpool.Pool, tablename string) (bool, error) {
	var tableExists bool
	err := dbpool.QueryRow(context.Background(), `
		SELECT
		    EXISTS (
			SELECT
			FROM
			    pg_tables
			WHERE
			    tablename = $1)
		`,
		tablename).Scan(&tableExists)

	return tableExists, err
}

func TestTables(t *testing.T) {
	conf := BuildTestConfig(nil, DefaultAPIKey)
	defer TeardownTest(conf)

	tables := []string{"peers", "infohashes", "peerids"}

	for _, table := range tables {
		ok, err := tableExists(conf.Dbpool, table)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if !ok {
			t.Fatalf("%s table does not exist", table)
		}

		_, err = conf.Dbpool.Exec(context.Background(), "DROP TABLE IF EXISTS "+table+" CASCADE")
		if err != nil {
			t.Fatalf("%v", err)
		}

		ok, err = tableExists(conf.Dbpool, table)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if ok {
			t.Fatalf("%s table exists after drop", table)
		}

	}
}
