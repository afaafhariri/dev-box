package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"devenv/internal/detector"
	"devenv/internal/store"
)

// stubScanner stands in for the detection engine.
type stubScanner struct {
	n    int
	runs int
}

func (s *stubScanner) RunAll(context.Context) { s.runs++ }
func (s *stubScanner) Len() int               { return s.n }

func item(name string, cat detector.Category, status detector.Status) detector.Item {
	return detector.Item{Name: name, Category: cat, Status: status, Version: "1.0.0"}
}

// newTestModel builds a model over a pre-populated store, sized like a normal
// terminal, with the list already built.
func newTestModel(t *testing.T, items ...detector.Item) (Model, *store.Store, *stubScanner) {
	t.Helper()

	st := store.New()
	t.Cleanup(st.Close)
	for _, it := range items {
		st.Set(it)
	}

	scanner := &stubScanner{n: len(items)}
	m := New(context.Background(), st, scanner)
	m.width, m.height = 100, 40
	m.refresh()

	return m, st, scanner
}

func TestRefreshOrdersByCategoryThenName(t *testing.T) {
	m, _, _ := newTestModel(t,
		item("Git", detector.CategoryTool, detector.StatusInstalled),
		item("Redis", detector.CategoryServer, detector.StatusRunning),
		item("Python", detector.CategoryLanguage, detector.StatusInstalled),
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
	)

	// Languages, then servers, then tools — the order in detector.Categories.
	want := []string{"Go", "Python", "Redis", "Git"}
	if len(m.visible) != len(want) {
		t.Fatalf("visible = %d items, want %d", len(m.visible), len(want))
	}
	for i := range want {
		if m.visible[i].Name != want[i] {
			t.Errorf("visible[%d] = %q, want %q", i, m.visible[i].Name, want[i])
		}
	}
}

func TestMissingItemsAreHiddenByDefault(t *testing.T) {
	m, _, _ := newTestModel(t,
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
		item("Rust", detector.CategoryLanguage, detector.StatusNotFound),
	)

	if len(m.visible) != 1 {
		t.Fatalf("visible = %d items, want only the installed one", len(m.visible))
	}
	if m.missingCount() != 1 {
		t.Errorf("missingCount() = %d, want 1", m.missingCount())
	}
}

func TestToggleRevealsMissingItems(t *testing.T) {
	m, _, _ := newTestModel(t,
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
		item("Rust", detector.CategoryLanguage, detector.StatusNotFound),
	)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = next.(Model)

	if len(m.visible) != 2 {
		t.Fatalf("visible = %d items after toggle, want 2", len(m.visible))
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if got := len(next.(Model).visible); got != 1 {
		t.Errorf("visible = %d after toggling back, want 1", got)
	}
}

func TestCursorMovementIsClamped(t *testing.T) {
	m, _, _ := newTestModel(t,
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
		item("Python", detector.CategoryLanguage, detector.StatusInstalled),
	)

	up := func(m Model) Model {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		return next.(Model)
	}
	down := func(m Model) Model {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		return next.(Model)
	}

	m = up(up(m))
	if m.cursor != 0 {
		t.Errorf("cursor = %d after moving up past the top, want 0", m.cursor)
	}

	m = down(down(down(m)))
	if m.cursor != 1 {
		t.Errorf("cursor = %d after moving down past the end, want 1", m.cursor)
	}
}

func TestCursorStaysOnTheSameItemAcrossRefresh(t *testing.T) {
	m, st, _ := newTestModel(t,
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
		item("Python", detector.CategoryLanguage, detector.StatusInstalled),
	)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = next.(Model)
	selected := m.visible[m.cursor].Name

	// A slower detector lands and sorts above the selection.
	st.Set(item("Docker", detector.CategoryServer, detector.StatusRunning))
	m.refresh()

	if got := m.visible[m.cursor].Name; got != selected {
		t.Errorf("cursor moved to %q, want it to stay on %q", got, selected)
	}
}

func TestItemUpdateRearmsTheListener(t *testing.T) {
	m, _, _ := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	next, cmd := m.Update(itemUpdatedMsg{Item: item("Redis", detector.CategoryServer, detector.StatusRunning)})
	if cmd == nil {
		t.Fatal("Update(itemUpdatedMsg) returned no command — the listener was not re-armed")
	}
	if !next.(Model).seen["Redis"] {
		t.Error("progress count did not record the item")
	}
}

func TestItemUpdateDoesNotWriteBackToTheStore(t *testing.T) {
	m, st, _ := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))

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

