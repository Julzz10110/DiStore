package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"distore/storage"
	"distore/testutils"
)

func TestInternalGetHandler(t *testing.T) {
	store := storage.NewMemoryStorage()
	_ = store.Set("x", "1")
	mock := testutils.NewMockReplicator(nil, 0)
	h := NewHandlers(store, mock, nil)

	req := httptest.NewRequest("GET", "/internal/get/x", nil)
	rr := httptest.NewRecorder()
	h.InternalGetHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
}
