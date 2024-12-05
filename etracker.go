package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

type Peer struct {
	ip   net.IP
	port int
}

func main() {
	dbpool, err := DbConnect(os.Getenv("PGDATABASE"))
	if err != nil {
		log.Fatalf("Unable to connect to DB: %v", err)
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
