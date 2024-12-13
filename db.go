package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DbConnect connects to the postgres db and ensures the basic tables are set up.
func DbConnect(db string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db_url := os.Getenv("DATABASE_URL") + "/" + db

	dbpool, err := pgxpool.New(ctx, db_url)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db: %w", err)
	}

	// cf. https://x-team.com/blog/automatic-timestamps-with-postgresql
	// "left" is a reserved word so we use amount_left.
	_, err = dbpool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS peers (
			peer_id BYTEA NOT NULL,
			ip_port BYTEA NOT NULL,
			info_hash BYTEA NOT NULL,
			amount_left INTEGER NOT NULL,
			last_announce TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (peer_id, info_hash)
		);

		CREATE INDEX IF NOT EXISTS idx_info_hash ON peers(info_hash);


		CREATE OR REPLACE FUNCTION trigger_set_timestamp()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.last_announce = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		CREATE OR REPLACE TRIGGER set_timestamp
		BEFORE UPDATE ON peers
		FOR EACH ROW
		EXECUTE PROCEDURE trigger_set_timestamp();
	`)
	if err != nil {
		return nil, fmt.Errorf("unable to create peers table: %w", err)
	}

	_, err = dbpool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS infohash_allowlist (
			info_hash BYTEA NOT NULL PRIMARY KEY,
			note TEXT
		);
		`)
	if err != nil {
		return nil, fmt.Errorf("unable to create infohash_allowlist table: %w", err)
	}

	return dbpool, nil
}
