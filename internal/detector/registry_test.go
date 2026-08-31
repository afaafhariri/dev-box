package detector

import "testing"

func TestRegistryPreservesOrderAndSkipsDuplicates(t *testing.T) {
	r := NewRegistry()
	r.Register(
		&fakeDetector{name: "Go", cat: CategoryLanguage},
		&fakeDetector{name: "Git", cat: CategoryTool},
	)
	// Names are the store's primary key, so a second "Go" must not be
	// registered — it would silently overwrite the first one's results.
	r.Register(&fakeDetector{name: "Go", cat: CategoryLanguage})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d detectors, want 2", len(all))
	}
	if all[0].Name() != "Go" || all[1].Name() != "Git" {
		t.Errorf("registration order not preserved: %q, %q", all[0].Name(), all[1].Name())
	}
}

func TestRegistryIgnoresNil(t *testing.T) {
	r := NewRegistry()
	r.Register(nil)
	r.Register(&fakeDetector{name: "Go", cat: CategoryLanguage})

	if got, want := r.Len(), 1; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
}

func TestRegistryAllReturnsACopy(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeDetector{name: "Go", cat: CategoryLanguage})

	all := r.All()
	all[0] = &fakeDetector{name: "Tampered", cat: CategoryTool}

	if r.All()[0].Name() != "Go" {
		t.Error("mutating the slice from All() changed the registry")
	}
}

func TestRegistryByCategory(t *testing.T) {
	r := NewRegistry()
	r.Register(
		&fakeDetector{name: "Go", cat: CategoryLanguage},
		&fakeDetector{name: "Python", cat: CategoryLanguage},
		&fakeDetector{name: "Redis", cat: CategoryServer},
	)

	if got, want := len(r.ByCategory(CategoryLanguage)), 2; got != want {
		t.Errorf("ByCategory(language) returned %d, want %d", got, want)
	}
	if got := r.ByCategory(CategoryAI); got != nil {
		t.Errorf("ByCategory(ai) = %v, want nil", got)
	}
}

func TestRegistryNamesAreSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(
		&fakeDetector{name: "Redis", cat: CategoryServer},
		&fakeDetector{name: "Docker", cat: CategoryServer},
		&fakeDetector{name: "Go", cat: CategoryLanguage},
	)

	want := []string{"Docker", "Go", "Redis"}
	got := r.Names()

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Names()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
