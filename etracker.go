package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	if _, ok := os.LookupEnv("DATABASE_URL"); !ok {
		log.Fatal("DATABASE_URL not set in environment")
	}
	if _, ok := os.LookupEnv("PGDATABASE"); !ok {
		log.Fatal("PGDATABASE not set in environment")
	}

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
