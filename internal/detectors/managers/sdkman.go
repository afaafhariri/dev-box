package managers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewSDKMAN detects SDKMAN!, which like nvm is a sourced shell function.
func NewSDKMAN(p probe.Prober) detector.Detector {
	return simple.Manager{
		ItemName: "sdkman",
		Cat:      detector.CategoryManager,
		Dirs:     []string{"~/.sdkman"},
		P:        p,
	}
}
