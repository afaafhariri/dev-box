package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devenv/internal/action"
	"devenv/internal/detector"
	"devenv/internal/probe"
	"devenv/internal/store"
)

// stubScanner stands in for the detection engine.
type stubScanner struct{ n, runs int }

func (s *stubScanner) RunAll(context.Context) { s.runs++ }
func (s *stubScanner) Len() int               { return s.n }

func item(name string, cat detector.Category, status detector.Status) detector.Item {
	return detector.Item{
		Name: name, Category: cat, Status: status,
		Version: "1.0.0", Path: "/opt/homebrew/bin/" + strings.ToLower(name),
	}
}

// sampleItems covers every category, plus one absent item.
func sampleItems() []detector.Item {
	return []detector.Item{
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
		item("Python", detector.CategoryLanguage, detector.StatusInstalled),
		item("Redis", detector.CategoryServer, detector.StatusStopped),
		item("Docker", detector.CategoryServer, detector.StatusRunning),
		item("Ollama", detector.CategoryAI, detector.StatusRunning),
		item("Git", detector.CategoryTool, detector.StatusInstalled),
		item("nvm", detector.CategoryManager, detector.StatusInstalled),
		item("Rust", detector.CategoryLanguage, detector.StatusNotFound),
	}
}

func newTestModel(t *testing.T, items ...detector.Item) (Model, *store.Store) {
	t.Helper()

	st := store.New()
	t.Cleanup(st.Close)
	for _, it := range items {
		st.Set(it)
	}

	// Homebrew present, so the service actions are registered.
	actions := action.DefaultRegistry(&probe.Fake{
		Paths: map[string]string{"brew": "/opt/homebrew/bin/brew"},
	})

	m := New(context.Background(), st, &stubScanner{n: len(items)}, actions)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return next.(Model), st
}

