package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	config := BuildConfig()

	s := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(http.HandlerFunc(PeerHandler(config)), time.Second, "Timeout"),
	}

	err := s.ListenAndServe()
	if err != nil {
		log.Fatalf("Unable to start HTTP server: %v", err)
	}
}
