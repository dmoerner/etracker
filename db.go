package main

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

// DbInitialize ensures that all required tables are set up. The infohashes
// table persists, but peerids and peers tables should be refreshed on each
// restart.
func DbInitialize(dbpool *pgxpool.Pool) error {
	// infohashes table. Includes info_hash, downloaded key (for use in /scrape),
	// and an optional name, which should match the "name" section in the info
	// section of the torrent file (for use in /scrape and searching), and
	// an optional license (for verification, moderation, and search).
	_, err := dbpool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS infohashes (
			id SERIAL PRIMARY KEY,
			info_hash BYTEA NOT NULL UNIQUE,
			downloaded INTEGER DEFAULT 0 NOT NULL,
			name TEXT NOT NULL,
			license TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_info_hash ON infohashes(info_hash);
		`)
	if err != nil {
		return fmt.Errorf("unable to create infohashes table: %w", err)
	}

	// peerids table. Includes stored score for each peer used to calculate
	// peer quality, and will in the future be extended to include
	// statistics to detect cheaters.
	_, err = dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS peerids CASCADE;

		CREATE TABLE IF NOT EXISTS peerids (
			id SERIAL PRIMARY KEY,
			peer_id BYTEA NOT NULL UNIQUE,
			peer_max_upload INTEGER DEFAULT 0 NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_peer_id ON peerids(peer_id);
		`)
	if err != nil {
		return fmt.Errorf("unable to create peerids table: %w", err)
	}

	// peers table, which includes information from announces.
	// "left" is a reserved word so we use amount_left.
	// For information on the triggers to keep track of announce times, see
	// https://x-team.com/blog/automatic-timestamps-with-postgresql
	_, err = dbpool.Exec(context.Background(), `
		DROP TABLE IF EXISTS peers;

		CREATE TABLE IF NOT EXISTS peers (
			id SERIAL PRIMARY KEY,
			peer_id_id INTEGER references peerids(id),
			ip_port BYTEA NOT NULL,
			info_hash_id INTEGER references infohashes(id),
			amount_left INTEGER NOT NULL,
			downloaded INTEGER NOT NULL,
			uploaded INTEGER NOT NULL,
			event INTEGER,
			last_announce TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (peer_id_id, info_hash_id)
		);



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
		return fmt.Errorf("unable to create peers table: %w", err)
	}

	return nil
}
