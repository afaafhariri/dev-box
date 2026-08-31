package managers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewNVM detects nvm.
//
// nvm is a shell function sourced into the user's profile, not a binary on
// PATH, so the install directory is the only reliable signal.
func NewNVM(p probe.Prober) detector.Detector {
	return simple.Manager{
		ItemName: "nvm",
		Cat:      detector.CategoryManager,
		Dirs:     []string{"~/.nvm"},
		P:        p,
	}
}
