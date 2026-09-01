package action

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/platform"
)

// owned builds an item as the scan would leave it: ownership already resolved.
func owned(name string, cat detector.Category, status detector.Status, managedBy, pkg string) detector.Item {
	return detector.Item{
		Name: name, Category: cat, Status: status,
		ManagedBy: managedBy, PackageID: pkg, Version: "1.0.0",
	}
}

func brewService(name string, status detector.Status, pkg string) detector.Item {
	return owned(name, detector.CategoryServer, status, platform.Homebrew, pkg)
}

func TestStartAppliesOnlyToStoppedServices(t *testing.T) {
	start := NewStart(platform.Darwin)

	if !start.Applicable(brewService("PostgreSQL", detector.StatusStopped, "postgresql@15")) {
		t.Error("Start should apply to a stopped service")
	}
	if start.Applicable(brewService("PostgreSQL", detector.StatusRunning, "postgresql@15")) {
		t.Error("Start should not apply to a service already running")
	}
	// A language is not a service, whoever installed it.
	if start.Applicable(owned("Python", detector.CategoryLanguage, detector.StatusInstalled, platform.Homebrew, "python@3.14")) {
		t.Error("Start should not apply to a language")
	}
}

func TestStopAppliesOnlyToRunningServices(t *testing.T) {
	stop := NewStop(platform.Darwin)

	if !stop.Applicable(brewService("Redis", detector.StatusRunning, "redis")) {
		t.Error("Stop should apply to a running service")
	}
	if stop.Applicable(brewService("Redis", detector.StatusStopped, "redis")) {
		t.Error("Stop should not apply to a service already stopped")
	}
}

func TestServiceUsesTheResolvedPackageID(t *testing.T) {
	// "brew services start postgresql" fails when postgresql@15 is what is
	// installed, so the resolved identifier has to be what runs.
	item := brewService("PostgreSQL", detector.StatusStopped, "postgresql@15")

	lines := Drain(NewStart(platform.Darwin).Run(context.Background(), item))
	if !strings.Contains(strings.Join(lines, "\n"), "postgresql@15") {
		t.Errorf("command did not use the resolved formula: %v", lines)
	}
}

func TestLinuxServicesUseSystemd(t *testing.T) {
	// A distribution package has no service template of its own, so systemd
	// is what drives it.
	item := owned("PostgreSQL", detector.CategoryServer, detector.StatusStopped, platform.APT, "postgresql")

	start := NewStart(platform.Linux)
	if !start.Applicable(item) {
		t.Fatal("Start should apply to an APT-installed service on Linux")
	}

	lines := Drain(start.Run(context.Background(), item))
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "systemctl") {
		t.Errorf("Linux service action did not use systemctl: %v", lines)
	}
	// --user needs no elevation; devenv cannot answer a sudo prompt.
	if !strings.Contains(joined, "--user") {
		t.Errorf("Linux service action did not stay in the user session: %v", lines)
	}
}

func TestWindowsOffersNoServiceControl(t *testing.T) {
	// Service control on Windows requires elevation, so it is not attempted.
	item := owned("PostgreSQL", detector.CategoryServer, detector.StatusStopped, platform.Scoop, "postgresql")

	if NewStart(platform.Windows).Applicable(item) {
		t.Error("Start was offered on Windows, where devenv cannot elevate")
	}
}

func TestUpgradeAndUninstallFollowTheOwningManager(t *testing.T) {
	tests := []struct {
		manager string
		pkg     string
		want    bool
		reason  string
	}{
		{platform.Homebrew, "git", true, "Homebrew needs no elevation"},
		{platform.Scoop, "git", true, "Scoop installs per-user"},
		{platform.APT, "git", false, "APT needs a sudo password devenv cannot supply"},
		{platform.DNF, "git", false, "DNF needs elevation"},
		{platform.Pacman, "git", false, "pacman needs elevation"},
		{platform.NVM, "22.13.1", false, "nvm has its own uninstall"},
		{platform.Pyenv, "3.12.4", false, "pyenv has its own uninstall"},
		{platform.System, "ruby", false, "system installs must never be removed"},
		{platform.Manual, "/usr/local/go", false, "a manual install is the user's to remove"},
	}

	uninstall := NewUninstall()
	for _, tt := range tests {
		item := owned("Thing", detector.CategoryTool, detector.StatusInstalled, tt.manager, tt.pkg)
		if got := uninstall.Applicable(item); got != tt.want {
			t.Errorf("Uninstall.Applicable(%s) = %v, want %v — %s", tt.manager, got, tt.want, tt.reason)
		}
	}
}

func TestSystemInstallsAreProtected(t *testing.T) {
	// /usr/bin/ruby belongs to macOS. Removing it breaks other software, so
	// no action may offer to.
	ruby := owned("Ruby", detector.CategoryLanguage, detector.StatusInstalled, platform.System, "ruby")

	for _, a := range RegistryFor(platform.Darwin).For(ruby) {
		t.Errorf("%q was offered for a system install", a.Label())
	}
}

func TestUnresolvedItemsOfferNothing(t *testing.T) {
	// Nothing resolved means nothing is safe to run.
	item := detector.Item{Name: "Mystery", Status: detector.StatusInstalled}

	for _, a := range RegistryFor(platform.Darwin).For(item) {
		t.Errorf("%q was offered for an item with no known owner", a.Label())
	}
}

