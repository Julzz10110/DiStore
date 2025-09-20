package replication

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Hint struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Node      string    `json:"node"`
	Timestamp time.Time `json:"timestamp"`
	Attempts  int       `json:"attempts"`
}

type HintedHandoff struct {
	mu          sync.Mutex
	hints       []Hint
	storageDir  string
	maxAttempts int
	retryDelay  time.Duration
}

func NewHintedHandoff(storageDir string) *HintedHandoff {
	hh := &HintedHandoff{
		storageDir:  storageDir,
		maxAttempts: 10,
		retryDelay:  30 * time.Second,
	}

	if err := os.MkdirAll(storageDir, 0755); err != nil {
		fmt.Printf("Warning: failed to create hinted handoff directory: %v\n", err)
	}

	hh.loadHints()
	go hh.startRetryWorker()

	return hh
}

func (hh *HintedHandoff) StoreHint(key, value, node string) error {
	hh.mu.Lock()
	defer hh.mu.Unlock()

	hint := Hint{
		Key:       key,
		Value:     value,
		Node:      node,
		Timestamp: time.Now(),
		Attempts:  0,
	}

	hh.hints = append(hh.hints, hint)
	return hh.saveHints()
}

func (hh *HintedHandoff) startRetryWorker() {
	ticker := time.NewTicker(hh.retryDelay)
	defer ticker.Stop()

	for range ticker.C {
		hh.retryHints()
	}
}

func (hh *HintedHandoff) retryHints() {
	hh.mu.Lock()
	defer hh.mu.Unlock()

	var remainingHints []Hint

	for _, hint := range hh.hints {
		if hint.Attempts >= hh.maxAttempts {
			fmt.Printf("Hint for node %s exceeded max attempts, discarding\n", hint.Node)
			continue
		}

		hint.Attempts++
		err := hh.tryDeliverHint(hint)
		if err != nil {
			fmt.Printf("Failed to deliver hint to %s (attempt %d): %v\n",
				hint.Node, hint.Attempts, err)
			remainingHints = append(remainingHints, hint)
		} else {
			fmt.Printf("Successfully delivered hint to %s\n", hint.Node)
		}
	}

	hh.hints = remainingHints
	hh.saveHints()
}

func (hh *HintedHandoff) tryDeliverHint(hint Hint) error {
	// Attempt to deliver a hint to the target node
	client := &http.Client{Timeout: 5 * time.Second}

	req := ReplicationRequest{Key: hint.Key, Value: hint.Value}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/internal/set", hint.Node)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func (hh *HintedHandoff) saveHints() error {
	filePath := filepath.Join(hh.storageDir, "hints.json")
	data, err := json.Marshal(hh.hints)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func (hh *HintedHandoff) loadHints() error {
	filePath := filepath.Join(hh.storageDir, "hints.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // The file doesn't exist (it's ok)
		}
		return err
	}

	return json.Unmarshal(data, &hh.hints)
}
