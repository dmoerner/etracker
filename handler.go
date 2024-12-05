package main

import (
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
