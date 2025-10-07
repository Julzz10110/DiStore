package cluster

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFailoverManager(t *testing.T) {
	// Create test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server2.Close()

	nodes := []string{server1.URL[7:], server2.URL[7:]}

	t.Run("NodeStatusMonitoring", func(t *testing.T) {
		fm := NewFailoverManager(nodes, 100*time.Millisecond, 1*time.Second)
		time.Sleep(200 * time.Millisecond) // wait checks

		status := fm.GetNodeStatus()
		if len(status) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(status))
		}

		if !status[nodes[0]].Online {
			t.Error("First node should be online")
		}

		if status[nodes[1]].Online {
			t.Error("Second node should be offline")
		}
	})

	t.Run("ActiveNodesFiltering", func(t *testing.T) {
		fm := NewFailoverManager(nodes, 100*time.Millisecond, 1*time.Second)
		time.Sleep(200 * time.Millisecond)

		activeNodes := fm.GetActiveNodes()
		if len(activeNodes) != 1 {
			t.Errorf("Expected 1 active node, got %d", len(activeNodes))
		}

		if activeNodes[0] != nodes[0] {
			t.Errorf("Expected active node %s, got %s", nodes[0], activeNodes[0])
		}
	})
}

func TestFailoverManager_TimeoutHandling(t *testing.T) {
	// Server with latency more than timeout
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // more than a timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	fm := NewFailoverManager([]string{slowServer.URL[7:]}, 100*time.Millisecond, 500*time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	status := fm.GetNodeStatus()
	if status[slowServer.URL[7:]].Online {
		t.Error("Slow node should be marked as offline")
	}
}

func TestFailoverManager_DynamicMembership(t *testing.T) {
	serverOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverOK.Close()

	fm := NewFailoverManager([]string{}, 50*time.Millisecond, 500*time.Millisecond)

	// Add node
	fm.AddNode(serverOK.URL[7:])
	time.Sleep(100 * time.Millisecond)
	if len(fm.GetActiveNodes()) != 1 {
		t.Fatalf("expected 1 active node after add")
	}

	// Remove node
	fm.RemoveNode(serverOK.URL[7:])
	time.Sleep(100 * time.Millisecond)
	if len(fm.GetActiveNodes()) != 0 {
		t.Fatalf("expected 0 active nodes after remove")
	}

	// Set nodes
	fm.SetNodes([]string{serverOK.URL[7:]})
	time.Sleep(100 * time.Millisecond)
	if len(fm.GetActiveNodes()) != 1 {
		t.Fatalf("expected 1 active node after set")
	}
}
