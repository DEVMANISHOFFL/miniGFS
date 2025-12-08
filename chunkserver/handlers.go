package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type WriteChunkRequest struct {
	ChunkID string `json:"chunk_id"`
	Data    []byte `json:"data"`
}

func helloHandler(serverAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "chunk-server %s online\n", serverAddr)
	}
}

func writeChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req WriteChunkRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ChunkID == "" {
		http.Error(w, "chunk_id required", http.StatusBadRequest)
		return
	}

	// saving file to disk
	filename := "data/" + req.ChunkID + ".bin"
	if err := os.WriteFile(filename, req.Data, 0644); err != nil {
		http.Error(w, "failed to write chunk", http.StatusInternalServerError)
		return
	}

	log.Printf("stored chunk %s", req.ChunkID)
	w.WriteHeader(http.StatusOK)
}

func readChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	chunkID := r.URL.Query().Get("chunk_id")
	if chunkID == "" {
		http.Error(w, "chunk_id required", http.StatusBadRequest)
		return
	}

	filename := "data/" + chunkID + ".bin"
	data, err := os.ReadFile(filename)
	if err != nil {
		http.Error(w, "chunk not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
