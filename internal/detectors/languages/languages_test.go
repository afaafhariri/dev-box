package languages

import (
	"context"
	"errors"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

func TestGoDetect(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"go": "/usr/local/go/bin/go"},
		Commands: map[string]probe.FakeResult{
			"go version": {Out: "go version go1.22.3 darwin/arm64\n"},
		},
	}

	item, err := NewGo(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "1.22.3" {
		t.Errorf("Version = %q, want 1.22.3", item.Version)
	}
	if item.Path != "/usr/local/go/bin/go" {
		t.Errorf("Path = %q", item.Path)
	}
	if item.Status != detector.StatusInstalled {
		t.Errorf("Status = %q, want installed", item.Status)
	}
	if got := item.Meta["platform"]; got != "darwin/arm64" {
		t.Errorf("Meta[platform] = %q, want darwin/arm64", got)
	}
	if item.Category != detector.CategoryLanguage {
		t.Errorf("Category = %q", item.Category)
	}
}

func TestGoDetectNotInstalled(t *testing.T) {
	_, err := NewGo(&probe.Fake{}).Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func TestGoDetectUnparseableOutput(t *testing.T) {
	p := &probe.Fake{
		Paths:    map[string]string{"go": "/usr/local/go/bin/go"},
		Commands: map[string]probe.FakeResult{"go version": {Out: "something else entirely\n"}},
	}

	// A binary that exists but does not answer as expected is a probe failure,
	// not an absence — the two must not be conflated.
	_, err := NewGo(p).Detect(context.Background())
	if err == nil {
		t.Fatal("Detect() error = nil, want parse failure")
	}
	if errors.Is(err, detector.ErrNotInstalled) {
		t.Error("parse failure reported as ErrNotInstalled")
	}
}

func TestGoVersionParsing(t *testing.T) {
	tests := []struct {
		out          string
		wantVersion  string
		wantPlatform string
	}{
		{"go version go1.22.3 darwin/arm64\n", "1.22.3", "darwin/arm64"},
		{"go version go1.25.6 linux/amd64", "1.25.6", "linux/amd64"},
		{"go version go1.21rc2 darwin/arm64", "1.21rc2", "darwin/arm64"},
		{"go version go1.22.3", "1.22.3", ""},
		{"", "", ""},
	}

	for _, tt := range tests {
		version, platform := parseGoVersion(tt.out)
		if version != tt.wantVersion || platform != tt.wantPlatform {
			t.Errorf("parseGoVersion(%q) = (%q, %q), want (%q, %q)",
				tt.out, version, platform, tt.wantVersion, tt.wantPlatform)
		}
	}
}

func TestPythonDetect(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"python3": "/opt/homebrew/bin/python3", "pip3": "/opt/homebrew/bin/pip3"},
		Commands: map[string]probe.FakeResult{
			"python3 --version": {Out: "Python 3.12.4\n"},
			"pip3 --version":    {Out: "pip 24.0 from /opt/homebrew/lib/python3.12/site-packages/pip (python 3.12)\n"},
		},
	}

	item, err := NewPython(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "3.12.4" {
		t.Errorf("Version = %q, want 3.12.4", item.Version)
	}
	if got := item.Meta["binary"]; got != "python3" {
		t.Errorf("Meta[binary] = %q, want python3", got)
	}
	if got := item.Meta["pip"]; got != "24.0" {
		t.Errorf("Meta[pip] = %q, want 24.0", got)
	}
}

func TestPythonFallsBackToPython(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"python": "/usr/bin/python"},
		Commands: map[string]probe.FakeResult{
			"python --version": {Out: "Python 2.7.18\n"},
		},
	}

	item, err := NewPython(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "2.7.18" {
		t.Errorf("Version = %q, want 2.7.18", item.Version)
	}
	if got := item.Meta["binary"]; got != "python" {
		t.Errorf("Meta[binary] = %q, want python", got)
	}
}

func TestPythonMissingPipIsNotFatal(t *testing.T) {
	p := &probe.Fake{
		Paths:    map[string]string{"python3": "/usr/bin/python3"},
		Commands: map[string]probe.FakeResult{"python3 --version": {Out: "Python 3.12.4"}},
	}

	item, err := NewPython(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if _, ok := item.Meta["pip"]; ok {
		t.Error("Meta[pip] set with no pip installed")
	}
}

func TestNodeDetect(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"node": "/opt/homebrew/bin/node", "npm": "/opt/homebrew/bin/npm"},
		Commands: map[string]probe.FakeResult{
			"node --version": {Out: "v22.3.0\n"},
			"npm --version":  {Out: "10.8.1\n"},
		},
	}

	item, err := NewNode(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "22.3.0" {
		t.Errorf("Version = %q, want 22.3.0", item.Version)
	}
	if got := item.Meta["npm"]; got != "10.8.1" {
		t.Errorf("Meta[npm] = %q, want 10.8.1", got)
	}
	if item.Name != "Node.js" {
		t.Errorf("Name = %q, want Node.js", item.Name)
	}
}

func TestNodeNotInstalled(t *testing.T) {
	_, err := NewNode(&probe.Fake{}).Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func TestAllReturnsEveryLanguageDetector(t *testing.T) {
	want := map[string]bool{
		"Go": true, "Python": true, "Node.js": true,
		"Java": true, "Rust": true, "Ruby": true,
	}

	got := make(map[string]bool)
	for _, d := range All(&probe.Fake{}) {
		got[d.Name()] = true
		if d.Category() != detector.CategoryLanguage {
			t.Errorf("%s has category %q, want language", d.Name(), d.Category())
		}
	}

	for name := range want {
		if !got[name] {
			t.Errorf("All() is missing the %s detector", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("All() returned %d detectors, want %d: %v", len(got), len(want), got)
	}
}

func TestJavaReportsVersionFromStderr(t *testing.T) {
	// java writes -version to stderr; the probe merges the streams so the
	// detector sees it.
	p := &probe.Fake{
		Paths: map[string]string{"java": "/usr/bin/java"},
		Commands: map[string]probe.FakeResult{
			"java -version": {Out: `openjdk version "21.0.3" 2024-04-16
OpenJDK Runtime Environment Homebrew (build 21.0.3)
`},
		},
	}

	item, err := All(p)[3].Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "21.0.3" {
		t.Errorf("Version = %q, want 21.0.3", item.Version)
	}
}

func TestRustIncludesItsToolchain(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"rustc": "/Users/x/.cargo/bin/rustc", "rustup": "/Users/x/.cargo/bin/rustup"},
		Commands: map[string]probe.FakeResult{
			"rustc --version":              {Out: "rustc 1.79.0 (129f3b996 2024-06-10)\n"},
			"rustup show active-toolchain": {Out: "stable-aarch64-apple-darwin (default)\n"},
		},
	}

	item, err := NewRust(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "1.79.0" {
		t.Errorf("Version = %q, want 1.79.0", item.Version)
	}
	if got := item.Meta["toolchain"]; got != "stable-aarch64-apple-darwin (default)" {
		t.Errorf("Meta[toolchain] = %q", got)
	}
}

func TestRustWithoutRustupIsStillDetected(t *testing.T) {
	p := &probe.Fake{
		Paths:    map[string]string{"rustc": "/usr/local/bin/rustc"},
		Commands: map[string]probe.FakeResult{"rustc --version": {Out: "rustc 1.79.0 (129f3b996 2024-06-10)"}},
	}

	item, err := NewRust(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if _, ok := item.Meta["toolchain"]; ok {
		t.Error("Meta[toolchain] set with no rustup installed")
	}
}