func press(m Model, key string) Model {
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		msg = tea.KeyMsg{Type: tea.KeyShiftTab}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

func TestTabNavigationWraps(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	if m.page != PageDashboard {
		t.Fatalf("page = %v, want the dashboard first", m.page)
	}

	m = press(m, "tab")
	if m.page != PageLanguages {
		t.Errorf("page = %v after tab, want languages", m.page)
	}

	// Wrap forwards off the end.
	for i := 0; i < len(tabs); i++ {
		m = press(m, "tab")
	}
	if m.page != PageLanguages {
		t.Errorf("page = %v after wrapping, want languages again", m.page)
	}

	// And backwards off the start.
	m = press(m, "shift+tab")
	m = press(m, "shift+tab")
	if m.page != PageManagers {
		t.Errorf("page = %v after wrapping backwards, want managers", m.page)
	}
}

func TestDigitJumpsToAPage(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	m = press(m, "3")
	if m.page != PageServers {
		t.Errorf("page = %v after '3', want servers", m.page)
	}

	// There is no seventh tab.
	m = press(m, "9")
	if m.page != PageServers {
		t.Errorf("page = %v after an out-of-range digit, want no change", m.page)
	}
}

func TestEachPageShowsOnlyItsCategory(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	m = press(m, "2") // Languages
	view := m.View()
	if !strings.Contains(view, "Go") || !strings.Contains(view, "Python") {
		t.Error("languages page is missing its own items")
	}
	if strings.Contains(view, "Redis") {
		t.Error("languages page is showing a server")
	}

	m = press(m, "3") // Servers
	view = m.View()
	if !strings.Contains(view, "Redis") {
		t.Error("servers page is missing Redis")
	}
	if strings.Contains(view, "Python") {
		t.Error("servers page is showing a language")
	}
}

func TestDashboardShowsEveryCategory(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	view := m.View()

	for _, want := range []string{"Go", "Redis", "Ollama", "Git", "nvm"} {
		if !strings.Contains(view, want) {
			t.Errorf("dashboard is missing %q", want)
		}
	}
}

func TestEnterOpensDetailAndEscapeReturns(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	m = press(m, "2") // Languages, cursor on Go
	m = press(m, "enter")

	if m.page != PageDetail {
		t.Fatalf("page = %v after enter, want the detail view", m.page)
	}
	if got := m.detail.Item().Name; got != "Go" {
		t.Errorf("detail shows %q, want Go", got)
	}

	view := m.View()
	if !strings.Contains(view, "/opt/homebrew/bin/go") {
		t.Error("detail view does not show the path")
	}

	m = press(m, "esc")
	if m.page != PageLanguages {
		t.Errorf("page = %v after esc, want the page it was opened from", m.page)
	}
}

func TestDetailOffersActionsForAStoppedService(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	m = press(m, "3") // Servers: Docker (running), Redis (stopped)
	for i := 0; i < 5; i++ {
		if sel, ok := m.list().Selected(); ok && sel.Name == "Redis" {
			break
		}
		m = press(m, "j")
	}
	m = press(m, "enter")

	if got := m.detail.Item().Name; got != "Redis" {
		t.Fatalf("detail shows %q, want Redis", got)
	}
	if !strings.Contains(m.View(), "Start") {
		t.Error("a stopped service offers no Start action")
	}
}

func TestDetailOfAPlainToolOffersNoServiceActions(t *testing.T) {
	m, _ := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	m = press(m, "enter")
	view := m.View()

	if strings.Contains(view, "Start") || strings.Contains(view, "Stop") {
		t.Error("a language offers service actions")
	}
}

func TestRunningAnActionStreamsOutput(t *testing.T) {
	m, _ := newTestModel(t, item("Redis", detector.CategoryServer, detector.StatusStopped))

	// Let the opening scan finish, so the rescan below is not merely queued.
	next0, _ := m.Update(scanCompleteMsg{Elapsed: time.Millisecond})
	m = next0.(Model)

	m = press(m, "enter")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if !m.detail.Running() {
		t.Fatal("the action did not start")
	}
	if cmd == nil {
		t.Fatal("no command returned to read the action output")
	}

	// Feed a line back in, as the output reader would.
	next, cmd = m.Update(actionOutputMsg{Line: "$ brew services start redis"})
	m = next.(Model)
	if len(m.detail.Output()) != 1 {
		t.Errorf("output pane holds %d lines, want 1", len(m.detail.Output()))
	}
	if cmd == nil {
		t.Error("the output reader was not re-armed")
	}

	// And finish it.
	next, cmd = m.Update(actionDoneMsg{})
	m = next.(Model)
	if m.detail.Running() {
		t.Error("still marked running after actionDoneMsg")
	}
	// Finishing an action invalidates what is on screen, so a rescan follows.
	if cmd == nil {
		t.Error("no rescan was triggered after the action finished")
	}
	if !m.scanning {
		t.Error("model is not scanning after an action completed")
	}
}

func TestARescanRequestedMidScanIsHonouredLater(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	// An action finishing during the opening scan invalidates results that
	// scan may already have collected.
	next, cmd := m.Update(actionDoneMsg{})
	m = next.(Model)
	if cmd != nil {
		t.Error("a second scan started while one was already running")
	}
	if !m.pendingScan {
		t.Fatal("the rescan was dropped instead of queued")
	}

	next, cmd = m.Update(scanCompleteMsg{Elapsed: time.Millisecond})
	m = next.(Model)
	if cmd == nil {
		t.Error("the queued rescan did not run when the scan finished")
	}
	if !m.scanning {
		t.Error("model is not scanning after the queued rescan started")
	}
	if m.pendingScan {
		t.Error("the queue was not cleared")
	}
}

func TestASecondActionCannotStartWhileOneRuns(t *testing.T) {
	m, _ := newTestModel(t, item("Redis", detector.CategoryServer, detector.StatusStopped))

	m = press(m, "enter")
	m = press(m, "enter") // starts the action

	if !m.detail.Running() {
		t.Fatal("the action did not start")
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("a second action started while one was already running")
	}
}

func TestMissingItemsAreHiddenUntilToggled(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	if strings.Contains(m.View(), "Rust") {
		t.Error("an absent item is shown by default")
	}
	if m.missingCount() != 1 {
		t.Errorf("missingCount() = %d, want 1", m.missingCount())
	}

	m = press(m, "a")
	if !strings.Contains(m.View(), "Rust") {
		t.Error("toggling did not reveal the absent item")
	}

	m = press(m, "a")
	if strings.Contains(m.View(), "Rust") {
		t.Error("toggling back did not hide the absent item")
	}
}

func TestItemUpdateDoesNotWriteBackToTheStore(t *testing.T) {
	m, st := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	// Writing the item back here would notify subscribers again and loop
	// forever, so the update path must only read.
	sub := st.Subscribe()
	m.Update(itemUpdatedMsg{Item: item("Go", detector.CategoryLanguage, detector.StatusInstalled)})

	select {
	case got := <-sub:
		t.Fatalf("handling an update wrote %q back to the store", got.Name)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestScanCompleteClearsScanningState(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	if !m.scanning {
		t.Fatal("the model should start scanning")
	}

	next, _ := m.Update(scanCompleteMsg{Elapsed: 250 * time.Millisecond})
	m = next.(Model)

	if m.scanning {
		t.Error("still scanning after scanCompleteMsg")
	}
	if !strings.Contains(m.View(), "found") {
		t.Error("header does not report the finished scan")
	}
}

func TestRescanIsIgnoredWhileScanning(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}); cmd != nil {
		t.Error("a rescan started while one was already in flight")
	}

	next, _ := m.Update(scanCompleteMsg{})
	next, cmd := next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("rescan did nothing once the previous scan finished")
	}
	if !next.(Model).scanning {
		t.Error("rescan did not re-enter the scanning state")
	}
}

