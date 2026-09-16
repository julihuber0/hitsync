package years

import (
	"container/list"
	"sync"
	"time"
)

// cacheEntry holds a cached Discogs lookup result, including negative
// results (found=false), per §9.5.
type cacheEntry struct {
	key       string
	year      int
	found     bool
	source    string
	expiresAt time.Time
}

// lruCache is a process-local, in-memory-only LRU cache keyed on normalised
// (title, artist). Nothing here is ever persisted; it is empty again after a
// restart (§9.5).
type lruCache struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	ll       *list.List
	items    map[string]*list.Element
}

func newLRUCache(capacity int, ttl time.Duration) *lruCache {
	if capacity <= 0 {
		capacity = 2000
	}
	return &lruCache{
		capacity: capacity,
		ttl:      ttl,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

func cacheKey(title, artist string) string {
	return NormalizeTitle(title) + "\x00" + NormalizeArtist(artist)
}

// get returns the cached entry and whether it is present and unexpired.
func (c *lruCache) get(key string) (cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return cacheEntry{}, false
	}
	entry := el.Value.(cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.ll.Remove(el)
		delete(c.items, key)
		return cacheEntry{}, false
	}
	c.ll.MoveToFront(el)
	return entry, true
}

func (c *lruCache) set(key string, year int, found bool, source string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := cacheEntry{key: key, year: year, found: found, source: source, expiresAt: time.Now().Add(c.ttl)}
	if el, ok := c.items[key]; ok {
		el.Value = entry
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(entry)
	c.items[key] = el

	for c.ll.Len() > c.capacity {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.ll.Remove(oldest)
		delete(c.items, oldest.Value.(cacheEntry).key)
	}
}

// Len returns the current number of cached entries (for admin stats).
func (c *lruCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
