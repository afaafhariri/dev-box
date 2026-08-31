// Package theme holds the colour tokens and Lip Gloss styles shared by the
// root model and its pages. It lives apart from both so pages can style
// themselves without importing the package that imports them.
package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette mirrors the architecture document's tokens. Adaptive colours keep
// the UI legible on light terminals without a second theme.
var (
	colAccent   = lipgloss.AdaptiveColor{Light: "#0B65C2", Dark: "#4A9EFF"}
	colGreen    = lipgloss.AdaptiveColor{Light: "#1A7F3C", Dark: "#38C96A"}
	colAmber    = lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#F0A83A"}
	colPurple   = lipgloss.AdaptiveColor{Light: "#5B3FD0", Dark: "#9B7EFF"}
	colText     = lipgloss.AdaptiveColor{Light: "#1C2733", Dark: "#C8D8E8"}
	colMuted    = lipgloss.AdaptiveColor{Light: "#5A6B7A", Dark: "#5A7A96"}
	colDim      = lipgloss.AdaptiveColor{Light: "#8494A2", Dark: "#3E5A76"}
	colSelected = lipgloss.AdaptiveColor{Light: "#DCE8F5", Dark: "#1B2639"}
)

// Styles is the full set of styles the UI draws with.
type Styles struct {
	Title     lipgloss.Style
	Badge     lipgloss.Style
	Subtitle  lipgloss.Style
	Group     lipgloss.Style
	Name      lipgloss.Style
	NameSel   lipgloss.Style
	Version   lipgloss.Style
	Meta      lipgloss.Style
	Path      lipgloss.Style
	Cursor    lipgloss.Style
	Row       lipgloss.Style
	RowSel    lipgloss.Style
	Help      lipgloss.Style
	StatusRun lipgloss.Style
	StatusOK  lipgloss.Style
	StatusOff lipgloss.Style
	StatusNo  lipgloss.Style
	Spinner   lipgloss.Style
}

// New returns the default styles.
func New() Styles {
	return Styles{
		Title:     lipgloss.NewStyle().Bold(true).Foreground(colText),
		Badge:     lipgloss.NewStyle().Foreground(colAccent),
		Subtitle:  lipgloss.NewStyle().Foreground(colMuted),
		Group:     lipgloss.NewStyle().Bold(true).Foreground(colDim).MarginTop(1),
		Name:      lipgloss.NewStyle().Foreground(colText),
		NameSel:   lipgloss.NewStyle().Bold(true).Foreground(colAccent),
		Version:   lipgloss.NewStyle().Foreground(colMuted),
		Meta:      lipgloss.NewStyle().Foreground(colDim),
		Path:      lipgloss.NewStyle().Foreground(colDim),
		Cursor:    lipgloss.NewStyle().Foreground(colAccent),
		Row:       lipgloss.NewStyle(),
		RowSel:    lipgloss.NewStyle().Background(colSelected),
		Help:      lipgloss.NewStyle().Foreground(colDim).MarginTop(1),
		StatusRun: lipgloss.NewStyle().Foreground(colGreen),
		StatusOK:  lipgloss.NewStyle().Foreground(colAccent),
		StatusOff: lipgloss.NewStyle().Foreground(colAmber),
		StatusNo:  lipgloss.NewStyle().Foreground(colDim),
		Spinner:   lipgloss.NewStyle().Foreground(colPurple),
	}
}

// Tab styles for the page bar.
var (
	tabActive   = lipgloss.NewStyle().Bold(true).Foreground(colAccent).Padding(0, 1)
	tabInactive = lipgloss.NewStyle().Foreground(colMuted).Padding(0, 1)
	tabBar      = lipgloss.NewStyle().Foreground(colDim)
)

// Tab renders one tab label.
func (s Styles) Tab(label string, active bool) string {
	if active {
		return tabActive.Render(label)
	}
	return tabInactive.Render(label)
}

// TabBar renders the separator beneath the tabs.
func (s Styles) TabBar(text string) string { return tabBar.Render(text) }

// StatusFor picks the style matching a status colour.
func (s Styles) StatusFor(status string) lipgloss.Style {
	switch status {
	case "running":
		return s.StatusRun
	case "installed":
		return s.StatusOK
	case "stopped":
		return s.StatusOff
	default:
		return s.StatusNo
	}
}

// Field styles for the detail view.
var (
	fieldKey   = lipgloss.NewStyle().Foreground(colDim)
	fieldValue = lipgloss.NewStyle().Foreground(colText)
	panel      = lipgloss.NewStyle().Foreground(colMuted)
	output     = lipgloss.NewStyle().Foreground(colMuted)
	outputCmd  = lipgloss.NewStyle().Foreground(colPurple)
	outputOK   = lipgloss.NewStyle().Foreground(colGreen)
	outputErr  = lipgloss.NewStyle().Foreground(colAmber)
	actionSel  = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	actionIdle = lipgloss.NewStyle().Foreground(colMuted)
	confirm    = lipgloss.NewStyle().Bold(true).Foreground(colAmber)
)

// FieldKey renders a detail-view label.
func (s Styles) FieldKey(text string) string { return fieldKey.Render(text) }

// FieldValue renders a detail-view value.
func (s Styles) FieldValue(text string) string { return fieldValue.Render(text) }

// Panel renders a section heading inside a page.
func (s Styles) Panel(text string) string { return panel.Render(text) }

// Action renders one action button.
func (s Styles) Action(text string, selected bool) string {
	if selected {
		return actionSel.Render(text)
	}
	return actionIdle.Render(text)
}

// Confirm renders the question asked before a destructive action runs. It is
// amber rather than accent-coloured so it does not read as just another hint.
func (s Styles) Confirm(text string) string { return confirm.Render(text) }

// OutputLine styles a line of action output by what it represents: the command
// that ran, a success, a failure, or ordinary output.
func (s Styles) OutputLine(line string) string {
	switch {
	case strings.HasPrefix(line, "$ "):
		return outputCmd.Render(line)
	case strings.HasPrefix(line, "✓"):
		return outputOK.Render(line)
	case strings.HasPrefix(line, "✗"):
		return outputErr.Render(line)
	default:
		return output.Render(line)
	}
}
