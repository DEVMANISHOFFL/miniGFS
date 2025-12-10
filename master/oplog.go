package main

import (
	"encoding/json"
	"log"
	"os"
)

func appendOpLog(event string, payload any) {
	entry := map[string]any{
		"event":   event,
		"payload": payload,
	}

	b, _ := json.Marshal(entry)

	f, err := os.OpenFile("oplog.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("master: op-log open error: %v", err)
		return
	}
	defer f.Close()

	f.Write(append(b, '\n'))
}
