package storage

import (
	"hash"
	"hash/fnv"
	"math"
	"sync"
)

type BloomFilter struct {
	bitset    []bool
	size      uint
	hashFuncs []hash.Hash64
	mu        sync.RWMutex
}

func NewBloomFilter(expectedElements int, falsePositiveRate float64) *BloomFilter {
	// Calculate optimal size and number of hash functions
	size := calculateSize(expectedElements, falsePositiveRate)
	numHashes := calculateNumHashes(expectedElements, size)

	// Create hash functions
	hashFuncs := make([]hash.Hash64, numHashes)
	for i := range hashFuncs {
		hashFuncs[i] = fnv.New64a()
	}

	return &BloomFilter{
		bitset:    make([]bool, size),
		size:      size,
		hashFuncs: hashFuncs,
	}
}

func calculateSize(n int, p float64) uint {
	// m = - (n * ln(p)) / (ln(2)^2)
	return uint(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2))
}

func calculateNumHashes(n int, m uint) int {
	// k = (m / n) * ln(2)
	return int(float64(m) / float64(n) * math.Ln2)
}

func (bf *BloomFilter) Add(key string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	indices := bf.getIndices(key)
	for _, idx := range indices {
		bf.bitset[idx] = true
	}
}

func (bf *BloomFilter) Contains(key string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	indices := bf.getIndices(key)
	for _, idx := range indices {
		if !bf.bitset[idx] {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) getIndices(key string) []uint {
	indices := make([]uint, len(bf.hashFuncs))

	for i, h := range bf.hashFuncs {
		h.Reset()
		h.Write([]byte(key))
		hashValue := h.Sum64()
		indices[i] = uint(hashValue) % bf.size
	}

	return indices
}

// OptimizedStorage with Bloom filter
type OptimizedStorage struct {
	Storage
	bloomFilter *BloomFilter
	mu          sync.RWMutex
}

func NewOptimizedStorage(base Storage, expectedElements int) *OptimizedStorage {
	return &OptimizedStorage{
		Storage:     base,
		bloomFilter: NewBloomFilter(expectedElements, 0.01), // 1% false positive rate
	}
}

func (os *OptimizedStorage) Get(key string) (string, error) {
	// quick Bloom filter check fefore accessing storage
	if !os.bloomFilter.Contains(key) {
		return "", ErrKeyNotFound
	}

	return os.Storage.Get(key)
}

func (os *OptimizedStorage) Set(key, value string) error {
	os.mu.Lock()
	defer os.mu.Unlock()

	if err := os.Storage.Set(key, value); err != nil {
		return err
	}

	// add a key to the Bloom filter
	os.bloomFilter.Add(key)
	return nil
}

func (os *OptimizedStorage) Delete(key string) error {
	os.mu.Lock()
	defer os.mu.Unlock()

	// Bloom filters can't be deleted, so just delete it from the main storage
	return os.Storage.Delete(key)
}

// GetStats returns Bloom filter statistics
func (bf *BloomFilter) GetStats() map[string]interface{} {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	// count set bits
	setBits := 0
	for _, bit := range bf.bitset {
		if bit {
			setBits++
		}
	}

	fillRatio := float64(setBits) / float64(len(bf.bitset))

	return map[string]interface{}{
		"size":           len(bf.bitset),
		"set_bits":       setBits,
		"fill_ratio":     fillRatio,
		"hash_functions": len(bf.hashFuncs),
	}
}
