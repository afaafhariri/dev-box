package action

import (
	"context"
	"fmt"
	"runtime"

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
	// p resolves the item's real formula.
	p probe.Prober
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
func NewBrewStart(p probe.Prober) BrewService {
	return BrewService{p: p, label: "Start", verb: "start", from: detector.StatusStopped, done: "started"}
}

// NewBrewStop returns the "Stop" action.
func NewBrewStop(p probe.Prober) BrewService {
	return BrewService{p: p, label: "Stop", verb: "stop", from: detector.StatusRunning, done: "stopped"}
}

// NewBrewRestart returns the "Restart" action.
func NewBrewRestart(p probe.Prober) BrewService {
	return BrewService{p: p, label: "Restart", verb: "restart", from: detector.StatusRunning, done: "restarted"}
}

func (a BrewService) Label() string { return a.label }

func (a BrewService) Applicable(item detector.Item) bool {
	_, managed := brewFormulae[item.Name]
	return managed && item.Status == a.from
}

func (a BrewService) Run(ctx context.Context, item detector.Item) <-chan string {
	// Prefer the formula resolved from the install path: versioned formulae
	// are common for databases, and "brew services start postgresql" fails
	// outright when what is installed is postgresql@15.
	name := formula(a.p, item)
	if name == "" {
		name = brewFormulae[item.Name]
	}
	if name == "" {
		return closed(fmt.Sprintf("✗ %s is not managed by Homebrew", item.Name))
	}
	return run(ctx, fmt.Sprintf("%s %s", item.Name, a.done), "brew", "services", a.verb, name)
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
type Upgrade struct{ p probe.Prober }

// NewUpgrade returns the "Upgrade" action.
func NewUpgrade(p probe.Prober) Upgrade { return Upgrade{p: p} }

func (a Upgrade) Label() string { return "Upgrade" }

func (a Upgrade) Applicable(item detector.Item) bool {
	// Only offer this for things Homebrew installed, which its own prefix
	// identifies. Upgrading anything else risks fighting the tool that
	// actually manages it — pyenv, nvm, a system package manager.
	return item.Status.Found() && IsBrewPath(item.Path)
}

func (a Upgrade) Run(ctx context.Context, item detector.Item) <-chan string {
	name := formula(a.p, item)
	if name == "" {
		return closed(fmt.Sprintf("✗ could not work out which Homebrew formula owns %s", binaryName(item)))
	}
	return run(ctx, fmt.Sprintf("%s upgraded", item.Name), "brew", "upgrade", name)
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
		// Uninstall is registered last so it sits at the far end of the
		// action bar, away from the one people reach for most.
		r.Register(NewBrewStart(p), NewBrewStop(p), NewBrewRestart(p), NewUpgrade(p), NewUninstall(p))
	}
	if runtime.GOOS == "darwin" {
		r.Register(NewLaunchApp())
	}
	return r
}
