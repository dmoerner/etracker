package testutils

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func tableExists(ctx context.Context, dbpool *pgxpool.Pool, tablename string) (bool, error) {
	var tableExists bool
	err := dbpool.QueryRow(ctx, `
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
	ctx := context.Background()
	tc, conf := BuildTestConfig(ctx, nil, DefaultAPIKey)
	defer TeardownTest(ctx, tc, conf)

	tables := []string{"announces", "infohashes", "peers"}

	for _, table := range tables {
		ok, err := tableExists(ctx, conf.Dbpool, table)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if !ok {
			t.Fatalf("%s table does not exist", table)
		}

		_, err = conf.Dbpool.Exec(ctx, "DROP TABLE IF EXISTS "+table+" CASCADE")
		if err != nil {
			t.Fatalf("%v", err)
		}

		ok, err = tableExists(ctx, conf.Dbpool, table)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if ok {
			t.Fatalf("%s table exists after drop", table)
		}

	}
}
