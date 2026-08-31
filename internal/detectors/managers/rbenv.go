package managers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewRbenv detects rbenv.
func NewRbenv(p probe.Prober) detector.Detector {
	return simple.Manager{
		ItemName: "rbenv",
		Cat:      detector.CategoryManager,
		Bins:     []string{"rbenv"},
		Dirs:     []string{"~/.rbenv"},
		P:        p,
	}
}
