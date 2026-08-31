// Package detector defines the contract every probe of the local machine
// implements, along with the Item shape those probes produce. It is the one
// package every other subsystem depends on, so it deliberately holds no
// behaviour beyond the types themselves.
package detector

import (
	"context"
	"errors"
	"time"
)

// ErrNotInstalled is the sentinel a detector returns when the thing it looks
// for simply is not on this machine. It is an expected outcome, not a failure:
// the engine turns it into a StatusNotFound item without logging noise.
var ErrNotInstalled = errors.New("not installed")

// Category groups items in the UI.
type Category string

const (
	CategoryLanguage Category = "language"
	CategoryServer   Category = "server"
	CategoryAI       Category = "ai"
	CategoryTool     Category = "tool"
	CategoryManager  Category = "manager"
)

// Categories is the display order used by the TUI.
var Categories = []Category{
	CategoryLanguage,
	CategoryServer,
	CategoryAI,
	CategoryTool,
	CategoryManager,
}

// Title is the human-readable heading for a category.
func (c Category) Title() string {
	switch c {
	case CategoryLanguage:
		return "Languages"
	case CategoryServer:
		return "Servers"
	case CategoryAI:
		return "AI / ML"
	case CategoryTool:
		return "Tools"
	case CategoryManager:
		return "Version Managers"
	default:
		return string(c)
	}
}

// Status is what we know about an item right now.
type Status string

const (
	// StatusUnknown is the zero value: not yet scanned.
	StatusUnknown   Status = ""
	StatusInstalled Status = "installed"
	StatusNotFound  Status = "not_found"
	StatusRunning   Status = "running"
	StatusStopped   Status = "stopped"
)

// Found reports whether the item exists on the machine in any form.
func (s Status) Found() bool {
	return s == StatusInstalled || s == StatusRunning || s == StatusStopped
}

// Label is the short string shown in the item list.
func (s Status) Label() string {
	switch s {
	case StatusInstalled:
		return "installed"
	case StatusNotFound:
		return "not found"
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	default:
		return "scanning"
	}
}

// Item is the shared data shape: one detected (or missing) piece of the
// development environment.
type Item struct {
	Name        string            `json:"name"`
	Category    Category          `json:"category"`
	Version     string            `json:"version,omitempty"`
	Path        string            `json:"path,omitempty"`
	Status      Status            `json:"status"`
	Meta        map[string]string `json:"meta,omitempty"` // tool-specific extras (model list, port, etc.)
	UpdateAvail bool              `json:"updateAvail"`    // reserved for Phase 3
	DetectedAt  time.Time         `json:"detectedAt"`
	Actions     []string          `json:"actions,omitempty"` // reserved for Phase 2
}

// WithMeta returns a copy of the item with one meta key set. Empty values are
// dropped so detectors can pass through optional lookups unconditionally.
func (i Item) WithMeta(key, value string) Item {
	if value == "" {
		return i
	}
	meta := make(map[string]string, len(i.Meta)+1)
	for k, v := range i.Meta {
		meta[k] = v
	}
	meta[key] = value
	i.Meta = meta
	return i
}

// Detector probes the machine for exactly one thing.
//
// Detect returns ErrNotInstalled when the thing is absent. Any other error
// means the probe itself failed (a command hung, output was unparseable) and
// is surfaced to the user as such.
type Detector interface {
	Name() string
	Category() Category
	Detect(ctx context.Context) (Item, error)
}

// Sink receives detection results. The store implements it; declaring it here
// keeps the dependency pointing one way (store imports detector, never the
// reverse).
type Sink interface {
	Set(Item)
}
