package pages

import (
	"strings"
	"testing"
	"time"

	"devenv/internal/action"
	"devenv/internal/detector"
	"devenv/internal/platform"
	"devenv/internal/tui/theme"
)

func item(name string, cat detector.Category, status detector.Status) detector.Item {
	return detector.Item{Name: name, Category: cat, Status: status, Version: "1.0.0"}
}

func items() []detector.Item {
	return []detector.Item{
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
		item("Python", detector.CategoryLanguage, detector.StatusInstalled),
		item("Redis", detector.CategoryServer, detector.StatusRunning),
		item("Rust", detector.CategoryLanguage, detector.StatusNotFound),
	}
}

func TestListFiltersToItsCategories(t *testing.T) {
	l := NewList("Servers", theme.New(), detector.CategoryServer)
	l.SetItems(items())

	if l.Len() != 1 {
		t.Fatalf("Len() = %d, want just the server", l.Len())
	}
	if got, _ := l.Selected(); got.Name != "Redis" {
		t.Errorf("Selected() = %q, want Redis", got.Name)
	}
}

func TestDashboardShowsEveryCategory(t *testing.T) {
	l := NewList("Dashboard", theme.New())
	l.SetItems(items())

	// Three found; the absent one is filtered until asked for.
	if l.Len() != 3 {
		t.Errorf("Len() = %d, want 3", l.Len())
	}
}

func TestShowMissingRevealsAbsentItems(t *testing.T) {
	l := NewList("Dashboard", theme.New())
	l.ShowMissing = true
	l.SetItems(items())

	if l.Len() != 4 {
		t.Errorf("Len() = %d, want every item including the absent one", l.Len())
	}
}

func TestSingleCategoryPageOmitsItsOwnHeading(t *testing.T) {
	// Repeating "SERVERS" on the Servers tab is noise.
	single := NewList("Servers", theme.New(), detector.CategoryServer)
	single.SetItems(items())
	if strings.Contains(single.View(), "SERVERS") {
		t.Error("a single-category page repeated its own heading")
	}

	dashboard := NewList("Dashboard", theme.New())
	dashboard.SetItems(items())
	if !strings.Contains(dashboard.View(), "SERVERS") {
		t.Error("the dashboard omitted a category heading")
	}
}

func TestCursorClamps(t *testing.T) {
	l := NewList("Dashboard", theme.New())
	l.SetItems(items())

	l.Move(-5)
	if got, _ := l.Selected(); got.Name != "Go" {
		t.Errorf("moving past the top selected %q", got.Name)
	}

	l.Move(50)
	if _, ok := l.Selected(); !ok {
		t.Error("moving past the end lost the selection")
	}
}

func TestSelectionSurvivesAnUpdate(t *testing.T) {
	l := NewList("Dashboard", theme.New())
	l.SetItems(items())
	l.Move(1)

	selected, _ := l.Selected()
	// A slower detector lands and sorts above the selection.
	l.SetItems(append(items(), item("Docker", detector.CategoryServer, detector.StatusRunning)))

	if got, _ := l.Selected(); got.Name != selected.Name {
		t.Errorf("selection moved from %q to %q", selected.Name, got.Name)
	}
}

func TestEmptyListSaysSo(t *testing.T) {
	l := NewList("AI", theme.New(), detector.CategoryAI)
	l.SetItems(items())

	if !strings.Contains(l.View(), "nothing to show") {
		t.Errorf("empty page rendered as %q", l.View())
	}
	if _, ok := l.Selected(); ok {
		t.Error("an empty list reported a selection")
	}
}

func TestListRespectsItsHeight(t *testing.T) {
	var many []detector.Item
	for i := 0; i < 40; i++ {
		many = append(many, item(string(rune('a'+i%26))+string(rune('0'+i/26)), detector.CategoryTool, detector.StatusInstalled))
	}

	l := NewList("Tools", theme.New(), detector.CategoryTool)
	l.SetItems(many)
	l.SetHeight(10)

	if got := strings.Count(l.View(), "\n") + 1; got > 10 {
		t.Errorf("rendered %d lines into a height of 10", got)
	}
}

func TestUpdateAvailableIsMarked(t *testing.T) {
	outdated := item("Git", detector.CategoryTool, detector.StatusInstalled)
	outdated.UpdateAvail = true

	l := NewList("Tools", theme.New(), detector.CategoryTool)
	l.SetItems([]detector.Item{outdated})

	if !strings.Contains(l.View(), "•") {
		t.Errorf("an available update is not visible in the list:\n%s", l.View())
	}
}

func TestSummaryPrefersMetadataOverPath(t *testing.T) {
	withMeta := detector.Item{
		Name: "Docker", Category: detector.CategoryServer, Status: detector.StatusRunning,
		Path: "/usr/local/bin/docker", Meta: map[string]string{"containers": "3"},
	}
	if got := Summary(withMeta); !strings.Contains(got, "containers 3") {
		t.Errorf("Summary() = %q, want the metadata", got)
	}

	bare := detector.Item{
		Name: "Git", Category: detector.CategoryTool, Status: detector.StatusInstalled,
		Path: "/usr/bin/git",
	}
	if got := Summary(bare); got != "/usr/bin/git" {
		t.Errorf("Summary() = %q, want the path as a fallback", got)
	}
}

func TestSummaryOfAnAbsentItemShowsAnyError(t *testing.T) {
	failed := detector.Item{
		Name: "Docker", Category: detector.CategoryServer, Status: detector.StatusNotFound,
		Meta: map[string]string{"error": "docker: exec failed"},
	}
	if got := Summary(failed); !strings.Contains(got, "exec failed") {
		t.Errorf("Summary() = %q, want the failure reason", got)
	}

	// A plain absence has nothing worth saying.
	absent := detector.Item{Name: "Rust", Status: detector.StatusNotFound}
	if got := Summary(absent); got != "" {
		t.Errorf("Summary() = %q, want empty for a plain absence", got)
	}
}

