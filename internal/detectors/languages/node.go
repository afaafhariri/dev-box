package languages

import (
	"context"
	"fmt"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// Node detects the Node.js runtime.
type Node struct{ p probe.Prober }

// NewNode returns a Node detector reading through p.
func NewNode(p probe.Prober) Node { return Node{p: p} }

func (d Node) Name() string                { return "Node.js" }
func (d Node) Category() detector.Category { return detector.CategoryLanguage }

func (d Node) Detect(ctx context.Context) (detector.Item, error) {
	path, err := d.p.LookPath("node")
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	out, err := d.p.Run(ctx, "node", "--version")
	if err != nil {
		return detector.Item{}, fmt.Errorf("node --version: %w", err)
	}

	version := parse.Version(out) // "v22.3.0"
	if version == "" {
		return detector.Item{}, fmt.Errorf("unrecognised node version output: %q", parse.FirstLine(out))
	}

	item := detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  version,
		Path:     path,
		Status:   detector.StatusInstalled,
	}

	// npm ships with node; showing the pair together answers the question
	// people actually have when they look up their node version.
	if _, err := d.p.LookPath("npm"); err == nil {
		if npmOut, err := d.p.Run(ctx, "npm", "--version"); err == nil {
			item = item.WithMeta("npm", parse.Version(npmOut))
		}
	}
	return item, nil
}
