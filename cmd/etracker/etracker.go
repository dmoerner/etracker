package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/dmoerner/etracker/internal/api"
	"github.com/dmoerner/etracker/internal/config"
	"github.com/dmoerner/etracker/internal/handler"
	"github.com/dmoerner/etracker/internal/prune"
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
	ctx := context.Background()

	conf := config.BuildConfig(ctx, handler.DefaultAlgorithm)

	// On startup, prune unused announce keys. This cannot be done
	// in the config package because it would be a circular dependency.
	err := prune.PruneAnnounceKeys(ctx, conf)
	if err != nil {
		log.Fatalf("Error pruning unused announce keys: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", serveFrontend("./frontend/dist"))

	api.MuxAPIRoutes(ctx, conf, mux)

	mux.HandleFunc("GET /{id}/announce", handler.PeerHandler(ctx, conf))
	mux.HandleFunc("GET /{id}/scrape", scrape.ScrapeHandler(ctx, conf))

	s := &http.Server{
		Addr:              fmt.Sprintf("localhost:%d", conf.BackendPort),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		Handler:           http.TimeoutHandler(mux, time.Second, "Timeout"),
	}

	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("Unable to start HTTP server: %v", err)
	}

	// Prune old announce keys and announces on a timer.
	pruneErrCh := make(chan error)
	prune.PruneTimer(ctx, conf, pruneErrCh)

	err = <-pruneErrCh
	if err != nil {
		log.Fatalf("Error while pruning on timer: %v", err)
	}
}
