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

var (
	chunkServers = make(map[string]*ChunkServerInfo)
	mu           sync.Mutex
)

const (
	heartbeatTimeout = 10 * time.Second
	sweepInterval    = 3 * time.Second
)
