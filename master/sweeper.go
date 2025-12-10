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
		nowUnix := time.Now().Unix()
		for cid, cm := range chunks {
			if cm.LeaseExpires != 0 && nowUnix >= cm.LeaseExpires {
				// lease expired — clear primary so a new one can be assigned
				log.Printf("master: lease expired for chunk %s (primary=%s)", cid, cm.Primary)
				cm.Primary = ""
				cm.LeaseExpires = 0
				chunks[cid] = cm
			}
		}
		for id, cs := range chunkServers {
			if cs.lastSeen.IsZero() {
				continue
			}

			if cs.Alive && now.Sub(cs.lastSeen) > heartbeatTimeout {
				cs.Alive = false
				log.Printf("master: detected DEAD chunkserver : %s (lastSeen=%s)",
					id, cs.lastSeen.Format(time.RFC3339))

				// kick off repairs for chunks that referenced this node
				go repairNode(id)
			}

		}
		mu.Unlock()
	}
}
