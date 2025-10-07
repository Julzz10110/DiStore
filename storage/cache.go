package storage

import (
	"container/list"
	"sync"
	"time"
)

type CacheEntry struct {
	key         string
	value       string
	accessTime  time.Time
	accessCount int
}

type CacheStrategy string

const (
	LRU CacheStrategy = "lru" // Least Recently Used
	LFU CacheStrategy = "lfu" // Least Frequently Used
	ARC CacheStrategy = "arc" // Adaptive Replacement Cache
)

type CacheStorage struct {
	Storage
	cache      map[string]*list.Element
	accessList *list.List // for LRU
	strategy   CacheStrategy
	maxSize    int
	ttl        time.Duration
	mu         sync.RWMutex
	stats      CacheStats
}

type CacheStats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	Evictions int64 `json:"evictions"`
}

func NewCacheStorage(base Storage, strategy CacheStrategy, maxSize int, ttl time.Duration) *CacheStorage {
	return &CacheStorage{
		Storage:    base,
		cache:      make(map[string]*list.Element),
		accessList: list.New(),
		strategy:   strategy,
		maxSize:    maxSize,
		ttl:        ttl,
	}
}

func (cs *CacheStorage) Get(key string) (string, error) {
	cs.mu.RLock()

	// check cache
	if elem, exists := cs.cache[key]; exists {
		entry := elem.Value.(*CacheEntry)

		// check TTL
		if time.Since(entry.accessTime) > cs.ttl {
			cs.mu.RUnlock()
			cs.mu.Lock()
			delete(cs.cache, key)
			cs.accessList.Remove(elem)
			cs.mu.Unlock()
			cs.stats.Misses++
		} else {
			// update access statistics
			entry.accessTime = time.Now()
			entry.accessCount++

			// move to the begin for LRU
			if cs.strategy == LRU {
				cs.mu.RUnlock()
				cs.mu.Lock()
				cs.accessList.MoveToFront(elem)
				cs.mu.Unlock()
			} else {
				cs.mu.RUnlock()
			}

			cs.stats.Hits++
			return entry.value, nil
		}
	} else {
		cs.mu.RUnlock()
		cs.stats.Misses++
	}

	// Not found in cache, search in main storage
	value, err := cs.Storage.Get(key)
	if err != nil {
		return "", err
	}

	// Add to cache
	cs.mu.Lock()
	cs.addToCache(key, value)
	cs.mu.Unlock()

	return value, nil
}

func (cs *CacheStorage) Set(key, value string) error {
	// Firstly, in the main storage
	if err := cs.Storage.Set(key, value); err != nil {
		return err
	}

	// then in cache
	cs.mu.Lock()
	cs.addToCache(key, value)
	cs.mu.Unlock()

	return nil
}

func (cs *CacheStorage) Delete(key string) error {
	// Delete from cache
	cs.mu.Lock()
	if elem, exists := cs.cache[key]; exists {
		delete(cs.cache, key)
		cs.accessList.Remove(elem)
	}
	cs.mu.Unlock()

	// Delete from the main cache
	return cs.Storage.Delete(key)
}

func (cs *CacheStorage) addToCache(key, value string) {
	// If the key is already in the cache, update it
	if elem, exists := cs.cache[key]; exists {
		entry := elem.Value.(*CacheEntry)
		entry.value = value
		entry.accessTime = time.Now()
		entry.accessCount++
		cs.accessList.MoveToFront(elem)
		return
	}

	// If the cache is full, delete old entries
	if len(cs.cache) >= cs.maxSize {
		cs.evict()
	}

	// Add a new entry
	entry := &CacheEntry{
		key:         key,
		value:       value,
		accessTime:  time.Now(),
		accessCount: 1,
	}

	elem := cs.accessList.PushFront(entry)
	cs.cache[key] = elem
}

func (cs *CacheStorage) evict() {
	var elem *list.Element

	switch cs.strategy {
	case LRU:
		// Delete the oldest one
		elem = cs.accessList.Back()
	case LFU:
		// Looking for the least used
		var minCount = int(^uint(0) >> 1) // Max int
		for e := cs.accessList.Back(); e != nil; e = e.Prev() {
			entry := e.Value.(*CacheEntry)
			if entry.accessCount < minCount {
				minCount = entry.accessCount
				elem = e
			}
		}
	default:
		elem = cs.accessList.Back()
	}

	if elem != nil {
		entry := elem.Value.(*CacheEntry)
		delete(cs.cache, entry.key)
		cs.accessList.Remove(elem)
		cs.stats.Evictions++
	}
}

func (cs *CacheStorage) GetCacheStats() CacheStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.stats
}

func (cs *CacheStorage) GetHitRate() float64 {
	stats := cs.GetCacheStats()
	total := stats.Hits + stats.Misses
	if total == 0 {
		return 0
	}
	return float64(stats.Hits) / float64(total)
}

// Method for preloading hot data
func (cs *CacheStorage) PreloadKeys(keys []string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for _, key := range keys {
		if _, exists := cs.cache[key]; !exists {
			if value, err := cs.Storage.Get(key); err == nil {
				cs.addToCache(key, value)
			}
		}
	}
}
