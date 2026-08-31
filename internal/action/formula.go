package action

import (
	"path/filepath"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

// brewPrefixes are the standard Homebrew install roots: Apple silicon, Intel
// macOS, and Linuxbrew.
var brewPrefixes = []string{
	"/opt/homebrew/",
	"/usr/local/Cellar/",
	"/usr/local/opt/",
	"/usr/local/bin/",
	"/home/linuxbrew/",
}

// IsBrewPath reports whether a path lies inside a Homebrew prefix.
func IsBrewPath(path string) bool {
	for _, prefix := range brewPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// formula works out which Homebrew formula owns an item.
//
// Guessing from the command name is not good enough: PostgreSQL's binary is
// psql but its formula is postgresql@15, and passing the wrong name to
// "brew uninstall" would act on the wrong package — or, worse, on a package
// the user did not mean. Homebrew puts the truth in its own layout, so the
// binary is resolved through its symlink and the formula read from the path:
//
//	/opt/homebrew/bin/psql
//	  -> /opt/homebrew/Cellar/postgresql@15/15.19/bin/psql
//	                          ^^^^^^^^^^^^^
//
// An empty return means the formula could not be established, and the caller
// must not run anything.
func formula(p probe.Prober, item detector.Item) string {
	if item.Path == "" || !IsBrewPath(item.Path) {
		return ""
	}

	resolved, err := p.Resolve(item.Path)
	if err != nil {
		return ""
	}
	return formulaFromPath(resolved)
}

// formulaFromPath reads the formula name out of a resolved Homebrew path.
func formulaFromPath(path string) string {
	// Cellar holds the real installs; opt holds version-independent symlinks
	// into them. Either names the formula in the segment that follows.
	//
	// The last occurrence is the one that counts: the Apple silicon prefix is
	// itself /opt/homebrew, so "/opt/homebrew/opt/node@20/bin/node" contains
	// the marker twice and only the second names a formula.
	for _, marker := range []string{"/Cellar/", "/opt/"} {
		i := strings.LastIndex(path, marker)
		if i < 0 {
			continue
		}
		name, _, _ := strings.Cut(path[i+len(marker):], "/")
		if name != "" && name != "bin" && name != "homebrew" {
			return name
		}
	}
	return ""
}

// binaryName is the command an item was detected by, used only for display.
func binaryName(item detector.Item) string {
	if bin := item.Meta["binary"]; bin != "" {
		return bin
	}
	return filepath.Base(item.Path)
}
