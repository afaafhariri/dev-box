// Package config loads the user's TOML configuration.
//
// Configuration is entirely optional: a machine with no config file gets the
// defaults, which is the case this package is tuned for.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"devenv/internal/probe"
	"devenv/internal/store"
)

// Config is the whole configuration file.
type Config struct {
	Scan      Scan      `toml:"scan"`
	Detectors Detectors `toml:"detectors"`
}

// Scan holds scan-wide settings.
type Scan struct {
	// Timeout bounds any single detector command.
	Timeout Duration `toml:"timeout"`
	// CacheTTL is how long a cached scan is treated as fresh.
	CacheTTL Duration `toml:"cache_ttl"`
	// NoCache disables reading and writing the disk cache entirely.
	NoCache bool `toml:"no_cache"`
}

// Detectors selects which detectors run.
type Detectors struct {
	// Disabled lists detector names to skip, matched case-insensitively.
	Disabled []string `toml:"disabled"`
}

// Default is the configuration used when there is no file.
func Default() Config {
	return Config{
		Scan: Scan{
			Timeout:  Duration(probe.DefaultTimeout),
			CacheTTL: Duration(store.DefaultTTL),
		},
	}
}

// Load reads the configuration at path.
//
// A missing file yields the defaults and no error — that is the normal case,
// not a problem. A malformed file is an error, because silently ignoring what
// someone deliberately wrote would be worse than saying so.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse %s: %w", path, err)
	}

	// An explicitly zeroed value in the file still means "use the default",
	// since zero is never a sensible timeout or TTL.
	if cfg.Scan.Timeout <= 0 {
		cfg.Scan.Timeout = Duration(probe.DefaultTimeout)
	}
	if cfg.Scan.CacheTTL <= 0 {
		cfg.Scan.CacheTTL = Duration(store.DefaultTTL)
	}
	return cfg, nil
}

// IsDisabled reports whether a detector name was switched off.
func (c Config) IsDisabled(name string) bool {
	for _, disabled := range c.Detectors.Disabled {
		if strings.EqualFold(disabled, name) {
			return true
		}
	}
	return false
}

// Duration is a time.Duration that reads from TOML as a string ("5s", "2m").
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler for TOML decoding.
func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", text, err)
	}
	*d = Duration(parsed)
	return nil
}

// MarshalText implements encoding.TextMarshaler, so a config can be written
// back out in the same form it was read.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Duration converts back to a time.Duration.
func (d Duration) Duration() time.Duration { return time.Duration(d) }
