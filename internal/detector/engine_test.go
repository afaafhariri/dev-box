package detector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeDetector is a scriptable Detector for engine tests.
type fakeDetector struct {
	name     string
	cat      Category
	item     Item
	err      error
	panics   bool
	delay    time.Duration
	observed chan context.Context
}

func (f *fakeDetector) Name() string       { return f.name }
func (f *fakeDetector) Category() Category { return f.cat }

func (f *fakeDetector) Detect(ctx context.Context) (Item, error) {
	if f.observed != nil {
		f.observed <- ctx
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return Item{}, ctx.Err()
		}
	}
	if f.panics {
		panic("boom")
	}
	return f.item, f.err
}

// recorder is a Sink that collects everything written to it.
type recorder struct {
	mu    sync.Mutex
	items map[string]Item
}

func newRecorder() *recorder { return &recorder{items: make(map[string]Item)} }

func (r *recorder) Set(item Item) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[item.Name] = item
}

func (r *recorder) get(name string) (Item, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[name]
	return item, ok
}

func (r *recorder) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

func TestRunAllWritesEveryDetectorToTheSink(t *testing.T) {
	rec := newRecorder()
	engine := NewEngine(rec,
		&fakeDetector{name: "Go", cat: CategoryLanguage, item: Item{Name: "Go", Version: "1.22.3"}},
		&fakeDetector{name: "Git", cat: CategoryTool, item: Item{Name: "Git", Version: "2.39.5"}},
		&fakeDetector{name: "Rust", cat: CategoryLanguage, err: ErrNotInstalled},
	)

	engine.RunAll(context.Background())

	if got, want := rec.len(), 3; got != want {
		t.Fatalf("sink holds %d items, want %d — absent tools must be recorded too", got, want)
	}
	if got, _ := rec.get("Go"); got.Version != "1.22.3" {
		t.Errorf("Go version = %q, want 1.22.3", got.Version)
	}
}

func TestRunAllFillsInBookkeepingFields(t *testing.T) {
	rec := newRecorder()
	// A detector that returns only the facts it discovered.
	engine := NewEngine(rec, &fakeDetector{
		name: "Go", cat: CategoryLanguage,
		item: Item{Version: "1.22.3"},
	})

	engine.RunAll(context.Background())

	got, ok := rec.get("Go")
	if !ok {
		t.Fatal("item not recorded under the detector name")
	}
	if got.Category != CategoryLanguage {
		t.Errorf("Category = %q, want language", got.Category)
	}
	if got.Status != StatusInstalled {
		t.Errorf("Status = %q, want installed by default", got.Status)
	}
	if got.DetectedAt.IsZero() {
		t.Error("DetectedAt not stamped")
	}
}

func TestRunAllPreservesAnExplicitStatus(t *testing.T) {
	rec := newRecorder()
	engine := NewEngine(rec, &fakeDetector{
		name: "Docker", cat: CategoryServer,
		item: Item{Name: "Docker", Status: StatusStopped},
	})

	engine.RunAll(context.Background())

	if got, _ := rec.get("Docker"); got.Status != StatusStopped {
		t.Errorf("Status = %q, want the detector's own stopped status", got.Status)
	}
}

func TestNotInstalledIsRecordedWithoutAnError(t *testing.T) {
	rec := newRecorder()
	engine := NewEngine(rec, &fakeDetector{name: "Rust", cat: CategoryLanguage, err: ErrNotInstalled})

	engine.RunAll(context.Background())

	got, _ := rec.get("Rust")
	if got.Status != StatusNotFound {
		t.Errorf("Status = %q, want not_found", got.Status)
	}
	// A plain absence is an expected outcome, so it must not be dressed up as
	// a failure in the UI.
	if reason, ok := got.Meta["error"]; ok && reason != "" {
		t.Errorf("Meta[error] = %q, want no error for a plain absence", reason)
	}
}

