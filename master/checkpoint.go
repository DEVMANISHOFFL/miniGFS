package main

import (
	"encoding/json"
	"log"
	"os"
)

type Checkpoint struct {
	Files        map[string]*FileMeta        `json:"files"`
	Chunks       map[string]*ChunkMeta       `json:"chunks"`
	ChunkServers map[string]*ChunkServerInfo `json:"chunk_servers"`
}

func writeCheckpoint() {
	mu.Lock()
	defer mu.Unlock()

	cp := Checkpoint{
		Files:        files,
		Chunks:       chunks,
		ChunkServers: chunkServers,
	}

	b, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		log.Printf("master: checkpoint marshal error: %v", err)
		return
	}

	err = os.WriteFile("checkpoint.json", b, 0644)
	if err != nil {
		log.Printf("master: checkpoint write error: %v", err)
		return
	}

	log.Printf("master: checkpoint saved (%d files, %d chunks)", len(files), len(chunks))
}
