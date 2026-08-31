package simple

import (
	"context"
	"errors"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

func TestToolDetect(t *testing.T) {
	p := &probe.Fake{
		Paths:    map[string]string{"gh": "/opt/homebrew/bin/gh"},
		Commands: map[string]probe.FakeResult{"gh --version": {Out: "gh version 2.52.0 (2024-06-24)\n"}},
	}

	item, err := Tool{ItemName: "GitHub CLI", Cat: detector.CategoryTool, Bins: []string{"gh"}, P: p}.
		Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "2.52.0" {
		t.Errorf("Version = %q, want 2.52.0", item.Version)
	}
	if item.Path != "/opt/homebrew/bin/gh" {
		t.Errorf("Path = %q", item.Path)
	}
	if item.Status != detector.StatusInstalled {
		t.Errorf("Status = %q, want installed", item.Status)
	}
	// With a single candidate binary there was no choice to record.
	if _, ok := item.Meta["binary"]; ok {
		t.Error("Meta[binary] set for a single-candidate tool")
	}
}

func TestToolUsesCustomArgs(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"kubectl": "/usr/local/bin/kubectl"},
		Commands: map[string]probe.FakeResult{
			"kubectl version --client": {Out: "Client Version: v1.30.2\n"},
		},
	}

	item, err := Tool{
		ItemName: "kubectl", Cat: detector.CategoryTool,
		Bins: []string{"kubectl"}, Args: []string{"version", "--client"}, P: p,
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "1.30.2" {
		t.Errorf("Version = %q, want 1.30.2", item.Version)
	}
}

func TestToolPrefersTheFirstAvailableBinary(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"gmake": "/opt/homebrew/bin/gmake"},
		Commands: map[string]probe.FakeResult{
			"gmake --version": {Out: "GNU Make 4.4.1\n"},
		},
	}

	item, err := Tool{
		ItemName: "Make", Cat: detector.CategoryTool,
		Bins: []string{"make", "gmake"}, P: p,
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got := item.Meta["binary"]; got != "gmake" {
		t.Errorf("Meta[binary] = %q, want gmake — which candidate answered matters", got)
	}
}

func TestToolAcceptsVersionFromAFailingCommand(t *testing.T) {
	// java and ollama print a usable version and still exit non-zero.
	p := &probe.Fake{
		Paths: map[string]string{"java": "/usr/bin/java"},
		Commands: map[string]probe.FakeResult{
			"java -version": {Out: `openjdk version "21.0.3" 2024-04-16`, Err: errors.New("exit status 1")},
		},
	}

	item, err := Tool{
		ItemName: "Java", Cat: detector.CategoryLanguage,
		Bins: []string{"java"}, Args: []string{"-version"}, P: p,
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v, want the version to be accepted anyway", err)
	}
	if item.Version != "21.0.3" {
		t.Errorf("Version = %q, want 21.0.3", item.Version)
	}
}

func TestToolFailsWhenTheCommandFailsWithNoVersion(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"gh": "/opt/homebrew/bin/gh"},
		Commands: map[string]probe.FakeResult{
			"gh --version": {Out: "permission denied", Err: errors.New("exit status 126")},
		},
	}

	_, err := Tool{ItemName: "GitHub CLI", Cat: detector.CategoryTool, Bins: []string{"gh"}, P: p}.
		Detect(context.Background())
	if err == nil {
		t.Fatal("Detect() error = nil, want the probe failure surfaced")
	}
	if errors.Is(err, detector.ErrNotInstalled) {
		t.Error("a broken binary was reported as simply absent")
	}
}

func TestToolNotInstalled(t *testing.T) {
	_, err := Tool{ItemName: "gh", Cat: detector.CategoryTool, Bins: []string{"gh"}, P: &probe.Fake{}}.
		Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func TestToolEnricherReceivesTheRawOutput(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"nvidia-smi": "/usr/bin/nvidia-smi"},
		Commands: map[string]probe.FakeResult{
			"nvidia-smi --query-gpu=driver_version,name --format=csv,noheader": {
				Out: "550.54.15, NVIDIA GeForce RTX 4090\n",
			},
		},
	}

	var seen string
	item, err := Tool{
		ItemName: "CUDA", Cat: detector.CategoryAI, Bins: []string{"nvidia-smi"},
		Args: []string{"--query-gpu=driver_version,name", "--format=csv,noheader"}, P: p,
		Enrich: func(_ context.Context, _ probe.Prober, item detector.Item, out string) detector.Item {
			seen = out
			return item.WithMeta("enriched", "yes")
		},
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if seen == "" {
		t.Error("enricher did not receive the command output")
	}
	if item.Meta["enriched"] != "yes" {
		t.Error("enricher result was discarded")
	}
}

