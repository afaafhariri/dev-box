package platform

import (
	"context"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

// Updates reports which packages have a newer version available.
//
// The check is per manager, not per item: "brew outdated" answers for every
// formula at once, where asking about each item separately would mean dozens
// of subprocesses per scan. The result is a set the resolver consults while
// tagging items.
type Updates struct {
	p probe.Prober
	// outdated maps a manager name to the set of its out-of-date packages.
	outdated map[string]map[string]bool
}

// NewUpdates returns an empty, unpopulated checker.
func NewUpdates(p probe.Prober) *Updates {
	return &Updates{p: p, outdated: make(map[string]map[string]bool)}
}

// Refresh asks every manager that can answer which of its packages are out of
// date. Managers that are not installed, or that fail, simply contribute
// nothing: an unavailable update check is not worth interrupting a scan for.
func (u *Updates) Refresh(ctx context.Context) {
	for _, m := range Managers() {
		if !m.CanCheckUpdates() {
			continue
		}
		if _, err := u.p.LookPath(m.Outdated[0]); err != nil {
			continue
		}

		out, err := u.p.Run(ctx, m.Outdated[0], m.Outdated[1:]...)
		if err != nil {
			continue
		}

		set := make(map[string]bool)
		for _, line := range strings.Split(out, "\n") {
			// Take the first field: some managers add a version column.
			if fields := strings.Fields(line); len(fields) > 0 {
				set[fields[0]] = true
			}
		}
		u.outdated[m.Name] = set
	}
}

// Outdated reports whether an item has a newer version available.
func (u *Updates) Outdated(item detector.Item) bool {
	if item.ManagedBy == "" || item.PackageID == "" {
		return false
	}
	return u.outdated[item.ManagedBy][item.PackageID]
}

// Count is how many out-of-date packages are known, across all managers.
func (u *Updates) Count() int {
	n := 0
	for _, set := range u.outdated {
		n += len(set)
	}
	return n
}
