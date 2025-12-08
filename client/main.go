package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const masterURL = "http://localhost:8080"

func sendPostJSON(url string, payload any) error {
	b, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	return nil
}

func sendPostJSONAndDecode(url string, payload, out any) error {
	b, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func writeData(filename string, data []byte) (string, error) {
	req := map[string]string{"file": filename}

	var allocResp struct {
		ChunkID   string   `json:"chunk_id"`
		Locations []string `json:"locations"`
	}

	url := masterURL + "/allocate"
	if err := sendPostJSONAndDecode(url, req, &allocResp); err != nil {
		return "", fmt.Errorf("allocation failed: %v", err)
	}

	writeReq := map[string]any{
		"chunk_id": allocResp.ChunkID,
		"data":     data,
	}

	for _, loc := range allocResp.Locations {

		wurl := "http://" + loc + "/write_chunk"
		if err := sendPostJSON(wurl, writeReq); err != nil {
			return "", fmt.Errorf("allocation failed: %v", err)
		}
		log.Printf("client: wrote chunk to %s", loc)
	}
	log.Printf("client: file %s stored as chunk %s", filename, allocResp.ChunkID)
	return allocResp.ChunkID, nil
}

func readChunk(chunkID string) ([]byte, error) {
	// asking master where chunk lives
	req := map[string]string{"chunk_id": chunkID}
	var locResp struct {
		Locations []string `json:"locations"`
	}

	if err := sendPostJSONAndDecode(masterURL+"/chunk_locations", req, &locResp); err != nil {
		return nil, fmt.Errorf("chunk_locations failed: %v", err)
	}

	// Try each replica
	for _, loc := range locResp.Locations {
		url := "http://" + loc + "/read_chunk?chunk_id=" + chunkID
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("read error from %s: %v", loc, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("read io error from %s: %v", loc, err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			log.Printf("client: read chunk from %s", loc)
			return body, nil
		}

		log.Printf("replica %s returned %s", loc, resp.Status)
	}

	return nil, fmt.Errorf("all replicas failed for chunk %s", chunkID)
}

func main() {
	data := []byte("hello mini gfs 101")
	chunkID, err := writeData("fresh.txt", data)
	if err != nil {
		log.Fatalf("write failed: %v", err)
	}

	out, err := readChunk(chunkID)
	if err != nil {
		log.Fatalf("read failed: %v", err)
	}

	fmt.Println("READ BACK:", string(out))
}
