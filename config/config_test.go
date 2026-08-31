package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"devenv/internal/probe"
	"devenv/internal/store"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMissingFileYieldsDefaults(t *testing.T) {
	// No config file is the normal case, not an error.
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatalf("Load() error = %v, want defaults and no error", err)
	}
	if cfg.Scan.Timeout.Duration() != probe.DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Scan.Timeout.Duration(), probe.DefaultTimeout)
	}
	if cfg.Scan.CacheTTL.Duration() != store.DefaultTTL {
		t.Errorf("CacheTTL = %v, want %v", cfg.Scan.CacheTTL.Duration(), store.DefaultTTL)
	}
}

func TestLoadFullConfig(t *testing.T) {
	path := writeConfig(t, `
[scan]
timeout = "12s"
cache_ttl = "30m"
no_cache = true

[detectors]
disabled = ["Caddy", "Gradle"]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Scan.Timeout.Duration(); got != 12*time.Second {
		t.Errorf("Timeout = %v, want 12s", got)
	}
	if got := cfg.Scan.CacheTTL.Duration(); got != 30*time.Minute {
		t.Errorf("CacheTTL = %v, want 30m", got)
	}
	if !cfg.Scan.NoCache {
		t.Error("NoCache = false, want true")
	}
	if len(cfg.Detectors.Disabled) != 2 {
		t.Errorf("Disabled = %v, want two entries", cfg.Detectors.Disabled)
	}
}

func TestPartialConfigKeepsDefaults(t *testing.T) {
	// Setting one field must not blank out the others.
	path := writeConfig(t, "[scan]\ntimeout = \"1s\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Scan.Timeout.Duration() != time.Second {
		t.Errorf("Timeout = %v, want 1s", cfg.Scan.Timeout.Duration())
	}
	if cfg.Scan.CacheTTL.Duration() != store.DefaultTTL {
		t.Errorf("CacheTTL = %v, want the default to survive", cfg.Scan.CacheTTL.Duration())
	}
}

func TestZeroDurationsFallBackToDefaults(t *testing.T) {
	path := writeConfig(t, "[scan]\ntimeout = \"0s\"\ncache_ttl = \"0s\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// A zero timeout would make every probe fail instantly.
	if cfg.Scan.Timeout.Duration() != probe.DefaultTimeout {
		t.Errorf("Timeout = %v, want the default", cfg.Scan.Timeout.Duration())
	}
}

func TestMalformedConfigIsReported(t *testing.T) {
	// Silently ignoring what someone deliberately wrote would be worse than
	// telling them it is broken.
	path := writeConfig(t, "[scan\ntimeout = ")

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil for malformed TOML")
	}
}

func TestInvalidDurationIsReported(t *testing.T) {
	path := writeConfig(t, "[scan]\ntimeout = \"soon\"\n")

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil for an unparseable duration")
	}
}

func TestMalformedConfigStillReturnsUsableDefaults(t *testing.T) {
	path := writeConfig(t, "[scan\n")

	cfg, _ := Load(path)
	if cfg.Scan.Timeout.Duration() != probe.DefaultTimeout {
		t.Error("a rejected config left an unusable timeout behind")
	}
}

func TestIsDisabledIsCaseInsensitive(t *testing.T) {
	cfg := Config{Detectors: Detectors{Disabled: []string{"caddy", "GRADLE"}}}

	for _, name := range []string{"Caddy", "caddy", "Gradle", "gradle"} {
		if !cfg.IsDisabled(name) {
			t.Errorf("IsDisabled(%q) = false, want true", name)
		}
	}
	if cfg.IsDisabled("Docker") {
		t.Error("IsDisabled(Docker) = true, want false")
	}
}

func TestDurationRoundTrip(t *testing.T) {
	var d Duration
	if err := d.UnmarshalText([]byte("90s")); err != nil {
		t.Fatalf("UnmarshalText() error = %v", err)
	}
	if d.Duration() != 90*time.Second {
		t.Errorf("Duration() = %v, want 90s", d.Duration())
	}

	text, err := d.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText() error = %v", err)
	}
	if string(text) != "1m30s" {
		t.Errorf("MarshalText() = %q, want 1m30s", text)
	}
}
