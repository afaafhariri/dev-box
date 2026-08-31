package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// timeSince is time.Since, indirected so the header can be tested.
var timeSince = time.Since

// Update handles one message and returns the next model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case itemUpdatedMsg:
		// The engine has already written to the store; the model only needs
		// to re-read it. Writing the item back here would re-notify
		// subscribers and loop forever.
		m.seen[msg.Item.Name] = true
		m.refresh()
		return m, waitForItem(m.sub)

	case scanCompleteMsg:
		m.scanning = false
		m.elapsed = msg.Elapsed
		m.lastScan = time.Now()
		// Once a fresh scan has landed, the cache notice no longer applies.
		m.cachedAt = time.Time{}
		m.refresh()

		// A scan requested mid-flight ran against state the request was meant
		// to invalidate, so honour it now rather than showing a stale result.
		if m.pendingScan {
			m.pendingScan = false
			return m, m.startScan()
		}
		return m, nil

	case subClosedMsg:
		// Store closed on shutdown: stop listening, leave the last state up.
		return m, nil

	case actionOutputMsg:
		m.detail.AppendOutput(msg.Line)
		return m, waitForOutput(m.output)

	case actionDoneMsg:
		m.detail.FinishRun()
		m.output = nil
		// An action that started or stopped something has just invalidated
		// what is on screen, so confirm the new state by rescanning.
		return m, m.startScan()
	}

	// Anything else is the spinner's own tick.
	var cmd tea.Cmd
	m.spin, cmd = m.spin.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Quit works everywhere, including mid-action: the context cancels the
	// subprocess on the way out.
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}

	if m.page == PageDetail {
		return m.handleDetailKey(msg)
	}
	return m.handleListKey(msg)
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.list().Move(-1)

	case key.Matches(msg, m.keys.Down):
		m.list().Move(1)

	case key.Matches(msg, m.keys.Top):
		m.list().MoveTo(0)

	case key.Matches(msg, m.keys.Bottom):
		m.list().MoveTo(m.list().Len() - 1)

	case key.Matches(msg, m.keys.NextTab):
		m.page = nextPage(m.page, 1)

	case key.Matches(msg, m.keys.PrevTab):
		m.page = nextPage(m.page, -1)

	case key.Matches(msg, m.keys.Jump):
		if page, ok := pageForDigit(msg.String()); ok {
			m.page = page
		}

	case key.Matches(msg, m.keys.Enter):
		item, ok := m.list().Selected()
		if !ok {
			return m, nil
		}
		m.detail.SetItem(item, m.actions.For(item))
		m.prev, m.page = m.page, PageDetail

	case key.Matches(msg, m.keys.Hidden):
		m.showMissing = !m.showMissing
		m.refresh()

	case key.Matches(msg, m.keys.Rescan):
		return m, m.startScan()
	}

	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.page = m.prev

	case key.Matches(msg, m.keys.Left), key.Matches(msg, m.keys.Up):
		m.detail.Move(-1)

	case key.Matches(msg, m.keys.Right), key.Matches(msg, m.keys.Down):
		m.detail.Move(1)

	case key.Matches(msg, m.keys.Enter):
		return m.runAction()

	case key.Matches(msg, m.keys.Rescan):
		return m, m.startScan()
	}

	return m, nil
}

// runAction starts the selected action and begins streaming its output.
func (m Model) runAction() (tea.Model, tea.Cmd) {
	// One at a time: a second start would orphan the first channel.
	if m.detail.Running() {
		return m, nil
	}

	selected, ok := m.detail.Selected()
	if !ok {
		return m, nil
	}

	m.detail.StartRun()
	m.output = selected.Run(m.ctx, m.detail.Item())
	return m, waitForOutput(m.output)
}

// startScan begins a scan, or queues one when a scan is already in flight.
func (m *Model) startScan() tea.Cmd {
	if m.scanning {
		m.pendingScan = true
		return nil
	}
	m.scanning = true
	m.seen = make(map[string]bool)
	return tea.Batch(m.spin.Tick, m.scanCmd())
}

// nextPage moves through the tabs, wrapping at both ends.
func nextPage(current Page, delta int) Page {
	// The detail view is not in the tab ring; stepping from it is a no-op.
	if current == PageDetail {
		return current
	}
	next := (int(current) + delta + len(tabs)) % len(tabs)
	return Page(next)
}

// pageForDigit maps "1".."6" to a tab.
func pageForDigit(s string) (Page, bool) {
	if len(s) != 1 || s[0] < '1' || s[0] > '9' {
		return 0, false
	}
	index := int(s[0] - '1')
	if index >= len(tabs) {
		return 0, false
	}
	return tabs[index].page, true
}
