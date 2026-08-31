// Package parse holds the small string helpers detectors share when reading
// version output from command line tools.
package parse

import (
	"regexp"
	"strings"
)

// versionPattern matches the first dotted number in a string, which covers
// every form the Phase 1 tools emit:
//
//	go version go1.22.3 darwin/arm64
//	Python 3.12.4
//	v22.3.0
//	git version 2.39.5 (Apple Git-154)
//	Redis server v=7.2.4 sha=00000000:0
var versionPattern = regexp.MustCompile(`\d+(?:\.\d+)+(?:[-+][0-9A-Za-z.]+)?`)

// Version extracts the first version-looking token from command output,
// returning "" when there is none.
func Version(out string) string {
	return versionPattern.FindString(out)
}

// FirstLine returns the first non-empty, trimmed line of output.
func FirstLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// Lines returns the non-empty, trimmed lines of output.
func Lines(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
