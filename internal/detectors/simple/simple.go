// Package simple provides the reusable detector shapes. Most of the catalog
// falls into one of three patterns — a tool identified by running a binary, a
// service that also listens on a port, and a manager identified by a directory
// on disk — so those patterns live here once rather than in twenty near
// identical files.
//
// Detectors with genuinely bespoke logic (Go, Docker, Ollama) implement
// detector.Detector directly instead.
package simple

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// Enricher adds tool-specific detail to an item after the version is known.
// It receives the raw output the version was parsed from, so a detector can
// read more out of it without running the command a second time.
type Enricher func(ctx context.Context, p probe.Prober, item detector.Item, out string) detector.Item

// Tool is detected by running a binary and reading a version from its output.
type Tool struct {
	// ItemName is the display name.
	ItemName string
	// Cat is the item's category.
	Cat detector.Category
	// Bins are candidate binaries, tried in order; the first on PATH wins.
	Bins []string
	// Args are the version arguments. Defaults to {"--version"}.
	Args []string
	// Meta is static metadata attached to every result.
	Meta map[string]string
	// Enrich optionally adds more detail.
	Enrich Enricher
	// P is the system probe.
	P probe.Prober
}

func (t Tool) Name() string                { return t.ItemName }
func (t Tool) Category() detector.Category { return t.Cat }

func (t Tool) Detect(ctx context.Context) (detector.Item, error) {
	bin, path, err := FirstBinary(t.P, t.Bins)
	if err != nil {
		return detector.Item{}, err
	}

	args := t.Args
	if len(args) == 0 {
		args = []string{"--version"}
	}

	// Some tools (java, ollama with its daemon down) print a usable version
	// and still exit non-zero, so the output is judged before the error.
	out, runErr := t.P.Run(ctx, bin, args...)
	version := parse.Version(out)
	if version == "" && runErr != nil {
		return detector.Item{}, fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), runErr)
	}

	item := detector.Item{
		Name:     t.ItemName,
		Category: t.Cat,
		Version:  version,
		Path:     path,
		Status:   detector.StatusInstalled,
	}
	for k, v := range t.Meta {
		item = item.WithMeta(k, v)
	}
	// Which of several candidates answered is only interesting when there was
	// a choice to make.
	if len(t.Bins) > 1 {
		item = item.WithMeta("binary", bin)
	}
	if t.Enrich != nil {
		item = t.Enrich(ctx, t.P, item, out)
	}
	return item, nil
}

// Service is a Tool that also reports whether it is answering on its port.
type Service struct {
	Tool
	// Port is the default port the service listens on.
	Port int
	// Ping, when set, is a command run to confirm the listener really is this
	// service. Its first output line lands in Meta["ping"].
	Ping []string
}

func (s Service) Detect(ctx context.Context) (detector.Item, error) {
	item, err := s.Tool.Detect(ctx)
	if err != nil {
		return item, err
	}

	item = item.WithMeta("port", strconv.Itoa(s.Port))

	// Installed says nothing about running, so assume down until a dial
	// proves otherwise. The dial is far cheaper than shelling out, and it
	// gates the ping — a CLI told to reach an absent server blocks.
	item.Status = detector.StatusStopped
	if !s.P.PortOpen(ctx, "127.0.0.1", s.Port) {
		return item, nil
	}
	item.Status = detector.StatusRunning

	if len(s.Ping) > 0 {
		if out, err := s.P.Run(ctx, s.Ping[0], s.Ping[1:]...); err == nil {
			item = item.WithMeta("ping", parse.FirstLine(out))
		}
	}
	return item, nil
}

// Manager is detected by a directory on disk, since version managers are
// often shell functions with no binary of their own.
type Manager struct {
	// ItemName is the display name.
	ItemName string
	// Cat is the item's category.
	Cat detector.Category
	// Bins are candidate binaries, tried before the directories.
	Bins []string
	// Args are the version arguments. Defaults to {"--version"}.
	Args []string
	// Dirs are install locations, "~" expanded against the user's home.
	Dirs []string
	// P is the system probe.
	P probe.Prober
}

func (m Manager) Name() string                { return m.ItemName }
func (m Manager) Category() detector.Category { return m.Cat }

func (m Manager) Detect(ctx context.Context) (detector.Item, error) {
	item := detector.Item{
		Name:     m.ItemName,
		Category: m.Cat,
		Status:   detector.StatusInstalled,
	}

	// A binary is the better source: it gives a version as well as presence.
	if bin, path, err := FirstBinary(m.P, m.Bins); err == nil {
		args := m.Args
		if len(args) == 0 {
			args = []string{"--version"}
		}
		if out, err := m.P.Run(ctx, bin, args...); err == nil {
			item.Version = parse.Version(out)
		}
		item.Path = path
		return item, nil
	}

	// nvm and sdkman are shell functions, so a directory is all there is.
	for _, dir := range m.Dirs {
		expanded := probe.Expand(m.P, dir)
		if m.P.Exists(expanded) {
			item.Path = expanded
			return item, nil
		}
	}

	return detector.Item{}, detector.ErrNotInstalled
}

// FirstBinary returns the first of names that resolves on PATH, reporting
// detector.ErrNotInstalled when none do.
func FirstBinary(p probe.Prober, names []string) (bin, path string, err error) {
	for _, name := range names {
		if path, err := p.LookPath(name); err == nil {
			return name, path, nil
		}
	}
	return "", "", detector.ErrNotInstalled
}
