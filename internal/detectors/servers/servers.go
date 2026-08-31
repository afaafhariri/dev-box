package servers

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// All returns every server detector, wired to p.
func All(p probe.Prober) []detector.Detector {
	return []detector.Detector{
		NewDocker(p),
		NewRedis(p),
		NewPostgres(p),
		NewMySQL(p),
		NewMongoDB(p),
		NewNginx(p),
		NewCaddy(p),
	}
}

// firstBinary is the shared PATH lookup, re-exported here so the bespoke
// detectors in this package read the same as the simple ones.
func firstBinary(p probe.Prober, names []string) (bin, path string, err error) {
	return simple.FirstBinary(p, names)
}
