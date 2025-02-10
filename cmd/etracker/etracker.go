package main

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/dmoerner/etracker/internal/api"
	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/scrape"
)

// serveFrontend provides the basic routing logic for the SPA.
func serveFrontend(frontendPath string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fs := http.Dir(frontendPath)
		path := filepath.Join(r.URL.Path)

		// Serve static assets, if they exist.
		if _, err := fs.Open(path); err == nil {
			http.FileServer(fs).ServeHTTP(w, r)
			return
		}

		// Route everything else through index.html.
		http.ServeFile(w, r, filepath.Join(frontendPath, "index.html"))
	}
}

func main() {
	conf := config.BuildConfig(handler.DefaultAlgorithm)

	mux := http.NewServeMux()

	mux.HandleFunc("/", serveFrontend("./frontend/dist"))

	api.MuxAPIRoutes(conf, mux)

	mux.HandleFunc("GET /{id}/announce", handler.PeerHandler(conf))
	mux.HandleFunc("GET /{id}/scrape", scrape.ScrapeHandler(conf))

	s := &http.Server{
		Addr:              conf.Hostname,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(mux, time.Second, "Timeout"),
	}

	if conf.Tls != (config.TLSConfig{}) {
		t := &http.Server{
			Addr:              conf.Tls.TlsHostname,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       5 * time.Second,
			Handler:           http.TimeoutHandler(mux, time.Second, "Timeout"),
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
