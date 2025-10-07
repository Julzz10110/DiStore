package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"distore/storage"
	"distore/testutils"
)

func TestAdminNodesHandlers(t *testing.T) {
	store := storage.NewMemoryStorage()
	mock := testutils.NewMockReplicator([]string{"n1"}, 1)
	h := NewHandlers(store, mock, nil)

	// List
	req := httptest.NewRequest("GET", "/admin/nodes", nil)
	rr := httptest.NewRecorder()
	h.ListNodesHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("list nodes status %d", rr.Code)
	}

	// Add
	body, _ := json.Marshal(map[string]string{"node": "n2"})
	req = httptest.NewRequest("POST", "/admin/nodes", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	h.AddNodeHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("add node status %d", rr.Code)
	}
	if len(mock.GetNodes()) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(mock.GetNodes()))
	}

	// Remove
	req = httptest.NewRequest("DELETE", "/admin/nodes/n1", nil)
	rr = httptest.NewRecorder()
	// mux vars are not required, handler reads from path string
	h.RemoveNodeHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("remove node status %d", rr.Code)
	}
}

func TestAdminConfigHandlers(t *testing.T) {
	store := storage.NewMemoryStorage()
	mock := testutils.NewMockReplicator([]string{"n1"}, 1)
	h := NewHandlers(store, mock, nil)

	// Get config
	req := httptest.NewRequest("GET", "/admin/config", nil)
	rr := httptest.NewRecorder()
	h.GetConfigHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get config status %d", rr.Code)
	}

	// Update config
	body, _ := json.Marshal(map[string]interface{}{"nodes": []string{"n1", "n2"}, "replica_count": 2})
	req = httptest.NewRequest("PATCH", "/admin/config", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	h.UpdateConfigHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("update config status %d", rr.Code)
	}
}

func TestBackupRestoreHandlers(t *testing.T) {
	store := storage.NewMemoryStorage()
	_ = store.Set("a", "1")
	_ = store.Set("b", "2")
	mock := testutils.NewMockReplicator([]string{}, 0)
	h := NewHandlers(store, mock, nil)

	tmpDir := t.TempDir()
	bpath := filepath.Join(tmpDir, "backup.json")

	// Backup
	body, _ := json.Marshal(map[string]string{"path": bpath})
	req := httptest.NewRequest("POST", "/admin/backup", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.BackupHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("backup status %d", rr.Code)
	}
	if _, err := os.Stat(bpath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	// Clear and restore
	_ = store.Delete("a")
	_ = store.Delete("b")
	body, _ = json.Marshal(map[string]string{"path": bpath})
	req = httptest.NewRequest("POST", "/admin/restore", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	h.RestoreHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("restore status %d", rr.Code)
	}

	if v, _ := store.Get("a"); v != "1" {
		t.Fatalf("expected restored a=1, got %s", v)
	}
	if v, _ := store.Get("b"); v != "2" {
		t.Fatalf("expected restored b=2, got %s", v)
	}
}
