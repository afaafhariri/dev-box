package action

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// outputBuffer is the per-action channel depth. Output is consumed by the UI
// as fast as it renders, so this only has to absorb bursts.
const outputBuffer = 64

// Scanner limits for reading action output. The initial buffer stays small
// because almost every line is short; the cap is what a pathological line is
// allowed to reach before the read is failed rather than silently truncated.
const (
	initialScanBuffer = 64 * 1024
	maxScanLine       = 4 * 1024 * 1024
)

// commandTimeout bounds a single action. Actions start and stop services, which
// should be quick; anything slower has hung.
const commandTimeout = 60 * time.Second

// stream runs a command and writes its output to ch line by line, merging
// stdout and stderr so failures read in sequence with progress.
//
// It never closes ch: the caller owns the channel and may run several commands
// into it.
func stream(ctx context.Context, ch chan<- string, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = commandTimeout
	// Actions must never block waiting for input the user cannot give: a
	// password prompt here would hang the UI with no way to answer it.
	cmd.Stdin = nil

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pipe stdout: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	emit(ch, "$ "+name+" "+strings.Join(args, " "))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", name, err)
	}

	// Read on a goroutine so a cancelled context can return while the pipe
	// drains behind us.
	var wg sync.WaitGroup
	var scanErr error
	wg.Add(1)
	go func() {
		defer wg.Done()

		scanner := bufio.NewScanner(pipe)
		// A single line can be far longer than the 64KB default: a download
		// progress bar redraws with carriage returns rather than newlines, so
		// the whole of it arrives as one line.
		scanner.Buffer(make([]byte, 0, initialScanBuffer), maxScanLine)

		for scanner.Scan() {
			send(ctx, ch, scanner.Text())
		}
		// Scan reports false for a read failure exactly as it does for EOF.
		// Without this check, truncated output would be indistinguishable
		// from complete output, and the action would still claim success.
		scanErr = scanner.Err()
	}()
	// Wait also publishes scanErr to this goroutine.
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s: %w", name, ctx.Err())
		}
		return fmt.Errorf("%s: %w", name, err)
	}
	if scanErr != nil {
		return fmt.Errorf("reading %s output: %w", name, scanErr)
	}
	return nil
}

// send delivers one line of streamed output. It gives up if the context is
// cancelled, so a stopped action cannot block forever on a channel nobody is
// reading.
func send(ctx context.Context, ch chan<- string, line string) {
	select {
	case ch <- line:
	case <-ctx.Done():
	}
}

// emit delivers a line without consulting the context, so the command echo and
// the closing status still reach the pane when the action was cancelled. An
// action that ends silently looks like a UI that stopped working.
//
// It never blocks: if the buffer really is full, the tail of the output is
// worth less than keeping the app responsive.
func emit(ch chan<- string, line string) {
	select {
	case ch <- line:
	default:
	}
}

// run is the shared body of every command-backed action: it opens the channel,
// streams the command, reports the outcome, and closes up.
func run(ctx context.Context, done string, name string, args ...string) <-chan string {
	ch := make(chan string, outputBuffer)

	go func() {
		defer close(ch)

		if err := stream(ctx, ch, name, args...); err != nil {
			emit(ch, "✗ "+err.Error())
			return
		}
		emit(ch, "✓ "+done)
	}()

	return ch
}

// Drain collects every line an action produces. It exists for tests and for
// the headless path, where there is no UI to stream into.
func Drain(ch <-chan string) []string {
	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	return lines
}
