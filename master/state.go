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
	File      string `json:"file"`
	SizeBytes int64  `json:"size_bytes"`
}

type AllocateResponse struct {
	ChunkIDs  []string   `json:"chunk_ids"`
	Locations [][]string `json:"locations"`
}

type ChunkLocationsResponse struct {
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
	ID           string   `json:"id"`
	FileName     string   `json:"file_name"`
	Index        int      `json:"index"`
	Replicas     []string `json:"replicas"`
	Primary      string   `json:"primary,omitempty"`
	LeaseExpires int64    `json:"lease_expires_unix"`
	Version      uint64   `json:"version,omitempty"`
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
	ChunkSize         = 4 * 1024 * 1024
)

func (c *ChunkMeta) LeaseValid() bool {
	if c.LeaseExpires == 0 {
		return false
	}
	return time.Now().Unix() < c.LeaseExpires
}
