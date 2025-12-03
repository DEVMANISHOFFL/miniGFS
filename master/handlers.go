package main

import (
	"encoding/json"
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
