package cluster

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFailoverManager(t *testing.T) {
	// Создаем тестовые серверы
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
		time.Sleep(200 * time.Millisecond) // Ждем проверок

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
	// Сервер с задержкой больше таймаута
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Больше чем таймаут
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
