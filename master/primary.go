package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type primaryReq struct {
	ChunkID   string `json:"chunk_id"`
	Preferred string `json:"preferred,omitempty"`
}

type primaryResp struct {
	Primary      string   `json:"primary"`
	LeaseSeconds int64    `json:"lease_seconds"`
	Replicas     []string `json:"replicas"`
	Version      uint64   `json:"version,omitempty"`
}

// /get_primary : returns existing primary if lease valid, else empty primary.
func getPrimaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req primaryReq
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

	if cm.LeaseExpires != 0 && time.Now().Unix() < cm.LeaseExpires {
		json.NewEncoder(w).Encode(primaryResp{
			Primary:      cm.Primary,
			LeaseSeconds: cm.LeaseExpires - time.Now().Unix(),
			Replicas:     cm.Replicas,
			Version:      cm.Version,
		})
		return
	}

	json.NewEncoder(w).Encode(primaryResp{
		Primary:      "",
		LeaseSeconds: 0,
		Replicas:     cm.Replicas,
		Version:      cm.Version,
	})
}

// /assign_primary : master chooses a primary and grants a lease
func assignPrimaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req primaryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	mu.Lock()
	cm, ok := chunks[req.ChunkID]
	if !ok {
		mu.Unlock()
		http.Error(w, "chunk not found", http.StatusNotFound)
		return
	}

	// pick candidate primary: prefer requested, else first alive replica
	chosen := ""
	for _, raddr := range cm.Replicas {
		if req.Preferred != "" && raddr == req.Preferred {
			if cs, ok := chunkServers[raddr]; ok && cs.Alive {
				chosen = raddr
				break
			}
		}
	}

	if chosen == "" {
		for _, raddr := range cm.Replicas {
			if cs, ok := chunkServers[raddr]; ok && cs.Alive {
				chosen = raddr
				break
			}
		}
	}

	if chosen == "" {
		mu.Unlock()
		http.Error(w, "no alive replica to assign primary", http.StatusServiceUnavailable)
		return
	}

	// grant lease
	leaseSec := int64(10)
	cm.Primary = chosen
	cm.LeaseExpires = time.Now().Unix() + leaseSec
	cm.Version += 1
	chunks[req.ChunkID] = cm
	mu.Unlock()

	appendOpLog("assign_primary", map[string]any{
		"chunk_id": req.ChunkID,
		"primary":  chosen,
		"version":  cm.Version,
	})

	log.Printf("master:  assigned primary %s for chunk %s lease %ds", chosen, req.ChunkID, leaseSec)
	json.NewEncoder(w).Encode(primaryResp{
		Primary:      chosen,
		LeaseSeconds: leaseSec,
		Replicas:     cm.Replicas,
		Version:      cm.Version,
	})
}

// /renew_lease : primary calls this periodically to renew its lease
type renewReq struct {
	ChunkID string `json:"chunk_id"`
	Primary string `json:"primary"`
}

type renewResp struct {
	Ok           bool  `json:"ok"`
	LeaseSeconds int64 `json:"lease_seconds"`
}

func renewLeaseHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req renewReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	mu.Lock()
	cm, ok := chunks[req.ChunkID]
	if !ok {
		mu.Unlock()
		http.Error(w, "chunk not found", http.StatusNotFound)
		return
	}
	if cm.Primary != req.Primary {
		mu.Unlock()
		json.NewEncoder(w).Encode(renewResp{Ok: false, LeaseSeconds: 0})
		return
	}
	// renew
	leaseSec := int64(10)
	cm.LeaseExpires = time.Now().Unix() + leaseSec
	chunks[req.ChunkID] = cm
	mu.Unlock()

	json.NewEncoder(w).Encode(renewResp{Ok: true, LeaseSeconds: leaseSec})
}
