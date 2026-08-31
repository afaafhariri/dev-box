package servers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewNginx detects nginx.
//
// Unlike the database servers, nginx gets no port liveness check: port 80 is
// shared by anything that wants it, so a dial there would report "running" for
// whatever else happens to be listening. Reporting only what is installed is
// less useful but honest. Phase 3's process probe can do better.
func NewNginx(p probe.Prober) detector.Detector {
	return simple.Tool{
		ItemName: "Nginx",
		Cat:      detector.CategoryServer,
		Bins:     []string{"nginx"},
		Args:     []string{"-v"}, // writes to stderr
		P:        p,
	}
}

// NewCaddy detects Caddy, for the same reasons and with the same caveat.
func NewCaddy(p probe.Prober) detector.Detector {
	return simple.Tool{
		ItemName: "Caddy",
		Cat:      detector.CategoryServer,
		Bins:     []string{"caddy"},
		Args:     []string{"version"},
		P:        p,
	}
}
