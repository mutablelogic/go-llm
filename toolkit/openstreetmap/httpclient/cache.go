package httpclient

import (
	"encoding/json"
	"sync"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// cache keeps responses, in memory, up to a number of them, for a time
type cache struct {
	mu      sync.Mutex
	size    int
	age     time.Duration
	now     func() time.Time
	entries map[string]cacheEntry
}

// cacheEntry is a response, and when it was kept
type cacheEntry struct {
	data json.RawMessage
	kept time.Time
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func newCache(size int, age time.Duration) *cache {
	return &cache{size: size, age: age, now: time.Now, entries: make(map[string]cacheEntry, size)}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// get returns the response of the key, when it was kept, and isn't too old
func (c *cache) get(key string) (json.RawMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, exists := c.entries[key]
	if !exists || c.now().Sub(entry.kept) >= c.age {
		delete(c.entries, key)
		return nil, false
	}
	return entry.data, true
}

// set keeps the response of the key, in place of the oldest, when there are as
// many as are kept
func (c *cache) set(key string, data json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.size {
		oldest := ""
		for other, entry := range c.entries {
			if oldest == "" || entry.kept.Before(c.entries[oldest].kept) {
				oldest = other
			}
		}
		delete(c.entries, oldest)
	}
	c.entries[key] = cacheEntry{data: data, kept: c.now()}
}
