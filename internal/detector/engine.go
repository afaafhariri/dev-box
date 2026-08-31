package detector

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Engine runs every detector concurrently and streams results into a Sink as
// they arrive, so the UI can render the fast probes without waiting on the
// slow ones.
type Engine struct {
	detectors []Detector
	sink      Sink
	now       func() time.Time // swappable for deterministic tests
}

// NewEngine wires a registry to a sink.
func NewEngine(sink Sink, detectors ...Detector) *Engine {
	return &Engine{detectors: detectors, sink: sink, now: time.Now}
}

// Len is the number of detectors the engine will run.
func (e *Engine) Len() int { return len(e.detectors) }

// RunAll runs every detector and blocks until all have finished or ctx is
// cancelled. Every result — including "not installed" — reaches the sink, so
// the UI can show absent tools as well as present ones.
func (e *Engine) RunAll(ctx context.Context) {
	var wg sync.WaitGroup

	for _, det := range e.detectors {
		wg.Add(1)
		go func(d Detector) {
			defer wg.Done()
			e.sink.Set(e.run(ctx, d))
		}(det)
	}

	wg.Wait()
}

// run executes one detector, normalising whatever comes back into an Item. A
// detector that panics is contained here: one broken probe must not take down
// the scan.
func (e *Engine) run(ctx context.Context, d Detector) (item Item) {
	defer func() {
		if r := recover(); r != nil {
			item = e.failed(d, fmt.Sprintf("detector panicked: %v", r))
		}
	}()

	item, err := d.Detect(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return e.failed(d, "scan cancelled")
		}
		item = e.failed(d, "")
		// A real probe failure is worth showing; a plain absence is not.
		if !errors.Is(err, ErrNotInstalled) {
			item = item.WithMeta("error", err.Error())
		}
		return item
	}

	// Detectors are allowed to leave the bookkeeping fields blank.
	if item.Name == "" {
		item.Name = d.Name()
	}
	if item.Category == "" {
		item.Category = d.Category()
	}
	if item.Status == StatusUnknown {
		item.Status = StatusInstalled
	}
	if item.DetectedAt.IsZero() {
		item.DetectedAt = e.now()
	}
	return item
}

func (e *Engine) failed(d Detector, reason string) Item {
	item := Item{
		Name:       d.Name(),
		Category:   d.Category(),
		Status:     StatusNotFound,
		DetectedAt: e.now(),
	}
	return item.WithMeta("error", reason)
}
