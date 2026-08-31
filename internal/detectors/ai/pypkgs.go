package ai

import (
	"context"
	"fmt"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// PyPackage detects a Python package by asking pip about it. It covers the
// ML libraries that matter but ship no CLI of their own.
type PyPackage struct {
	// ItemName is the display name ("PyTorch").
	ItemName string
	// Dist is the distribution name pip knows it by ("torch").
	Dist string
	// P is the system probe.
	P probe.Prober
}

// NewPyPackage returns a detector for one pip-installed package.
func NewPyPackage(p probe.Prober, name, dist string) PyPackage {
	return PyPackage{ItemName: name, Dist: dist, P: p}
}

func (d PyPackage) Name() string                { return d.ItemName }
func (d PyPackage) Category() detector.Category { return detector.CategoryAI }

func (d PyPackage) Detect(ctx context.Context) (detector.Item, error) {
	bin, _, err := simple.FirstBinary(d.P, []string{"pip3", "pip"})
	if err != nil {
		// No pip means no way to answer, which is not the same as the package
		// being absent — but from the user's view the result is the same.
		return detector.Item{}, detector.ErrNotInstalled
	}

	// "pip show" exits non-zero for a package that is not installed, which is
	// the ordinary case here rather than a failure.
	out, err := d.P.Run(ctx, bin, "show", d.Dist)
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	version := showField(out, "Version")
	if version == "" {
		return detector.Item{}, fmt.Errorf("pip show %s: no version in output", d.Dist)
	}

	item := detector.Item{
		Name:     d.ItemName,
		Category: detector.CategoryAI,
		Version:  version,
		Path:     showField(out, "Location"),
		Status:   detector.StatusInstalled,
	}
	return item.WithMeta("dist", d.Dist), nil
}

// showField reads one "Key: value" line out of pip show output.
func showField(out, key string) string {
	prefix := key + ":"
	for _, line := range parse.Lines(out) {
		if rest, found := strings.CutPrefix(line, prefix); found {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}
