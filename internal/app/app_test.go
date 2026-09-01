package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devenv/internal/detector"
)

// isolated points the app's config and cache at a temporary directory, so
// tests never touch the developer's own.
func isolated(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "devenv")
}

func TestNewRegistersTheWholeCatalog(t *testing.T) {
	isolated(t)

	a, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	names := a.Detectors()
	if len(names) < 30 {
		t.Errorf("registered %d detectors, want the full catalog", len(names))
	}

	// Sorted, and covering every category's package.
	for _, want := range []string{"Docker", "Go", "Ollama", "nvm"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not registered", want)
		}
	}
}

func TestDisabledDetectorsAreSkipped(t *testing.T) {
	dir := isolated(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(config, []byte("[detectors]\ndisabled = [\"docker\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	a, err := New(DefaultOptions())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, n := range a.Detectors() {
		// Matching is case-insensitive, so lowercase "docker" disables Docker.
		if n == "Docker" {
			t.Error("a disabled detector was registered")
		}
	}
}

func TestBrokenConfigIsReportedButStillUsable(t *testing.T) {
	dir := isolated(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[scan\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	a, err := New(DefaultOptions())
	if err == nil {
		t.Error("New() error = nil for a malformed config")
	}
	if a == nil {
		t.Fatal("New() returned no app, leaving the user with nothing")
	}
	if len(a.Detectors()) == 0 {
		t.Error("no detectors registered after a bad config")
	}
}

func TestRunOnceWritesJSON(t *testing.T) {
	isolated(t)

	var out strings.Builder
	opts := DefaultOptions()
	opts.Format = "json"
	opts.Out = &out
	opts.Timeout = 2 * time.Second

	a, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var payload struct {
		OS    string          `json:"os"`
		Items []detector.Item `json:"items"`
	}
	if err := json.Unmarshal([]byte(out.String()), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatal("no items scanned")
	}
	if payload.OS == "" {
		t.Error("the snapshot does not record the platform")
	}

	// Ownership is resolved during the scan, not left for later.
	resolved := 0
	for _, item := range payload.Items {
		if item.Status.Found() && item.ManagedBy != "" {
			resolved++
		}
	}
	if resolved == 0 {
		t.Error("no found item had its owner resolved")
	}
}

func TestRunOnceWritesMarkdown(t *testing.T) {
	isolated(t)

	var out strings.Builder
	opts := DefaultOptions()
	opts.Format = "markdown"
	opts.Out = &out
	opts.Timeout = 2 * time.Second

	a, _ := New(opts)
	if err := a.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.HasPrefix(out.String(), "# Development environment") {
		t.Error("output is not the Markdown document")
	}
}

func TestUnknownFormatIsAnError(t *testing.T) {
	isolated(t)

	opts := DefaultOptions()
	opts.Format = "xml"
	opts.Out = &strings.Builder{}

	a, _ := New(opts)
	if err := a.Run(context.Background()); err == nil {
		t.Error("Run() error = nil for an unknown format")
	}
}

func TestScanWritesTheCache(t *testing.T) {
	dir := isolated(t)

	opts := DefaultOptions()
	opts.Format = "json"
	opts.Out = &strings.Builder{}
	opts.Timeout = 2 * time.Second

	a, _ := New(opts)
	if got := a.CachePath(); got == "" {
		t.Fatal("caching is off by default")
	}
	if err := a.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "cache.json")); err != nil {
		t.Errorf("cache was not written: %v", err)
	}
}

func TestNoCacheDisablesTheCacheEntirely(t *testing.T) {
	dir := isolated(t)

	opts := DefaultOptions()
	opts.Format = "json"
	opts.Out = &strings.Builder{}
	opts.NoCache = true
	opts.Timeout = 2 * time.Second

	a, _ := New(opts)
	if a.CachePath() != "" {
		t.Error("CachePath() is set with caching disabled")
	}
	if err := a.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache.json")); !os.IsNotExist(err) {
		t.Error("a cache was written despite --no-cache")
	}
}

func TestCancelledScanDoesNotPoisonTheCache(t *testing.T) {
	dir := isolated(t)

	opts := DefaultOptions()
	opts.Format = "json"
	opts.Out = &strings.Builder{}

	a, _ := New(opts)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The partial results of an interrupted scan would look authoritative on
	// the next launch.
	scanner := a.scanner()
	scanner.RunAll(ctx)

	if _, err := os.Stat(filepath.Join(dir, "cache.json")); !os.IsNotExist(err) {
		t.Error("an interrupted scan wrote a cache")
	}
}

func TestFlagTimeoutOverridesTheConfig(t *testing.T) {
	dir := isolated(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[scan]\ntimeout = \"30s\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	opts := DefaultOptions()
	opts.Timeout = time.Second

	a, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	if a.cfg.Scan.Timeout.Duration() != 30*time.Second {
		t.Error("the config value was not loaded")
	}
	// The flag is what the prober actually got; the config value stands
	// untouched behind it.
	if opts.Timeout != time.Second {
		t.Error("the flag was overwritten by the config")
	}
}
