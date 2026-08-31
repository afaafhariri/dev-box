package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the whole screen.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(m.header())
	b.WriteString("\n")
	b.WriteString(m.tabBar())
	b.WriteString("\n\n")

	if m.page == PageDetail {
		b.WriteString(m.detail.View())
	} else {
		b.WriteString(m.list().View())
	}

	b.WriteString("\n")
	b.WriteString(m.help())
	return b.String()
}

// header is the title bar plus scan state.
func (m Model) header() string {
	return lipgloss.JoinHorizontal(lipgloss.Left,
		m.styles.Title.Render("devenv"), "  ",
		m.styles.Subtitle.Render("Dev Environment Manager"), "  ",
		m.styles.Badge.Render(m.scanState()),
	)
}

// scanState describes what the scanner is doing, or what it last did.
func (m Model) scanState() string {
	if m.scanning {
		// A primed store is showing cached data until the scan lands, and
		// saying so is the difference between "stale" and "wrong".
		if !m.cachedAt.IsZero() && len(m.seen) == 0 {
			return fmt.Sprintf("%s cached %s ago · rescanning",
				m.spin.View(), roundDuration(timeSince(m.cachedAt)))
		}
		return fmt.Sprintf("%s scanning %d/%d", m.spin.View(), len(m.seen), m.scanner.Len())
	}

	return fmt.Sprintf("%d of %d found · %s in %s",
		m.store.Found(), m.store.Len(),
		m.lastScan.Format("15:04:05"), roundDuration(m.elapsed),
	)
}

// tabBar renders the page tabs.
func (m Model) tabBar() string {
	active := m.page
	if active == PageDetail {
		active = m.prev
	}

	parts := make([]string, 0, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf("%d %s", i+1, t.title)
		parts = append(parts, m.styles.Tab(label, t.page == active))
	}

	return " " + strings.Join(parts, m.styles.TabBar("·"))
}

// help is the key hint line, which changes with the active page.
func (m Model) help() string {
	if m.page == PageDetail {
		hints := []string{"←/→ action", "enter run", "esc back", "q quit"}
		if m.detail.Running() {
			hints = []string{"running…", "esc back", "q quit"}
		}
		return m.styles.Help.Render("  " + strings.Join(hints, " · "))
	}

	hints := []string{"↑/↓ move", "tab page", "enter detail", "r rescan"}
	if m.showMissing {
		hints = append(hints, "a hide missing")
	} else if missing := m.missingCount(); missing > 0 {
		hints = append(hints, fmt.Sprintf("a show %d missing", missing))
	}
	hints = append(hints, "q quit")

	return m.styles.Help.Render("  " + strings.Join(hints, " · "))
}
