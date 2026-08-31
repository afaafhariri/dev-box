package store

import (
	"sync"
	"testing"
	"time"

	"devenv/internal/detector"
)

func item(name string, cat detector.Category, status detector.Status) detector.Item {
	return detector.Item{Name: name, Category: cat, Status: status}
}

func TestSetAndGet(t *testing.T) {
	s := New()
	s.Set(item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	got, ok := s.Get("Go")
	if !ok {
		t.Fatal("Get(Go) not found after Set")
	}
	if got.Status != detector.StatusInstalled {
		t.Errorf("Status = %q, want installed", got.Status)
	}

	if _, ok := s.Get("Rust"); ok {
		t.Error("Get(Rust) found an item that was never set")
	}
}

func TestSetOverwritesByName(t *testing.T) {
	s := New()
	s.Set(item("Docker", detector.CategoryServer, detector.StatusStopped))
	s.Set(item("Docker", detector.CategoryServer, detector.StatusRunning))

	if got, _ := s.Get("Docker"); got.Status != detector.StatusRunning {
		t.Errorf("Status = %q, want the later write to win", got.Status)
	}
	if s.Len() != 1 {
		t.Errorf("Len() = %d, want 1 — name is the primary key", s.Len())
	}
}

func TestAllIsSortedCaseInsensitively(t *testing.T) {
	s := New()
	for _, name := range []string{"redis", "Go", "docker", "Ollama"} {
		s.Set(item(name, detector.CategoryTool, detector.StatusInstalled))
	}

	want := []string{"docker", "Go", "Ollama", "redis"}
	got := s.All()

	if len(got) != len(want) {
		t.Fatalf("All() returned %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("All()[%d] = %q, want %q", i, got[i].Name, want[i])
		}
	}
}

func TestByCategoryFilters(t *testing.T) {
	s := New()
	s.Set(item("Go", detector.CategoryLanguage, detector.StatusInstalled))
	s.Set(item("Python", detector.CategoryLanguage, detector.StatusInstalled))
	s.Set(item("Redis", detector.CategoryServer, detector.StatusRunning))

	langs := s.ByCategory(detector.CategoryLanguage)
	if len(langs) != 2 {
		t.Fatalf("ByCategory(language) returned %d items, want 2", len(langs))
	}
	if langs[0].Name != "Go" || langs[1].Name != "Python" {
		t.Errorf("ByCategory(language) = %q, %q — want sorted", langs[0].Name, langs[1].Name)
	}
	if got := s.ByCategory(detector.CategoryAI); got != nil {
		t.Errorf("ByCategory(ai) = %v, want nil", got)
	}
}

func TestFoundCountsPresentItemsOnly(t *testing.T) {
	s := New()
	s.Set(item("Go", detector.CategoryLanguage, detector.StatusInstalled))
	s.Set(item("Redis", detector.CategoryServer, detector.StatusRunning))
	s.Set(item("Docker", detector.CategoryServer, detector.StatusStopped))
	s.Set(item("Rust", detector.CategoryLanguage, detector.StatusNotFound))

	if got, want := s.Found(), 3; got != want {
		t.Errorf("Found() = %d, want %d — stopped still counts as present", got, want)
	}
	if got, want := s.Len(), 4; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
}

func TestSubscribeReceivesUpdates(t *testing.T) {
	s := New()
	sub := s.Subscribe()

	s.Set(item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	select {
	case got := <-sub:
		if got.Name != "Go" {
			t.Errorf("received %q, want Go", got.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a subscription update")
	}
}

func TestSubscribeFanOut(t *testing.T) {
	s := New()
	a, b := s.Subscribe(), s.Subscribe()

	s.Set(item("Go", detector.CategoryLanguage, detector.StatusInstalled))

	for i, sub := range []<-chan detector.Item{a, b} {
		select {
		case got := <-sub:
			if got.Name != "Go" {
				t.Errorf("subscriber %d received %q, want Go", i, got.Name)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d received nothing", i)
		}
	}
}

func TestSetDoesNotBlockOnAStalledSubscriber(t *testing.T) {
	s := New()
	_ = s.Subscribe() // never drained

	// The detection engine must never be held up by a subscriber that has
	// stopped reading, so writes past the buffer are shed instead of blocking.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < subBuffer*2; i++ {
			s.Set(item(string(rune('a'+i%26))+string(rune('0'+i/26)), detector.CategoryTool, detector.StatusInstalled))
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Set blocked on a subscriber that was not draining")
	}

	if s.Dropped() == 0 {
		t.Error("Dropped() = 0, want the shed notifications to be counted")
	}
}

func TestCloseIsIdempotentAndStopsWrites(t *testing.T) {
	s := New()
	sub := s.Subscribe()
	s.Set(item("Go", detector.CategoryLanguage, detector.StatusInstalled))
	<-sub

	s.Close()
	s.Close() // must not panic on a double close

	select {
	case _, ok := <-sub:
		if ok {
			t.Error("subscription delivered a value after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("subscription channel was not closed by Close")
	}

	s.Set(item("Rust", detector.CategoryLanguage, detector.StatusInstalled))
	if _, ok := s.Get("Rust"); ok {
		t.Error("Set after Close was applied, want a no-op")
	}
}

func TestSubscribeAfterCloseReturnsAClosedChannel(t *testing.T) {
	s := New()
	s.Close()

	select {
	case _, ok := <-s.Subscribe():
		if ok {
			t.Error("Subscribe after Close delivered a value")
		}
	case <-time.After(time.Second):
		t.Fatal("Subscribe after Close returned an open channel")
	}
}

// TestConcurrentAccess is the reason the store exists: the engine writes from
// one goroutine per detector while the TUI reads. Run under -race.
func TestConcurrentAccess(t *testing.T) {
	s := New()
	sub := s.Subscribe()

	// Drain, so the writers are never shed.
	go func() {
		for range sub {
		}
	}()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				s.Set(item(string(rune('A'+w)), detector.CategoryLanguage, detector.StatusInstalled))
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = s.All()
				_ = s.ByCategory(detector.CategoryLanguage)
				_ = s.Found()
			}
		}()
	}

	wg.Wait()
	s.Close()

	if got, want := s.Len(), 8; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
}
