package main

import "sync"

var (
	seqMu         sync.Mutex
	lastApplied   = map[string]uint64{} // chunkID -> last seq applied (persist if desired)
	lastCommitted = map[string]uint64{}
	recentReqIDs  = map[string]map[string]bool{} // chunkID -> map[reqID]bool for idempotency (optional)
)
