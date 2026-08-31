package tools

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// tool is the shorthand these one-line detectors share.
func tool(p probe.Prober, name string, bins []string, args ...string) detector.Detector {
	return simple.Tool{
		ItemName: name,
		Cat:      detector.CategoryTool,
		Bins:     bins,
		Args:     args,
		P:        p,
	}
}

// NewGH detects the GitHub CLI.
func NewGH(p probe.Prober) detector.Detector {
	return tool(p, "GitHub CLI", []string{"gh"})
}

// NewKubectl detects kubectl.
func NewKubectl(p probe.Prober) detector.Detector {
	// Without --client, kubectl blocks trying to reach a cluster.
	return tool(p, "kubectl", []string{"kubectl"}, "version", "--client")
}

// NewHelm detects Helm.
func NewHelm(p probe.Prober) detector.Detector {
	return tool(p, "Helm", []string{"helm"}, "version", "--short")
}

// NewMake detects make.
func NewMake(p probe.Prober) detector.Detector {
	return tool(p, "Make", []string{"make", "gmake"})
}

// NewCMake detects CMake.
func NewCMake(p probe.Prober) detector.Detector {
	return tool(p, "CMake", []string{"cmake"})
}

// NewCargo detects Cargo, Rust's package manager.
func NewCargo(p probe.Prober) detector.Detector {
	return tool(p, "Cargo", []string{"cargo"})
}

// NewHomebrew detects Homebrew.
func NewHomebrew(p probe.Prober) detector.Detector {
	return tool(p, "Homebrew", []string{"brew"})
}

// NewPnpm detects pnpm.
func NewPnpm(p probe.Prober) detector.Detector {
	return tool(p, "pnpm", []string{"pnpm"})
}

// NewYarn detects Yarn.
func NewYarn(p probe.Prober) detector.Detector {
	return tool(p, "Yarn", []string{"yarn"})
}

// NewMaven detects Apache Maven.
func NewMaven(p probe.Prober) detector.Detector {
	return tool(p, "Maven", []string{"mvn"}, "--version")
}

// NewGradle detects Gradle.
func NewGradle(p probe.Prober) detector.Detector {
	return tool(p, "Gradle", []string{"gradle"}, "--version")
}