func TestScanCompleteClearsTheScanningState(t *testing.T) {
	m, _, _ := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))
	if !m.scanning {
		t.Fatal("model should start in the scanning state")
	}

	next, _ := m.Update(scanCompleteMsg{Elapsed: 250 * time.Millisecond})
	m = next.(Model)

	if m.scanning {
		t.Error("scanning still set after scanCompleteMsg")
	}
	if m.elapsed != 250*time.Millisecond {
		t.Errorf("elapsed = %v, want 250ms", m.elapsed)
	}
	if m.lastScan.IsZero() {
		t.Error("lastScan not stamped")
	}
}

func TestRescanIsIgnoredWhileAlreadyScanning(t *testing.T) {
	m, _, _ := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	// The model starts scanning; 'r' must not stack a second scan.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		t.Error("rescan started while a scan was already in flight")
	}

	next, _ := m.Update(scanCompleteMsg{})
	next, cmd = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("rescan did nothing once the previous scan had finished")
	}
	if !next.(Model).scanning {
		t.Error("rescan did not re-enter the scanning state")
	}
}

func TestQuitKeys(t *testing.T) {
	m, _, _ := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyEsc},
		{Type: tea.KeyCtrlC},
	} {
		if _, cmd := m.Update(msg); cmd == nil {
			t.Errorf("key %v did not quit", msg)
		}
	}
}

func TestWindowSizeUpdatesTheModel(t *testing.T) {
	m, _, _ := newTestModel(t, item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	m = next.(Model)

	if m.width != 120 || m.height != 50 {
		t.Errorf("size = %dx%d, want 120x50", m.width, m.height)
	}
}

func TestViewRendersItemsAndHeadings(t *testing.T) {
	m, _, _ := newTestModel(t,
		item("Go", detector.CategoryLanguage, detector.StatusInstalled),
		item("Redis", detector.CategoryServer, detector.StatusRunning),
	)
	m, _ = mustUpdate(m, scanCompleteMsg{Elapsed: time.Second})

	view := m.View()

	for _, want := range []string{"devenv", "Go", "Redis", "LANGUAGES", "SERVERS", "running", "rescan"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() is missing %q\n%s", want, view)
		}
	}
}

func TestViewHandlesAnEmptyStore(t *testing.T) {
	m, _, _ := newTestModel(t)

	if view := m.View(); !strings.Contains(view, "nothing detected yet") {
		t.Errorf("View() on an empty store = %q", view)
	}
}

func TestViewFitsTheTerminalHeight(t *testing.T) {
	var items []detector.Item
	for i := 0; i < 40; i++ {
		items = append(items, item(string(rune('A'+i%26))+string(rune('0'+i/26)), detector.CategoryTool, detector.StatusInstalled))
	}

	m, _, _ := newTestModel(t, items...)
	m.height = 20
	m.syncOffset()

	if got := strings.Count(m.View(), "\n"); got > 20 {
		t.Errorf("View() rendered %d lines into a 20-line terminal", got)
	}
}

func TestScrollingKeepsTheCursorVisible(t *testing.T) {
	var items []detector.Item
	for i := 0; i < 30; i++ {
		items = append(items, item(string(rune('a'+i)), detector.CategoryTool, detector.StatusInstalled))
	}

	m, _, _ := newTestModel(t, items...)
	m.height = 12
	m.syncOffset()

	// Jump to the bottom; the cursor's line must be inside the window.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = next.(Model)

	height := m.height - chromeLines
	line := m.cursorLine()

	if line < m.offset || line >= m.offset+height {
		t.Errorf("cursor line %d outside window [%d, %d)", line, m.offset, m.offset+height)
	}
}

func mustUpdate(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}
