package tools

import (
	"context"
	"errors"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

func TestGitDetect(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"git": "/usr/bin/git"},
		Commands: map[string]probe.FakeResult{
			"git --version": {Out: "git version 2.39.5 (Apple Git-154)\n"},
		},
	}

	item, err := NewGit(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "2.39.5" {
		t.Errorf("Version = %q, want 2.39.5", item.Version)
	}
	if item.Category != detector.CategoryTool {
		t.Errorf("Category = %q, want tool", item.Category)
	}
	if item.Status != detector.StatusInstalled {
		t.Errorf("Status = %q, want installed", item.Status)
	}
}

func TestGitNotInstalled(t *testing.T) {
	_, err := NewGit(&probe.Fake{}).Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}
