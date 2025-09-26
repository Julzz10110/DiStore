package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

var authToken string

func main() {
	baseURL := "http://localhost:8080"

	// Wait until the server becomes available
	if !waitForServer(baseURL, 10*time.Second) {
		log.Fatalf("Server is not available at %s", baseURL)
	}

	fmt.Println("Server is ready. Testing advanced features...")

	// Skip authentication and test directly
	fmt.Println("Skipping authentication, testing endpoints directly...")

	// Example of TTL operation
	setWithTTL(baseURL, "temp-key", "temporary-value", 60)

	// Example of atomic increment
	incrementValue(baseURL, "counter", 5)

	// Example of a batch operation
	batchOperations(baseURL)

	// Example of CAS operation
	casOperation(baseURL, "cas-key", "old-value", "new-value")

	// Blocking example
	acquireLock(baseURL, "resource-lock", 30)

	fmt.Println("All tests completed!")
}

func waitForServer(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url + "/health")
		if err == nil {
			defer resp.Body.Close()

			// Status check
			if resp.StatusCode == 200 {
				fmt.Println("Server is ready!")
				return true
			}
		}
		fmt.Printf("Waiting for server... (%v)\n", err)
		time.Sleep(1 * time.Second)
	}
	return false
}

func setWithTTL(baseURL, key, value string, ttl int) {
	req := map[string]interface{}{
		"key":   key,
		"value": value,
		"ttl":   ttl,
	}

	fmt.Printf("Testing TTL set for key: %s\n", key)
	resp, err := postJSON(baseURL+"/advanced/ttl", req, "")
	if err != nil {
		log.Printf("TTL set failed: %v", err)
		return
	}
	fmt.Printf("TTL set successful: %s\n", resp)
}

func incrementValue(baseURL, key string, delta int64) {
	req := map[string]interface{}{
		"key":   key,
		"delta": delta,
	}

	fmt.Printf("Testing increment for key: %s, delta: %d\n", key, delta)
	resp, err := postJSON(baseURL+"/advanced/increment", req, "")
	if err != nil {
		log.Printf("Increment failed: %v", err)
		return
	}
	fmt.Printf("Increment successful: %s\n", resp)
}

func batchOperations(baseURL string) {
	req := map[string]interface{}{
		"operations": []map[string]interface{}{
			{
				"type":  "set",
				"key":   "batch-key-1",
				"value": "batch-value-1",
			},
			{
				"type":  "set",
				"key":   "batch-key-2",
				"value": "batch-value-2",
			},
		},
	}

	fmt.Printf("Testing batch operations\n")
	resp, err := postJSON(baseURL+"/advanced/batch", req, "")
	if err != nil {
		log.Printf("Batch operations failed: %v", err)
		return
	}
	fmt.Printf("Batch operations successful: %s\n", resp)
}

func casOperation(baseURL, key, expectedValue, newValue string) {
	// Set the initial value
	fmt.Printf("Setting initial value for CAS test\n")
	setWithTTL(baseURL, key, expectedValue, 300)

	// Try to change with a check
	req := map[string]interface{}{
		"key":            key,
		"expected_value": expectedValue,
		"new_value":      newValue,
	}

	fmt.Printf("Testing CAS operation for key: %s\n", key)
	resp, err := postJSON(baseURL+"/advanced/cas", req, "")
	if err != nil {
		log.Printf("CAS operation failed: %v", err)
		return
	}
	fmt.Printf("CAS operation successful: %s\n", resp)
}

func acquireLock(baseURL, key string, timeout int64) {
	req := map[string]interface{}{
		"timeout": timeout,
	}

	fmt.Printf("Testing lock acquisition for key: %s\n", key)
	resp, err := postJSON(baseURL+"/advanced/lock/"+key, req, "")
	if err != nil {
		log.Printf("Acquire lock failed: %v", err)
		return
	}
	fmt.Printf("Lock acquisition successful: %s\n", resp)

	// Release the lock after some time
	time.Sleep(2 * time.Second)
	releaseLock(baseURL, key)
}

func releaseLock(baseURL, key string) {
	client := &http.Client{}
	req, err := http.NewRequest("DELETE", baseURL+"/advanced/lock/"+key, nil)
	if err != nil {
		log.Printf("Create release lock request failed: %v", err)
		return
	}

	fmt.Printf("Releasing lock for key: %s\n", key)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Release lock failed: %v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Lock release response: %s\n", string(body))
}

func postJSON(url string, data interface{}, token string) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal error: %v", err)
	}

	fmt.Printf("Sending request to: %s\n", url)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("request creation error: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request error: %v", err)
	}
	defer resp.Body.Close()

	// Read the entire answer for diagnostics
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body error: %v", err)
	}

	bodyString := string(bodyBytes)

	// Output diagnostic information
	fmt.Printf("Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
	fmt.Printf("Response Body (first 200 chars): %.200s\n", bodyString)

	// Check the status code
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyString)
	}

	// Try parsing it as JSON
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("JSON parse error: %v, body: %s", err, bodyString)
	}

	return fmt.Sprintf("%v", result), nil
}