func serviceFake() *probe.Fake {
	return &probe.Fake{
		Paths:    map[string]string{"psql": "/opt/homebrew/bin/psql"},
		Commands: map[string]probe.FakeResult{"psql --version": {Out: "psql (PostgreSQL) 16.3\n"}},
	}
}

func TestServiceRunning(t *testing.T) {
	p := serviceFake()
	p.Ports = map[string]bool{"127.0.0.1:5432": true}
	p.Commands["pg_isready -q"] = probe.FakeResult{Out: "accepting connections\n"}

	item, err := Service{
		Tool: Tool{ItemName: "PostgreSQL", Cat: detector.CategoryServer, Bins: []string{"psql"}, P: p},
		Port: 5432,
		Ping: []string{"pg_isready", "-q"},
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Status != detector.StatusRunning {
		t.Errorf("Status = %q, want running", item.Status)
	}
	if item.Meta["port"] != "5432" {
		t.Errorf("Meta[port] = %q, want 5432", item.Meta["port"])
	}
	if item.Meta["ping"] != "accepting connections" {
		t.Errorf("Meta[ping] = %q", item.Meta["ping"])
	}
}

func TestServiceStoppedDoesNotPing(t *testing.T) {
	p := serviceFake()

	item, err := Service{
		Tool: Tool{ItemName: "PostgreSQL", Cat: detector.CategoryServer, Bins: []string{"psql"}, P: p},
		Port: 5432,
		Ping: []string{"pg_isready", "-q"},
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Status != detector.StatusStopped {
		t.Errorf("Status = %q, want stopped", item.Status)
	}
	// The whole point of gating on the dial: a client told to reach an absent
	// server blocks, and would stall every scan.
	if p.Ran("pg_isready", "-q") {
		t.Error("pinged a service whose port was closed")
	}
}

func TestServiceNotInstalled(t *testing.T) {
	_, err := Service{
		Tool: Tool{ItemName: "PostgreSQL", Cat: detector.CategoryServer, Bins: []string{"psql"}, P: &probe.Fake{}},
		Port: 5432,
	}.Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func TestManagerPrefersABinary(t *testing.T) {
	p := &probe.Fake{
		Paths:    map[string]string{"pyenv": "/opt/homebrew/bin/pyenv"},
		Commands: map[string]probe.FakeResult{"pyenv --version": {Out: "pyenv 2.4.7\n"}},
		Files:    map[string]bool{"/Users/test/.pyenv": true},
		HomeDir:  "/Users/test",
	}

	item, err := Manager{
		ItemName: "pyenv", Cat: detector.CategoryManager,
		Bins: []string{"pyenv"}, Dirs: []string{"~/.pyenv"}, P: p,
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "2.4.7" {
		t.Errorf("Version = %q, want 2.4.7 — a binary should supply a version", item.Version)
	}
	if item.Path != "/opt/homebrew/bin/pyenv" {
		t.Errorf("Path = %q, want the binary path", item.Path)
	}
}

func TestManagerFallsBackToItsDirectory(t *testing.T) {
	// nvm is a shell function with no binary, so the directory is all there is.
	p := &probe.Fake{
		Files:   map[string]bool{"/Users/test/.nvm": true},
		HomeDir: "/Users/test",
	}

	item, err := Manager{
		ItemName: "nvm", Cat: detector.CategoryManager, Dirs: []string{"~/.nvm"}, P: p,
	}.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Path != "/Users/test/.nvm" {
		t.Errorf("Path = %q, want the expanded install directory", item.Path)
	}
	if item.Status != detector.StatusInstalled {
		t.Errorf("Status = %q, want installed", item.Status)
	}
	if item.Version != "" {
		t.Errorf("Version = %q, want empty — a directory carries no version", item.Version)
	}
}

func TestManagerNotInstalled(t *testing.T) {
	_, err := Manager{
		ItemName: "nvm", Cat: detector.CategoryManager,
		Dirs: []string{"~/.nvm"}, P: &probe.Fake{HomeDir: "/Users/test"},
	}.Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func TestFirstBinary(t *testing.T) {
	p := &probe.Fake{Paths: map[string]string{"python": "/usr/bin/python"}}

	bin, path, err := FirstBinary(p, []string{"python3", "python"})
	if err != nil {
		t.Fatalf("FirstBinary() error = %v", err)
	}
	if bin != "python" || path != "/usr/bin/python" {
		t.Errorf("FirstBinary() = (%q, %q)", bin, path)
	}

	if _, _, err := FirstBinary(p, []string{"nope"}); !errors.Is(err, detector.ErrNotInstalled) {
		t.Errorf("error = %v, want ErrNotInstalled", err)
	}
}
