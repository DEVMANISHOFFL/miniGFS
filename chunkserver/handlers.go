package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type copyChunkReq struct {
	ChunkID string `json:"chunk_id"`
	Target  string `json:"target"` // "localhost:9002"
}

type receiveChunkReq struct {
	ChunkID string `json:"chunk_id"`
	Data    []byte `json:"data"`
}

type genericResp struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

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

// copyChunkHandler: source chunkserver reads local chunk and posts to target's /receive_chunk.
func copyChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req copyChunkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ChunkID == "" || req.Target == "" {
		http.Error(w, "chunk_id and target required", http.StatusBadRequest)
		return
	}

	// read local chunk file
	filename := "data/" + req.ChunkID + ".bin"
	data, err := os.ReadFile(filename)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: "local chunk not found"})
		return
	}

	// build receive payload
	recv := receiveChunkReq{
		ChunkID: req.ChunkID,
		Data:    data,
	}
	b, _ := json.Marshal(recv)
	url := fmt.Sprintf("http://%s/receive_chunk", req.Target)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: string(respBody)})
		return
	}

	// OK
	json.NewEncoder(w).Encode(genericResp{Status: "ok"})
}

// receiveChunkHandler: accept chunk bytes and write locally.
func receiveChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req receiveChunkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ChunkID == "" {
		http.Error(w, "chunk_id required", http.StatusBadRequest)
		return
	}

	filename := "data/" + req.ChunkID + ".bin"
	if err := os.WriteFile(filename, req.Data, 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: "failed to write chunk"})
		return
	}

	json.NewEncoder(w).Encode(genericResp{Status: "ok"})
}
