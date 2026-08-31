package languages

import (
	"devenv/internal/detector"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewJava detects the Java runtime.
//
// java reports its version on stderr and exits non-zero on some builds, which
// is why the probe merges the streams and simple.Tool judges output before
// error.
func NewJava(p probe.Prober) detector.Detector {
	return simple.Tool{
		ItemName: "Java",
		Cat:      detector.CategoryLanguage,
		Bins:     []string{"java"},
		Args:     []string{"-version"},
		P:        p,
	}
}
