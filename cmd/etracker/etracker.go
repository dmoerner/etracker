package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dmoerner/etracker/internal/api"
	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/frontendapi"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/scrape"
)

func main() {
	conf := config.BuildConfig(handler.DefaultAlgorithm)

	frontendMux := http.NewServeMux()
	frontendMux.HandleFunc("/frontendapi/stats", frontendapi.StatsHandler(conf))
	frontendMux.HandleFunc("/frontendapi/generate", frontendapi.GenerateHandler(conf))
	frontendMux.HandleFunc("/frontendapi/infohashes", frontendapi.InfohashesHandler(conf))

	f := &http.Server{
		Addr:              "localhost:9000",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(frontendMux, time.Second, "Timeout"),
	}
	go func() {
		if err := f.ListenAndServe(); err != nil {
			log.Fatalf("Unable to start frontend endpoint: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./frontend/dist")))
	// mux.HandleFunc("/allowlist", web.AllowlistHandler(conf))
	// Use improved routing in Go 1.22.
	mux.HandleFunc("GET /{id}/announce", handler.PeerHandler(conf))
	mux.HandleFunc("GET /{id}/scrape", scrape.ScrapeHandler(conf))

	s := &http.Server{
		Addr:              fmt.Sprintf(":%d", conf.Port),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(mux, time.Second, "Timeout"),
	}

	if conf.Tls != (config.TLSConfig{}) {
		tlsMux := http.NewServeMux()
		tlsMux.Handle("/", http.FileServer(http.Dir("./frontend/dist")))
		tlsMux.HandleFunc("/api", api.APIHandler(conf))
		tlsMux.HandleFunc("GET /{id}/announce", handler.PeerHandler(conf))
		tlsMux.HandleFunc("GET /{id}/scrape", scrape.ScrapeHandler(conf))

		t := &http.Server{
			Addr:              fmt.Sprintf(":%d", conf.Tls.TlsPort),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       5 * time.Second,
			Handler:           http.TimeoutHandler(tlsMux, time.Second, "Timeout"),
		}

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
