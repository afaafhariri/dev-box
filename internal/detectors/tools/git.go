// Package tools detects general purpose command line development tools.
package tools

import (
	"context"
	"fmt"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// Git detects the git client.
type Git struct{ p probe.Prober }

// NewGit returns a Git detector reading through p.
func NewGit(p probe.Prober) Git { return Git{p: p} }

func (d Git) Name() string                { return "Git" }
func (d Git) Category() detector.Category { return detector.CategoryTool }

func (d Git) Detect(ctx context.Context) (detector.Item, error) {
	path, err := d.p.LookPath("git")
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	out, err := d.p.Run(ctx, "git", "--version")
	if err != nil {
		return detector.Item{}, fmt.Errorf("git --version: %w", err)
	}

	version := parse.Version(out) // "git version 2.39.5 (Apple Git-154)"
	if version == "" {
		return detector.Item{}, fmt.Errorf("unrecognised git version output: %q", parse.FirstLine(out))
	}

	return detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  version,
		Path:     path,
		Status:   detector.StatusInstalled,
	}, nil
}
