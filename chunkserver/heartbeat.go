package main

import (
	"log"
	"time"
)

const (
	masterURL          = "http://localhost:8080"
	heartbeatInterval  = 3 * time.Second
	registerRetryDelay = 2 * time.Second
)

func startRegistration(port string) {
	payload := RegisterRequest{Port: port}

	for {
		err := sendPostJSON(masterURL+"/register", payload)
		if err == nil {
			log.Printf("chunk-server: registered with master")
			return
		}
		log.Printf("register failed: %v | retrying...", err)
		time.Sleep(registerRetryDelay)
	}
}

func startHeartbeats(port string, stopChan <-chan struct{}) {
	ticker := time.NewTicker(heartbeatInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				hb := HeartbeatRequest{Port: port}
				err := sendPostJSON(masterURL+"/heartbeat", hb)
				if err != nil {
					log.Printf("heartbeat error: %v", err)
				} else {
					log.Printf("\033[31mheartbeat sent:\033[0m from %s\n", port)
				}
			case <-stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}
