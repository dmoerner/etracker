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
	_, err = dbpool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS peers (
			id SERIAL PRIMARY KEY,
			peer_id VARCHAR(20) NOT NULL,
			ip_port VARCHAR(6) NOT NULL,
			info_hash VARCHAR(20) NOT NULL,
			last_announce TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
		return nil, fmt.Errorf("unable to create tables: %w", err)
	}

	return dbpool, nil
}
