package search

import (
	"testing"

	"devenv/internal/detector"
)

func item(name string, cat detector.Category) detector.Item {
	return detector.Item{Name: name, Category: cat, Status: detector.StatusInstalled}
}

func corpus() []detector.Item {
	return []detector.Item{
		item("Go", detector.CategoryLanguage),
		item("Gradle", detector.CategoryTool),
		item("PostgreSQL", detector.CategoryServer),
		item("Node.js", detector.CategoryLanguage),
		item("MongoDB", detector.CategoryServer),
		item("Docker", detector.CategoryServer),
	}
}

func names(items []detector.Item) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}
	return out
}

func TestEmptyQueryReturnsEverything(t *testing.T) {
	if got := Filter(corpus(), "  "); len(got) != len(corpus()) {
		t.Errorf("Filter() returned %d items, want all %d", len(got), len(corpus()))
	}
}

func TestExactNameRanksFirst(t *testing.T) {
	// "go" must find Go before Gradle and MongoDB, both of which contain it.
	got := names(Filter(corpus(), "go"))

	if len(got) == 0 || got[0] != "Go" {
		t.Errorf("Filter(go) = %v, want Go first", got)
	}
}

func TestPrefixBeatsSubstring(t *testing.T) {
	got := names(Filter(corpus(), "mo"))

	if len(got) == 0 || got[0] != "MongoDB" {
		t.Errorf("Filter(mo) = %v, want MongoDB first", got)
	}
}

func TestSubsequenceMatching(t *testing.T) {
	// The point of fuzzy matching: the letters are there, in order.
	got := names(Filter(corpus(), "psql"))

	found := false
	for _, n := range got {
		if n == "PostgreSQL" {
			found = true
		}
	}
	if !found {
		t.Errorf("Filter(psql) = %v, want PostgreSQL among them", got)
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	if got := names(Filter(corpus(), "DOCKER")); len(got) == 0 || got[0] != "Docker" {
		t.Errorf("Filter(DOCKER) = %v, want Docker", got)
	}
}

func TestSearchByCategory(t *testing.T) {
	got := names(Filter(corpus(), "server"))

	if len(got) != 3 {
		t.Errorf("Filter(server) = %v, want the three servers", got)
	}
}

func TestSearchByManager(t *testing.T) {
	items := []detector.Item{
		{Name: "Node.js", Status: detector.StatusInstalled, ManagedBy: "nvm", PackageID: "22.13.1"},
		{Name: "Git", Status: detector.StatusInstalled, ManagedBy: "Homebrew", PackageID: "git"},
	}

	got := names(Filter(items, "nvm"))
	if len(got) != 1 || got[0] != "Node.js" {
		t.Errorf("Filter(nvm) = %v, want just Node.js", got)
	}
}

func TestNoMatchReturnsNothing(t *testing.T) {
	if got := Filter(corpus(), "zzzz"); len(got) != 0 {
		t.Errorf("Filter(zzzz) = %v, want nothing", names(got))
	}
}

func TestOrderIsStableForEqualScores(t *testing.T) {
	// Ties break by name, so the list never wobbles between renders.
	items := []detector.Item{
		item("beta", detector.CategoryServer),
		item("alpha", detector.CategoryServer),
	}

	first := names(Filter(items, "server"))
	for i := 0; i < 5; i++ {
		if got := names(Filter(items, "server")); got[0] != first[0] || got[1] != first[1] {
			t.Fatalf("order changed between runs: %v then %v", first, got)
		}
	}
	if first[0] != "alpha" {
		t.Errorf("tie broken as %v, want alphabetical", first)
	}
}

func TestSubsequence(t *testing.T) {
	tests := []struct {
		s, query string
		want     bool
	}{
		{"postgresql", "psql", true},
		{"node.js", "nde", true},
		{"go", "og", false},
		{"go", "", true},
		{"", "x", false},
	}

	for _, tt := range tests {
		if got := subsequence(tt.s, tt.query); got != tt.want {
			t.Errorf("subsequence(%q, %q) = %v, want %v", tt.s, tt.query, got, tt.want)
		}
	}
}
