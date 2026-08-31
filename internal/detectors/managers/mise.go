package managers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewMise detects mise (formerly rtx).
func NewMise(p probe.Prober) detector.Detector {
	return simple.Manager{
		ItemName: "mise",
		Cat:      detector.CategoryManager,
		Bins:     []string{"mise", "rtx"},
		Dirs:     []string{"~/.local/share/mise"},
		P:        p,
	}
}
