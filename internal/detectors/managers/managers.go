// Package managers detects language version managers. These are usually shell
// functions rather than binaries, so most are found by their install
// directory.
package managers

import (
	"devenv/internal/detector"
	"devenv/internal/probe"
)

// All returns every version manager detector, wired to p.
func All(p probe.Prober) []detector.Detector {
	return []detector.Detector{
		NewNVM(p),
		NewPyenv(p),
		NewSDKMAN(p),
		NewMise(p),
		NewRbenv(p),
	}
}
