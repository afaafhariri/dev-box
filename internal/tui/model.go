// Package tui is the terminal interface: an Elm-style Bubble Tea model over
// the store. Phase 1 is a single list view of everything detected.
package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"devenv/internal/detector"
	"devenv/internal/store"
)

// Scanner is the detection engine as the TUI needs it.
type Scanner interface {
	// RunAll blocks until every detector has finished or ctx is cancelled.
	RunAll(ctx context.Context)
	// Len is the number of detectors, used for the progress readout.
	Len() int
}

// Messages carried from the engine and store into the update loop.
type (
	// itemUpdatedMsg means the store changed. The item is carried for
	// progress counting; the view always re-reads the store.
	itemUpdatedMsg struct{ Item detector.Item }
	// scanCompleteMsg means every detector has returned.
	scanCompleteMsg struct{ Elapsed time.Duration }
	// subClosedMsg means the store closed, so no further updates will arrive.
	subClosedMsg struct{}
)

// Model is the root model. Phase 2 splits the list into per-category pages and
// adds a detail view; the sub-model seams are not built yet.
type Model struct {
	ctx     context.Context
	store   *store.Store
	scanner Scanner
	sub     <-chan detector.Item

	keys   keyMap
	styles styles
	spin   spinner.Model

	width  int
	height int

	// visible is the flattened, ordered list the cursor moves through.
	visible []detector.Item
	cursor  int
	offset  int

	showMissing bool

	scanning  bool
	seen      map[string]bool
	scanStart time.Time
	lastScan  time.Time
	elapsed   time.Duration
}

// New builds the root model. The store subscription is taken once here rather
// than per message, so the update loop reuses one channel instead of
// registering a new subscriber on every item.
func New(ctx context.Context, s *store.Store, scanner Scanner) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	st := newStyles()
	sp.Style = st.spinner

	return Model{
		ctx: ctx,
		// The first scan is kicked off by Init, so the model starts busy.
		scanning:  true,
		scanStart: time.Now(),
		store:     s,
		scanner:   scanner,
		sub:       s.Subscribe(),
		keys:      newKeyMap(),
		styles:    st,
		spin:      sp,
		seen:      make(map[string]bool),
	}
}

// Init starts the first scan and arms the store listener.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spin.Tick,
		m.scanCmd(),
		waitForItem(m.sub),
	)
}

// scanCmd runs the engine off the update loop and reports how long it took.
func (m Model) scanCmd() tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		m.scanner.RunAll(m.ctx)
		return scanCompleteMsg{Elapsed: time.Since(start)}
	}
}

// waitForItem blocks on the subscription and turns the next item into a
// message. It is re-armed after each delivery with the same channel.
func waitForItem(sub <-chan detector.Item) tea.Cmd {
	return func() tea.Msg {
		item, ok := <-sub
		if !ok {
			return subClosedMsg{}
		}
		return itemUpdatedMsg{Item: item}
	}
}

// refresh rebuilds the visible list from the store, keeping the cursor on the
// same item where it still exists.
func (m *Model) refresh() {
	var selected string
	if m.cursor >= 0 && m.cursor < len(m.visible) {
		selected = m.visible[m.cursor].Name
	}

	m.visible = m.visible[:0]
	for _, cat := range detector.Categories {
		for _, item := range m.store.ByCategory(cat) {
			if !m.showMissing && !item.Status.Found() {
				continue
			}
			m.visible = append(m.visible, item)
		}
	}

	m.cursor = 0
	for i, item := range m.visible {
		if item.Name == selected {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.syncOffset()
}

// cursorLine is the rendered line index of the selected item, counting the
// category headings rendered above it.
func (m Model) cursorLine() int {
	line := 0
	var lastCat detector.Category
	for i, item := range m.visible {
		if item.Category != lastCat {
			lastCat = item.Category
			line++
		}
		if i == m.cursor {
			return line
		}
		line++
	}
	return 0
}

// totalLines is how many lines the list renders to, headings included.
func (m Model) totalLines() int {
	total := len(m.visible)
	var lastCat detector.Category
	for _, item := range m.visible {
		if item.Category != lastCat {
			lastCat = item.Category
			total++
		}
	}
	return total
}

// syncOffset scrolls the list the minimum amount needed to keep the cursor on
// screen. It runs on every cursor or size change so the view itself stays a
// pure function of the model.
func (m *Model) syncOffset() {
	height := m.height - chromeLines
	total := m.totalLines()

	if m.height == 0 || height >= total {
		m.offset = 0
		return
	}
	if height < 1 {
		height = 1
	}

	line := m.cursorLine()
	if line < m.offset {
		m.offset = line
	}
	if line >= m.offset+height {
		m.offset = line - height + 1
	}
	if max := total - height; m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// roundDuration trims a scan duration to something readable in the header.
func roundDuration(d time.Duration) time.Duration {
	switch {
	case d >= time.Second:
		return d.Round(10 * time.Millisecond)
	default:
		return d.Round(time.Millisecond)
	}
}

// missingCount is how many detected-as-absent items the filter is hiding.
func (m Model) missingCount() int {
	return m.store.Len() - m.store.Found()
}
