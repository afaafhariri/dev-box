package action

import (
	"context"
	"fmt"

	"devenv/internal/detector"
	"devenv/internal/platform"
)

// Destructive marks an action whose effect the user cannot casually undo. The
// UI asks for confirmation before running one, so a stray keypress in a list
// of actions cannot remove something.
//
// It is an optional interface: an action that does not implement it runs
// straight away.
type Destructive interface {
	Action
	// Confirm is the question put to the user before the action runs.
	Confirm(item detector.Item) string
}

// manager returns the manager that owns an item, and whether it is known.
// Everything here is decided from the item alone: ownership was resolved once
// during the scan, so Applicable never touches the machine.
func manager(item detector.Item) (platform.Manager, bool) {
	if !item.Status.Found() || item.ManagedBy == "" {
		return platform.Manager{}, false
	}
	return platform.Lookup(item.ManagedBy)
}

// Service starts, stops, or restarts a service through whatever manages it.
type Service struct {
	os    platform.OS
	label string
	verb  string
	from  detector.Status
	done  string
}

// NewStart returns the "Start" action.
func NewStart(os platform.OS) Service {
	return Service{os: os, label: "Start", verb: "start", from: detector.StatusStopped, done: "started"}
}

// NewStop returns the "Stop" action.
func NewStop(os platform.OS) Service {
	return Service{os: os, label: "Stop", verb: "stop", from: detector.StatusRunning, done: "stopped"}
}

// NewRestart returns the "Restart" action.
func NewRestart(os platform.OS) Service {
	return Service{os: os, label: "Restart", verb: "restart", from: detector.StatusRunning, done: "restarted"}
}

func (a Service) Label() string { return a.label }

func (a Service) Applicable(item detector.Item) bool {
	// Only things that are actually services, in the state where the verb
	// makes sense.
	if item.Status != a.from || !isService(item) {
		return false
	}

	m, ok := manager(item)
	if !ok || m.Protected {
		return false
	}
	return len(platform.ServiceTemplate(a.os, m)) > 0 && item.PackageID != ""
}

func (a Service) Run(ctx context.Context, item detector.Item) <-chan string {
	m, ok := manager(item)
	if !ok {
		return closed(fmt.Sprintf("✗ nothing known to manage %s", item.Name))
	}

	template := platform.ServiceTemplate(a.os, m)
	if len(template) == 0 || item.PackageID == "" {
		return closed(fmt.Sprintf("✗ no way to %s %s on %s", a.verb, item.Name, a.os))
	}

	argv := platform.ServiceCommand(template, item.PackageID, a.verb)
	return run(ctx, fmt.Sprintf("%s %s", item.Name, a.done), argv[0], argv[1:]...)
}

// isService reports whether an item is the kind of thing that runs.
func isService(item detector.Item) bool {
	return item.Category == detector.CategoryServer || item.Category == detector.CategoryAI
}

// Upgrade updates an item through the manager that installed it.
type Upgrade struct{}

// NewUpgrade returns the "Upgrade" action.
func NewUpgrade() Upgrade { return Upgrade{} }

func (a Upgrade) Label() string { return "Upgrade" }

func (a Upgrade) Applicable(item detector.Item) bool {
	m, ok := manager(item)
	return ok && m.CanUpgrade() && item.PackageID != ""
}

func (a Upgrade) Run(ctx context.Context, item detector.Item) <-chan string {
	m, ok := manager(item)
	if !ok || !m.CanUpgrade() || item.PackageID == "" {
		return closed(refusal(item, "upgrade"))
	}

	argv := platform.Command(m.Upgrade, item.PackageID)
	return run(ctx, fmt.Sprintf("%s upgraded", item.Name), argv[0], argv[1:]...)
}

// Uninstall removes an item through the manager that installed it.
type Uninstall struct{}

// NewUninstall returns the "Uninstall" action.
func NewUninstall() Uninstall { return Uninstall{} }

func (a Uninstall) Label() string { return "Uninstall" }

func (a Uninstall) Applicable(item detector.Item) bool {
	m, ok := manager(item)
	// Protected installs belong to the operating system, and managers with no
	// uninstall template are ones devenv deliberately will not drive — a
	// distribution package manager needing a sudo password it cannot supply.
	return ok && m.CanUninstall() && item.PackageID != ""
}

// Confirm names the exact command that will run. "Uninstall" alone does not
// say which package goes, and for PostgreSQL the formula (postgresql@15) is
// not the name on screen.
func (a Uninstall) Confirm(item detector.Item) string {
	m, ok := manager(item)
	if !ok || !m.CanUninstall() || item.PackageID == "" {
		return refusal(item, "uninstall")
	}

	argv := platform.Command(m.Uninstall, item.PackageID)
	command := argv[0]
	for _, arg := range argv[1:] {
		command += " " + arg
	}

	warning := ""
	if isService(item) {
		// Package managers generally leave data directories behind, but
		// people reasonably assume otherwise.
		warning = " Stored data is left in place."
	}
	return fmt.Sprintf("Run '%s'?%s Press enter again to confirm, esc to cancel.", command, warning)
}

func (a Uninstall) Run(ctx context.Context, item detector.Item) <-chan string {
	m, ok := manager(item)
	if !ok || !m.CanUninstall() || item.PackageID == "" {
		return closed(refusal(item, "uninstall"))
	}

	// No force and no dependency overrides: if something else depends on this,
	// the package manager refuses and says so, which is the right outcome.
	argv := platform.Command(m.Uninstall, item.PackageID)
	return run(ctx, fmt.Sprintf("%s uninstalled", item.Name), argv[0], argv[1:]...)
}

// refusal explains why devenv will not do something, naming what the user
// should run instead. This is the answer to "why is there no uninstall
// button" — a question the UI should never leave unanswered.
func refusal(item detector.Item, verb string) string {
	m, ok := manager(item)
	if !ok {
		return fmt.Sprintf("✗ cannot %s %s: nothing is known to manage it", verb, item.Name)
	}
	if advice := m.Advice(item.PackageID); advice != "" {
		return fmt.Sprintf("✗ cannot %s %s: %s", verb, item.Name, advice)
	}
	return fmt.Sprintf("✗ cannot %s %s: %s does not support it", verb, item.Name, m.Name)
}

// Guidance is what to tell the user about an item devenv cannot act on, or ""
// when there is nothing worth saying.
func Guidance(item detector.Item) string {
	m, ok := manager(item)
	if !ok {
		return ""
	}
	return m.Advice(item.PackageID)
}

// closed returns an already-finished channel carrying one line, for the guard
// paths that cannot run anything.
func closed(line string) <-chan string {
	ch := make(chan string, 1)
	ch <- line
	close(ch)
	return ch
}

// DefaultRegistry returns the actions available on this machine.
func DefaultRegistry() *Registry {
	return RegistryFor(platform.Current())
}

// RegistryFor returns the actions available on a given OS, which is what makes
// the per-platform behaviour testable from any machine.
func RegistryFor(os platform.OS) *Registry {
	r := NewRegistry()
	// Uninstall is registered last so it sits at the far end of the action
	// bar, away from the one people reach for most.
	r.Register(NewStart(os), NewStop(os), NewRestart(os), NewUpgrade(), NewUninstall())
	return r
}
