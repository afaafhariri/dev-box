// Package search provides the fuzzy matching behind the TUI's filter.
//
// The corpus here is tiny — a few dozen items — so this favours predictable,
// explainable ranking over the cleverness a large index would need.
package search

import (
	"sort"
	"strings"

	"devenv/internal/detector"
)

// Match is one item that matched a query, with the score that ordered it.
type Match struct {
	Item  detector.Item
	Score int
}

// Score bands. An exact name match must always outrank a coincidental
// substring hit somewhere in an item's metadata.
const (
	scoreExact      = 1000
	scorePrefix     = 800
	scoreSubstring  = 600
	scoreFuzzy      = 400
	scoreOtherField = 200
)

// Filter returns the items matching a query, best first. An empty query
// returns everything, unchanged.
func Filter(items []detector.Item, query string) []detector.Item {
	query = strings.TrimSpace(query)
	if query == "" {
		return items
	}

	matches := make([]Match, 0, len(items))
	for _, item := range items {
		if score := Score(item, query); score > 0 {
			matches = append(matches, Match{Item: item, Score: score})
		}
	}

	sort.SliceStable(matches, func(a, b int) bool {
		if matches[a].Score != matches[b].Score {
			return matches[a].Score > matches[b].Score
		}
		// Ties break by name, so the order never wobbles between renders.
		return strings.ToLower(matches[a].Item.Name) < strings.ToLower(matches[b].Item.Name)
	})

	out := make([]detector.Item, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Item)
	}
	return out
}

// Score rates one item against a query. Zero means no match.
func Score(item detector.Item, query string) int {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0
	}

	name := strings.ToLower(item.Name)

	switch {
	case name == query:
		return scoreExact
	case strings.HasPrefix(name, query):
		// Longer names are a slightly weaker prefix match than short ones, so
		// "go" ranks Go above Gradle.
		return scorePrefix - len(name)
	case strings.Contains(name, query):
		return scoreSubstring - len(name)
	}

	// Subsequence matching: "psql" finds PostgreSQL, "nde" finds Node.js.
	if subsequence(name, query) {
		return scoreFuzzy - len(name)
	}

	// Finally the other fields people reasonably search by: the category,
	// who manages it, and the path.
	for _, field := range []string{
		string(item.Category), item.ManagedBy, item.PackageID, item.Path, item.Version,
	} {
		if field != "" && strings.Contains(strings.ToLower(field), query) {
			return scoreOtherField
		}
	}
	return 0
}

// subsequence reports whether every rune of query appears in s in order.
func subsequence(s, query string) bool {
	if query == "" {
		return true
	}

	remaining := []rune(query)
	for _, r := range s {
		if r == remaining[0] {
			remaining = remaining[1:]
			if len(remaining) == 0 {
				return true
			}
		}
	}
	return false
}
