package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devenv/internal/action"
	"devenv/internal/detector"
	"devenv/internal/platform"
	"devenv/internal/store"
)

// stubScanner stands in for the detection engine.
type stubScanner struct{ n, runs int }

func (s *stubScanner) RunAll(context.Context) { s.runs++ }
func (s *stubScanner) Len() int               { return s.n }

// item is an item as a scan leaves it: ownership already resolved, which is
// what decides the actions offered for it.
func item(name string, cat detector.Category, status detector.Status) detector.Item {
	return detector.Item{
		Name: name, Category: cat, Status: status,
		Version:   "1.0.0",
		Path:      "/opt/homebrew/bin/" + strings.ToLower(name),
		ManagedBy: platform.Homebrew, PackageID: strings.ToLower(name),
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

	actions := action.RegistryFor(platform.Darwin)

	m := New(context.Background(), st, &stubScanner{n: len(items)}, actions, 0)
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
	m, _ := newTestModel(t, brewItem("Redis", detector.CategoryServer, detector.StatusStopped))

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
	m, _ := newTestModel(t, brewItem("Redis", detector.CategoryServer, detector.StatusStopped))

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

// brewItem is item under its older name, kept for the destructive-action
// tests that read better spelling out what they rely on.
func brewItem(name string, cat detector.Category, status detector.Status) detector.Item {
	return item(name, cat, status)
}

// destructiveModel wires a probe whose symlinks resolve into the Cellar.
func destructiveModel(t *testing.T, items ...detector.Item) Model {
	t.Helper()

	st := store.New()
	t.Cleanup(st.Close)
	for _, it := range items {
		st.Set(it)
	}

	actions := action.RegistryFor(platform.Darwin)

	m := New(context.Background(), st, &stubScanner{n: len(items)}, actions, 0)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	next, _ = next.(Model).Update(scanCompleteMsg{Elapsed: time.Millisecond})
	return next.(Model)
}

// selectAction moves the cursor onto the action with the given label.
func selectAction(t *testing.T, m Model, label string) Model {
	t.Helper()

	for i := 0; i < 8; i++ {
		if sel, ok := m.detail.Selected(); ok && sel.Label() == label {
			return m
		}
		m = press(m, "l")
	}
	t.Fatalf("no %q action offered", label)
	return m
}

func TestDestructiveActionAsksBeforeRunning(t *testing.T) {
	m := destructiveModel(t, brewItem("Redis", detector.CategoryServer, detector.StatusStopped))
	m = press(m, "enter")
	m = selectAction(t, m, "Uninstall")

	// First enter must only ask.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if cmd != nil {
		t.Error("a destructive action started on the first keypress")
	}
	if !m.detail.Confirming() {
		t.Fatal("no confirmation was requested")
	}
	if m.detail.Running() {
		t.Error("the action is running despite an unanswered confirmation")
	}

	view := m.View()
	if !strings.Contains(view, "brew uninstall") {
		t.Errorf("the confirmation does not name the command:\n%s", view)
	}
	if !strings.Contains(view, "enter confirm") {
		t.Error("the help line does not explain how to answer")
	}
}

func TestConfirmingRunsTheAction(t *testing.T) {
	m := destructiveModel(t, brewItem("Redis", detector.CategoryServer, detector.StatusStopped))
	m = press(m, "enter")
	m = selectAction(t, m, "Uninstall")

	m = press(m, "enter") // ask
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if cmd == nil {
		t.Fatal("the confirmed action did not start")
	}
	if !m.detail.Running() {
		t.Error("the action is not marked running after confirmation")
	}
	if m.detail.Confirming() {
		t.Error("the confirmation outlived the answer")
	}
}

func TestEscapeCancelsTheConfirmationWithoutLeavingThePage(t *testing.T) {
	m := destructiveModel(t, brewItem("Redis", detector.CategoryServer, detector.StatusStopped))
	m = press(m, "enter")
	m = selectAction(t, m, "Uninstall")
	m = press(m, "enter") // ask

	m = press(m, "esc")

	if m.detail.Confirming() {
		t.Error("esc did not cancel the confirmation")
	}
	if m.detail.Running() {
		t.Error("esc ran the action")
	}
	// Saying no should not also lose your place.
	if m.page != PageDetail {
		t.Errorf("page = %v, want to stay on the detail view", m.page)
	}

	// A second esc leaves, as usual.
	m = press(m, "esc")
	if m.page == PageDetail {
		t.Error("esc did not leave the detail view once nothing was pending")
	}
}

func TestMovingCancelsAPendingConfirmation(t *testing.T) {
	m := destructiveModel(t, brewItem("Redis", detector.CategoryServer, detector.StatusStopped))
	m = press(m, "enter")
	m = selectAction(t, m, "Uninstall")
	m = press(m, "enter") // ask

	// Moving to another action must not leave a yes armed on it.
	m = press(m, "h")

	if m.detail.Confirming() {
		t.Error("moving between actions left the confirmation armed")
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected the newly selected action to run")
	}
	if got, _ := next.(Model).detail.Selected(); got.Label() == "Uninstall" {
		t.Error("the cursor was still on Uninstall")
	}
}

func TestNonDestructiveActionsRunImmediately(t *testing.T) {
	m := destructiveModel(t, brewItem("Redis", detector.CategoryServer, detector.StatusStopped))
	m = press(m, "enter")
	m = selectAction(t, m, "Start")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Start did not run")
	}
	if next.(Model).detail.Confirming() {
		t.Error("Start asked for confirmation — only destructive actions should")
	}
}

