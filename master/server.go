package main

import (
	"net/http"
)

func setupServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/heartbeat", heartbeatHandler)
	mux.HandleFunc("/list", listHandler)
	mux.HandleFunc("/allocate", allocateHandler)
	mux.HandleFunc("/chunk_locations", ChunkLocationsHandler)
	mux.HandleFunc("/get_primary", getPrimaryHandler)
	mux.HandleFunc("/assign_primary", assignPrimaryHandler)
	mux.HandleFunc("/renew_lease", renewLeaseHandler)

	return &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
}
