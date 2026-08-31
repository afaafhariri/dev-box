package action

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

// brewFormulae maps an item name to the Homebrew formula that manages it.
// Only services listed here can be started and stopped; anything else has no
// safe, general way to be controlled.
var brewFormulae = map[string]string{
	"PostgreSQL": "postgresql",
	"MySQL":      "mysql",
	"Redis":      "redis",
	"MongoDB":    "mongodb-community",
	"Nginx":      "nginx",
	"Ollama":     "ollama",
}

// BrewService starts, stops, or restarts a service through Homebrew.
type BrewService struct {
	// label is the button text.
	label string
	// verb is the brew services subcommand.
	verb string
	// from is the status the item must be in for this to make sense.
	from detector.Status
	// done is the past-tense confirmation line.
	done string
}

// NewBrewStart returns the "Start" action.
func NewBrewStart() BrewService {
	return BrewService{label: "Start", verb: "start", from: detector.StatusStopped, done: "started"}
}

// NewBrewStop returns the "Stop" action.
func NewBrewStop() BrewService {
	return BrewService{label: "Stop", verb: "stop", from: detector.StatusRunning, done: "stopped"}
}

// NewBrewRestart returns the "Restart" action.
func NewBrewRestart() BrewService {
	return BrewService{label: "Restart", verb: "restart", from: detector.StatusRunning, done: "restarted"}
}

func (a BrewService) Label() string { return a.label }

func (a BrewService) Applicable(item detector.Item) bool {
	_, managed := brewFormulae[item.Name]
	return managed && item.Status == a.from
}

func (a BrewService) Run(ctx context.Context, item detector.Item) <-chan string {
	formula, ok := brewFormulae[item.Name]
	if !ok {
		return closed(fmt.Sprintf("✗ %s is not managed by Homebrew", item.Name))
	}
	return run(ctx, fmt.Sprintf("%s %s", item.Name, a.done), "brew", "services", a.verb, formula)
}

// macApps maps an item to the macOS application that provides it, for the
// things that ship as an app bundle rather than a Homebrew service.
var macApps = map[string]string{
	"Docker": "Docker",
	"Ollama": "Ollama",
}

// LaunchApp starts a macOS application bundle.
type LaunchApp struct{}

// NewLaunchApp returns the "Launch app" action.
func NewLaunchApp() LaunchApp { return LaunchApp{} }

func (a LaunchApp) Label() string { return "Launch app" }

func (a LaunchApp) Applicable(item detector.Item) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, ok := macApps[item.Name]
	return ok && item.Status == detector.StatusStopped
}

func (a LaunchApp) Run(ctx context.Context, item detector.Item) <-chan string {
	app, ok := macApps[item.Name]
	if !ok {
		return closed(fmt.Sprintf("✗ no application known for %s", item.Name))
	}
	// "open" returns as soon as the app is launched; the daemon behind it may
	// take a few more seconds to accept connections, so the next scan is what
	// confirms it is really up.
	return run(ctx, fmt.Sprintf("%s launching — rescan to confirm", app), "open", "-a", app)
}

// Upgrade updates a Homebrew-installed tool.
type Upgrade struct{}

// NewUpgrade returns the "Upgrade" action.
func NewUpgrade() Upgrade { return Upgrade{} }

func (a Upgrade) Label() string { return "Upgrade" }

func (a Upgrade) Applicable(item detector.Item) bool {
	// Only offer this for things Homebrew installed, which its own prefix
	// identifies. Upgrading anything else risks fighting the tool that
	// actually manages it — pyenv, nvm, a system package manager.
	return item.Status.Found() && isBrewPath(item.Path)
}

func (a Upgrade) Run(ctx context.Context, item detector.Item) <-chan string {
	formula := brewFormulae[item.Name]
	if formula == "" {
		formula = brewName(item)
	}
	return run(ctx, fmt.Sprintf("%s upgraded", item.Name), "brew", "upgrade", formula)
}

// brewPrefixes are the standard Homebrew install roots: Apple silicon,
// Intel macOS, and Linuxbrew.
var brewPrefixes = []string{"/opt/homebrew/", "/usr/local/Cellar/", "/usr/local/opt/", "/home/linuxbrew/"}

func isBrewPath(path string) bool {
	for _, prefix := range brewPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// brewName guesses the formula name from the binary that was detected, which
// is right for the many tools whose formula matches their command.
func brewName(item detector.Item) string {
	if bin := item.Meta["binary"]; bin != "" {
		return bin
	}
	return filepath.Base(item.Path)
}

// closed returns an already-finished channel carrying one line, for the
// guard paths that cannot run anything.
func closed(line string) <-chan string {
	ch := make(chan string, 1)
	ch <- line
	close(ch)
	return ch
}

// DefaultRegistry returns the actions available on this machine.
//
// Homebrew actions are only registered when brew is actually installed, so
// Applicable stays a cheap in-memory check rather than a PATH lookup on every
// render.
func DefaultRegistry(p probe.Prober) *Registry {
	r := NewRegistry()

	if _, err := p.LookPath("brew"); err == nil {
		r.Register(NewBrewStart(), NewBrewStop(), NewBrewRestart(), NewUpgrade())
	}
	if runtime.GOOS == "darwin" {
		r.Register(NewLaunchApp())
	}
	return r
}
