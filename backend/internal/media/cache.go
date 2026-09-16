package media

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const fileExt = ".mp3"

// Cache tracks transcoded files under dir, evicting least-recently-accessed
// files once the total size exceeds maxBytes.
type Cache struct {
	dir      string
	maxBytes int64

	mu     sync.Mutex
	access map[string]time.Time // trackID -> last access time
	sizes  map[string]int64     // trackID -> file size in bytes
}

// NewCache creates the cache directory if needed and rebuilds the access-time
// map from existing file mtimes. Leftover temp files from an interrupted
// transcode are removed.
func NewCache(dir string, maxBytes int64) (*Cache, error) {
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
		name := e.Name()
		if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".tmp") {
			_ = os.Remove(filepath.Join(dir, name))
			continue
		}
		trackID, ok := strings.CutSuffix(name, fileExt)
		if !ok || trackID == "" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		c.access[trackID] = info.ModTime()
		c.sizes[trackID] = info.Size()
	}
	c.evictIfNeeded()
	return c, nil
}

// Path returns the on-disk path for a track's cached file.
func (c *Cache) Path(trackID string) string {
	return filepath.Join(c.dir, trackID+fileExt)
}

// TempPath returns a temp file path for an in-progress transcode of trackID.
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

// Commit registers a newly-written file of the given size and evicts older
// entries if the cache now exceeds its budget.
func (c *Cache) Commit(trackID string, size int64) {
	c.mu.Lock()
	c.access[trackID] = time.Now()
	c.sizes[trackID] = size
	c.mu.Unlock()
	c.evictIfNeeded()
}

// evictIfNeeded deletes least-recently-accessed files until the cache is
// under 90% of its byte budget.
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
