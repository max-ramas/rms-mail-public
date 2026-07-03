package api

import (
	"sync"
	"time"
)

// memCacheEntry holds a cached value with its expiration time.
type memCacheEntry struct {
	value   string
	expires time.Time
}

// MemoryCache is a simple in-memory TTL cache used as Redis fallback in Mono edition.
// Thread-safe, no external dependencies. Cleanup runs every 5 minutes.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]*memCacheEntry
}

// NewMemoryCache creates a new in-memory cache with periodic cleanup.
func NewMemoryCache() *MemoryCache {
	c := &MemoryCache{
		items: make(map[string]*memCacheEntry),
	}
	go c.cleanupLoop()
	return c
}

func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expires) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}

// Get returns the cached value and true if found and not expired.
func (c *MemoryCache) Get(key string) (string, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expires) {
		return "", false
	}
	return entry.value, true
}

// Set stores a value with TTL.
func (c *MemoryCache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	c.items[key] = &memCacheEntry{
		value:   value,
		expires: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Del removes one or more keys.
func (c *MemoryCache) Del(keys ...string) {
	c.mu.Lock()
	for _, k := range keys {
		delete(c.items, k)
	}
	c.mu.Unlock()
}

// Keys returns all non-expired keys in the cache.
func (c *MemoryCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	now := time.Now()
	var keys []string
	for k, v := range c.items {
		if now.Before(v.expires) {
			keys = append(keys, k)
		}
	}
	return keys
}