func TestCachedStateIsAnnouncedUntilTheScanLands(t *testing.T) {
	st := store.New()
	defer st.Close()

	// A store primed from a cache, with a scan still running over it.
	st.Prime(store.Snapshot{
		CachedAt: time.Now().Add(-3 * time.Minute),
		Items:    []detector.Item{item("Go", detector.CategoryLanguage, detector.StatusInstalled)},
	})

	m := New(context.Background(), st, &stubScanner{n: 1}, action.NewRegistry())
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(Model)

	// Showing cached data without saying so is the difference between stale
	// and wrong.
	if !strings.Contains(m.View(), "cached") {
		t.Errorf("header does not mention the cache:\n%s", m.View())
	}

	next, _ = m.Update(scanCompleteMsg{Elapsed: time.Second})
	if strings.Contains(next.(Model).View(), "cached") {
		t.Error("the cache notice survived a completed scan")
	}
}

func TestQuitWorksFromEveryPage(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	for _, page := range []string{"1", "2", "3"} {
		m = press(m, page)
		if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); cmd == nil {
			t.Errorf("q did not quit from page %s", page)
		}
	}

	// Including from the detail view, where esc is bound to going back.
	m = press(m, "enter")
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("ctrl+c did not quit from the detail view")
	}
}

func TestViewFitsTheTerminalHeight(t *testing.T) {
	var items []detector.Item
	for i := 0; i < 60; i++ {
		items = append(items, item(string(rune('a'+i%26))+string(rune('0'+i/26)), detector.CategoryTool, detector.StatusInstalled))
	}

	m, _ := newTestModel(t, items...)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = next.(Model)

	if got := strings.Count(m.View(), "\n"); got > 24 {
		t.Errorf("View() rendered %d lines into a 24-line terminal", got)
	}
}

// TestRenderSample prints the real views. Run with -v to eyeball them.
func TestRenderSample(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	next, _ := m.Update(scanCompleteMsg{Elapsed: 280 * time.Millisecond})
	m = next.(Model)

	t.Logf("dashboard:\n%s\n", m.View())

	m = press(m, "3")
	m = press(m, "enter")
	m = press(m, "enter")
	m = m.appendSample()
	t.Logf("detail with action output:\n%s\n", m.View())
}

// appendSample feeds representative output into the pane for the sample.
func (m Model) appendSample() Model {
	for _, line := range []string{
		"$ brew services start redis",
		"==> Successfully started `redis` (label: homebrew.mxcl.redis)",
	} {
		next, _ := m.Update(actionOutputMsg{Line: line})
		m = next.(Model)
	}
	return m
}