func TestGuidanceExplainsWhatDevenvWillNotDo(t *testing.T) {
	tests := []struct {
		manager string
		pkg     string
		want    string
	}{
		{platform.NVM, "22.13.1", "nvm uninstall 22.13.1"},
		{platform.Pyenv, "3.12.4", "pyenv uninstall 3.12.4"},
		{platform.APT, "golang", "sudo apt-get remove golang"},
		{platform.System, "ruby", "operating system"},
		{platform.Manual, "/usr/local/go", "/usr/local/go"},
	}

	for _, tt := range tests {
		item := owned("Thing", detector.CategoryLanguage, detector.StatusInstalled, tt.manager, tt.pkg)
		got := Guidance(item)
		if !strings.Contains(got, tt.want) {
			t.Errorf("Guidance(%s) = %q, want it to mention %q", tt.manager, got, tt.want)
		}
	}
}

func TestHomebrewItemsNeedNoGuidance(t *testing.T) {
	// devenv can act on these itself, so there is nothing to explain.
	item := owned("Git", detector.CategoryTool, detector.StatusInstalled, platform.Homebrew, "git")
	if got := Guidance(item); got != "" {
		t.Errorf("Guidance() = %q, want empty for something devenv can act on", got)
	}
}

func TestUninstallIsDestructiveAndNamesTheCommand(t *testing.T) {
	var a Action = NewUninstall()

	d, ok := a.(Destructive)
	if !ok {
		t.Fatal("Uninstall does not implement Destructive, so the UI would run it without asking")
	}

	confirm := d.Confirm(brewService("PostgreSQL", detector.StatusRunning, "postgresql@15"))
	if !strings.Contains(confirm, "brew uninstall postgresql@15") {
		t.Errorf("Confirm() = %q, want the exact command", confirm)
	}
	if !strings.Contains(confirm, "data") {
		t.Errorf("Confirm() = %q, want the data warning for a service", confirm)
	}
}

func TestUpgradeIsNotDestructive(t *testing.T) {
	// Only genuinely irreversible actions should cost a second keypress.
	var a Action = NewUpgrade()
	if _, ok := a.(Destructive); ok {
		t.Error("Upgrade is marked Destructive")
	}
}

func TestRefusalNamesTheRightCommand(t *testing.T) {
	// The answer to "why is there no uninstall button".
	node := owned("Node.js", detector.CategoryLanguage, detector.StatusInstalled, platform.NVM, "22.13.1")

	lines := Drain(NewUninstall().Run(context.Background(), node))
	if len(lines) != 1 {
		t.Fatalf("lines = %v, want a single refusal", lines)
	}
	if !strings.Contains(lines[0], "nvm uninstall 22.13.1") {
		t.Errorf("refusal = %q, want the nvm command named", lines[0])
	}
}

func TestRegistryFiltersByApplicability(t *testing.T) {
	r := RegistryFor(platform.Darwin)

	stopped := brewService("Redis", detector.StatusStopped, "redis")
	labels := r.Labels(stopped)
	if len(labels) == 0 || labels[0] != "Start" {
		t.Errorf("Labels(stopped) = %v, want Start first", labels)
	}
	for _, l := range labels {
		if l == "Stop" {
			t.Error("Stop was offered for a stopped service")
		}
	}

	running := brewService("Redis", detector.StatusRunning, "redis")
	for _, l := range r.Labels(running) {
		if l == "Start" {
			t.Error("Start was offered for a running service")
		}
	}
}

func TestRegistryIgnoresNil(t *testing.T) {
	r := NewRegistry()
	r.Register(nil)

	if r.Len() != 0 {
		t.Errorf("Len() = %d, want 0", r.Len())
	}
}

func TestRunStreamsOutputAndClosesTheChannel(t *testing.T) {
	lines := Drain(run(context.Background(), "done", "sh", "-c", "echo first; echo second"))

	if len(lines) < 3 {
		t.Fatalf("got %d lines, want the command echo plus output and a result: %v", len(lines), lines)
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

func TestLongLinesAreNotTruncated(t *testing.T) {
	// A download progress bar redraws with carriage returns, so it reaches us
	// as one line far past bufio's 64KB default.
	const size = 300 * 1024
	lines := Drain(run(context.Background(), "done", "sh", "-c",
		fmt.Sprintf("printf '%%0.sx' $(seq 1 %d); echo", size)))

	var longest int
	for _, line := range lines {
		if len(line) > longest {
			longest = len(line)
		}
	}
	if longest < size {
		t.Errorf("longest line was %d bytes, want the full %d — output was truncated", longest, size)
	}
	if last := lines[len(lines)-1]; last != "✓ done" {
		t.Errorf("last line = %q, want success", last)
	}
}

func TestAnUnreadableStreamIsReportedNotSilentlyTruncated(t *testing.T) {
	lines := Drain(run(context.Background(), "done", "sh", "-c",
		fmt.Sprintf("printf '%%0.sx' $(seq 1 %d); echo", maxScanLine+1024)))

	last := lines[len(lines)-1]
	if last == "✓ done" {
		t.Fatal("a truncated stream reported success")
	}
	if !strings.HasPrefix(last, "✗") {
		t.Errorf("last line = %q, want a failure marker", last)
	}
	if !strings.Contains(last, "too long") {
		t.Errorf("last line = %q, want the reason named", last)
	}
}
