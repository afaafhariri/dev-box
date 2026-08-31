package languages

import (
	"context"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewRust detects the Rust toolchain.
func NewRust(p probe.Prober) detector.Detector {
	return simple.Tool{
		ItemName: "Rust",
		Cat:      detector.CategoryLanguage,
		Bins:     []string{"rustc"},
		P:        p,
		Enrich:   rustToolchain,
	}
}

// rustToolchain adds the rustup channel, which is what people actually switch
// between when they have more than one Rust installed.
func rustToolchain(ctx context.Context, p probe.Prober, item detector.Item, _ string) detector.Item {
	if _, err := p.LookPath("rustup"); err != nil {
		return item
	}
	out, err := p.Run(ctx, "rustup", "show", "active-toolchain")
	if err != nil {
		return item
	}
	return item.WithMeta("toolchain", parse.FirstLine(out))
}
