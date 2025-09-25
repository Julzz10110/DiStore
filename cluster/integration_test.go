package cluster

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"distore/synchro"
	"distore/testutils"
)

func TestIntegration_FailoverAndReadOnly(t *testing.T) {
	// Создаем работающие и падающие серверы
	workingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer workingServer.Close()

	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	nodes := []string{workingServer.URL[7:], failingServer.URL[7:], "nonexistent:8080"}

	t.Run("FailoverIntegration", func(t *testing.T) {
		fm := NewFailoverManager(nodes, 100*time.Millisecond, 1*time.Second)
		rom := NewReadOnlyManager(2) // Кворум = 2

		time.Sleep(200 * time.Millisecond)

		activeNodes := fm.GetActiveNodes()
		rom.UpdateNodeCount(len(activeNodes))

		if rom.IsReadOnly() {
			t.Error("Should not be read-only with working nodes")
		}
	})

	t.Run("ReadOnlyOnFailure", func(t *testing.T) {
		fm := NewFailoverManager(nodes, 100*time.Millisecond, 1*time.Second)
		rom := NewReadOnlyManager(3) // Высокий кворум

		time.Sleep(200 * time.Millisecond)

		activeNodes := fm.GetActiveNodes()
		rom.UpdateNodeCount(len(activeNodes))

		if !rom.IsReadOnly() {
			t.Error("Should be read-only without quorum")
		}
	})
}

func TestIntegration_RepairAndSync(t *testing.T) {
	t.Run("MerkleTreeWithRepair", func(t *testing.T) {
		storage1 := testutils.NewMockStorage() // Используем testutils
		storage1.Set("key1", "value1")
		storage1.Set("key2", "value2")

		storage2 := testutils.NewMockStorage() // Используем testutils
		storage2.Set("key1", "value1")         // Такое же значение
		storage2.Set("key3", "value3")         // Разное значение

		// Создаем Merkle trees
		items1, _ := storage1.GetAll()
		keys1 := make([]string, len(items1))
		for i, item := range items1 {
			keys1[i] = item.Key
		}

		items2, _ := storage2.GetAll()
		keys2 := make([]string, len(items2))
		for i, item := range items2 {
			keys2[i] = item.Key
		}

		tree1 := synchro.NewMerkleTree(keys1)
		tree2 := synchro.NewMerkleTree(keys2)

		diffs := synchro.CompareTrees(tree1, tree2)
		if len(diffs) == 0 {
			t.Error("Should detect differences between storages")
		}
	})
}
