package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// REGISTER HANDLER
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	id := "localhost:" + req.Port

	mu.Lock()
	cs, exists := chunkServers[id]
	if !exists {
		cs = &ChunkServerInfo{Port: req.Port}
		chunkServers[id] = cs
	}
	cs.lastSeen = time.Now()
	cs.LastSeenUnix = cs.lastSeen.Unix()
	cs.Alive = true
	mu.Unlock()

	log.Printf("master: registered chunkserver %s", id)
	w.Write([]byte(`{"status":"ok"}`))
}

// HEARTBEAT HANDLER
func heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	id := "localhost:" + req.Port

	mu.Lock()
	cs, exists := chunkServers[id]
	if !exists {
		cs = &ChunkServerInfo{Port: req.Port}
		chunkServers[id] = cs
	}
	cs.lastSeen = time.Now()
	cs.LastSeenUnix = cs.lastSeen.Unix()
	cs.Alive = true
	mu.Unlock()

	log.Printf("master: heartbeat from %s", id)
	w.Write([]byte(`{"status":"ok"}`))
}

// LIST HANDLER
func listHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	out := make(map[string]ChunkServerInfo)
	for id, cs := range chunkServers {
		out[id] = ChunkServerInfo{
			Port:         cs.Port,
			LastSeenUnix: cs.LastSeenUnix,
			Alive:        cs.Alive,
		}
	}
	mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func allocateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req AllocateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.File == "" {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}

	resp, err := allocateChunks(req.File)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func allocateChunks(file string) (*AllocateResponse, error) {
	mu.Lock()
	defer mu.Unlock()

	// collecting alive chunkservres
	var alive []string
	for id, cs := range chunkServers {
		if cs.Alive {
			alive = append(alive, id)
		}
	}

	if len(alive) < replicationFactor {
		return nil, fmt.Errorf("not enough alive chunk-servers: have %d, need %d", len(alive), replicationFactor)
	}

	// get or create file metadata
	fm, exist := files[file]
	if !exist {
		fm = &FileMeta{
			Name: file,
		}
		files[file] = fm
	}

	// decide the chunk index
	index := len(fm.Chunks)
	chunkID := fmt.Sprintf("%s_%d", file, index)

	// for now, just picking the first N alive servers
	replicas := alive
	if len(replicas) > replicationFactor {
		replicas = replicas[:replicationFactor]
	}

	// save chunk metadata
	cm := &ChunkMeta{
		ID:       chunkID,
		FileName: file,
		Index:    index,
		Replicas: replicas,
	}

	// just a mapping and appending things
	chunks[chunkID] = cm
	fm.Chunks = append(fm.Chunks, chunkID)

	// building response to client
	return &AllocateResponse{
		ChunkID:   chunkID,
		Locations: replicas,
	}, nil
}

func ChunkLocationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	var req ChunkLocationsRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	mu.Lock()
	cm, ok := chunks[req.ChunkID]
	mu.Unlock()

	if !ok {
		http.Error(w, "chunk not found", http.StatusNotFound)
		return
	}

	resp := ChunkLocationsResponse{
		Locations: cm.Replicas,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
