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

type ChunkServerInfo struct {
	Port         string `json:"port"`
	LastSeenUnix int64  `json:"last_seen_unix"`
	Alive        bool   `json:"alive"`
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
	chunkServers = make(map[string]*ChunkServerInfo)
	mu           sync.Mutex
	files        = make(map[string]*FileMeta)
	chunks       = make(map[string]*ChunkMeta)
)

const (
	heartbeatTimeout  = 10 * time.Second
	sweepInterval     = 3 * time.Second
	replicationFactor = 2
)
