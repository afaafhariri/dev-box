package pages

import (
	"sort"
	"strings"

	"devenv/internal/action"
	"devenv/internal/detector"
	"devenv/internal/tui/theme"
)

// maxOutputLines is how much action output the pane keeps. Older lines are
// dropped: the tail is what matters when something goes wrong.
const maxOutputLines = 200

// Detail shows everything known about one item, the actions available for it,
// and the output of the action currently running.
type Detail struct {
	item    detector.Item
	actions []action.Action
	cursor  int
	output  []string
	running bool
	height  int
	styles  theme.Styles
}

// NewDetail returns an empty detail page.
func NewDetail(styles theme.Styles) *Detail {
	return &Detail{styles: styles}
}

// SetItem opens an item, with the actions applicable to it. Output from a
// previous item is cleared.
func (d *Detail) SetItem(item detector.Item, actions []action.Action) {
	d.item = item
	d.actions = actions
	d.cursor = 0
	d.output = nil
	d.running = false
}

// Update refreshes the open item in place, so a rescan behind the detail view
// is reflected without closing it.
func (d *Detail) Update(item detector.Item, actions []action.Action) {
	if item.Name != d.item.Name {
		return
	}
	d.item = item
	// Actions are only re-listed while nothing is running, so the list cannot
	// shift under a selection mid-run.
	if !d.running {
		d.actions = actions
		if d.cursor >= len(d.actions) {
			d.cursor = 0
		}
	}
}

// SetHeight tells the page how many lines it may draw.
func (d *Detail) SetHeight(height int) { d.height = height }

// Item is the item on show.
func (d *Detail) Item() detector.Item { return d.item }

// Move shifts the action selection.
func (d *Detail) Move(delta int) {
	if len(d.actions) == 0 {
		return
	}
	d.cursor += delta
	if d.cursor >= len(d.actions) {
		d.cursor = len(d.actions) - 1
	}
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// Selected returns the action under the cursor.
func (d *Detail) Selected() (action.Action, bool) {
	if d.cursor < 0 || d.cursor >= len(d.actions) {
		return nil, false
	}
	return d.actions[d.cursor], true
}

// Running reports whether an action is in flight.
func (d *Detail) Running() bool { return d.running }

// StartRun clears the pane and marks an action as running.
func (d *Detail) StartRun() {
	d.output = nil
	d.running = true
}

// FinishRun marks the running action as done.
func (d *Detail) FinishRun() { d.running = false }

// AppendOutput adds one line to the output pane.
func (d *Detail) AppendOutput(line string) {
	d.output = append(d.output, line)
	if len(d.output) > maxOutputLines {
		d.output = d.output[len(d.output)-maxOutputLines:]
	}
}

// Output is the captured action output.
func (d *Detail) Output() []string { return d.output }

// View renders the page.
func (d *Detail) View() string {
	var b strings.Builder

	b.WriteString(d.styles.NameSel.Render("  " + d.item.Name))
	b.WriteString("  ")
	b.WriteString(d.styles.StatusFor(string(d.item.Status)).Render(d.item.Status.Label()))
	b.WriteString("\n\n")

	for _, f := range d.fields() {
		b.WriteString("  ")
		b.WriteString(d.styles.FieldKey(pad(f.key, 12)))
		b.WriteString(d.styles.FieldValue(f.value))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(d.actionBar())

	if len(d.output) > 0 {
		b.WriteString("\n\n")
		b.WriteString(d.outputPane())
	}

	return b.String()
}

// field is one label/value row.
type field struct{ key, value string }

// fields is everything known about the item, with the fixed columns first and
// the tool-specific metadata after them in a stable order.
func (d *Detail) fields() []field {
	fields := []field{
		{"category", d.item.Category.Title()},
	}
	if d.item.Version != "" {
		fields = append(fields, field{"version", d.item.Version})
	}
	if d.item.Path != "" {
		fields = append(fields, field{"path", d.item.Path})
	}
	if !d.item.DetectedAt.IsZero() {
		fields = append(fields, field{"detected", d.item.DetectedAt.Format("15:04:05")})
	}

	keys := make([]string, 0, len(d.item.Meta))
	for k := range d.item.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fields = append(fields, field{k, d.item.Meta[k]})
	}
	return fields
}

// actionBar renders the available actions, or why there are none.
func (d *Detail) actionBar() string {
	if len(d.actions) == 0 {
		return d.styles.Meta.Render("  no actions available for this item")
	}

	parts := make([]string, 0, len(d.actions))
	for i, a := range d.actions {
		label := a.Label()
		if i == d.cursor {
			label = "▸ " + label
		} else {
			label = "  " + label
		}
		parts = append(parts, d.styles.Action(label, i == d.cursor))
	}

	bar := "  " + strings.Join(parts, "  ")
	if d.running {
		return bar + d.styles.Meta.Render("   running…")
	}
	return bar
}

// outputPane renders the tail of the action output that fits.
func (d *Detail) outputPane() string {
	lines := d.output

	// The pane gets whatever is left after the fields and action bar; showing
	// the tail matters more than showing the start.
	if budget := d.height - len(d.fields()) - 6; budget > 0 && len(lines) > budget {
		lines = lines[len(lines)-budget:]
	}

	var b strings.Builder
	b.WriteString(d.styles.Panel("  OUTPUT"))
	b.WriteString("\n")
	for _, line := range lines {
		b.WriteString("  ")
		b.WriteString(d.styles.OutputLine(line))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
