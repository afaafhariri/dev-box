// Package export writes a scan out in the formats people share: JSON for
// machines, Markdown for a README or an issue report.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"devenv/internal/detector"
)

// Snapshot is a scan, ready to be written.
type Snapshot struct {
	ScannedAt time.Time       `json:"scannedAt"`
	Elapsed   string          `json:"elapsed"`
	Host      string          `json:"host,omitempty"`
	OS        string          `json:"os,omitempty"`
	Items     []detector.Item `json:"items"`
}

// JSON writes the snapshot as indented JSON.
func JSON(w io.Writer, snap Snapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snap); err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	return nil
}

// Markdown writes the snapshot as a document: a summary line, then one table
// per category listing what was found.
//
// Absent items are listed at the end rather than in the tables. Someone
// pasting this into an issue wants what they have first, and what they lack
// as a footnote.
func Markdown(w io.Writer, snap Snapshot) error {
	var b strings.Builder

	b.WriteString("# Development environment\n\n")

	found, missing := partition(snap.Items)

	b.WriteString(fmt.Sprintf("%d of %d detected", len(found), len(snap.Items)))
	if !snap.ScannedAt.IsZero() {
		b.WriteString(fmt.Sprintf(" · scanned %s", snap.ScannedAt.Format("2006-01-02 15:04")))
	}
	if snap.OS != "" {
		b.WriteString(fmt.Sprintf(" · %s", snap.OS))
	}
	b.WriteString("\n")

	for _, cat := range detector.Categories {
		items := byCategory(found, cat)
		if len(items) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("\n## %s\n\n", cat.Title()))
		b.WriteString("| Name | Version | Status | Managed by | Path |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")

		for _, item := range items {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				escape(item.Name),
				escape(orDash(item.Version)),
				escape(item.Status.Label()),
				escape(orDash(managedBy(item))),
				code(item.Path),
			))
		}
	}

	if len(missing) > 0 {
		b.WriteString("\n## Not installed\n\n")
		names := make([]string, 0, len(missing))
		for _, item := range missing {
			names = append(names, item.Name)
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n")
	}

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}
	return nil
}

func partition(items []detector.Item) (found, missing []detector.Item) {
	for _, item := range items {
		if item.Status.Found() {
			found = append(found, item)
		} else {
			missing = append(missing, item)
		}
	}
	return found, missing
}

func byCategory(items []detector.Item, cat detector.Category) []detector.Item {
	var out []detector.Item
	for _, item := range items {
		if item.Category == cat {
			out = append(out, item)
		}
	}
	return out
}

func managedBy(item detector.Item) string {
	if item.ManagedBy == "" {
		return ""
	}
	if item.PackageID == "" {
		return item.ManagedBy
	}
	return fmt.Sprintf("%s (%s)", item.ManagedBy, item.PackageID)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func code(s string) string {
	if s == "" {
		return "—"
	}
	return "`" + escape(s) + "`"
}

// escape protects the table structure from content containing pipes.
func escape(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}
