package languages

import (
	"context"
	"fmt"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// pythonBinaries are tried in order; python3 is preferred because a bare
// "python" is Python 2 on some older systems.
var pythonBinaries = []string{"python3", "python"}

// Python detects the Python interpreter.
type Python struct{ p probe.Prober }

// NewPython returns a Python detector reading through p.
func NewPython(p probe.Prober) Python { return Python{p: p} }

func (d Python) Name() string                { return "Python" }
func (d Python) Category() detector.Category { return detector.CategoryLanguage }

func (d Python) Detect(ctx context.Context) (detector.Item, error) {
	bin, path, err := firstBinary(d.p, pythonBinaries)
	if err != nil {
		return detector.Item{}, err
	}

	// Python 2 wrote --version to stderr; probe merges the streams for us.
	out, err := d.p.Run(ctx, bin, "--version")
	if err != nil {
		return detector.Item{}, fmt.Errorf("%s --version: %w", bin, err)
	}

	version := parse.Version(out)
	if version == "" {
		return detector.Item{}, fmt.Errorf("unrecognised python version output: %q", parse.FirstLine(out))
	}

	item := detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  version,
		Path:     path,
		Status:   detector.StatusInstalled,
	}
	item = item.WithMeta("binary", bin)

	// pip travels with Python often enough to be worth surfacing next to it.
	if pipBin, _, err := firstBinary(d.p, []string{"pip3", "pip"}); err == nil {
		if pipOut, err := d.p.Run(ctx, pipBin, "--version"); err == nil {
			item = item.WithMeta("pip", parse.Version(pipOut))
		}
	}
	return item, nil
}
