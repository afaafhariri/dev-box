package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"devenv/internal/detector"
)

// Column widths for the item rows. Names and versions are padded so the
// status column lines up down the whole list.
const (
	colName    = 16
	colVersion = 12
	colStatus  = 11
	// chromeLines is the header plus help lines the list has to share the
	// terminal with.
	chromeLines = 5
)

// View renders the whole screen.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(m.header())
	b.WriteString("\n")

	lines := m.listLines()
	if len(lines) == 0 {
		b.WriteString(m.styles.meta.Render("  nothing detected yet…"))
		b.WriteString("\n")
	} else {
		for _, line := range m.window(lines) {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	b.WriteString(m.help())
	return b.String()
}

// header is the title bar plus scan state.
func (m Model) header() string {
	title := m.styles.title.Render("devenv")
	sub := m.styles.subtitle.Render("Dev Environment Manager")

	var state string
	switch {
	case m.scanning:
		state = fmt.Sprintf("%s scanning %d/%d", m.spin.View(), len(m.seen), m.scanner.Len())
	default:
		state = fmt.Sprintf("%d of %d found · %s in %s",
			m.store.Found(),
			m.store.Len(),
			m.lastScan.Format("15:04:05"),
			roundDuration(m.elapsed),
		)
	}

	return lipgloss.JoinHorizontal(lipgloss.Left,
		title, "  ", sub, "  ", m.styles.badge.Render(state),
	)
}

// listLines renders every visible item, with a heading each time the category
// changes. Line indices here match Model.cursorLine, which is what drives
// scrolling.
func (m Model) listLines() []string {
	var lines []string
	var lastCat detector.Category

	for i, item := range m.visible {
		if item.Category != lastCat {
			lastCat = item.Category
			lines = append(lines, m.styles.group.Render("  "+strings.ToUpper(item.Category.Title())))
		}
		lines = append(lines, m.itemLine(item, i == m.cursor))
	}
	return lines
}

// itemLine renders one item as a single row.
func (m Model) itemLine(item detector.Item, selected bool) string {
	cursor := "  "
	name := m.styles.name.Render(pad(item.Name, colName))
	if selected {
		cursor = m.styles.cursor.Render("▸ ")
		name = m.styles.nameSel.Render(pad(item.Name, colName))
	}

	version := item.Version
	if version == "" {
		version = "—"
	}

	row := lipgloss.JoinHorizontal(lipgloss.Left,
		cursor,
		name,
		m.styles.version.Render(pad(version, colVersion)),
		m.statusStyle(item.Status).Render(pad(item.Status.Label(), colStatus)),
		m.styles.meta.Render(m.detail(item)),
	)

	if selected {
		return m.styles.rowSel.Render(row)
	}
	return m.styles.row.Render(row)
}

// detail is the trailing free-text column: the most useful thing we know about
// this item that is not its version.
func (m Model) detail(item detector.Item) string {
	if item.Status == detector.StatusNotFound {
		if reason := item.Meta["error"]; reason != "" && reason != "not installed" {
			return reason
		}
		return ""
	}

	var parts []string
	for _, key := range metaOrder(item.Category) {
		if v := item.Meta[key]; v != "" {
			parts = append(parts, fmt.Sprintf("%s %s", key, v))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " · ")
	}
	return item.Path
}

// metaOrder picks which meta keys to surface per category, most interesting
// first. Phase 2's detail view shows the rest.
func metaOrder(cat detector.Category) []string {
	switch cat {
	case detector.CategoryServer:
		return []string{"containers", "port", "images"}
	case detector.CategoryAI:
		return []string{"models", "modelList", "port"}
	case detector.CategoryLanguage:
		return []string{"npm", "pip", "platform"}
	default:
		return nil
	}
}

func (m Model) statusStyle(s detector.Status) lipgloss.Style {
	switch s {
	case detector.StatusRunning:
		return m.styles.statusRun
	case detector.StatusInstalled:
		return m.styles.statusOK
	case detector.StatusStopped:
		return m.styles.statusOff
	default:
		return m.styles.statusNo
	}
}

// window scrolls the rendered lines so the cursor stays visible.
func (m Model) window(lines []string) []string {
	height := m.height - chromeLines
	if m.height == 0 || height >= len(lines) {
		return lines
	}
	if height < 1 {
		height = 1
	}

	// Update keeps m.offset in sync via syncOffset; clamp defensively so a
	// stale offset can never slice out of range.
	offset := m.offset
	if max := len(lines) - height; offset > max {
		offset = max
	}
	if offset < 0 {
		offset = 0
	}

	return lines[offset : offset+height]
}

// help is the key hint line.
func (m Model) help() string {
	hints := []string{"↑/↓ move", "r rescan", "q quit"}

	if missing := m.missingCount(); m.showMissing {
		hints = append(hints[:len(hints)-1], "a hide missing", "q quit")
	} else if missing > 0 {
		hints = append(hints[:len(hints)-1],
			fmt.Sprintf("a show %d missing", missing), "q quit")
	}

	return m.styles.help.Render("  " + strings.Join(hints, " · "))
}

// pad right-pads s to width, leaving over-long values intact rather than
// truncating something the user needs to read.
func pad(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}
