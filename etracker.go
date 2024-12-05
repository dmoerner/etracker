package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Peer struct {
	ip   net.IP
	port int
}

func main() {
	dbpool, err := dbConnect()
	if err != nil {
		log.Fatalf("%v", err)
	}

	s := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(http.HandlerFunc(PeerHandler(dbpool)), time.Second, "Timeout"),
	}

	err = s.ListenAndServe()
	if err != nil {
		log.Fatalf("Unable to start HTTP server: %v", err)
	}

}

// dbConnect connects to the postgres db and ensures the basic tables are set up.
func dbConnect() (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to db: %w", err)
	}
	defer dbpool.Close()

	_, err = dbpool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS peers (
			id SERIAL PRIMARY KEY,
			peer_id VARCHAR(20) NOT NULL,
			ip_port VARCHAR(6) NOT NULL,
			info_hash VARCHAR(20) NOT NULL,
			last_announce TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)

	if err != nil {
		return nil, fmt.Errorf("unable to create tables: %w", err)
	}

	return dbpool, nil
}

func PeerHandler(dbpool *pgxpool.Pool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		_ = query

		_, err := w.Write(FailureReason("not implemented"))
		if err != nil {
			log.Printf("Error handling connection: %v", err)
		}
	}
}
