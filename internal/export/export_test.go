package export

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"devenv/internal/detector"
)

func snapshot() Snapshot {
	return Snapshot{
		ScannedAt: time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC),
		Elapsed:   "280ms",
		OS:        "darwin/arm64",
		Items: []detector.Item{
			{Name: "Go", Category: detector.CategoryLanguage, Version: "1.25.6",
				Status: detector.StatusInstalled, Path: "/usr/local/go/bin/go",
				ManagedBy: "manual install"},
			{Name: "PostgreSQL", Category: detector.CategoryServer, Version: "15.19",
				Status: detector.StatusRunning, Path: "/opt/homebrew/bin/psql",
				ManagedBy: "Homebrew", PackageID: "postgresql@15"},
			{Name: "Rust", Category: detector.CategoryLanguage, Status: detector.StatusNotFound},
		},
	}
}

func TestMarkdownStructure(t *testing.T) {
	var b strings.Builder
	if err := Markdown(&b, snapshot()); err != nil {
		t.Fatalf("Markdown() error = %v", err)
	}
	out := b.String()

	for _, want := range []string{
		"# Development environment",
		"2 of 3 detected",
		"2026-01-15 09:30",
		"darwin/arm64",
		"## Languages",
		"## Servers",
		"| Go | 1.25.6 | installed |",
		"Homebrew (postgresql@15)",
		"`/opt/homebrew/bin/psql`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Markdown() is missing %q\n%s", want, out)
		}
	}
}

func TestMarkdownListsAbsentItemsSeparately(t *testing.T) {
	var b strings.Builder
	if err := Markdown(&b, snapshot()); err != nil {
		t.Fatal(err)
	}
	out := b.String()

	// Someone pasting this into an issue wants what they have first.
	notInstalled := strings.Index(out, "## Not installed")
	languages := strings.Index(out, "## Languages")

	if notInstalled < 0 {
		t.Fatal("absent items were not listed")
	}
	if notInstalled < languages {
		t.Error("absent items were listed before what was found")
	}
	if !strings.Contains(out[notInstalled:], "Rust") {
		t.Error("Rust is missing from the not-installed list")
	}
	// And not in the tables above it.
	if strings.Contains(out[:notInstalled], "| Rust |") {
		t.Error("an absent item appeared in a table")
	}
}

func TestMarkdownEscapesPipes(t *testing.T) {
	// A pipe in content would otherwise break the table.
	snap := Snapshot{Items: []detector.Item{
		{Name: "Odd|Name", Category: detector.CategoryTool, Status: detector.StatusInstalled,
			Path: "/usr/bin/a|b"},
	}}

	var b strings.Builder
	if err := Markdown(&b, snap); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(b.String(), `Odd\|Name`) {
		t.Errorf("pipe was not escaped:\n%s", b.String())
	}
}

func TestMarkdownSkipsEmptyCategories(t *testing.T) {
	snap := Snapshot{Items: []detector.Item{
		{Name: "Go", Category: detector.CategoryLanguage, Status: detector.StatusInstalled},
	}}

	var b strings.Builder
	if err := Markdown(&b, snap); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(b.String(), "## Servers") {
		t.Error("an empty category got a heading")
	}
}

func TestMarkdownHandlesAnEmptyScan(t *testing.T) {
	var b strings.Builder
	if err := Markdown(&b, Snapshot{}); err != nil {
		t.Fatalf("Markdown() error = %v", err)
	}
	if !strings.Contains(b.String(), "0 of 0 detected") {
		t.Errorf("empty scan rendered as:\n%s", b.String())
	}
}

func TestJSONRoundTrip(t *testing.T) {
	var b strings.Builder
	if err := JSON(&b, snapshot()); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}

	var got Snapshot
	if err := json.Unmarshal([]byte(b.String()), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(got.Items) != 3 {
		t.Errorf("round trip produced %d items, want 3", len(got.Items))
	}
	if got.Items[1].PackageID != "postgresql@15" {
		t.Error("ownership did not survive the round trip")
	}
}
