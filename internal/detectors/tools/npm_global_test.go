package tools

import (
	"context"
	"testing"

	"devenv/internal/probe"
)

const npmJSON = `{
  "name": "npm-globals",
  "dependencies": {
    "typescript": {"version": "5.5.3"},
    "npm": {"version": "10.8.1"},
    "@angular/cli": {"version": "18.0.6"}
  }
}`

func TestNpmGlobalsDetect(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"npm": "/opt/homebrew/bin/npm"},
		Commands: map[string]probe.FakeResult{
			"npm ls -g --depth=0 --json": {Out: npmJSON},
		},
	}

	item, err := NewNpmGlobals(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got := item.Meta["packages"]; got != "3" {
		t.Errorf("Meta[packages] = %q, want 3", got)
	}
	// Sorted, so the display does not reshuffle between scans.
	if got := item.Meta["packageList"]; got != "@angular/cli, npm, typescript" {
		t.Errorf("Meta[packageList] = %q", got)
	}
}

func TestNpmGlobalsWithNoPackages(t *testing.T) {
	p := &probe.Fake{
		Paths:    map[string]string{"npm": "/opt/homebrew/bin/npm"},
		Commands: map[string]probe.FakeResult{"npm ls -g --depth=0 --json": {Out: `{"dependencies":{}}`}},
	}

	item, err := NewNpmGlobals(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got := item.Meta["packages"]; got != "0" {
		t.Errorf("Meta[packages] = %q, want 0", got)
	}
	if _, ok := item.Meta["packageList"]; ok {
		t.Error("Meta[packageList] set with no packages")
	}
}

func TestParseGlobalPackagesIgnoresGarbage(t *testing.T) {
	if got := parseGlobalPackages("not json at all"); got != nil {
		t.Errorf("parseGlobalPackages() = %v, want nil", got)
	}
}
