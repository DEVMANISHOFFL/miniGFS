package main

import (
	"log"
	"net/http"
)

func setupServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler(addr))

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

func startHTTPServer(srv *http.Server) {
	go func() {
		log.Printf("chunk-server: starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()
}
