package replication

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestIntegration_QuorumWithHintedHandoff(t *testing.T) {
	// Create a mixed environment: some nodes are running, some are not
	workingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer workingServer.Close()

	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	nodes := []string{
		workingServer.URL[7:], // running node
		failingServer.URL[7:], // falling node
		"nonexistent:8080",    // non-existent node
	}

	// Create a temporary directory for hinted handoff
	tempDir, err := os.MkdirTemp("", "integration-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	replicator := NewReplicator(nodes, 2)
	replicator.hintedHandoff = NewHintedHandoff(tempDir)

	t.Run("QuorumWithFailedNodes", func(t *testing.T) {
		err := replicator.ReplicateSet("test-key", "test-value")
		if err != nil {
			t.Errorf("Replication should succeed with quorum despite failed nodes: %v", err)
		}

		// Check if hints were saved for failed nodes
		if replicator.hintedHandoff != nil && len(replicator.hintedHandoff.hints) < 1 {
			t.Error("Expected hints to be stored for failed nodes")
		}
	})
}

func TestIntegration_ReadYourWrites(t *testing.T) {
	servers := make([]*httptest.Server, 3)
	for i := range servers {
		i := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value": "data-from-node-` + fmt.Sprint(i) + `"}`))
		}))
		defer servers[i].Close()
	}

	nodeURLs := make([]string, len(servers))
	for i, server := range servers {
		nodeURLs[i] = server.URL[7:]
	}

	replicator := NewReplicator(nodeURLs, 2)
	replicator.consistencyMgr = NewConsistencyManager()

	t.Run("StickyReading", func(t *testing.T) {
		// Write data through a specific node
		replicator.consistencyMgr.RecordWrite("sticky-key", "node0")
		replicator.consistencyMgr.UpdateClientSession("test-client")

		// Should try to read from the same node
		preferredNode, err := replicator.consistencyMgr.EnsureReadYourWrites("test-client", "sticky-key")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if preferredNode != "node0" {
			t.Errorf("Expected to prefer node0, got %s", preferredNode)
		}
	})
}
