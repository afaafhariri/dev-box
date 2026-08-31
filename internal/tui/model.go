// Package tui is the terminal interface: an Elm-style Bubble Tea model over
// the store. The root model owns the tabs and routes messages to whichever
// page is active.
package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"devenv/internal/action"
	"devenv/internal/detector"
	"devenv/internal/store"
	"devenv/internal/tui/pages"
	"devenv/internal/tui/theme"
)

// Scanner is the detection engine as the TUI needs it.
type Scanner interface {
	// RunAll blocks until every detector has finished or ctx is cancelled.
	RunAll(ctx context.Context)
	// Len is the number of detectors, used for the progress readout.
	Len() int
}

// Page identifies a tab.
type Page int

const (
	PageDashboard Page = iota
	PageLanguages
	PageServers
	PageAI
	PageTools
	PageManagers
	// PageDetail is not a tab: it is pushed over the current page and
	// dismissed with Esc.
	PageDetail
)

// tabs are the pages reachable from the tab bar, in order.
var tabs = []struct {
	page  Page
	title string
	cats  []detector.Category
}{
	{PageDashboard, "Dashboard", nil},
	{PageLanguages, "Languages", []detector.Category{detector.CategoryLanguage}},
	{PageServers, "Servers", []detector.Category{detector.CategoryServer}},
	{PageAI, "AI", []detector.Category{detector.CategoryAI}},
	{PageTools, "Tools", []detector.Category{detector.CategoryTool}},
	{PageManagers, "Managers", []detector.Category{detector.CategoryManager}},
}

// Messages carried from the engine, store, and actions into the update loop.
type (
	// itemUpdatedMsg means the store changed. The item is carried for
	// progress counting; the pages always re-read the store.
	itemUpdatedMsg struct{ Item detector.Item }
	// scanCompleteMsg means every detector has returned.
	scanCompleteMsg struct{ Elapsed time.Duration }
	// subClosedMsg means the store closed, so no further updates will arrive.
	subClosedMsg struct{}
	// actionOutputMsg is one line from a running action.
	actionOutputMsg struct{ Line string }
	// actionDoneMsg means the running action finished.
	actionDoneMsg struct{}
)

// Model is the root model.
type Model struct {
	ctx     context.Context
	store   *store.Store
	scanner Scanner
	actions *action.Registry
	sub     <-chan detector.Item

	keys   keyMap
	styles theme.Styles
	spin   spinner.Model

	width  int
	height int

	page Page
	// prev is the tab to return to when the detail view is dismissed.
	prev   Page
	lists  map[Page]*pages.List
	detail *pages.Detail

	// output is the channel of the action currently running.
	output <-chan string

	showMissing bool

	scanning bool
	// pendingScan records a scan asked for while one was already running, so
	// it can be honoured rather than dropped.
	pendingScan bool
	seen        map[string]bool
	lastScan    time.Time
	elapsed     time.Duration
	cachedAt    time.Time
}

// New builds the root model. The store subscription is taken once here rather
// than per message, so the update loop reuses one channel instead of
// registering a new subscriber on every item.
func New(ctx context.Context, s *store.Store, scanner Scanner, actions *action.Registry) Model {
	styles := theme.New()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styles.Spinner

	lists := make(map[Page]*pages.List, len(tabs))
	for _, t := range tabs {
		lists[t.page] = pages.NewList(t.title, styles, t.cats...)
	}

	m := Model{
		ctx:     ctx,
		store:   s,
		scanner: scanner,
		actions: actions,
		sub:     s.Subscribe(),
		keys:    newKeyMap(),
		styles:  styles,
		spin:    sp,
		lists:   lists,
		detail:  pages.NewDetail(styles),
		seen:    make(map[string]bool),
		// The first scan is kicked off by Init, so the model starts busy.
		scanning: true,
		cachedAt: s.PrimedAt(),
	}
	m.refresh()
	return m
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

// waitForOutput reads one line from a running action.
func waitForOutput(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return actionDoneMsg{}
		}
		return actionOutputMsg{Line: line}
	}
}

// refresh pushes the current store contents into every page.
func (m *Model) refresh() {
	items := m.store.All()

	for _, list := range m.lists {
		list.ShowMissing = m.showMissing
		list.SetItems(items)
	}

	// Keep an open detail view in step with the rescan behind it.
	if open := m.detail.Item(); open.Name != "" {
		if item, ok := m.store.Get(open.Name); ok {
			m.detail.Update(item, m.actions.For(item))
		}
	}
	m.layout()
}

// layout hands each page the space it may draw in.
func (m *Model) layout() {
	// Header, tab bar, spacing, and help line.
	body := m.height - 6
	if body < 1 {
		body = 1
	}

	for _, list := range m.lists {
		list.SetHeight(body)
	}
	m.detail.SetHeight(body)
}

// list is the active list page, or the dashboard when the detail view is open.
func (m Model) list() *pages.List {
	if l, ok := m.lists[m.page]; ok {
		return l
	}
	return m.lists[m.prev]
}

// missingCount is how many detected-as-absent items the filter is hiding.
func (m Model) missingCount() int {
	return m.store.Len() - m.store.Found()
}

// roundDuration trims a scan duration to something readable in the header.
func roundDuration(d time.Duration) time.Duration {
	if d >= time.Second {
		return d.Round(10 * time.Millisecond)
	}
	return d.Round(time.Millisecond)
}
