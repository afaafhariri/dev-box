package tools

import (
	"context"
	"errors"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

// Each case pins the arguments as well as the parsing. The arguments are the
// fragile part: "kubectl version" without --client blocks trying to reach a
// cluster, and a change there would hang every scan.
func TestCLIDetectors(t *testing.T) {
	tests := []struct {
		name        string
		detector    func(probe.Prober) detector.Detector
		bin         string
		command     string
		out         string
		wantName    string
		wantVersion string
	}{
		{
			name: "GitHub CLI", detector: NewGH, bin: "gh",
			command: "gh --version", out: "gh version 2.52.0 (2024-06-24)\nhttps://github.com/cli/cli/releases/latest\n",
			wantName: "GitHub CLI", wantVersion: "2.52.0",
		},
		{
			name: "kubectl stays client-side", detector: NewKubectl, bin: "kubectl",
			// Without --client this contacts the cluster and blocks.
			command: "kubectl version --client", out: "Client Version: v1.30.2\nKustomize Version: v5.0.4\n",
			wantName: "kubectl", wantVersion: "1.30.2",
		},
		{
			name: "Helm short form", detector: NewHelm, bin: "helm",
			command: "helm version --short", out: "v3.15.2+g1a500d5\n",
			wantName: "Helm", wantVersion: "3.15.2",
		},
		{
			name: "Make", detector: NewMake, bin: "make",
			command: "make --version", out: "GNU Make 3.81\nCopyright (C) 2006 Free Software Foundation, Inc.\n",
			wantName: "Make", wantVersion: "3.81",
		},
		{
			name: "CMake", detector: NewCMake, bin: "cmake",
			command: "cmake --version", out: "cmake version 3.29.6\n\nCMake suite maintained and supported by Kitware\n",
			wantName: "CMake", wantVersion: "3.29.6",
		},
		{
			name: "Cargo", detector: NewCargo, bin: "cargo",
			command: "cargo --version", out: "cargo 1.79.0 (ffa9cf99a 2024-06-03)\n",
			wantName: "Cargo", wantVersion: "1.79.0",
		},
		{
			name: "Homebrew", detector: NewHomebrew, bin: "brew",
			command: "brew --version", out: "Homebrew 4.3.8\nHomebrew/homebrew-core (git revision abc; last commit 2024-06-01)\n",
			wantName: "Homebrew", wantVersion: "4.3.8",
		},
		{
			name: "pnpm", detector: NewPnpm, bin: "pnpm",
			command: "pnpm --version", out: "9.4.0\n",
			wantName: "pnpm", wantVersion: "9.4.0",
		},
		{
			name: "Yarn", detector: NewYarn, bin: "yarn",
			command: "yarn --version", out: "1.22.22\n",
			wantName: "Yarn", wantVersion: "1.22.22",
		},
		{
			name: "Maven", detector: NewMaven, bin: "mvn",
			command: "mvn --version", out: "Apache Maven 3.9.8 (36645f6c9b5079805ea5009217e36f2cffd34d3c)\nMaven home: /opt/homebrew/Cellar/maven\n",
			wantName: "Maven", wantVersion: "3.9.8",
		},
		{
			name: "Gradle", detector: NewGradle, bin: "gradle",
			command: "gradle --version", out: "\n------------------------------------------------------------\nGradle 8.8\n------------------------------------------------------------\n",
			wantName: "Gradle", wantVersion: "8.8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &probe.Fake{
				Paths:    map[string]string{tt.bin: "/opt/homebrew/bin/" + tt.bin},
				Commands: map[string]probe.FakeResult{tt.command: {Out: tt.out}},
			}

			item, err := tt.detector(p).Detect(context.Background())
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if item.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", item.Name, tt.wantName)
			}
			if item.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", item.Version, tt.wantVersion)
			}
			if item.Category != detector.CategoryTool {
				t.Errorf("Category = %q, want tool", item.Category)
			}
			// The exact command matters as much as the parsing.
			bin, args := splitCommand(tt.command)
			if !p.Ran(bin, args...) {
				t.Errorf("did not run %q; ran %v", tt.command, p.Calls)
			}
		})
	}
}

// splitCommand turns "kubectl version --client" into the args Ran expects.
func splitCommand(command string) (string, []string) {
	fields := fieldsOf(command)
	return fields[0], fields[1:]
}

func fieldsOf(s string) []string {
	var out []string
	for _, f := range splitSpace(s) {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func splitSpace(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ' ' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}

func TestCLIDetectorsReportAbsence(t *testing.T) {
	p := &probe.Fake{}

	for _, d := range All(p) {
		if _, err := d.Detect(context.Background()); !errors.Is(err, detector.ErrNotInstalled) {
			t.Errorf("%s: error = %v, want ErrNotInstalled", d.Name(), err)
		}
	}
}

func TestAllToolsAreInTheToolCategory(t *testing.T) {
	for _, d := range All(&probe.Fake{}) {
		if d.Category() != detector.CategoryTool {
			t.Errorf("%s has category %q, want tool", d.Name(), d.Category())
		}
	}
}

func TestMakePrefersGNUMakeWhenBothExist(t *testing.T) {
	// gmake is GNU make on systems where make is BSD; either is acceptable,
	// but which one answered has to be recorded.
	p := &probe.Fake{
		Paths:    map[string]string{"make": "/usr/bin/make"},
		Commands: map[string]probe.FakeResult{"make --version": {Out: "GNU Make 3.81\n"}},
	}

	item, err := NewMake(p).Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if item.Meta["binary"] != "make" {
		t.Errorf("Meta[binary] = %q, want make", item.Meta["binary"])
	}
}
