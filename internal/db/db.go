package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DbConnect connects to the postgres db.
func DbConnect() (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("unable to get db config from environment: %w", err)
	}

	dbpool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db: %w", err)
	}

	return dbpool, nil
}

// DbInitialize ensures that all required tables are set up.
func DbInitialize(dbpool *pgxpool.Pool) error {
	// infohashes table. Includes info_hash, downloaded key (for use in /scrape),
	// and an optional name, which should match the "name" section in the info
	// section of the torrent file (for use in /scrape and searching), and
	// an optional license (for verification, moderation, and search).
	_, err := dbpool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS infohashes (
		    id serial PRIMARY KEY,
		    info_hash bytea NOT NULL UNIQUE,
		    downloaded integer DEFAULT 0 NOT NULL,
		    name text NOT NULL,
		    license text
		);

		CREATE INDEX IF NOT EXISTS idx_info_hash ON infohashes (info_hash);
		`)
	if err != nil {
		return fmt.Errorf("unable to create infohashes table: %w", err)
	}

	// peers table. Includes stored score for each peer used to calculate
	// peer quality, and will in the future be extended to include
	// statistics to detect cheaters. At the moment, the peer_max_upload
	// key is written but not read.
	_, err = dbpool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS peers (
		    id serial PRIMARY KEY,
		    announce_key text NOT NULL UNIQUE,
		    snatched integer DEFAULT 0 NOT NULL,
		    downloaded integer DEFAULT 0 NOT NULL,
		    uploaded integer DEFAULT 0 NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_announce_key ON peers (announce_key);
		`)
	if err != nil {
		return fmt.Errorf("unable to create peers table: %w", err)
	}

	// announces table, which includes information from announces.
	// "left" is a reserved word so we use amount_left.
	// For information on the triggers to keep track of announce times, see
	// https://x-team.com/blog/automatic-timestamps-with-postgresql
	_, err = dbpool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS announces (
		    id serial PRIMARY KEY,
		    peers_id integer references peers(id),
		    ip_port bytea NOT NULL,
		    info_hash_id integer REFERENCES infohashes (id),
		    amount_left integer NOT NULL,
		    downloaded integer NOT NULL,
		    uploaded integer NOT NULL,
		    event INTEGER,
		    last_announce timestamptz NOT NULL DEFAULT NOW(),
		    UNIQUE (peers_id, info_hash_id)
		);

		CREATE OR REPLACE FUNCTION trigger_set_timestamp ()
		    RETURNS TRIGGER
		    AS $$
		BEGIN
		    NEW.last_announce = NOW();
		    RETURN NEW;
		END;
		$$
		LANGUAGE plpgsql;

		CREATE OR REPLACE TRIGGER set_timestamp
		    BEFORE UPDATE ON announces
		    FOR EACH ROW
		    EXECUTE PROCEDURE trigger_set_timestamp ();
		`)
	if err != nil {
		return fmt.Errorf("unable to create announces table: %w", err)
	}

	return nil
}
