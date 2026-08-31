package managers

import (
	"context"
	"errors"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

func TestNVMIsFoundByItsDirectory(t *testing.T) {
	// nvm is sourced into the shell and never appears on PATH, so the
	// directory is the only signal there is.
	p := &probe.Fake{
		Files:   map[string]bool{"/Users/test/.nvm": true},
		HomeDir: "/Users/test",
	}

	item, err := NewNVM(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Path != "/Users/test/.nvm" {
		t.Errorf("Path = %q", item.Path)
	}
	if item.Category != detector.CategoryManager {
		t.Errorf("Category = %q, want manager", item.Category)
	}
}

func TestPyenvReportsItsVersion(t *testing.T) {
	p := &probe.Fake{
		Paths:    map[string]string{"pyenv": "/opt/homebrew/bin/pyenv"},
		Commands: map[string]probe.FakeResult{"pyenv --version": {Out: "pyenv 2.4.7\n"}},
		HomeDir:  "/Users/test",
	}

	item, err := NewPyenv(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "2.4.7" {
		t.Errorf("Version = %q, want 2.4.7", item.Version)
	}
}

func TestMiseAcceptsItsOldName(t *testing.T) {
	// mise was called rtx until 2024; installs from that era still work.
	p := &probe.Fake{
		Paths:    map[string]string{"rtx": "/opt/homebrew/bin/rtx"},
		Commands: map[string]probe.FakeResult{"rtx --version": {Out: "rtx 2024.1.1 macos-arm64\n"}},
		HomeDir:  "/Users/test",
	}

	item, err := NewMise(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "2024.1.1" {
		t.Errorf("Version = %q, want 2024.1.1", item.Version)
	}
}

func TestNothingInstalled(t *testing.T) {
	p := &probe.Fake{HomeDir: "/Users/test"}

	for _, d := range All(p) {
		if _, err := d.Detect(context.Background()); !errors.Is(err, detector.ErrNotInstalled) {
			t.Errorf("%s: error = %v, want ErrNotInstalled", d.Name(), err)
		}
	}
}

func TestAllAreManagers(t *testing.T) {
	want := map[string]bool{"nvm": true, "pyenv": true, "sdkman": true, "mise": true, "rbenv": true}

	got := make(map[string]bool)
	for _, d := range All(&probe.Fake{}) {
		got[d.Name()] = true
		if d.Category() != detector.CategoryManager {
			t.Errorf("%s has category %q, want manager", d.Name(), d.Category())
		}
	}

	for name := range want {
		if !got[name] {
			t.Errorf("All() is missing %s", name)
		}
	}
}
