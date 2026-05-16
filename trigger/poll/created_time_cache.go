package poll

import (
	"sync"
	"time"
)

type createdTimeCache struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func newCreatedTimeCache() *createdTimeCache {
	return &createdTimeCache{items: map[string]time.Time{}}
}

func (c *createdTimeCache) Get(key string) (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.items[key]
	return t, ok
}

func (c *createdTimeCache) Set(key string, t time.Time) {
	if c == nil || t.IsZero() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = t
}
