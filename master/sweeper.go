package main

import (
	"log"
	"time"
)

func sweeper() {
	for {
		time.Sleep(sweepInterval)
		now := time.Now()

		mu.Lock()
		for id, cs := range chunkServers {
			if cs.lastSeen.IsZero() {
				continue
			}

			if cs.Alive && now.Sub(cs.lastSeen) > heartbeatTimeout {
				cs.Alive = false
				log.Printf("master: detected DEAD chunkserver : %s (lastSeen=%s)",
					id, cs.lastSeen.Format(time.RFC3339))
			}
		}
		mu.Unlock()
	}
}
