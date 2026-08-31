package detector

import "testing"

func TestStatusFound(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{StatusInstalled, true},
		{StatusRunning, true},
		{StatusStopped, true}, // installed but not running is still present
		{StatusNotFound, false},
		{StatusUnknown, false},
	}

	for _, tt := range tests {
		if got := tt.status.Found(); got != tt.want {
			t.Errorf("Status(%q).Found() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestWithMetaDropsEmptyValues(t *testing.T) {
	// Detectors pass optional lookups through unconditionally, so an empty
	// value must not create a blank row in the detail column.
	item := Item{Name: "Go"}.WithMeta("platform", "")
	if len(item.Meta) != 0 {
		t.Errorf("Meta = %v, want empty", item.Meta)
	}
}

func TestWithMetaDoesNotMutateTheOriginal(t *testing.T) {
	original := Item{Name: "Go"}.WithMeta("platform", "darwin/arm64")
	derived := original.WithMeta("goroot", "/usr/local/go")

	if _, ok := original.Meta["goroot"]; ok {
		t.Error("WithMeta mutated the receiver's map")
	}
	if derived.Meta["platform"] != "darwin/arm64" {
		t.Error("WithMeta lost an existing key")
	}
	if derived.Meta["goroot"] != "/usr/local/go" {
		t.Error("WithMeta did not set the new key")
	}
}

func TestCategoryTitle(t *testing.T) {
	if got, want := CategoryAI.Title(), "AI / ML"; got != want {
		t.Errorf("CategoryAI.Title() = %q, want %q", got, want)
	}
	if got, want := Category("custom").Title(), "custom"; got != want {
		t.Errorf("unknown category title = %q, want %q", got, want)
	}
}

func TestCategoriesCoversEveryDefinedCategory(t *testing.T) {
	// The TUI iterates Categories to build its list; a category missing here
	// would be silently invisible.
	defined := []Category{
		CategoryLanguage, CategoryServer, CategoryAI, CategoryTool, CategoryManager,
	}

	for _, cat := range defined {
		found := false
		for _, listed := range Categories {
			if listed == cat {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("category %q is not in Categories, so it would never render", cat)
		}
	}
}
