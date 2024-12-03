package main

import (
	"log"
	"net"
	"net/http"
	"time"
)

type Peer struct {
	ip   net.IP
	port int
}

func main() {
	s := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 500 * time.Millisecond,
		ReadTimeout:       500 * time.Millisecond,
		Handler:           http.TimeoutHandler(ClientHandler{}, time.Second, "Timeout"),
	}

	err := s.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}

type ClientHandler struct{}

func (ch ClientHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := w.Write(FailureReason("not implemented"))
	if err != nil {
		log.Print(err)
	}
}
