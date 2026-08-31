package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"devenv/internal/detector"
)

// DefaultTTL is how long a cached scan is considered fresh.
const DefaultTTL = 5 * time.Minute

// cacheVersion is bumped whenever the on-disk shape changes, so an old cache
// is ignored rather than misread.
const cacheVersion = 1

// Cache persists the last scan to disk, so a later launch can render
// immediately from stale data while a fresh scan runs behind it.
type Cache struct {
	// Path is the cache file location.
	Path string
	// TTL is how long a snapshot stays fresh. Zero means DefaultTTL.
	TTL time.Duration
}

// Snapshot is one cached scan.
type Snapshot struct {
	Version  int             `json:"version"`
	CachedAt time.Time       `json:"cachedAt"`
	Items    []detector.Item `json:"items"`
}

// DefaultPath is ~/.config/devenv/cache.json, honouring XDG_CONFIG_HOME.
func DefaultPath(file string) (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "devenv", file), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".config", "devenv", file), nil
}

// NewCache returns a cache at path. A zero ttl means DefaultTTL.
func NewCache(path string, ttl time.Duration) *Cache {
	return &Cache{Path: path, TTL: ttl}
}

func (c *Cache) ttl() time.Duration {
	if c.TTL > 0 {
		return c.TTL
	}
	return DefaultTTL
}

// Load reads the cached snapshot.
//
// A missing, unreadable, or unrecognised cache is not an error worth showing
// the user: the app simply starts with nothing and scans. Only genuine I/O
// trouble is reported.
func (c *Cache) Load() (Snapshot, bool) {
	if c.Path == "" {
		return Snapshot{}, false
	}

	data, err := os.ReadFile(c.Path)
	if err != nil {
		return Snapshot{}, false
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, false
	}
	if snap.Version != cacheVersion || len(snap.Items) == 0 {
		return Snapshot{}, false
	}
	return snap, true
}

// Save writes a snapshot atomically, so an interrupted write cannot leave a
// half-written cache behind for the next launch to choke on.
func (c *Cache) Save(items []detector.Item) error {
	if c.Path == "" || len(items) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	data, err := json.MarshalIndent(Snapshot{
		Version:  cacheVersion,
		CachedAt: time.Now(),
		Items:    items,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(c.Path), ".cache-*.json")
	if err != nil {
		return fmt.Errorf("create temp cache: %w", err)
	}
	tmpName := tmp.Name()
	// Best effort cleanup: harmless if the rename below already consumed it.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close cache: %w", err)
	}
	if err := os.Rename(tmpName, c.Path); err != nil {
		return fmt.Errorf("replace cache: %w", err)
	}
	return nil
}

// Stale reports whether a snapshot has aged past the TTL.
func (c *Cache) Stale(snap Snapshot) bool {
	return time.Since(snap.CachedAt) > c.ttl()
}

// Age is how long ago a snapshot was taken.
func (c *Cache) Age(snap Snapshot) time.Duration {
	return time.Since(snap.CachedAt)
}
