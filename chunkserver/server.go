package main

import (
	"log"
	"net/http"
)

func setupServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler(addr))
	mux.HandleFunc("/write_chunk", writeChunkHandler)
	mux.HandleFunc("/read_chunk", readChunkHandler)
	mux.HandleFunc("/copy_chunk", copyChunkHandler)
	mux.HandleFunc("/receive_chunk", receiveChunkHandler)

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

func startHTTPServer(srv *http.Server) {
	go func() {
		log.Printf("chunk-server: starting on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("chunk-server: ListenAndServe error: %v", err)
		}
	}()
}
