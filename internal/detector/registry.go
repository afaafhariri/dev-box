package detector

import "sort"

// Registry holds the detectors an engine will run. Registration order is
// preserved; the TUI sorts for display, so the registry does not.
type Registry struct {
	detectors []Detector
	seen      map[string]bool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{seen: make(map[string]bool)}
}

// Register adds detectors, skipping any whose Name duplicates one already
// registered. Names are the store's primary key, so duplicates would silently
// overwrite each other.
func (r *Registry) Register(detectors ...Detector) {
	for _, d := range detectors {
		if d == nil || r.seen[d.Name()] {
			continue
		}
		r.seen[d.Name()] = true
		r.detectors = append(r.detectors, d)
	}
}

// All returns a copy of the registered detectors.
func (r *Registry) All() []Detector {
	out := make([]Detector, len(r.detectors))
	copy(out, r.detectors)
	return out
}

// ByCategory returns the registered detectors in one category.
func (r *Registry) ByCategory(cat Category) []Detector {
	var out []Detector
	for _, d := range r.detectors {
		if d.Category() == cat {
			out = append(out, d)
		}
	}
	return out
}

// Names returns the registered detector names, sorted.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.detectors))
	for _, d := range r.detectors {
		out = append(out, d.Name())
	}
	sort.Strings(out)
	return out
}

// Len is the number of registered detectors.
func (r *Registry) Len() int { return len(r.detectors) }
