package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the key set for both the list pages and the detail view.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Top     key.Binding
	Bottom  key.Binding
	NextTab key.Binding
	PrevTab key.Binding
	Jump    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Rescan  key.Binding
	Hidden  key.Binding
	Quit    key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "previous action")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next action")),
		Top:     key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
		NextTab: key.NewBinding(key.WithKeys("tab", "]"), key.WithHelp("tab", "next page")),
		PrevTab: key.NewBinding(key.WithKeys("shift+tab", "["), key.WithHelp("shift+tab", "previous page")),
		Jump:    key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6"), key.WithHelp("1-6", "jump to page")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Rescan:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rescan")),
		Hidden:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "toggle missing")),
		// Esc is the detail view's back key, so it cannot also quit.
		Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}