func TestDetailExplainsWhyThereAreNoActions(t *testing.T) {
	// "No actions available" on its own sends people to the source to work
	// out why. An nvm-managed Node has a real answer, and this is where it
	// belongs.
	node := detector.Item{
		Name: "Node.js", Category: detector.CategoryLanguage, Status: detector.StatusInstalled,
		Version: "22.13.1", Path: "/Users/x/.nvm/versions/node/v22.13.1/bin/node",
		ManagedBy: platform.NVM, PackageID: "22.13.1",
	}

	m, _ := newTestModel(t, node)
	m = press(m, "enter")
	view := m.View()

	if !strings.Contains(view, "nvm") {
		t.Errorf("detail view does not name the owner:\n%s", view)
	}
	if !strings.Contains(view, "nvm uninstall 22.13.1") {
		t.Errorf("detail view does not say what to run instead:\n%s", view)
	}
	if strings.Contains(view, "no actions available for this item") {
		t.Error("fell back to the unexplained message despite knowing the owner")
	}
}

func TestDetailShowsTheOwnerOfEveryInstall(t *testing.T) {
	m, _ := newTestModel(t, item("Git", detector.CategoryTool, detector.StatusInstalled))
	m = press(m, "enter")

	if !strings.Contains(m.View(), "managed by") {
		t.Error("detail view does not show who manages the install")
	}
}

func TestSystemInstallsOfferNoActionsButExplainWhy(t *testing.T) {
	ruby := detector.Item{
		Name: "Ruby", Category: detector.CategoryLanguage, Status: detector.StatusInstalled,
		Version: "2.6.10", Path: "/usr/bin/ruby",
		ManagedBy: platform.System, PackageID: "ruby",
	}

	m, _ := newTestModel(t, ruby)
	m = press(m, "enter")
	view := m.View()

	if strings.Contains(view, "Uninstall") {
		t.Error("Uninstall was offered for a system install")
	}
	if !strings.Contains(view, "operating system") {
		t.Errorf("no explanation for a protected install:\n%s", view)
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

	m := New(context.Background(), st, &stubScanner{n: 1}, action.NewRegistry(), 0)
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

func TestSearchFiltersEveryPage(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)

	m = press(m, "/")
	if !m.searching {
		t.Fatal("'/' did not enter search mode")
	}

	for _, r := range "redis" {
		m = press(m, string(r))
	}

	view := m.View()
	if !strings.Contains(view, "Redis") {
		t.Error("the matching item was filtered out")
	}
	if strings.Contains(view, "Ollama") {
		t.Error("a non-matching item survived the filter")
	}

	// Enter keeps the filter and hands the keyboard back.
	m = press(m, "enter")
	if m.searching {
		t.Error("enter did not leave search mode")
	}
	if m.query != "redis" {
		t.Errorf("query = %q, want it kept after leaving search mode", m.query)
	}
	if !strings.Contains(m.View(), "filter: redis") {
		t.Error("the standing filter is not shown once the input loses focus")
	}
}

func TestLettersTypedIntoSearchAreNotCommands(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	m = press(m, "/")

	// "a" toggles missing items and "r" rescans outside search mode; inside
	// it they are just letters.
	before := m.showMissing
	m = press(m, "a")
	m = press(m, "r")

	if m.showMissing != before {
		t.Error("typing 'a' into the search box toggled the missing filter")
	}
	if m.query != "ar" {
		t.Errorf("query = %q, want the letters to have been typed", m.query)
	}
}

func TestBackspaceAndEscapeInSearch(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	m = press(m, "/")
	for _, r := range "redis" {
		m = press(m, string(r))
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = next.(Model)
	if m.query != "redi" {
		t.Errorf("query = %q after backspace, want redi", m.query)
	}

	m = press(m, "esc")
	if m.searching || m.query != "" {
		t.Errorf("esc left searching=%v query=%q, want both cleared", m.searching, m.query)
	}
	if !strings.Contains(m.View(), "Ollama") {
		t.Error("clearing the search did not restore the full list")
	}
}

func TestEscapeClearsAStandingFilter(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	m = press(m, "/")
	m = press(m, "r")
	m = press(m, "enter") // keep the filter, leave the input

	m = press(m, "esc")
	if m.query != "" {
		t.Errorf("query = %q, want esc to clear a standing filter", m.query)
	}
}

func TestBackgroundRefreshRearmsItself(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	next, _ := m.Update(scanCompleteMsg{Elapsed: time.Millisecond})
	m = next.(Model)
	m.refreshEvery = time.Minute

	next, cmd := m.Update(refreshTickMsg{})
	m = next.(Model)

	if cmd == nil {
		t.Fatal("the refresh tick produced no work")
	}
	if !m.scanning {
		t.Error("the refresh tick did not start a scan")
	}
}

func TestRefreshIsOffByDefault(t *testing.T) {
	m, _ := newTestModel(t, sampleItems()...)
	if cmd := m.refreshCmd(); cmd != nil {
		t.Error("background refresh scheduled itself without being asked for")
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
