package flags

import (
	"sync"
	"time"
)

type ConfigCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	entries map[string]cacheEntry
}

type cacheEntry struct {
	flag      Flag
	expiresAt time.Time
}

func NewConfigCache(ttl time.Duration) *ConfigCache {
	return &ConfigCache{
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

func (c *ConfigCache) Get(tenantID, flagKey string, now time.Time) (Flag, bool) {
	cacheKey := tenantID + "::" + flagKey

	c.mu.RLock()
	entry, ok := c.entries[cacheKey]
	c.mu.RUnlock()
	if !ok {
		return Flag{}, false
	}
	if now.After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.entries, cacheKey)
		c.mu.Unlock()
		return Flag{}, false
	}
	return entry.flag, true
}

func (c *ConfigCache) Set(tenantID string, flag Flag, now time.Time) {
	cacheKey := tenantID + "::" + flag.Key

	c.mu.Lock()
	c.entries[cacheKey] = cacheEntry{
		flag:      flag,
		expiresAt: now.Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *ConfigCache) Delete(tenantID, flagKey string) {
	cacheKey := tenantID + "::" + flagKey

	c.mu.Lock()
	delete(c.entries, cacheKey)
	c.mu.Unlock()
}
