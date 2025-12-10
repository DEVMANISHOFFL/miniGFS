package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

func loadCheckpoint() {
	b, err := os.ReadFile("checkpoint.json")
	if err != nil {
		log.Printf("master: no checkpoint found, starting fresh")
		return
	}

	var cp Checkpoint
	if err := json.Unmarshal(b, &cp); err != nil {
		log.Printf("master: checkpoint corrupt, ignoring: %v", err)
		return
	}

	mu.Lock()
	// replace maps with checkpoint copies
	files = cp.Files
	chunks = cp.Chunks
	chunkServers = cp.ChunkServers
	mu.Unlock()

	log.Printf("master: checkpoint loaded (%d files, %d chunks)", len(files), len(chunks))
}

func replayOpLog() {
	f, err := os.Open("oplog.jsonl")
	if err != nil {
		log.Printf("master: no op-log found")
		return
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	for {
		var entry map[string]any
		if err := dec.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("master: op-log decode error: %v", err)
			break
		}
		applyLogEntry(entry)
	}
	log.Printf("master: op-log replay complete")
}

// applyLogEntry is intentionally conservative: it only applies ops that are safe to idempotently re-run.
// Extend it as you add more operation types.
func applyLogEntry(e map[string]any) {
	ev, _ := e["event"].(string)
	payload := e["payload"]

	switch ev {
	case "allocate":
		// payload expected: {"file": string, "chunks": []string}
		m, ok := payload.(map[string]any)
		if !ok {
			return
		}
		fileName, _ := m["file"].(string)
		chunksIface, _ := m["chunks"].([]any)

		mu.Lock()
		if _, exists := files[fileName]; !exists {
			fm := &FileMeta{Name: fileName}
			for _, ci := range chunksIface {
				if csid, ok := ci.(string); ok {
					fm.Chunks = append(fm.Chunks, csid)
				}
			}
			files[fileName] = fm
		}
		mu.Unlock()

	case "assign_primary":
		// payload expected: {"chunk_id": string, "primary": string, "version": number}
		m, ok := payload.(map[string]any)
		if !ok {
			return
		}
		cid, _ := m["chunk_id"].(string)
		p, _ := m["primary"].(string)
		verFloat, _ := m["version"].(float64)
		ver := uint64(verFloat)

		mu.Lock()
		if cm, ok := chunks[cid]; ok {
			cm.Primary = p
			cm.Version = ver
			chunks[cid] = cm
		}
		mu.Unlock()

	case "repair":
		// payload: {"chunk_id": string, "new_replica": string}
		m, ok := payload.(map[string]any)
		if !ok {
			return
		}
		cid, _ := m["chunk_id"].(string)
		newr, _ := m["new_replica"].(string)
		mu.Lock()
		if cm, ok := chunks[cid]; ok {
			// idempotent add if missing
			found := false
			for _, r := range cm.Replicas {
				if r == newr {
					found = true
					break
				}
			}
			if !found {
				cm.Replicas = append(cm.Replicas, newr)
				chunks[cid] = cm
			}
		}
		mu.Unlock()
	}
}
