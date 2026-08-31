package servers

import (
	"devenv/internal/detector"
	"devenv/internal/probe"
)

// All returns every server detector, wired to p.
func All(p probe.Prober) []detector.Detector {
	return []detector.Detector{
		NewDocker(p),
		NewRedis(p),
	}
}

// firstBinary returns the first of names that resolves on PATH. It reports
// ErrNotInstalled when none do.
func firstBinary(p probe.Prober, names []string) (bin, path string, err error) {
	for _, name := range names {
		if path, err := p.LookPath(name); err == nil {
			return name, path, nil
		}
	}
	return "", "", detector.ErrNotInstalled
}
