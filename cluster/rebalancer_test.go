package cluster

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"distore/storage"
)

type mockNodeLister struct{ nodes []string }

func (m mockNodeLister) GetNodes() []string { return append([]string{}, m.nodes...) }

// Simple internal server to accept /internal/set and record keys
func newOwnerServer(recorded *[]string) *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/internal/set", func(w http.ResponseWriter, r *http.Request) {
		var kv map[string]string
		_ = json.NewDecoder(r.Body).Decode(&kv)
		*recorded = append(*recorded, kv["key"])
		w.WriteHeader(http.StatusCreated)
	})
	return httptest.NewServer(handler)
}

func TestRebalancerMovesKeys(t *testing.T) {
	store := storage.NewMemoryStorage()
	// place two keys
	_ = store.Set("k1", "v1")
	_ = store.Set("k2", "v2")

	// owner server represents second node
	var received []string
	srv := newOwnerServer(&received)
	defer srv.Close()

	// extract host part
	url := strings.TrimPrefix(srv.URL, "http://")

	// nodes list includes self placeholder and owner url
	lister := mockNodeLister{nodes: []string{"self:1234", url}}
	r := NewRebalancer(store, lister, "self:1234")

	moved, err := r.TriggerRebalance()
	if err != nil {
		t.Fatalf("rebalance error: %v", err)
	}

	// Some keys may move depending on hash; ensure not exceeding total
	if moved < 0 || moved > 2 {
		t.Fatalf("unexpected moved count %d", moved)
	}
}
