package languages

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewRuby detects the Ruby interpreter.
func NewRuby(p probe.Prober) detector.Detector {
	return simple.Tool{
		ItemName: "Ruby",
		Cat:      detector.CategoryLanguage,
		Bins:     []string{"ruby"},
		P:        p,
	}
}
