// Package cache provides an in-memory, TTL-based, sharded cache. It is the Go
// counterpart of the Java MapCache singleton, but sharded across N maps to cut
// lock contention under high concurrency, with a background janitor evicting
// expired entries so memory does not grow unbounded.
package cache

import (
	"hash/fnv"
	"sync"
	"time"
)

const shardCount = 32

type item struct {
	value   interface{}
	expired int64 // unix seconds; <=0 means never expires
}

type shard struct {
	mu sync.RWMutex
	m  map[string]item
}

// Cache is a sharded TTL cache safe for concurrent use.
type Cache struct {
	shards [shardCount]*shard
	stop   chan struct{}
}

// New builds a cache and starts its janitor goroutine.
func New() *Cache {
	c := &Cache{stop: make(chan struct{})}
	for i := range c.shards {
		c.shards[i] = &shard{m: make(map[string]item)}
	}
	go c.janitor()
	return c
}

func (c *Cache) shardFor(key string) *shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return c.shards[h.Sum32()%shardCount]
}

// Set stores value under key. expired is a TTL in seconds; <=0 means no expiry.
func (c *Cache) Set(key string, value interface{}, expired int64) {
	exp := expired
	if expired > 0 {
		exp = time.Now().Unix() + expired
	}
	s := c.shardFor(key)
	s.mu.Lock()
	s.m[key] = item{value: value, expired: exp}
	s.mu.Unlock()
}

// SetNX stores value only when key is missing or expired. The check and write
// happen under one shard lock, which makes it suitable for frequency controls.
func (c *Cache) SetNX(key string, value interface{}, expired int64) bool {
	now := time.Now().Unix()
	exp := expired
	if expired > 0 {
		exp = now + expired
	}
	s := c.shardFor(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, exists := s.m[key]; exists && (current.expired <= 0 || current.expired > now) {
		return false
	}
	s.m[key] = item{value: value, expired: exp}
	return true
}

// Get returns the value for key, or (nil,false) if missing/expired.
func (c *Cache) Get(key string) (interface{}, bool) {
	s := c.shardFor(key)
	s.mu.RLock()
	it, ok := s.m[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if it.expired > 0 && it.expired <= time.Now().Unix() {
		return nil, false
	}
	return it.value, true
}

// GetInt is a typed convenience wrapper.
func (c *Cache) GetInt(key string) (int, bool) {
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	n, ok := v.(int)
	return n, ok
}

// Incr atomically increments an integer value and refreshes its TTL.
func (c *Cache) Incr(key string, expired int64) int {
	now := time.Now().Unix()
	exp := expired
	if expired > 0 {
		exp = now + expired
	}
	s := c.shardFor(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	current := 0
	if existing, exists := s.m[key]; exists && (existing.expired <= 0 || existing.expired > now) {
		current, _ = existing.value.(int)
	}
	current++
	s.m[key] = item{value: current, expired: exp}
	return current
}

// GetString is a typed convenience wrapper.
func (c *Cache) GetString(key string) (string, bool) {
	v, ok := c.Get(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// Del removes a key.
func (c *Cache) Del(key string) {
	s := c.shardFor(key)
	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()
}

// HSet/HGet emulate the Java hash-namespaced helpers (key:field).
func (c *Cache) HSet(key, field string, value interface{}, expired int64) {
	c.Set(key+":"+field, value, expired)
}

func (c *Cache) HSetNX(key, field string, value interface{}, expired int64) bool {
	return c.SetNX(key+":"+field, value, expired)
}

func (c *Cache) HGet(key, field string) (interface{}, bool) {
	return c.Get(key + ":" + field)
}

func (c *Cache) HGetInt(key, field string) (int, bool) {
	return c.GetInt(key + ":" + field)
}

func (c *Cache) HGetString(key, field string) (string, bool) {
	return c.GetString(key + ":" + field)
}

func (c *Cache) HDel(key, field string) { c.Del(key + ":" + field) }

// Close stops the janitor goroutine.
func (c *Cache) Close() { close(c.stop) }

func (c *Cache) janitor() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			now := time.Now().Unix()
			for _, s := range c.shards {
				s.mu.Lock()
				for k, it := range s.m {
					if it.expired > 0 && it.expired <= now {
						delete(s.m, k)
					}
				}
				s.mu.Unlock()
			}
		}
	}
}
