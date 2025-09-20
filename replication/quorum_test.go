package replication

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuorumReplicator(t *testing.T) {
	// Create test servers to simulate nodes
	servers := make([]*httptest.Server, 3)
	for i := range servers {
		i := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if i == 0 {
				// The first node always responds successfully
				w.WriteHeader(http.StatusCreated)
			} else {
				// The remaining nodes may respond differently
				w.WriteHeader(http.StatusCreated)
			}
		}))
		defer servers[i].Close()
	}

	nodeURLs := make([]string, len(servers))
	for i, server := range servers {
		nodeURLs[i] = server.URL[7:] // remove "http://"
	}

	t.Run("WriteQuorumSuccess", func(t *testing.T) {
		replicator := NewQuorumReplicator(nodeURLs, 2, 2)

		err := replicator.ReplicateSetWithQuorum("test-key", "test-value")
		if err != nil {
			t.Errorf("Expected successful quorum write, got error: %v", err)
		}
	})

	t.Run("WriteQuorumFailure", func(t *testing.T) {
		// Create servers where most crashes occur
		failingServers := make([]*httptest.Server, 3)
		for i := range failingServers {
			i := i
			failingServers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if i == 0 {
					w.WriteHeader(http.StatusCreated) // only one works
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer failingServers[i].Close()
		}

		failingURLs := make([]string, len(failingServers))
		for i, server := range failingServers {
			failingURLs[i] = server.URL[7:]
		}

		replicator := NewQuorumReplicator(failingURLs, 2, 2)

		err := replicator.ReplicateSetWithQuorum("test-key", "test-value")
		if err == nil {
			t.Error("Expected quorum failure, got success")
		}

		expectedError := "write quorum not reached"
		if err != nil && !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
		}
	})
}
