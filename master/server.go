package main

import (
	"net/http"
)

func setupServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/heartbeat", heartbeatHandler)
	mux.HandleFunc("/list", listHandler)

	return &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
}
