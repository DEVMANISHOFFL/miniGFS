package main

import (
	"net/http"
)

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			return
		}

		h.ServeHTTP(w, r)
	})
}

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
	mux.HandleFunc("/cluster_info", clusterInfoHandler)

	return &http.Server{
		Addr:    ":8080",
		Handler: withCORS(mux),
	}
}
