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

	// infohashes table. Includes info_hash, downloaded key (for use in /scrape),
	// and an optional name, which should match the "name" section in the info
	// section of the torrent file (for use in /scrape and searching), and
	// an optional license (for verification, moderation, and search).
	_, err = dbpool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS infohashes (
			info_hash BYTEA NOT NULL PRIMARY KEY,
			downloaded INTEGER DEFAULT 0 NOT NULL,
			name TEXT,
			license TEXT
		);
		`)
	if err != nil {
		return nil, fmt.Errorf("unable to create infohashes table: %w", err)
	}

	// peerids table. Includes stored score for each peer used to calculate
	// peer quality, and will in the future be extended to include
	// statistics to detect cheaters.
	_, err = dbpool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS peerids (
			peer_id BYTEA NOT NULL PRIMARY KEY,
			peer_max_upload INTEGER DEFAULT 0 NOT NULL
		);
		`)
	if err != nil {
		return nil, fmt.Errorf("unable to create peerids table: %w", err)
	}

	// peers table, which includes information from announces.
	// "left" is a reserved word so we use amount_left.
	// For information on the triggers to keep track of announce times, see
	// https://x-team.com/blog/automatic-timestamps-with-postgresql
	_, err = dbpool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS peers (
			peer_id BYTEA NOT NULL references peerids(peer_id),
			ip_port BYTEA NOT NULL,
			info_hash BYTEA NOT NULL references infohashes(info_hash),
			amount_left INTEGER NOT NULL,
			downloaded INTEGER NOT NULL,
			uploaded INTEGER NOT NULL,
			event INTEGER,
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

	return dbpool, nil
}
