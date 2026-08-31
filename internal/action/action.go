// Package action defines the operations a user can trigger from the TUI, and
// runs them with their output streamed back for display.
//
// Every action is explicit: nothing here runs as part of a scan. The app reads
// the machine on its own, and changes it only when asked.
package action

import (
	"context"

	"devenv/internal/detector"
)

// Action is one operation a user can trigger against an item.
type Action interface {
	// Label is the text shown in the UI ("Start", "Stop", "Upgrade").
	Label() string
	// Applicable reports whether this action makes sense for an item right
	// now — Start only for a stopped service, and so on.
	Applicable(item detector.Item) bool
	// Run executes the action, returning a channel of output lines. The
	// channel is closed when the action finishes. Cancelling ctx stops it.
	Run(ctx context.Context, item detector.Item) <-chan string
}

// Registry holds the available actions.
type Registry struct {
	actions []Action
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds actions.
func (r *Registry) Register(actions ...Action) {
	for _, a := range actions {
		if a != nil {
			r.actions = append(r.actions, a)
		}
	}
}

// For returns the actions applicable to an item, in registration order.
func (r *Registry) For(item detector.Item) []Action {
	var out []Action
	for _, a := range r.actions {
		if a.Applicable(item) {
			out = append(out, a)
		}
	}
	return out
}

// Labels returns the labels of the actions applicable to an item. The TUI
// stores these on the item so the list can hint at what is possible.
func (r *Registry) Labels(item detector.Item) []string {
	actions := r.For(item)
	labels := make([]string, 0, len(actions))
	for _, a := range actions {
		labels = append(labels, a.Label())
	}
	return labels
}

// Len is the number of registered actions.
func (r *Registry) Len() int { return len(r.actions) }
