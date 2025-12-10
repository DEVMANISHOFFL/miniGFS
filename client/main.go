package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const ChunkSize = 4 * 1024 * 1024

type primaryResp struct {
	Primary      string   `json:"primary"`
	LeaseSeconds int64    `json:"lease_seconds"`
	Replicas     []string `json:"replicas"`
	Version      uint64   `json:"version,omitempty"`
}

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

func SendPostJSONAndDecode(url string, payload, out any) error {
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

func getOrAssignPrimary(chunkID string) (string, error) {
	// ask master for current primary
	var pResp primaryResp
	if err := SendPostJSONAndDecode(masterURL+"/get_primary", map[string]string{"chunk_id": chunkID}, &pResp); err == nil {
		if pResp.Primary != "" {
			return pResp.Primary, nil
		}
	}

	// no primary, ask assign (no preferred)
	if err := SendPostJSONAndDecode(masterURL+"/assign_primary", map[string]string{"chunk_id": chunkID}, &pResp); err != nil {
		return "", fmt.Errorf("assign_primary failed: %v", err)
	}
	if pResp.Primary == "" {
		return "", fmt.Errorf("no primary assigned")
	}
	return pResp.Primary, nil
}

func uploadFile(filename string, data []byte) ([]string, error) {
	size := int64(len(data))

	// 1) ask master to allocate chunks
	var allocResp struct {
		ChunkIDs  []string   `json:"chunk_ids"`
		Locations [][]string `json:"locations"`
	}
	req := map[string]any{"file": filename, "size_bytes": size}
	if err := SendPostJSONAndDecode(masterURL+"/allocate", req, &allocResp); err != nil {
		return nil, fmt.Errorf("allocate failed: %v", err)
	}

	if len(allocResp.ChunkIDs) == 0 {
		return nil, fmt.Errorf("no chunk ids returned")
	}

	// 2) split and upload chunk-by-chunk
	chunkIDs := allocResp.ChunkIDs
	num := len(chunkIDs)
	for i, cid := range chunkIDs {
		// compute slice bounds
		start := int64(i) * ChunkSize
		end := start + ChunkSize
		if end > size {
			end = size
		}
		part := data[start:end]

		// get/assign primary for this chunk
		primary, err := getOrAssignPrimary(cid)
		if err != nil {
			return nil, fmt.Errorf("primary lookup failed for %s: %v", cid, err)
		}

		// write to primary
		writeReq := map[string]any{
			"chunk_id": cid,
			"data":     part,
		}
		wurl := "http://" + primary + "/write_primary"
		if err := sendPostJSON(wurl, writeReq); err != nil {
			// retry once after reassign
			log.Printf("client: primary write failed for %s: %v, refreshing primary", cid, err)
			primary, err2 := getOrAssignPrimary(cid)
			if err2 != nil {
				return nil, fmt.Errorf("primary reassign failed for %s: %v", cid, err2)
			}
			wurl = "http://" + primary + "/write_primary"
			if err := sendPostJSON(wurl, writeReq); err != nil {
				return nil, fmt.Errorf("write_primary failed after retry for %s: %v", cid, err)
			}
		}

		log.Printf("client: wrote chunk %s (%d/%d) via primary %s", cid, i+1, num, primary)
	}

	return chunkIDs, nil
}
func readChunk(chunkID string) ([]byte, error) {
	// asking master where chunk lives
	req := map[string]string{"chunk_id": chunkID}
	var locResp struct {
		Locations []string `json:"locations"`
	}

	if err := SendPostJSONAndDecode(masterURL+"/chunk_locations", req, &locResp); err != nil {
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

func downloadFile(chunkIDs []string) ([]byte, error) {
	var out []byte

	for _, cid := range chunkIDs {
		part, err := readChunk(cid)
		if err != nil {
			return nil, fmt.Errorf("download for %s failed: %v", cid, err)
		}
		out = append(out, part...)
	}

	return out, nil
}

func main() {
	data := make([]byte, 6*1024*1024)
	copy(data, []byte("hello this is our giant file"))

	// UPLOAD (MULTIPLE CHUNK IDs)
	chunkIDs, err := uploadFile("fresh.txt", data)
	if err != nil {
		log.Fatalf("upload failed: %v", err)
	}

	// DOWNLOAD (READ ALL CHUNKS)
	out, err := downloadFile(chunkIDs)
	if err != nil {
		log.Fatalf("download failed: %v", err)
	}

	fmt.Println("READ BACK:", string(out))
}
