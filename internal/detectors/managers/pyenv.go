package managers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewPyenv detects pyenv.
func NewPyenv(p probe.Prober) detector.Detector {
	return simple.Manager{
		ItemName: "pyenv",
		Cat:      detector.CategoryManager,
		Bins:     []string{"pyenv"},
		Dirs:     []string{"~/.pyenv"},
		P:        p,
	}
}