func TestProbeFailureIsRecordedWithItsReason(t *testing.T) {
	rec := newRecorder()
	engine := NewEngine(rec, &fakeDetector{
		name: "Go", cat: CategoryLanguage,
		err: errors.New("go version: unrecognised output"),
	})

	engine.RunAll(context.Background())

	got, _ := rec.get("Go")
	if got.Status != StatusNotFound {
		t.Errorf("Status = %q, want not_found", got.Status)
	}
	if got.Meta["error"] != "go version: unrecognised output" {
		t.Errorf("Meta[error] = %q, want the underlying reason", got.Meta["error"])
	}
}

func TestWrappedNotInstalledIsStillAPlainAbsence(t *testing.T) {
	rec := newRecorder()
	engine := NewEngine(rec, &fakeDetector{
		name: "Rust", cat: CategoryLanguage,
		err: fmt.Errorf("looking for rustc: %w", ErrNotInstalled),
	})

	engine.RunAll(context.Background())

	got, _ := rec.get("Rust")
	if reason, ok := got.Meta["error"]; ok && reason != "" {
		t.Errorf("Meta[error] = %q, want a wrapped ErrNotInstalled treated as absence", reason)
	}
}

func TestPanickingDetectorDoesNotTakeDownTheScan(t *testing.T) {
	rec := newRecorder()
	engine := NewEngine(rec,
		&fakeDetector{name: "Bad", cat: CategoryTool, panics: true},
		&fakeDetector{name: "Go", cat: CategoryLanguage, item: Item{Name: "Go", Version: "1.22.3"}},
	)

	engine.RunAll(context.Background())

	if got, want := rec.len(), 2; got != want {
		t.Fatalf("sink holds %d items, want %d — one bad detector must not lose the others", got, want)
	}
	bad, _ := rec.get("Bad")
	if bad.Status != StatusNotFound {
		t.Errorf("Status = %q, want not_found", bad.Status)
	}
	if bad.Meta["error"] == "" {
		t.Error("a panicking detector should record why it failed")
	}
	if got, _ := rec.get("Go"); got.Version != "1.22.3" {
		t.Error("the healthy detector's result was lost")
	}
}

func TestCancellationPropagatesToDetectors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rec := newRecorder()
	engine := NewEngine(rec, &fakeDetector{
		name: "Slow", cat: CategoryTool,
		delay: 30 * time.Second,
	})

	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		engine.RunAll(ctx)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunAll did not return promptly after cancellation")
	}

	got, _ := rec.get("Slow")
	if got.Meta["error"] != "scan cancelled" {
		t.Errorf("Meta[error] = %q, want 'scan cancelled'", got.Meta["error"])
	}
}

func TestEveryDetectorReceivesTheContext(t *testing.T) {
	observed := make(chan context.Context, 1)
	rec := newRecorder()
	engine := NewEngine(rec, &fakeDetector{
		name: "Go", cat: CategoryLanguage, observed: observed,
		item: Item{Name: "Go"},
	})

	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("trace"), "abc")
	engine.RunAll(ctx)

	got := <-observed
	if got.Value(ctxKey("trace")) != "abc" {
		t.Error("detector did not receive the caller's context")
	}
}

func TestRunAllIsConcurrent(t *testing.T) {
	rec := newRecorder()
	const n = 8
	detectors := make([]Detector, 0, n)
	for i := 0; i < n; i++ {
		detectors = append(detectors, &fakeDetector{
			name:  fmt.Sprintf("d%d", i),
			cat:   CategoryTool,
			delay: 100 * time.Millisecond,
			item:  Item{Name: fmt.Sprintf("d%d", i)},
		})
	}

	engine := NewEngine(rec, detectors...)

	start := time.Now()
	engine.RunAll(context.Background())
	elapsed := time.Since(start)

	// Serial execution would take n*100ms; concurrent should be near 100ms.
	if elapsed > 500*time.Millisecond {
		t.Errorf("RunAll took %v for %d detectors — looks serial", elapsed, n)
	}
	if rec.len() != n {
		t.Errorf("sink holds %d items, want %d", rec.len(), n)
	}
}

func TestEngineLen(t *testing.T) {
	engine := NewEngine(newRecorder(),
		&fakeDetector{name: "a", cat: CategoryTool},
		&fakeDetector{name: "b", cat: CategoryTool},
	)
	if got, want := engine.Len(), 2; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
}
