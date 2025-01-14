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
	mux.HandleFunc("/scrape", ScrapeHandler(config))

	s := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(mux, time.Second, "Timeout"),
	}

	t := &http.Server{
		Addr:              ":8443",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(mux, time.Second, "Timeout"),
	}

	if config.tls != (tlsConfig{}) {
		go func() {
			if err := t.ListenAndServeTLS(config.tls.certFile, config.tls.keyFile); err != nil {
				log.Fatalf("Unable to start HTTPS server: %v", err)
			}
		}()
	}

	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("Unable to start HTTP server: %v", err)
	}
}
