package tui

import "github.com/charmbracelet/lipgloss"

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

type styles struct {
	title     lipgloss.Style
	badge     lipgloss.Style
	subtitle  lipgloss.Style
	group     lipgloss.Style
	name      lipgloss.Style
	nameSel   lipgloss.Style
	version   lipgloss.Style
	meta      lipgloss.Style
	path      lipgloss.Style
	cursor    lipgloss.Style
	row       lipgloss.Style
	rowSel    lipgloss.Style
	help      lipgloss.Style
	statusRun lipgloss.Style
	statusOK  lipgloss.Style
	statusOff lipgloss.Style
	statusNo  lipgloss.Style
	spinner   lipgloss.Style
}

func newStyles() styles {
	return styles{
		title:     lipgloss.NewStyle().Bold(true).Foreground(colText),
		badge:     lipgloss.NewStyle().Foreground(colAccent),
		subtitle:  lipgloss.NewStyle().Foreground(colMuted),
		group:     lipgloss.NewStyle().Bold(true).Foreground(colDim).MarginTop(1),
		name:      lipgloss.NewStyle().Foreground(colText),
		nameSel:   lipgloss.NewStyle().Bold(true).Foreground(colAccent),
		version:   lipgloss.NewStyle().Foreground(colMuted),
		meta:      lipgloss.NewStyle().Foreground(colDim),
		path:      lipgloss.NewStyle().Foreground(colDim),
		cursor:    lipgloss.NewStyle().Foreground(colAccent),
		row:       lipgloss.NewStyle(),
		rowSel:    lipgloss.NewStyle().Background(colSelected),
		help:      lipgloss.NewStyle().Foreground(colDim).MarginTop(1),
		statusRun: lipgloss.NewStyle().Foreground(colGreen),
		statusOK:  lipgloss.NewStyle().Foreground(colAccent),
		statusOff: lipgloss.NewStyle().Foreground(colAmber),
		statusNo:  lipgloss.NewStyle().Foreground(colDim),
		spinner:   lipgloss.NewStyle().Foreground(colPurple),
	}
}
