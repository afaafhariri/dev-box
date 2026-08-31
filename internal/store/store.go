// Package store holds the single source of truth for detected state. The
// detection engine writes to it, the TUI reads from it, and subscribers are
// notified as items change.
package store

import (
	"sort"
	"strings"
	"sync"

	"devenv/internal/detector"
)

// subBuffer is the per-subscriber queue depth. It is sized well above the
// number of detectors so a full scan never fills it in practice.
const subBuffer = 128

// Store is a thread-safe map of items keyed by Item.Name.
type Store struct {
	mu     sync.RWMutex
	items  map[string]detector.Item
	subs   []chan detector.Item
	closed bool

	// dropped counts notifications shed because a subscriber was not
	// draining. Exposed via Dropped for diagnostics.
	dropped int
}

// New returns an empty store.
func New() *Store {
	return &Store{items: make(map[string]detector.Item)}
}

// Set writes an item and notifies subscribers.
//
// The send is non-blocking: a subscriber that has stopped draining is skipped
// rather than deadlocking the detection engine. Consumers therefore treat the
// channel as a "something changed" hint and re-read the store, rather than
// assuming they see every individual item.
func (s *Store) Set(item detector.Item) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.items[item.Name] = item
	// Snapshot the subscriber list so the sends below happen outside the lock.
	subs := make([]chan detector.Item, len(s.subs))
	copy(subs, s.subs)
	s.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- item:
		default:
			s.mu.Lock()
			s.dropped++
			s.mu.Unlock()
		}
	}
}

// Get returns one item by name.
func (s *Store) Get(name string) (detector.Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[name]
	return item, ok
}

// All returns every item, sorted by name.
func (s *Store) All() []detector.Item {
	s.mu.RLock()
	out := make([]detector.Item, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	s.mu.RUnlock()

	sortItems(out)
	return out
}

// ByCategory returns the items in one category, sorted by name.
func (s *Store) ByCategory(cat detector.Category) []detector.Item {
	s.mu.RLock()
	var out []detector.Item
	for _, item := range s.items {
		if item.Category == cat {
			out = append(out, item)
		}
	}
	s.mu.RUnlock()

	sortItems(out)
	return out
}

// Len is the number of items held.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items)
}

// Found is the number of items actually present on the machine.
func (s *Store) Found() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := 0
	for _, item := range s.items {
		if item.Status.Found() {
			n++
		}
	}
	return n
}

// Dropped is the number of notifications shed because a subscriber was slow.
func (s *Store) Dropped() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.dropped
}

// Subscribe returns a channel that receives every item written after the call.
//
// Subscribe once and hold the channel: each call registers a new subscriber,
// so calling it per received message would leak one channel per update.
func (s *Store) Subscribe() <-chan detector.Item {
	ch := make(chan detector.Item, subBuffer)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		close(ch)
		return ch
	}
	s.subs = append(s.subs, ch)
	return ch
}

// Close closes every subscription channel and makes further Sets no-ops. It is
// idempotent, and is called once on shutdown so listening goroutines unblock.
func (s *Store) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	for _, ch := range s.subs {
		close(ch)
	}
	s.subs = nil
}

// sortItems orders items by name, case-insensitively, so the list does not
// reshuffle as results stream in.
func sortItems(items []detector.Item) {
	sort.Slice(items, func(a, b int) bool {
		return strings.ToLower(items[a].Name) < strings.ToLower(items[b].Name)
	})
}
