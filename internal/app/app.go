// Package app wires the subsystems together: it builds the prober, registers
// detectors, connects the engine to the store, and hands both to the TUI.
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

	"devenv/internal/detector"
	"devenv/internal/detectors/ai"
	"devenv/internal/detectors/languages"
	"devenv/internal/detectors/servers"
	"devenv/internal/detectors/tools"
	"devenv/internal/probe"
	"devenv/internal/store"
	"devenv/internal/tui"
)

// Options are the knobs the CLI exposes. Phase 2 reads these from a config
// file as well as flags.
type Options struct {
	// Timeout bounds any single detector command.
	Timeout time.Duration
	// JSON prints one scan as JSON instead of starting the TUI.
	JSON bool
	// Out is where JSON output goes.
	Out io.Writer
}

// DefaultOptions returns the built-in defaults.
func DefaultOptions() Options {
	return Options{Timeout: probe.DefaultTimeout, Out: os.Stdout}
}

// App holds the wired-up subsystems.
type App struct {
	opts     Options
	store    *store.Store
	registry *detector.Registry
	engine   *detector.Engine
}

// New builds an App with every Phase 1 detector registered.
func New(opts Options) *App {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	p := &probe.System{Timeout: opts.Timeout}

	registry := detector.NewRegistry()
	registry.Register(languages.All(p)...)
	registry.Register(servers.All(p)...)
	registry.Register(ai.All(p)...)
	registry.Register(tools.All(p)...)

	st := store.New()

	return &App{
		opts:     opts,
		store:    st,
		registry: registry,
		engine:   detector.NewEngine(st, registry.All()...),
	}
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
// terminal.
func (a *App) runOnce(ctx context.Context) error {
	start := time.Now()
	a.engine.RunAll(ctx)

	payload := struct {
		ScannedAt time.Time       `json:"scannedAt"`
		Elapsed   string          `json:"elapsed"`
		Items     []detector.Item `json:"items"`
	}{
		ScannedAt: start,
		Elapsed:   time.Since(start).Round(time.Millisecond).String(),
		Items:     a.store.All(),
	}

	enc := json.NewEncoder(a.opts.Out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return fmt.Errorf("encode scan: %w", err)
	}
	return nil
}

// runTUI starts the Bubble Tea program.
func (a *App) runTUI(ctx context.Context) error {
	model := tui.New(ctx, a.store, a.engine)

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

// Detectors is the list of registered detector names, for --list.
func (a *App) Detectors() []string { return a.registry.Names() }
