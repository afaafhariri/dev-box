// Package app wires the subsystems together: it loads configuration, builds
// the prober, registers detectors, connects the engine to the store and the
// cache, and hands the result to the TUI.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devenv/config"
	"devenv/internal/action"
	"devenv/internal/detector"
	"devenv/internal/detectors/ai"
	"devenv/internal/detectors/languages"
	"devenv/internal/detectors/managers"
	"devenv/internal/detectors/servers"
	"devenv/internal/detectors/tools"
	"devenv/internal/probe"
	"devenv/internal/store"
	"devenv/internal/tui"
)

// Options are the knobs the CLI exposes. Flags win over the config file.
type Options struct {
	// ConfigPath overrides the config file location.
	ConfigPath string
	// Timeout, when non-zero, overrides the configured per-command timeout.
	Timeout time.Duration
	// NoCache disables the disk cache for this run.
	NoCache bool
	// JSON prints one scan as JSON instead of starting the TUI.
	JSON bool
	// Out is where JSON output goes.
	Out io.Writer
}

// DefaultOptions returns the built-in defaults.
func DefaultOptions() Options {
	return Options{Out: os.Stdout}
}

// App holds the wired-up subsystems.
type App struct {
	opts     Options
	cfg      config.Config
	store    *store.Store
	registry *detector.Registry
	engine   *detector.Engine
	actions  *action.Registry
	cache    *store.Cache
}

// New builds an App from options, loading configuration from disk.
//
// A broken config file is reported but not fatal: the app runs on defaults so
// a typo cannot lock someone out of their own tool.
func New(opts Options) (*App, error) {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	path := opts.ConfigPath
	if path == "" {
		// A home directory we cannot find is not worth failing over; it just
		// means no config and no cache.
		path, _ = store.DefaultPath("config.toml")
	}

	cfg, cfgErr := config.Load(path)

	timeout := cfg.Scan.Timeout.Duration()
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	p := &probe.System{Timeout: timeout}

	registry := detector.NewRegistry()
	for _, d := range allDetectors(p) {
		if cfg.IsDisabled(d.Name()) {
			continue
		}
		registry.Register(d)
	}

	st := store.New()

	app := &App{
		opts:     opts,
		cfg:      cfg,
		store:    st,
		registry: registry,
		engine:   detector.NewEngine(st, registry.All()...),
		actions:  action.DefaultRegistry(p),
	}

	if !opts.NoCache && !cfg.Scan.NoCache {
		if cachePath, err := store.DefaultPath("cache.json"); err == nil {
			app.cache = store.NewCache(cachePath, cfg.Scan.CacheTTL.Duration())
		}
	}

	return app, cfgErr
}

// allDetectors is every detector this build knows about.
func allDetectors(p probe.Prober) []detector.Detector {
	var all []detector.Detector
	all = append(all, languages.All(p)...)
	all = append(all, servers.All(p)...)
	all = append(all, ai.All(p)...)
	all = append(all, tools.All(p)...)
	all = append(all, managers.All(p)...)
	return all
}

// Run starts the app and blocks until the user quits or a signal arrives.
func (a *App) Run(ctx context.Context) error {
	// Ctrl-C and SIGTERM cancel the context, which propagates through every
	// detector goroutine and kills their subprocesses.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer a.store.Close()

	if a.opts.JSON {
		return a.runOnce(ctx)
	}
	return a.runTUI(ctx)
}

// runOnce performs a single scan and writes the result as JSON. It is the
// headless path used for scripting and for verifying detection without a
// terminal, so it always scans fresh rather than trusting the cache.
func (a *App) runOnce(ctx context.Context) error {
	start := time.Now()
	a.engine.RunAll(ctx)
	items := a.store.All()

	if a.cache != nil {
		// A cache write failure must not fail the scan the user asked for.
		_ = a.cache.Save(items)
	}

	payload := struct {
		ScannedAt time.Time       `json:"scannedAt"`
		Elapsed   string          `json:"elapsed"`
		Items     []detector.Item `json:"items"`
	}{
		ScannedAt: start,
		Elapsed:   time.Since(start).Round(time.Millisecond).String(),
		Items:     items,
	}

	enc := json.NewEncoder(a.opts.Out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("encode scan: %w", err)
	}
	return nil
}

// runTUI starts the Bubble Tea program, priming the store from the cache first
// so the first frame has something in it.
func (a *App) runTUI(ctx context.Context) error {
	if a.cache != nil {
		if snap, ok := a.cache.Load(); ok {
			a.store.Prime(snap)
		}
	}

	model := tui.New(ctx, a.store, a.scanner(), a.actions)

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithContext(ctx),
	)

	if _, err := program.Run(); err != nil {
		// A cancelled context is the normal signal-driven exit path.
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}

// scanner returns the engine, wrapped so every completed scan updates the
// cache. The TUI drives scans, so the write has to happen underneath it.
func (a *App) scanner() tui.Scanner {
	if a.cache == nil {
		return a.engine
	}
	return &cachingScanner{engine: a.engine, store: a.store, cache: a.cache}
}

// cachingScanner saves the store to disk after each completed scan.
type cachingScanner struct {
	engine *detector.Engine
	store  *store.Store
	cache  *store.Cache
}

func (c *cachingScanner) Len() int { return c.engine.Len() }

func (c *cachingScanner) RunAll(ctx context.Context) {
	c.engine.RunAll(ctx)

	// Never persist the partial results of an interrupted scan: they would
	// look authoritative on the next launch.
	if ctx.Err() != nil {
		return
	}
	_ = c.cache.Save(c.store.All())
}

// Detectors is the list of registered detector names, for --list.
func (a *App) Detectors() []string { return a.registry.Names() }

// CachePath is where the disk cache lives, or "" when caching is off.
func (a *App) CachePath() string {
	if a.cache == nil {
		return ""
	}
	return a.cache.Path
}
