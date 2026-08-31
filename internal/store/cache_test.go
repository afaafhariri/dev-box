package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"devenv/internal/detector"
)

func tempCache(t *testing.T) *Cache {
	t.Helper()
	return NewCache(filepath.Join(t.TempDir(), "cache.json"), time.Minute)
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	c := tempCache(t)
	items := []detector.Item{
		{Name: "Go", Category: detector.CategoryLanguage, Version: "1.22.3", Status: detector.StatusInstalled,
			Meta: map[string]string{"platform": "darwin/arm64"}, DetectedAt: time.Now()},
		{Name: "Redis", Category: detector.CategoryServer, Status: detector.StatusRunning},
	}

	if err := c.Save(items); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	snap, ok := c.Load()
	if !ok {
		t.Fatal("Load() reported no usable cache after Save")
	}
	if len(snap.Items) != 2 {
		t.Fatalf("loaded %d items, want 2", len(snap.Items))
	}
	if snap.Items[0].Version != "1.22.3" {
		t.Errorf("Version = %q, want 1.22.3", snap.Items[0].Version)
	}
	if snap.Items[0].Meta["platform"] != "darwin/arm64" {
		t.Error("Meta did not survive the round trip")
	}
	if snap.CachedAt.IsZero() {
		t.Error("CachedAt not stamped")
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	// A first run has no cache; that is ordinary, not a failure to report.
	if _, ok := tempCache(t).Load(); ok {
		t.Error("Load() reported a usable cache where no file exists")
	}
}

func TestLoadRejectsCorruptCache(t *testing.T) {
	c := tempCache(t)
	if err := os.WriteFile(c.Path, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok := c.Load(); ok {
		t.Error("Load() accepted a corrupt cache — it must fall back to scanning")
	}
}

func TestLoadRejectsAnOlderCacheVersion(t *testing.T) {
	c := tempCache(t)
	if err := os.WriteFile(c.Path, []byte(`{"version":0,"items":[{"name":"Go"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, ok := c.Load(); ok {
		t.Error("Load() accepted a cache written by an older shape")
	}
}

func TestSaveCreatesTheDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "cache.json")
	c := NewCache(path, time.Minute)

	if err := c.Save([]detector.Item{{Name: "Go"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("cache file not created: %v", err)
	}
}

func TestSaveLeavesNoTempFilesBehind(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(filepath.Join(dir, "cache.json"), time.Minute)

	for i := 0; i < 3; i++ {
		if err := c.Save([]detector.Item{{Name: "Go"}}); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("cache directory holds %d files, want just the cache", len(entries))
	}
}

func TestSaveIgnoresAnEmptyScan(t *testing.T) {
	c := tempCache(t)
	if err := c.Save(nil); err != nil {
		t.Fatalf("Save(nil) error = %v", err)
	}
	// Writing an empty snapshot would erase a good cache after a failed scan.
	if _, err := os.Stat(c.Path); !os.IsNotExist(err) {
		t.Error("Save(nil) wrote a cache file")
	}
}

func TestStale(t *testing.T) {
	c := NewCache(filepath.Join(t.TempDir(), "cache.json"), time.Minute)

	fresh := Snapshot{CachedAt: time.Now().Add(-30 * time.Second)}
	if c.Stale(fresh) {
		t.Error("a 30s-old snapshot is stale under a 1m TTL")
	}

	old := Snapshot{CachedAt: time.Now().Add(-2 * time.Minute)}
	if !c.Stale(old) {
		t.Error("a 2m-old snapshot is fresh under a 1m TTL")
	}
}

func TestZeroTTLFallsBackToTheDefault(t *testing.T) {
	c := NewCache(filepath.Join(t.TempDir(), "cache.json"), 0)

	if c.Stale(Snapshot{CachedAt: time.Now().Add(-time.Minute)}) {
		t.Error("a 1m-old snapshot is stale under the 5m default TTL")
	}
	if !c.Stale(Snapshot{CachedAt: time.Now().Add(-time.Hour)}) {
		t.Error("an hour-old snapshot is fresh under the 5m default TTL")
	}
}

func TestEmptyPathIsANoOp(t *testing.T) {
	c := NewCache("", time.Minute)

	if err := c.Save([]detector.Item{{Name: "Go"}}); err != nil {
		t.Errorf("Save() with no path error = %v, want a silent no-op", err)
	}
	if _, ok := c.Load(); ok {
		t.Error("Load() with no path reported a cache")
	}
}

func TestPrimeFillsTheStoreWithoutNotifying(t *testing.T) {
	s := New()
	defer s.Close()

	cachedAt := time.Now().Add(-time.Minute)
	sub := s.Subscribe()

	s.Prime(Snapshot{CachedAt: cachedAt, Items: []detector.Item{
		{Name: "Go", Category: detector.CategoryLanguage, Status: detector.StatusInstalled},
	}})

	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1", s.Len())
	}
	if !s.PrimedAt().Equal(cachedAt) {
		t.Errorf("PrimedAt() = %v, want %v", s.PrimedAt(), cachedAt)
	}

	// Priming happens before the UI subscribes; a notification storm here
	// would be pure noise.
	select {
	case got := <-sub:
		t.Errorf("Prime notified subscribers with %q", got.Name)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFreshScanOverwritesPrimedItems(t *testing.T) {
	s := New()
	defer s.Close()

	s.Prime(Snapshot{CachedAt: time.Now(), Items: []detector.Item{
		{Name: "Docker", Category: detector.CategoryServer, Status: detector.StatusRunning},
	}})
	s.Set(detector.Item{Name: "Docker", Category: detector.CategoryServer, Status: detector.StatusStopped})

	if got, _ := s.Get("Docker"); got.Status != detector.StatusStopped {
		t.Errorf("Status = %q, want the fresh scan to win over the cache", got.Status)
	}
}

func TestDefaultPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")

	got, err := DefaultPath("cache.json")
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if got != "/tmp/xdg/devenv/cache.json" {
		t.Errorf("DefaultPath() = %q", got)
	}
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	got, err := DefaultPath("config.toml")
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	home, _ := os.UserHomeDir()
	if want := filepath.Join(home, ".config", "devenv", "config.toml"); got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}
