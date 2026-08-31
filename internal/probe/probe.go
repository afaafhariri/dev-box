// Package probe wraps the operating-system interactions a detector needs —
// running a command, looking up a binary, checking a path or a port — behind a
// small interface. Detectors depend on Prober rather than os/exec directly, so
// they can be unit tested against a Fake without touching the host machine.
package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// ErrNotFound is returned by LookPath when a binary is not on PATH.
var ErrNotFound = errors.New("binary not found on PATH")

// Prober is the surface detectors are allowed to touch.
type Prober interface {
	// Run executes a command and returns its combined stdout+stderr. Many
	// tools (java, redis-server) report their version on stderr, so the
	// streams are merged rather than separated.
	Run(ctx context.Context, name string, args ...string) (string, error)

	// LookPath resolves a binary to its absolute path, or ErrNotFound.
	LookPath(name string) (string, error)

	// Exists reports whether a file or directory exists.
	Exists(path string) bool

	// PortOpen reports whether something is accepting TCP connections.
	PortOpen(ctx context.Context, host string, port int) bool

	// Home is the current user's home directory ("" if undeterminable).
	Home() string
}

// System is the real Prober, backed by the host machine.
type System struct {
	// Timeout bounds a single Run call. Zero means DefaultTimeout.
	Timeout time.Duration
	// DialTimeout bounds a single PortOpen call. Zero means DefaultDialTimeout.
	DialTimeout time.Duration
}

const (
	DefaultTimeout     = 5 * time.Second
	DefaultDialTimeout = 300 * time.Millisecond
)

// NewSystem returns a System with the default timeouts.
func NewSystem() *System { return &System{} }

func (s *System) runTimeout() time.Duration {
	if s.Timeout > 0 {
		return s.Timeout
	}
	return DefaultTimeout
}

func (s *System) dialTimeout() time.Duration {
	if s.DialTimeout > 0 {
		return s.DialTimeout
	}
	return DefaultDialTimeout
}

func (s *System) Run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.runTimeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	// A killed process can leave grandchildren holding the output pipes open,
	// which would block Wait forever. WaitDelay caps that — but it must stay
	// generous: it also fires when a process has exited and its I/O is merely
	// still draining, and a cold first exec (Gatekeeper, cold dyld cache) can
	// take seconds. Too short a delay reports healthy tools as missing.
	cmd.WaitDelay = s.runTimeout()
	// Detectors only ever read; never let a subprocess prompt on stdin.
	cmd.Stdin = nil

	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return string(out), fmt.Errorf("%s: %w", name, ctxErr)
		}
		return string(out), fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

func (s *System) LookPath(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	return path, nil
}

func (s *System) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *System) PortOpen(ctx context.Context, host string, port int) bool {
	ctx, cancel := context.WithTimeout(ctx, s.dialTimeout())
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *System) Home() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// Expand resolves a leading "~" against the prober's home directory.
func Expand(p Prober, path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home := p.Home()
	if home == "" {
		return path
	}
	return filepath.Join(home, path[1:])
}
