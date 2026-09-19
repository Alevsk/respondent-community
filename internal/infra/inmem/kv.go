package inmem

import (
	"context"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// kvEntry holds a cached value with an optional expiry.
type kvEntry struct {
	value     string
	expiresAt time.Time
}

func (e *kvEntry) expired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

// KVCache is a thread-safe in-memory key-value cache implementing domain.KeyValueCache.
type KVCache struct {
	mu      sync.RWMutex
	entries map[string]*kvEntry
}

// NewKVCache creates a new KVCache.
func NewKVCache() *KVCache {
	return &KVCache{
		entries: make(map[string]*kvEntry),
	}
}

// Get retrieves a value by key. Returns domain.NotFoundError if missing or expired.
func (c *KVCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || entry.expired() {
		return "", domain.NewNotFoundError("key not found: "+key, nil)
	}
	return entry.value, nil
}

// Set stores a value with an optional TTL. ttl=0 means the entry never expires.
func (c *KVCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &kvEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.entries[key] = entry
	return nil
}
