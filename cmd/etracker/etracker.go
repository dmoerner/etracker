package main

import (
	"log"
	"net/http"
	"time"

	"github.com/dmoerner/etracker/internal/api"
	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/scrape"
	"github.com/dmoerner/etracker/internal/web"
)

func main() {
	conf := config.BuildConfig(handler.DefaultAlgorithm)

	mux := http.NewServeMux()
	mux.HandleFunc("/", web.WebHandler(conf))
	mux.HandleFunc("/api", api.APIHandler(conf))
	mux.HandleFunc("/announce", handler.PeerHandler(conf))
	mux.HandleFunc("/scrape", scrape.ScrapeHandler(conf))

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
