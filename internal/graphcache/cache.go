// Package graphcache bounds graph response caching and coalesces concurrent reads.
package graphcache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

var revision atomic.Uint64

func Revision() uint64 { return revision.Load() }
func Invalidate()      { revision.Add(1) }

type entry struct {
	body    []byte
	expires time.Time
	used    uint64
}
type flight struct {
	done chan struct{}
	body []byte
	err  error
}
type Cache struct {
	mu      sync.Mutex
	items   map[string]entry
	pending map[string]*flight
	bytes   int
	clock   uint64
}

func New() *Cache { return &Cache{items: map[string]entry{}, pending: map[string]*flight{}} }

func (c *Cache) Get(ctx context.Context, key string, load func() ([]byte, error)) ([]byte, bool, error) {
	c.mu.Lock()
	c.clock++
	for k, v := range c.items {
		if time.Now().After(v.expires) {
			delete(c.items, k)
			c.bytes -= len(v.body)
		}
	}
	if item, ok := c.items[key]; ok {
		item.used = c.clock
		c.items[key] = item
		c.mu.Unlock()
		return item.body, true, nil
	}
	if current, ok := c.pending[key]; ok {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-current.done:
			return current.body, true, current.err
		}
	}
	current := &flight{done: make(chan struct{})}
	c.pending[key] = current
	c.mu.Unlock()
	body, err := load()
	c.mu.Lock()
	defer c.mu.Unlock()
	current.body, current.err = body, err
	if err == nil && len(body) <= 1<<20 {
		for len(c.items) >= 64 || c.bytes+len(body) > 8<<20 {
			oldest := ""
			var stamp uint64
			for k, v := range c.items {
				if oldest == "" || v.used < stamp {
					oldest = k
					stamp = v.used
				}
			}
			if oldest == "" {
				break
			}
			c.bytes -= len(c.items[oldest].body)
			delete(c.items, oldest)
		}
		c.clock++
		c.items[key] = entry{body: body, expires: time.Now().Add(15 * time.Second), used: c.clock}
		c.bytes += len(body)
	}
	delete(c.pending, key)
	close(current.done)
	return body, false, err
}
