package tools

import (
	"devenv/internal/detector"
	"devenv/internal/probe"
)

// All returns every CLI tool detector, wired to p.
func All(p probe.Prober) []detector.Detector {
	return []detector.Detector{
		NewGit(p),
		NewGH(p),
		NewKubectl(p),
		NewHelm(p),
		NewMake(p),
		NewCMake(p),
		NewCargo(p),
		NewHomebrew(p),
		NewPnpm(p),
		NewYarn(p),
		NewMaven(p),
		NewGradle(p),
		NewNpmGlobals(p),
	}
}
