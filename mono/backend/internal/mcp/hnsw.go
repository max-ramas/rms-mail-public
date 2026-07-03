package mcp

import (
	"container/list"
	"sync"
)

// VectorNode represents a vector in the HNSW graph with LRU eviction.
type VectorNode struct {
	ID       string
	Vector   []float32
	Metadata map[string]string
}

// HNSWCache is an LRU cache for vector nodes to prevent OOM.
type HNSWCache struct {
	mu      sync.RWMutex
	maxSize int
	items   map[string]*list.Element
	lruList *list.List
}

type cacheEntry struct {
	key   string
	value *VectorNode
}

// NewHNSWCache creates an LRU vector cache with the given max size.
func NewHNSWCache(maxSize int) *HNSWCache {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &HNSWCache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element),
		lruList: list.New(),
	}
}

// Get retrieves a vector node (marks as recently used).
func (c *HNSWCache) Get(id string) *VectorNode {
	c.mu.RLock()
	elem, ok := c.items[id]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	c.mu.Lock()
	c.lruList.MoveToFront(elem)
	c.mu.Unlock()

	return elem.Value.(*cacheEntry).value
}

// Set adds or updates a vector node (evicts LRU if at capacity).
func (c *HNSWCache) Set(id string, node *VectorNode) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[id]; ok {
		c.lruList.MoveToFront(elem)
		elem.Value.(*cacheEntry).value = node
		return
	}

	if c.lruList.Len() >= c.maxSize {
		// Evict least recently used
		back := c.lruList.Back()
		if back != nil {
			entry := back.Value.(*cacheEntry)
			delete(c.items, entry.key)
			c.lruList.Remove(back)
		}
	}

	elem := c.lruList.PushFront(&cacheEntry{key: id, value: node})
	c.items[id] = elem
}

// Remove deletes a node from cache.
func (c *HNSWCache) Remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[id]; ok {
		delete(c.items, id)
		c.lruList.Remove(elem)
	}
}

// Len returns current number of cached nodes.
func (c *HNSWCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}

// Clear empties the cache.
func (c *HNSWCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.lruList.Init()
}
