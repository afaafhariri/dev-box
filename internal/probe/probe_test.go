package probe

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunCapturesStdout(t *testing.T) {
	out, err := NewSystem().Run(context.Background(), "sh", "-c", "echo hello")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("out = %q, want hello", out)
	}
}

func TestRunCapturesStderr(t *testing.T) {
	// Tools like java and redis-server report their version on stderr, so the
	// streams have to be merged or those detectors see nothing.
	out, err := NewSystem().Run(context.Background(), "sh", "-c", "echo oops >&2")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(out) != "oops" {
		t.Errorf("out = %q, want oops — stderr must be captured", out)
	}
}

func TestRunReportsNonZeroExitButKeepsOutput(t *testing.T) {
	out, err := NewSystem().Run(context.Background(), "sh", "-c", "echo partial; exit 1")
	if err == nil {
		t.Fatal("Run() error = nil, want a non-zero exit error")
	}
	// Ollama prints its version and still exits non-zero when its daemon is
	// down, so output on failure must survive.
	if strings.TrimSpace(out) != "partial" {
		t.Errorf("out = %q, want the output produced before the failure", out)
	}
}

func TestRunHonoursItsOwnTimeout(t *testing.T) {
	s := &System{Timeout: 100 * time.Millisecond}

	start := time.Now()
	_, err := s.Run(context.Background(), "sleep", "10")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Run() error = nil, want a timeout")
	}
	if elapsed > 5*time.Second {
		t.Errorf("Run() took %v — the timeout did not kill the process", elapsed)
	}
}

func TestRunHonoursParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := NewSystem().Run(ctx, "sleep", "10")

	if err == nil {
		t.Fatal("Run() error = nil, want cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Run() took %v after cancellation", elapsed)
	}
}

// TestConcurrentRunsAllSucceed is a regression test. WaitDelay also fires when
// a process has exited and its I/O is merely still draining, so a short delay
// made healthy tools report as missing under a cold, loaded first scan.
func TestConcurrentRunsAllSucceed(t *testing.T) {
	s := NewSystem()

	var wg sync.WaitGroup
	errs := make(chan error, 24)

	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Run(context.Background(), "sh", "-c", "echo ok"); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Run failed: %v", err)
	}
}

func TestLookPath(t *testing.T) {
	s := NewSystem()

	path, err := s.LookPath("sh")
	if err != nil {
		t.Fatalf("LookPath(sh) error = %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("LookPath(sh) = %q, want an absolute path", path)
	}

	if _, err := s.LookPath("definitely-not-a-real-binary-xyz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestExists(t *testing.T) {
	s := NewSystem()
	dir := t.TempDir()
	file := filepath.Join(dir, "present")

	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !s.Exists(file) {
		t.Error("Exists() = false for a file that exists")
	}
	if !s.Exists(dir) {
		t.Error("Exists() = false for a directory — version managers are found by directory")
	}
	if s.Exists(filepath.Join(dir, "absent")) {
		t.Error("Exists() = true for a missing path")
	}
}

func TestPortOpen(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	s := NewSystem()

	if !s.PortOpen(context.Background(), "127.0.0.1", port) {
		t.Error("PortOpen() = false for a port being listened on")
	}

	listener.Close()
	if s.PortOpen(context.Background(), "127.0.0.1", port) {
		t.Error("PortOpen() = true after the listener closed")
	}
}

func TestPortOpenIsFastWhenClosed(t *testing.T) {
	// Every scan dials the ports of servers this machine may not have, so a
	// closed port must fail fast rather than hang the scan.
	s := NewSystem()

	start := time.Now()
	s.PortOpen(context.Background(), "127.0.0.1", 1)

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("PortOpen on a closed port took %v", elapsed)
	}
}

func TestExpand(t *testing.T) {
	f := &Fake{HomeDir: "/Users/test"}

	tests := []struct {
		in   string
		want string
	}{
		{"~/.nvm", "/Users/test/.nvm"},
		{"/usr/local/bin", "/usr/local/bin"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := Expand(f, tt.in); got != tt.want {
			t.Errorf("Expand(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	// With no home directory known, the path is left untouched rather than
	// silently resolved against the filesystem root.
	if got := Expand(&Fake{}, "~/.nvm"); got != "~/.nvm" {
		t.Errorf("Expand with no home = %q, want the input unchanged", got)
	}
}
