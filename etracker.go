package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	config := BuildConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/api", APIHandler(config))
	mux.HandleFunc("/announce", PeerHandler(config))

	s := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(mux, time.Second, "Timeout"),
	}

	err := s.ListenAndServe()
	if err != nil {
		log.Fatalf("Unable to start HTTP server: %v", err)
	}
}
