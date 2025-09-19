package storage

import (
	"fmt"
	"os"
	"testing"
)

func TestMemoryStorage(t *testing.T) {
	store := NewMemoryStorage()

	t.Run("Set and Get", func(t *testing.T) {
		err := store.Set("test-key", "test-value")
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		value, err := store.Get("test-key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if value != "test-value" {
			t.Errorf("Expected 'test-value', got '%s'", value)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, err := store.Get("non-existent")
		if err != ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound, got %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store.Set("to-delete", "value")
		err := store.Delete("to-delete")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = store.Get("to-delete")
		if err != ErrKeyNotFound {
			t.Errorf("Expected ErrKeyNotFound after delete, got %v", err)
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		store := NewMemoryStorage()
		store.Set("key1", "value1")
		store.Set("key2", "value2")

		items, err := store.GetAll()
		if err != nil {
			t.Fatalf("GetAll failed: %v", err)
		}

		if len(items) != 2 {
			t.Errorf("Expected 2 items, got %d", len(items))
		}
	})
}

func TestDiskStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "distore-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewDiskStorage(tempDir)
	if err != nil {
		t.Fatalf("Failed to create disk storage: %v", err)
	}
	defer store.Close()

	t.Run("Set and Get with persistence", func(t *testing.T) {
		err := store.Set("persistent-key", "persistent-value")
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// Recreate storage to test persistence
		store2, err := NewDiskStorage(tempDir)
		if err != nil {
			t.Fatalf("Failed to recreate disk storage: %v", err)
		}
		defer store2.Close()

		value, err := store2.Get("persistent-key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if value != "persistent-value" {
			t.Errorf("Expected 'persistent-value', got '%s'", value)
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(i int) {
				key := fmt.Sprintf("concurrent-%d", i)
				store.Set(key, fmt.Sprintf("value-%d", i))
				store.Get(key)
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
