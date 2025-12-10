package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type copyRequestToSource struct {
	ChunkID string `json:"chunk_id"`
	Target  string `json:"target"` // "localhost:9002" style
}

type copyResponseFromSource struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// repairNode inspects all chunks that referenced deadID and enqueues repairs.
func repairNode(deadID string) {
	log.Printf("master: starting repair for dead node %s", deadID)

	// collect chunk IDs that referenced deadID and are under-replicated
	mu.Lock()
	var toRepair []string
	for chunkID, cm := range chunks {
		foundDead := false
		for _, r := range cm.Replicas {
			if r == deadID {
				foundDead = true
				break
			}
		}
		if !foundDead {
			continue
		}

		aliveCount := 0
		for _, r := range cm.Replicas {
			if cs, ok := chunkServers[r]; ok && cs.Alive {
				aliveCount++
			}
		}

		if aliveCount < replicationFactor {
			toRepair = append(toRepair, chunkID)
		}
	}
	mu.Unlock()

	for _, cid := range toRepair {
		go func(chunkID string) {
			if err := repairChunk(chunkID, deadID); err != nil {
				log.Printf("master: repair failed for %s: %v", chunkID, err)
			}
		}(cid)
	}
}

// repairChunk picks a healthy source and an available target and requests a copy.
func repairChunk(chunkID, deadID string) error {
	// pick source and target under lock-snapshot
	mu.Lock()
	cm, ok := chunks[chunkID]
	if !ok {
		mu.Unlock()
		return fmt.Errorf("chunk not found: %s", chunkID)
	}

	// build set of current replicas and list alive replicas
	replicaSet := make(map[string]bool)
	var aliveReplicas []string
	for _, r := range cm.Replicas {
		replicaSet[r] = true
		if cs, ok := chunkServers[r]; ok && cs.Alive {
			aliveReplicas = append(aliveReplicas, r)
		}
	}

	// find candidate targets (alive chunkservers not already hosting this chunk)
	var candidateTargets []string
	for id, cs := range chunkServers {
		if !cs.Alive {
			continue
		}
		if replicaSet[id] {
			continue
		}
		candidateTargets = append(candidateTargets, id)
	}
	mu.Unlock()

	if len(aliveReplicas) == 0 {
		return fmt.Errorf("no alive source replicas for chunk %s", chunkID)
	}
	if len(candidateTargets) == 0 {
		return fmt.Errorf("no available targets to host new replica for chunk %s", chunkID)
	}

	// simple selection: first alive source, first candidate target
	source := aliveReplicas[0]
	target := candidateTargets[0]

	// ask source to copy to target
	cp := copyRequestToSource{
		ChunkID: chunkID,
		Target:  target,
	}
	body, _ := json.Marshal(cp)
	url := fmt.Sprintf("http://%s/copy_chunk", source)
	client := &http.Client{Timeout: 20 * time.Second}

	// retry loop with small backoff (3 attempts)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("post to source failed: %w", err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		respBody, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("source returned %d: %s", resp.StatusCode, string(respBody))
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		var cr copyResponseFromSource
		if err := json.Unmarshal(respBody, &cr); err != nil {
			lastErr = fmt.Errorf("invalid copy response: %w", err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		if cr.Status != "ok" {
			lastErr = fmt.Errorf("copy failed: %s", cr.Message)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		// success -> update master metadata
		mu.Lock()
		// remove deadID, ensure no dup target
		newReplicas := make([]string, 0, len(cm.Replicas))
		seen := map[string]bool{}
		for _, r := range cm.Replicas {
			if r == deadID {
				continue
			}
			if !seen[r] {
				newReplicas = append(newReplicas, r)
				seen[r] = true
			}
		}
		if !seen[target] {
			newReplicas = append(newReplicas, target)
		}
		cm.Replicas = newReplicas
		chunks[chunkID] = cm
		mu.Unlock()

		log.Printf("master: repaired chunk %s - added replica %s (removed %s)", chunkID, target, deadID)
		return nil
	}

	return lastErr
}
