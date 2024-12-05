package main

import (
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
	dbpool, err := DbConnect(os.Getenv("PGDATABASE"))
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
