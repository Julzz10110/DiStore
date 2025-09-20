package replication

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHintedHandoff(t *testing.T) {
	// Create a temporary directory for tests
	tempDir, err := os.MkdirTemp("", "hinted-handoff-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	hh := NewHintedHandoff(tempDir)

	t.Run("StoreAndRetrieveHint", func(t *testing.T) {
		err := hh.StoreHint("test-key", "test-value", "node1:8080")
		if err != nil {
			t.Errorf("Failed to store hint: %v", err)
		}

		if len(hh.hints) != 1 {
			t.Errorf("Expected 1 hint, got %d", len(hh.hints))
		}

		hint := hh.hints[0]
		if hint.Key != "test-key" || hint.Value != "test-value" {
			t.Errorf("Hint data mismatch: %+v", hint)
		}
	})

	t.Run("HintDelivery", func(t *testing.T) {
		// Create a test server to receive hints
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		nodeURL := server.URL[7:] // remove "http://"

		err := hh.StoreHint("delivery-key", "delivery-value", nodeURL)
		if err != nil {
			t.Errorf("Failed to store hint: %v", err)
		}

		// Try to deliver a hint
		err = hh.tryDeliverHint(hh.hints[0])
		if err != nil {
			t.Errorf("Failed to deliver hint: %v", err)
		}
	})

	t.Run("Persistence", func(t *testing.T) {
		// Save hints
		err := hh.StoreHint("persistent-key", "persistent-value", "node1:8080")
		if err != nil {
			t.Errorf("Failed to store hint: %v", err)
		}

		err = hh.saveHints()
		if err != nil {
			t.Errorf("Failed to save hints: %v", err)
		}

		// Load into a new instance
		hh2 := NewHintedHandoff(tempDir)
		if len(hh2.hints) != 1 {
			t.Errorf("Expected 1 persisted hint, got %d", len(hh2.hints))
		}
	})
}
