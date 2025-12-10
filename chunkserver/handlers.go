package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type copyChunkReq struct {
	ChunkID string `json:"chunk_id"`
	Target  string `json:"target"` // "localhost:9002"
}

type receiveChunkReq struct {
	ChunkID string `json:"chunk_id"`
	Data    []byte `json:"data"`
}

type genericResp struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Seq     uint64 `json:"seq,omitempty"`
}

type WriteChunkRequest struct {
	ChunkID string `json:"chunk_id"`
	Data    []byte `json:"data"`
}

type writePrimaryReq struct {
	ChunkID string `json:"chunk_id"`
	Data    []byte `json:"data"`
	ReqID   string `json:"req_id,omitempty"` // optional idempotency
}

type applyWriteReq struct {
	ChunkID string `json:"chunk_id"`
	Seq     uint64 `json:"seq"`
	Data    []byte `json:"data"`
	Version uint64 `json:"version,omitempty"`
}

func helloHandler(serverAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "chunk-server %s online\n", serverAddr)
	}
}

func writeChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req WriteChunkRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ChunkID == "" {
		http.Error(w, "chunk_id required", http.StatusBadRequest)
		return
	}

	// saving file to disk
	filename := "data/" + req.ChunkID + ".bin"
	if err := os.WriteFile(filename, req.Data, 0644); err != nil {
		http.Error(w, "failed to write chunk", http.StatusInternalServerError)
		return
	}

	log.Printf("stored chunk %s", req.ChunkID)
	w.WriteHeader(http.StatusOK)
}

func readChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	chunkID := r.URL.Query().Get("chunk_id")
	if chunkID == "" {
		http.Error(w, "chunk_id required", http.StatusBadRequest)
		return
	}

	filename := "data/" + chunkID + ".bin"
	data, err := os.ReadFile(filename)
	if err != nil {
		http.Error(w, "chunk not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// copyChunkHandler: source chunkserver reads local chunk and posts to target's /receive_chunk.
func copyChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req copyChunkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ChunkID == "" || req.Target == "" {
		http.Error(w, "chunk_id and target required", http.StatusBadRequest)
		return
	}

	// read local chunk file
	filename := "data/" + req.ChunkID + ".bin"
	data, err := os.ReadFile(filename)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: "local chunk not found"})
		return
	}

	// build receive payload
	recv := receiveChunkReq{
		ChunkID: req.ChunkID,
		Data:    data,
	}
	b, _ := json.Marshal(recv)
	url := fmt.Sprintf("http://%s/receive_chunk", req.Target)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: string(respBody)})
		return
	}

	// OK
	json.NewEncoder(w).Encode(genericResp{Status: "ok"})
}

// receiveChunkHandler: accept chunk bytes and write locally.
func receiveChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req receiveChunkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.ChunkID == "" {
		http.Error(w, "chunk_id required", http.StatusBadRequest)
		return
	}

	filename := "data/" + req.ChunkID + ".bin"
	if err := os.WriteFile(filename, req.Data, 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(genericResp{Status: "error", Message: "failed to write chunk"})
		return
	}

	json.NewEncoder(w).Encode(genericResp{Status: "ok"})
}

func writePrimaryHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req writePrimaryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ChunkID == "" {
		http.Error(w, "chunk_id required", http.StatusBadRequest)
		return
	}
	log.Printf("WRITE_PRIMARY on %s for chunk=%s", serverAddr, req.ChunkID)
	log.Printf("seq(before commit)=%d", lastApplied[req.ChunkID]+1)

	// 1) increment local seq
	seqMu.Lock()
	lastApplied[req.ChunkID]++
	seq := lastApplied[req.ChunkID]
	seqMu.Unlock()

	// 2) write to local temp file: data/<chunkID>.seq.tmp
	tmpFile := fmt.Sprintf("data/%s.%d.tmp", req.ChunkID, seq)
	if err := os.WriteFile(tmpFile, req.Data, 0644); err != nil {
		http.Error(w, "failed to write temp", http.StatusInternalServerError)
		return
	}

	// 3) fetch replica list from master (so primary knows followers)
	var locResp struct {
		Replicas []string `json:"replicas"`
		Primary  string   `json:"primary"`
	}
	_ = SendPostJSONAndDecode(masterURL+"/get_primary", map[string]string{"chunk_id": req.ChunkID}, &locResp)
	// locResp.Replicas contains all replicas; primary is this server

	// build follower list (exclude self)
	var followers []string
	addr := serverAddr // assume you have serverAddr string for this chunkserver e.g. "localhost:9001"
	for _, raddr := range locResp.Replicas {
		if raddr == addr {
			continue
		}
		followers = append(followers, raddr)
	}

	// 4) send /apply_write to followers in parallel
	apply := applyWriteReq{
		ChunkID: req.ChunkID,
		Seq:     seq,
		Data:    req.Data,
	}
	b, _ := json.Marshal(apply)
	client := &http.Client{Timeout: 10 * time.Second}
	ackCh := make(chan error, len(followers))

	for _, f := range followers {
		go func(faddr string) {
			resp, err := client.Post("http://"+faddr+"/apply_write", "application/json", bytes.NewReader(b))
			if err != nil {
				ackCh <- err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := ioutil.ReadAll(resp.Body)
				ackCh <- fmt.Errorf("bad status %d: %s", resp.StatusCode, string(body))
				return
			}
			ackCh <- nil
		}(f)
	}

	// wait for follower ACKs (require all)
	for i := 0; i < len(followers); i++ {
		if err := <-ackCh; err != nil {
			// rollback local temp file
			_ = os.Remove(tmpFile)
			http.Error(w, fmt.Sprintf("follower ack failed: %v", err), http.StatusBadGateway)
			return
		}
	}

	// 5) commit locally: rename tmp -> stable
	finalFile := fmt.Sprintf("data/%s.bin", req.ChunkID)
	if err := os.Rename(tmpFile, finalFile); err != nil {
		http.Error(w, "failed to commit", http.StatusInternalServerError)
		return
	}

	// update committed seq
	seqMu.Lock()
	lastCommitted[req.ChunkID] = seq
	seqMu.Unlock()

	// 6) optionally tell followers to commit (we assume apply_write performed durable write already)
	// respond to client
	json.NewEncoder(w).Encode(genericResp{Status: "ok", Seq: seq})
}

// /apply_write : primary -> follower (write temp and ack)
func applyWriteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req applyWriteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	tmpFile := fmt.Sprintf("data/%s.%d.tmp", req.ChunkID, req.Seq)
	if err := os.WriteFile(tmpFile, req.Data, 0644); err != nil {
		http.Error(w, "failed to write temp", http.StatusInternalServerError)
		return
	}

	// mark applied seq (durable enough for our toy)
	seqMu.Lock()
	if lastApplied[req.ChunkID] < req.Seq {
		lastApplied[req.ChunkID] = req.Seq
	}
	seqMu.Unlock()

	// ack
	json.NewEncoder(w).Encode(genericResp{Status: "ok", Seq: req.Seq})
}

// /commit : primary -> follower commit (rename tmp -> final)
type commitReq struct {
	ChunkID string `json:"chunk_id"`
	Seq     uint64 `json:"seq"`
}

func commitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req commitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	tmpFile := fmt.Sprintf("data/%s.%d.tmp", req.ChunkID, req.Seq)
	finalFile := fmt.Sprintf("data/%s.bin", req.ChunkID)
	if err := os.Rename(tmpFile, finalFile); err != nil {
		http.Error(w, "commit failed", http.StatusInternalServerError)
		return
	}
	seqMu.Lock()
	if lastCommitted[req.ChunkID] < req.Seq {
		lastCommitted[req.ChunkID] = req.Seq
	}
	seqMu.Unlock()

	json.NewEncoder(w).Encode(genericResp{Status: "ok", Seq: req.Seq})
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
