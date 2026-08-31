package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles one message and returns the next model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.syncOffset()
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
		m.refresh()
		return m, nil

	case subClosedMsg:
		// Store closed on shutdown: stop listening, leave the last state up.
		return m, nil
	}

	// Anything else is the spinner's own tick.
	var cmd tea.Cmd
	m.spin, cmd = m.spin.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		m.cursor--
		m.clampCursor()
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.cursor++
		m.clampCursor()
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
		m.clampCursor()
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.cursor = len(m.visible) - 1
		m.clampCursor()
		return m, nil

	case key.Matches(msg, m.keys.Hidden):
		m.showMissing = !m.showMissing
		m.refresh()
		return m, nil

	case key.Matches(msg, m.keys.Rescan):
		if m.scanning {
			return m, nil
		}
		m.scanning = true
		m.seen = make(map[string]bool)
		m.scanStart = time.Now()
		return m, tea.Batch(m.spin.Tick, m.scanCmd())
	}

	return m, nil
}
