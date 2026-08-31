package probe

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

// Fake is an in-memory Prober for tests. It lives in the main package rather
// than a _test.go file so every detector package can build its fixtures from it.
//
// A zero Fake answers "nothing is installed" to every question, which is the
// case most detector tests want for their not-found path.
type Fake struct {
	mu sync.Mutex

	// Commands maps a command line ("go version") to its result.
	Commands map[string]FakeResult
	// Paths maps a binary name ("go") to the path LookPath should return.
	Paths map[string]string
	// Files is the set of paths Exists should report as present.
	Files map[string]bool
	// Ports is the set of "host:port" strings PortOpen should accept.
	Ports map[string]bool
	// Links maps a path to what Resolve should return for it. A path with no
	// entry resolves to itself, as an ordinary non-symlink file would.
	Links map[string]string
	// HomeDir is what Home returns.
	HomeDir string

	// Calls records every command line passed to Run, in order.
	Calls []string
}

// FakeResult is one canned command outcome.
type FakeResult struct {
	Out string
	Err error
}

// Key builds the map key Fake.Commands is indexed by.
func Key(name string, args ...string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

func (f *Fake) Run(ctx context.Context, name string, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	key := Key(name, args...)

	f.mu.Lock()
	f.Calls = append(f.Calls, key)
	res, ok := f.Commands[key]
	f.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("%s: %w", name, ErrNotFound)
	}
	return res.Out, res.Err
}

func (f *Fake) LookPath(name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if path, ok := f.Paths[name]; ok {
		return path, nil
	}
	return "", fmt.Errorf("%s: %w", name, ErrNotFound)
}

func (f *Fake) Exists(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.Files[path]
}

func (f *Fake) Resolve(path string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if resolved, ok := f.Links[path]; ok {
		return resolved, nil
	}
	return path, nil
}

func (f *Fake) PortOpen(_ context.Context, host string, port int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.Ports[net.JoinHostPort(host, strconv.Itoa(port))]
}

func (f *Fake) Home() string { return f.HomeDir }

// Ran reports whether a command line was executed.
func (f *Fake) Ran(name string, args ...string) bool {
	key := Key(name, args...)

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range f.Calls {
		if c == key {
			return true
		}
	}
	return false
}
