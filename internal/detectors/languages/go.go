// Package languages detects programming language toolchains.
package languages

import (
	"context"
	"fmt"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// Go detects the Go toolchain.
type Go struct{ p probe.Prober }

// NewGo returns a Go detector reading through p.
func NewGo(p probe.Prober) Go { return Go{p: p} }

func (d Go) Name() string                { return "Go" }
func (d Go) Category() detector.Category { return detector.CategoryLanguage }

func (d Go) Detect(ctx context.Context) (detector.Item, error) {
	path, err := d.p.LookPath("go")
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	out, err := d.p.Run(ctx, "go", "version")
	if err != nil {
		return detector.Item{}, fmt.Errorf("go version: %w", err)
	}

	version, platform := parseGoVersion(out)
	if version == "" {
		return detector.Item{}, fmt.Errorf("unrecognised go version output: %q", parse.FirstLine(out))
	}

	item := detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  version,
		Path:     path,
		Status:   detector.StatusInstalled,
	}
	return item.WithMeta("platform", platform), nil
}

// parseGoVersion reads "go version go1.22.3 darwin/arm64".
func parseGoVersion(out string) (version, platform string) {
	fields := strings.Fields(out)
	for i, f := range fields {
		if !strings.HasPrefix(f, "go1") && !strings.HasPrefix(f, "go2") {
			continue
		}
		version = strings.TrimPrefix(f, "go")
		if i+1 < len(fields) && strings.Contains(fields[i+1], "/") {
			platform = fields[i+1]
		}
		return version, platform
	}
	return "", ""
}
