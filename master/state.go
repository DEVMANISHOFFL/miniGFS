package main

import (
	"sync"
	"time"
)

type RegisterRequest struct {
	Port string `json:"port"`
}

type HeartbeatRequest struct {
	Port string `json:"port"`
}

type ChunkLocationsRequest struct {
	ChunkID string `json:"chunk_id"`
}

type AllocateRequest struct {
	File string `json:"file"`
}

type ChunkLocationsResponse struct {
	Locations []string `json:"locations"`
}

type AllocateResponse struct {
	ChunkID   string   `json:"chunk_id"`
	Locations []string `json:"locations"`
}

type ChunkServerInfo struct {
	Port         string `json:"port"`
	Alive        bool   `json:"alive"`
	LastSeenUnix int64  `json:"last_seen_unix"`
	lastSeen     time.Time
}

type FileMeta struct {
	Name   string   `json:"name"`
	Chunks []string `json:"chunks"`
}

type ChunkMeta struct {
	ID       string   `json:"id"`
	FileName string   `json:"file_name"`
	Index    int      `json:"index"`
	Replicas []string `json:"replicas"`
}

var (
	mu           sync.Mutex
	chunkServers = make(map[string]*ChunkServerInfo)
	files        = make(map[string]*FileMeta)
	chunks       = make(map[string]*ChunkMeta)
)

const (
	heartbeatTimeout  = 10 * time.Second
	sweepInterval     = 3 * time.Second
	replicationFactor = 2
)
