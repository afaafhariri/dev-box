package ai

import (
	"devenv/internal/detector"
	"devenv/internal/probe"
)

// All returns every AI/ML detector, wired to p.
func All(p probe.Prober) []detector.Detector {
	return []detector.Detector{
		NewOllama(p),
	}
}
