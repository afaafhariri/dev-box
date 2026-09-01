// Package pages holds the TUI's sub-models. Each page owns its own state and
// rendering; the root model routes messages to whichever page is active.
package pages

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"devenv/internal/detector"
	"devenv/internal/tui/theme"
)

// Column widths for item rows. Names and versions are padded so the status
// column lines up down the whole list.
const (
	colName    = 16
	colVersion = 12
	colStatus  = 11
)

// List shows items, grouped by category. One List backs every tab: the
// dashboard passes no categories and so shows everything, while each other tab
// passes the one category it covers.
type List struct {
	// Title names the page.
	Title string
	// Categories filters what is shown. Empty means every category.
	Categories []detector.Category
	// ShowMissing includes items that were not found on this machine.
	ShowMissing bool

	items  []detector.Item
	cursor int
	offset int
	height int
	styles theme.Styles
}

// NewList returns a list page. Passing no categories makes it a dashboard.
func NewList(title string, styles theme.Styles, categories ...detector.Category) *List {
	return &List{Title: title, Categories: categories, styles: styles}
}

// SetHeight tells the list how many lines it may draw.
func (l *List) SetHeight(height int) {
	l.height = height
	l.syncOffset()
}

// SetItems replaces the contents, filtering and ordering them, and keeps the
// cursor on the same item where it still exists.
func (l *List) SetItems(items []detector.Item) {
	var selected string
	if item, ok := l.Selected(); ok {
		selected = item.Name
	}

	l.items = l.items[:0]
	for _, cat := range detector.Categories {
		if !l.covers(cat) {
			continue
		}
		for _, item := range items {
			if item.Category != cat {
				continue
			}
			if !l.ShowMissing && !item.Status.Found() {
				continue
			}
			l.items = append(l.items, item)
		}
	}

	l.cursor = 0
	for i, item := range l.items {
		if item.Name == selected {
			l.cursor = i
			break
		}
	}
	l.clamp()
}

// covers reports whether this page shows a category.
func (l *List) covers(cat detector.Category) bool {
	if len(l.Categories) == 0 {
		return true
	}
	for _, c := range l.Categories {
		if c == cat {
			return true
		}
	}
	return false
}

// Move shifts the cursor by delta, clamped to the list.
func (l *List) Move(delta int) {
	l.cursor += delta
	l.clamp()
}

// MoveTo puts the cursor at an absolute position, clamped to the list.
func (l *List) MoveTo(index int) {
	l.cursor = index
	l.clamp()
}

// Selected returns the item under the cursor.
func (l *List) Selected() (detector.Item, bool) {
	if l.cursor < 0 || l.cursor >= len(l.items) {
		return detector.Item{}, false
	}
	return l.items[l.cursor], true
}

// Len is the number of items shown.
func (l *List) Len() int { return len(l.items) }

func (l *List) clamp() {
	if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
	l.syncOffset()
}

// cursorLine is the rendered line the cursor sits on, counting the category
// headings drawn above it.
func (l *List) cursorLine() int {
	line := 0
	var lastCat detector.Category
	for i, item := range l.items {
		if item.Category != lastCat {
			lastCat = item.Category
			line++
		}
		if i == l.cursor {
			return line
		}
		line++
	}
	return 0
}

// totalLines is how many lines the list renders to, headings included.
func (l *List) totalLines() int {
	total := len(l.items)
	var lastCat detector.Category
	for _, item := range l.items {
		if item.Category != lastCat {
			lastCat = item.Category
			total++
		}
	}
	return total
}

// syncOffset scrolls the minimum amount needed to keep the cursor on screen,
// so View stays a pure function of the model.
func (l *List) syncOffset() {
	total := l.totalLines()
	if l.height <= 0 || l.height >= total {
		l.offset = 0
		return
	}

	line := l.cursorLine()
	if line < l.offset {
		l.offset = line
	}
	if line >= l.offset+l.height {
		l.offset = line - l.height + 1
	}
	if max := total - l.height; l.offset > max {
		l.offset = max
	}
	if l.offset < 0 {
		l.offset = 0
	}
}

// View renders the list.
func (l *List) View() string {
	if len(l.items) == 0 {
		return l.styles.Meta.Render("  nothing to show here yet…")
	}

	var lines []string
	var lastCat detector.Category

	for i, item := range l.items {
		// The dashboard groups by category; a single-category page has no need
		// to repeat its own name.
		if item.Category != lastCat {
			lastCat = item.Category
			if len(l.Categories) != 1 {
				lines = append(lines, l.styles.Group.Render("  "+strings.ToUpper(item.Category.Title())))
			}
		}
		lines = append(lines, l.row(item, i == l.cursor))
	}

	return strings.Join(l.window(lines), "\n")
}

// window slices the rendered lines to the visible range.
func (l *List) window(lines []string) []string {
	if l.height <= 0 || l.height >= len(lines) {
		return lines
	}

	offset := l.offset
	if max := len(lines) - l.height; offset > max {
		offset = max
	}
	if offset < 0 {
		offset = 0
	}
	return lines[offset : offset+l.height]
}

// row renders one item.
func (l *List) row(item detector.Item, selected bool) string {
	cursor := "  "
	name := l.styles.Name.Render(pad(item.Name, colName))
	if selected {
		cursor = l.styles.Cursor.Render("▸ ")
		name = l.styles.NameSel.Render(pad(item.Name, colName))
	}

	version := item.Version
	if version == "" {
		version = "—"
	}
	// A bullet beside the version is enough to spot an update at a glance
	// without widening the column for every row that has nothing to say.
	if item.UpdateAvail {
		version += " •"
	}

	row := lipgloss.JoinHorizontal(lipgloss.Left,
		cursor,
		name,
		l.styles.Version.Render(pad(version, colVersion)),
		l.styles.StatusFor(string(item.Status)).Render(pad(item.Status.Label(), colStatus)),
		l.styles.Meta.Render(Summary(item)),
	)

	if selected {
		return l.styles.RowSel.Render(row)
	}
	return l.styles.Row.Render(row)
}

// Summary is the trailing free-text column: the most useful thing known about
// an item that is not its version.
func Summary(item detector.Item) string {
	if item.Status == detector.StatusNotFound {
		if reason := item.Meta["error"]; reason != "" && reason != "not installed" {
			return reason
		}
		return ""
	}

	var parts []string
	for _, key := range metaOrder(item.Category) {
		v := item.Meta[key]
		if v == "" {
			continue
		}
		if label, ok := metaLabels[key]; ok {
			// The value already reads as a phrase; a key in front of it would
			// only add noise.
			parts = append(parts, label+v)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", key, v))
	}
	if len(parts) > 0 {
		return strings.Join(parts, " · ")
	}
	return item.Path
}

// metaLabels overrides how a meta key is introduced. A key mapped to "" has
// its value rendered bare.
var metaLabels = map[string]string{"modelList": "", "packageList": ""}

// metaOrder picks which meta keys to surface per category, most interesting
// first. The detail view shows the rest.
func metaOrder(cat detector.Category) []string {
	switch cat {
	case detector.CategoryServer:
		return []string{"containers", "port", "images"}
	case detector.CategoryAI:
		return []string{"models", "modelList", "gpu"}
	case detector.CategoryLanguage:
		return []string{"npm", "pip", "toolchain", "platform"}
	case detector.CategoryTool:
		return []string{"packages", "packageList"}
	case detector.CategoryManager:
		return nil
	default:
		return nil
	}
}

// pad right-pads s to width, leaving over-long values intact rather than
// truncating something the user needs to read.
func pad(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}
