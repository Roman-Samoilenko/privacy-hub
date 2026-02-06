package dnsresolver

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type cacheEntry struct {
	msg       *dns.Msg
	expiresAt time.Time
}

type Cache struct {
	entries    map[string]*cacheEntry
	mu         sync.RWMutex
	maxSize    int
	defaultTTL time.Duration
}

func NewCache(maxSize int, defaultTTL time.Duration) *Cache {
	c := &Cache{
		entries:    make(map[string]*cacheEntry),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}

	// Cleanup goroutine
	go c.cleanup()

	return c
}

func (c *Cache) makeKey(domain string, qtype uint16) string {
	return domain + ":" + dns.TypeToString[qtype]
}

func (c *Cache) Get(domain string, qtype uint16) *dns.Msg {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.makeKey(domain, qtype)
	entry, exists := c.entries[key]
	if !exists {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil
	}

	// Return a copy
	return entry.msg.Copy()
}

func (c *Cache) Set(domain string, qtype uint16, msg *dns.Msg) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check size limit
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	key := c.makeKey(domain, qtype)
	ttl := c.getTTL(msg)

	c.entries[key] = &cacheEntry{
		msg:       msg.Copy(),
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *Cache) getTTL(msg *dns.Msg) time.Duration {
	if len(msg.Answer) == 0 {
		return c.defaultTTL
	}

	// Use minimum TTL from answers
	minTTL := uint32(c.defaultTTL.Seconds())
	for _, rr := range msg.Answer {
		if rr.Header().Ttl < minTTL {
			minTTL = rr.Header().Ttl
		}
	}

	return time.Duration(minTTL) * time.Second
}

func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.After(entry.expiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
