package probe

import (
	"context"
	"errors"
	"testing"
)

func TestFakeRunReturnsCannedResults(t *testing.T) {
	f := &Fake{Commands: map[string]FakeResult{
		"go version": {Out: "go version go1.22.3 darwin/arm64"},
	}}

	out, err := f.Run(context.Background(), "go", "version")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != "go version go1.22.3 darwin/arm64" {
		t.Errorf("out = %q", out)
	}
}

func TestZeroFakeReportsNothingInstalled(t *testing.T) {
	f := &Fake{}

	if _, err := f.Run(context.Background(), "go", "version"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Run() error = %v, want ErrNotFound", err)
	}
	if _, err := f.LookPath("go"); !errors.Is(err, ErrNotFound) {
		t.Errorf("LookPath() error = %v, want ErrNotFound", err)
	}
	if f.Exists("/anything") {
		t.Error("Exists() = true on a zero Fake")
	}
	if f.PortOpen(context.Background(), "127.0.0.1", 6379) {
		t.Error("PortOpen() = true on a zero Fake")
	}
}

func TestFakeRecordsCalls(t *testing.T) {
	f := &Fake{Commands: map[string]FakeResult{"git --version": {Out: "git version 2.39.5"}}}

	if _, err := f.Run(context.Background(), "git", "--version"); err != nil {
		t.Fatal(err)
	}

	if !f.Ran("git", "--version") {
		t.Error("Ran() = false for a command that was run")
	}
	if f.Ran("git", "status") {
		t.Error("Ran() = true for a command that was never run")
	}
	if len(f.Calls) != 1 {
		t.Errorf("Calls = %v, want one entry", f.Calls)
	}
}

func TestFakeRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := &Fake{Commands: map[string]FakeResult{"go version": {Out: "irrelevant"}}}

	if _, err := f.Run(ctx, "go", "version"); !errors.Is(err, context.Canceled) {
		t.Errorf("Run() error = %v, want context.Canceled", err)
	}
}

func TestKey(t *testing.T) {
	if got, want := Key("go", "version"), "go version"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
	if got, want := Key("ollama"), "ollama"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}
