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
	if req.File == "" || req.SizeBytes <= 0 {
		http.Error(w, "file and positive size_bytes required", http.StatusBadRequest)
		return
	}

	resp, err := allocateChunks(req.File, req.SizeBytes)
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

// allocateChunks: create ceil(size / ChunkSize) chunk metas and assign replicas per chunk
func allocateChunks(file string, sizeBytes int64) (*AllocateResponse, error) {
	mu.Lock()
	defer mu.Unlock()

	// collect alive nodes
	var alive []string
	for id, cs := range chunkServers {
		if cs.Alive {
			alive = append(alive, id)
		}
	}
	if len(alive) < replicationFactor {
		return nil, fmt.Errorf("not enough alive chunk-servers: have %d, need %d", len(alive), replicationFactor)
	}

	// compute number of chunks
	num := int((sizeBytes + ChunkSize - 1) / ChunkSize)
	if num <= 0 {
		num = 1
	}

	// get or create file metadata
	fm, exist := files[file]
	if !exist {
		fm = &FileMeta{Name: file}
		files[file] = fm
	}

	chunkIDs := make([]string, 0, num)
	locations := make([][]string, 0, num)

	for i := 0; i < num; i++ {
		index := len(fm.Chunks)
		chunkID := fmt.Sprintf("%s_%d", file, index)

		// choose replicas: simple round-robin slice of alive servers
		// rotate so chunk placement spreads across nodes
		start := index % len(alive)
		replicas := make([]string, 0, replicationFactor)
		for j := 0; j < replicationFactor; j++ {
			replicas = append(replicas, alive[(start+j)%len(alive)])
		}

		cm := &ChunkMeta{
			ID:       chunkID,
			FileName: file,
			Index:    index,
			Replicas: replicas,
		}

		chunks[chunkID] = cm
		fm.Chunks = append(fm.Chunks, chunkID)

		chunkIDs = append(chunkIDs, chunkID)
		// copy replicas slice
		locCopy := make([]string, len(replicas))
		copy(locCopy, replicas)
		locations = append(locations, locCopy)
	}

	appendOpLog("allocate", map[string]any{
		"file":   file,
		"chunks": chunkIDs,
	})

	return &AllocateResponse{
		ChunkIDs:  chunkIDs,
		Locations: locations,
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

func clusterInfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	mu.Lock()
	defer mu.Unlock()

	resp := map[string]any{
		"chunkservers": chunkServers,
		"files":        files,
		"chunks":       chunks,
	}

	json.NewEncoder(w).Encode(resp)
}
