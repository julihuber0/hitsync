// Package cache manages the media service's on-disk MP3 cache with LRU
// eviction by access time (§10.1 of the spec).
package cache

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Cache tracks cached track files under dir, evicting least-recently-accessed
// files once the total size exceeds maxBytes.
type Cache struct {
	dir      string
	maxBytes int64

	mu     sync.Mutex
	access map[string]time.Time // trackID -> last access time
	sizes  map[string]int64     // trackID -> file size in bytes
}

// New creates the cache directory if needed and rebuilds the access-time map
// from existing file mtimes.
func New(dir string, maxBytes int64) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	c := &Cache{
		dir:      dir,
		maxBytes: maxBytes,
		access:   map[string]time.Time{},
		sizes:    map[string]int64{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		trackID := trackIDFromFilename(e.Name())
		if trackID == "" {
			continue
		}
		c.access[trackID] = info.ModTime()
		c.sizes[trackID] = info.Size()
	}
	return c, nil
}

func trackIDFromFilename(name string) string {
	const ext = ".mp3"
	if len(name) <= len(ext) || name[len(name)-len(ext):] != ext {
		return ""
	}
	return name[:len(name)-len(ext)]
}

// Path returns the on-disk path for a track's cached file.
func (c *Cache) Path(trackID string) string {
	return filepath.Join(c.dir, trackID+".mp3")
}

// TempPath returns a temp file path for an in-progress download of trackID.
func (c *Cache) TempPath(trackID string) string {
	return filepath.Join(c.dir, "."+trackID+".tmp")
}

// Has reports whether trackID is already cached on disk.
func (c *Cache) Has(trackID string) bool {
	_, err := os.Stat(c.Path(trackID))
	return err == nil
}

// Touch records a fresh access, persisting it as the file's mtime so
// recency survives a restart.
func (c *Cache) Touch(trackID string) {
	now := time.Now()
	c.mu.Lock()
	c.access[trackID] = now
	c.mu.Unlock()
	_ = os.Chtimes(c.Path(trackID), now, now)
}

// Commit registers a newly-downloaded file of the given size and evicts
// older entries if the cache now exceeds its budget.
func (c *Cache) Commit(trackID string, size int64) {
	now := time.Now()
	c.mu.Lock()
	c.access[trackID] = now
	c.sizes[trackID] = size
	c.mu.Unlock()
	c.evictIfNeeded()
}

// evictIfNeeded deletes least-recently-accessed files until the cache is
// under 90% of its byte budget (§10.1).
func (c *Cache) evictIfNeeded() {
	c.mu.Lock()
	var total int64
	for _, s := range c.sizes {
		total += s
	}
	if total <= c.maxBytes {
		c.mu.Unlock()
		return
	}

	type entry struct {
		id       string
		accessed time.Time
	}
	entries := make([]entry, 0, len(c.access))
	for id, t := range c.access {
		entries = append(entries, entry{id, t})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].accessed.Before(entries[j].accessed) })

	target := int64(float64(c.maxBytes) * 0.9)
	toDelete := []string{}
	for _, e := range entries {
		if total <= target {
			break
		}
		total -= c.sizes[e.id]
		delete(c.sizes, e.id)
		delete(c.access, e.id)
		toDelete = append(toDelete, e.id)
	}
	c.mu.Unlock()

	for _, id := range toDelete {
		_ = os.Remove(c.Path(id))
	}
}
