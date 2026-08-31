package action

import (
	"context"
	"strings"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

func service(name string, status detector.Status) detector.Item {
	return detector.Item{Name: name, Category: detector.CategoryServer, Status: status}
}

func TestStartAppliesOnlyToStoppedServices(t *testing.T) {
	start := NewBrewStart(brewProbe())

	if !start.Applicable(service("PostgreSQL", detector.StatusStopped)) {
		t.Error("Start should apply to a stopped service")
	}
	if start.Applicable(service("PostgreSQL", detector.StatusRunning)) {
		t.Error("Start should not apply to a service already running")
	}
	// Nothing here knows how to start Go.
	if start.Applicable(detector.Item{Name: "Go", Status: detector.StatusInstalled}) {
		t.Error("Start should not apply to something Homebrew does not manage")
	}
}

func TestStopAppliesOnlyToRunningServices(t *testing.T) {
	stop := NewBrewStop(brewProbe())

	if !stop.Applicable(service("Redis", detector.StatusRunning)) {
		t.Error("Stop should apply to a running service")
	}
	if stop.Applicable(service("Redis", detector.StatusStopped)) {
		t.Error("Stop should not apply to a service already stopped")
	}
}

// brewProbe resolves Homebrew symlinks the way the real prefix does.
func brewProbe() *probe.Fake {
	return &probe.Fake{
		Paths: map[string]string{"brew": "/opt/homebrew/bin/brew"},
		Links: map[string]string{
			"/opt/homebrew/bin/psql": "/opt/homebrew/Cellar/postgresql@15/15.19/bin/psql",
			"/opt/homebrew/bin/git":  "/opt/homebrew/Cellar/git/2.45.2/bin/git",
		},
	}
}

func TestUpgradeOnlyForHomebrewInstalls(t *testing.T) {
	upgrade := NewUpgrade(brewProbe())

	brewed := detector.Item{Name: "Git", Status: detector.StatusInstalled, Path: "/opt/homebrew/bin/git"}
	if !upgrade.Applicable(brewed) {
		t.Error("Upgrade should apply to a Homebrew install")
	}

	// Upgrading a pyenv-managed Python through brew would fight the tool that
	// actually manages it.
	managed := detector.Item{Name: "Python", Status: detector.StatusInstalled, Path: "/Users/x/.pyenv/shims/python3"}
	if upgrade.Applicable(managed) {
		t.Error("Upgrade should not apply outside the Homebrew prefix")
	}

	absent := detector.Item{Name: "Rust", Status: detector.StatusNotFound}
	if upgrade.Applicable(absent) {
		t.Error("Upgrade should not apply to something that is not installed")
	}
}

func TestFormulaComesFromTheCellarPathNotTheBinaryName(t *testing.T) {
	// The whole point: psql belongs to postgresql@15. Guessing from the
	// command name would target the wrong formula.
	p := brewProbe()
	item := detector.Item{Name: "PostgreSQL", Status: detector.StatusRunning, Path: "/opt/homebrew/bin/psql"}

	if got := formula(p, item); got != "postgresql@15" {
		t.Errorf("formula() = %q, want postgresql@15", got)
	}
}

func TestFormulaFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/opt/homebrew/Cellar/postgresql@15/15.19/bin/psql", "postgresql@15"},
		{"/usr/local/Cellar/redis/7.2.4/bin/redis-server", "redis"},
		{"/opt/homebrew/opt/node@20/bin/node", "node@20"},
		{"/usr/bin/git", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := formulaFromPath(tt.path); got != tt.want {
			t.Errorf("formulaFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestFormulaIsEmptyOutsideHomebrew(t *testing.T) {
	// An unknown formula must produce nothing to run, never a guess.
	item := detector.Item{Name: "Python", Status: detector.StatusInstalled, Path: "/Users/x/.pyenv/shims/python3"}

	if got := formula(brewProbe(), item); got != "" {
		t.Errorf("formula() = %q, want empty for a non-Homebrew path", got)
	}
}

func TestServiceActionUsesTheVersionedFormula(t *testing.T) {
	// "brew services start postgresql" fails when postgresql@15 is what is
	// installed, so the resolved formula has to win over the name.
	p := brewProbe()
	item := detector.Item{Name: "PostgreSQL", Status: detector.StatusStopped, Path: "/opt/homebrew/bin/psql"}

	lines := Drain(NewBrewStart(p).Run(context.Background(), item))
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "postgresql@15") {
		t.Errorf("command did not use the versioned formula:\n%s", joined)
	}
}

func TestServiceActionFallsBackToTheKnownFormula(t *testing.T) {
	// Nothing resolves, but Redis is a service we know by name.
	p := &probe.Fake{Paths: map[string]string{"brew": "/opt/homebrew/bin/brew"}}
	item := detector.Item{Name: "Redis", Status: detector.StatusStopped, Path: "/usr/bin/redis-server"}

	lines := Drain(NewBrewStart(p).Run(context.Background(), item))
	if !strings.Contains(strings.Join(lines, "\n"), "redis") {
		t.Errorf("fallback formula was not used: %v", lines)
	}
}

func TestUninstallOnlyOffersItselfWhenTheFormulaIsKnown(t *testing.T) {
	uninstall := NewUninstall(brewProbe())

	known := detector.Item{Name: "Git", Status: detector.StatusInstalled, Path: "/opt/homebrew/bin/git"}
	if !uninstall.Applicable(known) {
		t.Error("Uninstall should apply to a Homebrew install with a known formula")
	}

	// Inside the prefix, but nothing resolves to a Cellar path: offering an
	// action that cannot name its target would be worse than not offering it.
	unknown := detector.Item{Name: "Mystery", Status: detector.StatusInstalled, Path: "/opt/homebrew/bin/mystery"}
	if uninstall.Applicable(unknown) {
		t.Error("Uninstall was offered for an item whose formula is unknown")
	}

	outside := detector.Item{Name: "Node.js", Status: detector.StatusInstalled, Path: "/Users/x/.nvm/versions/node/v22/bin/node"}
	if uninstall.Applicable(outside) {
		t.Error("Uninstall was offered for an nvm-managed install")
	}
}

func TestUninstallIsDestructiveAndNamesTheCommand(t *testing.T) {
	var a Action = NewUninstall(brewProbe())

	d, ok := a.(Destructive)
	if !ok {
		t.Fatal("Uninstall does not implement Destructive, so the UI would run it without asking")
	}

	item := detector.Item{Name: "PostgreSQL", Status: detector.StatusRunning, Path: "/opt/homebrew/bin/psql"}
	confirm := d.Confirm(item)

	// The confirmation has to name the formula: "Uninstall PostgreSQL" would
	// not tell the user that postgresql@15 is what goes.
	if !strings.Contains(confirm, "postgresql@15") {
		t.Errorf("Confirm() = %q, want the formula named", confirm)
	}
	if !strings.Contains(confirm, "data") {
		t.Errorf("Confirm() = %q, want the data warning for a service", confirm)
	}
}

func TestUpgradeIsNotDestructive(t *testing.T) {
	// Only genuinely irreversible actions should cost a second keypress.
	var a Action = NewUpgrade(brewProbe())
	if _, ok := a.(Destructive); ok {
		t.Error("Upgrade is marked Destructive")
	}
}

func TestUninstallRefusesWithoutAFormula(t *testing.T) {
	item := detector.Item{Name: "Mystery", Status: detector.StatusInstalled, Path: "/opt/homebrew/bin/mystery"}

	lines := Drain(NewUninstall(brewProbe()).Run(context.Background(), item))
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "✗") {
		t.Errorf("lines = %v, want a single refusal", lines)
	}
}

func TestIsBrewPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/bin/git", true},
		{"/usr/local/Cellar/redis/7.2.4/bin/redis-server", true},
		{"/home/linuxbrew/.linuxbrew/bin/gh", true},
		{"/usr/bin/git", false},
		{"/Users/x/.cargo/bin/rustc", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsBrewPath(tt.path); got != tt.want {
			t.Errorf("IsBrewPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestRegistryFiltersByApplicability(t *testing.T) {
	r := NewRegistry()
	r.Register(NewBrewStart(brewProbe()), NewBrewStop(brewProbe()), NewUpgrade(brewProbe()))

	stopped := service("Redis", detector.StatusStopped)
	labels := r.Labels(stopped)

	if len(labels) != 1 || labels[0] != "Start" {
		t.Errorf("Labels(stopped redis) = %v, want [Start]", labels)
	}

	running := service("Redis", detector.StatusRunning)
	if got := r.Labels(running); len(got) != 1 || got[0] != "Stop" {
		t.Errorf("Labels(running redis) = %v, want [Stop]", got)
	}
}

func TestRegistryIgnoresNil(t *testing.T) {
	r := NewRegistry()
	r.Register(nil)

	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0", r.Len())
	}
}

func TestDefaultRegistrySkipsBrewActionsWithoutBrew(t *testing.T) {
	// Offering "Start" on a machine with no brew would produce an action that
	// can only ever fail.
	r := DefaultRegistry(&probe.Fake{})

	for _, a := range r.For(service("Redis", detector.StatusStopped)) {
		if a.Label() == "Start" {
			t.Error("Start was registered with no Homebrew installed")
		}
	}
}

func TestDefaultRegistryIncludesBrewActionsWhenPresent(t *testing.T) {
	p := &probe.Fake{Paths: map[string]string{"brew": "/opt/homebrew/bin/brew"}}
	r := DefaultRegistry(p)

	labels := r.Labels(service("Redis", detector.StatusStopped))
	if len(labels) == 0 {
		t.Fatal("no actions offered for a stopped Redis with brew installed")
	}
	if labels[0] != "Start" {
		t.Errorf("Labels() = %v, want Start first", labels)
	}
}

func TestRunStreamsOutputAndClosesTheChannel(t *testing.T) {
	// A real command, so the streaming path itself is exercised.
	lines := Drain(run(context.Background(), "done", "sh", "-c", "echo first; echo second"))

	if len(lines) < 3 {
		t.Fatalf("got %d lines, want the command echo plus two output lines and a result: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "$ sh") {
		t.Errorf("first line = %q, want the command that ran", lines[0])
	}
	if lines[len(lines)-1] != "✓ done" {
		t.Errorf("last line = %q, want the success confirmation", lines[len(lines)-1])
	}
}

func TestRunReportsFailure(t *testing.T) {
	lines := Drain(run(context.Background(), "done", "sh", "-c", "echo problem >&2; exit 3"))

	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "✗") {
		t.Errorf("last line = %q, want a failure marker", last)
	}
	// stderr is merged in, so the reason is visible above the marker.
	if !strings.Contains(strings.Join(lines, "\n"), "problem") {
		t.Errorf("stderr was not streamed: %v", lines)
	}
}

func TestRunStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Must return rather than hang, and must say why it stopped: an action
	// that ends silently looks like a UI that stopped working.
	lines := Drain(run(ctx, "done", "sleep", "30"))

	if len(lines) == 0 {
		t.Fatal("no output at all from a cancelled action")
	}
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "✗") {
		t.Errorf("last line = %q, want an explanation of the cancellation", last)
	}
	if !strings.Contains(last, "context canceled") {
		t.Errorf("last line = %q, want the cancellation named", last)
	}
}

func TestUnmanagedServiceFailsCleanly(t *testing.T) {
	lines := Drain(NewBrewStart(brewProbe()).Run(context.Background(), detector.Item{Name: "Nothing"}))

	if len(lines) != 1 || !strings.HasPrefix(lines[0], "✗") {
		t.Errorf("lines = %v, want a single failure line", lines)
	}
}
