package platform

import (
	"context"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

func TestUpdatesMarksOutdatedPackages(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"brew": "/opt/homebrew/bin/brew"},
		Commands: map[string]probe.FakeResult{
			"brew outdated --quiet": {Out: "git\npostgresql@15\n"},
		},
	}

	u := NewUpdates(p)
	u.Refresh(context.Background())

	outdated := detector.Item{Name: "Git", ManagedBy: Homebrew, PackageID: "git"}
	if !u.Outdated(outdated) {
		t.Error("a listed package was not marked outdated")
	}

	current := detector.Item{Name: "pnpm", ManagedBy: Homebrew, PackageID: "pnpm"}
	if u.Outdated(current) {
		t.Error("an unlisted package was marked outdated")
	}

	if got := u.Count(); got != 2 {
		t.Errorf("Count() = %d, want 2", got)
	}
}

func TestUpdatesAsksEachManagerOnce(t *testing.T) {
	// Asking per item would mean dozens of subprocesses per scan.
	p := &probe.Fake{
		Paths:    map[string]string{"brew": "/opt/homebrew/bin/brew"},
		Commands: map[string]probe.FakeResult{"brew outdated --quiet": {Out: "git\n"}},
	}

	u := NewUpdates(p)
	u.Refresh(context.Background())

	calls := 0
	for _, c := range p.Calls {
		if c == "brew outdated --quiet" {
			calls++
		}
	}
	if calls != 1 {
		t.Errorf("brew outdated ran %d times, want once per scan", calls)
	}
}

func TestUpdatesSkipsManagersThatAreNotInstalled(t *testing.T) {
	p := &probe.Fake{}

	u := NewUpdates(p)
	u.Refresh(context.Background())

	if len(p.Calls) != 0 {
		t.Errorf("ran %v with no package manager installed", p.Calls)
	}
	if u.Count() != 0 {
		t.Errorf("Count() = %d, want 0", u.Count())
	}
}

func TestUpdatesSurvivesAFailedCheck(t *testing.T) {
	// An unavailable update check is not worth interrupting a scan for.
	p := &probe.Fake{Paths: map[string]string{"brew": "/opt/homebrew/bin/brew"}}

	u := NewUpdates(p)
	u.Refresh(context.Background())

	if u.Count() != 0 {
		t.Error("a failed check contributed results")
	}
	if u.Outdated(detector.Item{ManagedBy: Homebrew, PackageID: "git"}) {
		t.Error("a failed check marked something outdated")
	}
}

func TestUpdatesIgnoresUnownedItems(t *testing.T) {
	u := NewUpdates(&probe.Fake{})

	if u.Outdated(detector.Item{Name: "Go"}) {
		t.Error("an item with no owner was marked outdated")
	}
}

func TestResolverTagsOutdatedItems(t *testing.T) {
	p := &probe.Fake{
		HomeDir: home,
		Paths:   map[string]string{"brew": "/opt/homebrew/bin/brew"},
		Commands: map[string]probe.FakeResult{
			"brew outdated --quiet": {Out: "git\n"},
		},
		Links: map[string]string{
			"/opt/homebrew/bin/git": "/opt/homebrew/Cellar/git/2.45.2/bin/git",
		},
	}

	u := NewUpdates(p)
	u.Refresh(context.Background())

	r := NewResolverFor(p, Darwin).WithUpdates(u)
	got := r.Resolve(context.Background(), installed("/opt/homebrew/bin/git"))

	if !got.UpdateAvail {
		t.Error("the resolver did not mark an outdated item")
	}
}

func TestOnlyManagersWithACheckAreAsked(t *testing.T) {
	// A manager devenv cannot drive has nothing useful to report either.
	apt, _ := Lookup(APT)
	if apt.CanCheckUpdates() {
		t.Error("APT claims an update check devenv does not implement")
	}

	brew, _ := Lookup(Homebrew)
	if !brew.CanCheckUpdates() {
		t.Error("Homebrew should support an update check")
	}
}
