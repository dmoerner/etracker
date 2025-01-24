package main

import (
	"etracker/internal/api"
	"etracker/internal/config"
	"etracker/internal/handler"
	"etracker/internal/scrape"
	"log"
	"net/http"
	"time"
)

func main() {
	conf := config.BuildConfig(handler.DefaultAlgorithm)

	mux := http.NewServeMux()
	mux.HandleFunc("/api", api.APIHandler(conf))
	mux.HandleFunc("/announce", handler.PeerHandler(conf))
	mux.HandleFunc("/scrape", scrape.ScrapeHandler(conf))

	// Rumor has it that some firewalls will block traffic if there is not
	// a 200 response from the root path.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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

	if conf.Tls != (config.TLSConfig{}) {
		go func() {
			if err := t.ListenAndServeTLS(conf.Tls.CertFile, conf.Tls.KeyFile); err != nil {
				log.Fatalf("Unable to start HTTPS server: %v", err)
			}
		}()
	}

	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("Unable to start HTTP server: %v", err)
	}
}