func TestDetailShowsEverythingKnown(t *testing.T) {
	d := NewDetail(theme.New())
	d.SetHeight(30)
	d.SetItem(detector.Item{
		Name: "PostgreSQL", Category: detector.CategoryServer, Status: detector.StatusRunning,
		Version: "15.19", Path: "/opt/homebrew/bin/psql", DetectedAt: time.Now(),
		ManagedBy: platform.Homebrew, PackageID: "postgresql@15",
		Meta: map[string]string{"port": "5432"},
	}, nil)

	view := d.View()
	for _, want := range []string{"PostgreSQL", "running", "15.19", "/opt/homebrew/bin/psql", "Homebrew", "postgresql@15", "port", "5432"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail view is missing %q\n%s", want, view)
		}
	}
}

func TestDetailExplainsAnEmptyActionBar(t *testing.T) {
	d := NewDetail(theme.New())
	d.SetItem(detector.Item{
		Name: "Node.js", Category: detector.CategoryLanguage, Status: detector.StatusInstalled,
		ManagedBy: platform.NVM, PackageID: "22.13.1",
	}, nil)

	if !strings.Contains(d.View(), "nvm uninstall 22.13.1") {
		t.Errorf("no guidance for an item devenv cannot act on:\n%s", d.View())
	}
}

func TestDetailFallsBackWhenThereIsNothingToExplain(t *testing.T) {
	d := NewDetail(theme.New())
	d.SetItem(detector.Item{Name: "Mystery", Status: detector.StatusInstalled}, nil)

	if !strings.Contains(d.View(), "no actions available") {
		t.Errorf("no fallback message:\n%s", d.View())
	}
}

func TestDetailActionSelection(t *testing.T) {
	actions := action.RegistryFor(platform.Darwin).For(detector.Item{
		Name: "Redis", Category: detector.CategoryServer, Status: detector.StatusStopped,
		ManagedBy: platform.Homebrew, PackageID: "redis",
	})

	d := NewDetail(theme.New())
	d.SetItem(detector.Item{Name: "Redis", Status: detector.StatusStopped}, actions)

	if len(actions) < 2 {
		t.Fatalf("expected several actions, got %d", len(actions))
	}

	first, _ := d.Selected()
	d.Move(1)
	second, _ := d.Selected()
	if first.Label() == second.Label() {
		t.Error("moving did not change the selected action")
	}

	// Clamped at both ends.
	d.Move(-10)
	if got, _ := d.Selected(); got.Label() != first.Label() {
		t.Error("moving past the start did not clamp")
	}
	d.Move(100)
	if _, ok := d.Selected(); !ok {
		t.Error("moving past the end lost the selection")
	}
}

func TestDetailOutputPaneKeepsTheTail(t *testing.T) {
	d := NewDetail(theme.New())
	d.SetHeight(40)
	d.SetItem(item("Redis", detector.CategoryServer, detector.StatusStopped), nil)

	d.StartRun()
	for i := 0; i < maxOutputLines+50; i++ {
		d.AppendOutput("line")
	}

	if got := len(d.Output()); got > maxOutputLines {
		t.Errorf("output pane holds %d lines, want at most %d", got, maxOutputLines)
	}
	if !d.Running() {
		t.Error("Running() = false during a run")
	}

	d.FinishRun()
	if d.Running() {
		t.Error("Running() = true after FinishRun")
	}
}

func TestOpeningAnItemClearsThePreviousOutput(t *testing.T) {
	d := NewDetail(theme.New())
	d.SetItem(item("Redis", detector.CategoryServer, detector.StatusStopped), nil)
	d.StartRun()
	d.AppendOutput("from the last item")

	d.SetItem(item("Docker", detector.CategoryServer, detector.StatusRunning), nil)

	if len(d.Output()) != 0 {
		t.Error("output from the previous item survived")
	}
	if d.Running() {
		t.Error("still marked running after opening another item")
	}
}

func TestDetailUpdateIgnoresOtherItems(t *testing.T) {
	d := NewDetail(theme.New())
	d.SetItem(item("Redis", detector.CategoryServer, detector.StatusStopped), nil)

	// A rescan result for something else must not replace what is on screen.
	d.Update(item("Docker", detector.CategoryServer, detector.StatusRunning), nil)

	if d.Item().Name != "Redis" {
		t.Errorf("open item became %q", d.Item().Name)
	}

	d.Update(item("Redis", detector.CategoryServer, detector.StatusRunning), nil)
	if d.Item().Status != detector.StatusRunning {
		t.Error("a rescan of the open item was not applied")
	}
}

func TestConfirmationState(t *testing.T) {
	d := NewDetail(theme.New())
	d.SetItem(item("Redis", detector.CategoryServer, detector.StatusStopped), nil)

	d.RequestConfirm("Really?")
	if !d.Confirming() {
		t.Fatal("Confirming() = false after RequestConfirm")
	}
	if !strings.Contains(d.View(), "Really?") {
		t.Error("the question is not shown")
	}

	d.CancelConfirm()
	if d.Confirming() {
		t.Error("Confirming() = true after CancelConfirm")
	}

	// Starting a run clears any pending question.
	d.RequestConfirm("Really?")
	d.StartRun()
	if d.Confirming() {
		t.Error("a pending confirmation survived the run starting")
	}
}
