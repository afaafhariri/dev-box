package action

import (
	"context"
	"fmt"

	"devenv/internal/detector"
	"devenv/internal/probe"
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

// Uninstall removes a Homebrew-installed package.
type Uninstall struct{ p probe.Prober }

// NewUninstall returns the "Uninstall" action.
func NewUninstall(p probe.Prober) Uninstall { return Uninstall{p: p} }

func (a Uninstall) Label() string { return "Uninstall" }

func (a Uninstall) Applicable(item detector.Item) bool {
	// Homebrew installs only. Trying to uninstall a system Python, an
	// nvm-managed node, or a rustup toolchain would either fail or damage the
	// tool that actually manages it.
	if !item.Status.Found() || !IsBrewPath(item.Path) {
		return false
	}
	// Without a formula there is nothing safe to run, so do not offer the
	// action at all rather than failing at the point of confirmation.
	return formula(a.p, item) != ""
}

// Confirm names the exact command that will run, because "Uninstall" alone
// does not tell the user which formula is about to be removed — and for
// PostgreSQL the formula (postgresql@15) is not the name on screen.
func (a Uninstall) Confirm(item detector.Item) string {
	name := formula(a.p, item)
	if name == "" {
		return fmt.Sprintf("Cannot uninstall %s: its Homebrew formula is unknown.", item.Name)
	}

	warning := ""
	if _, isService := brewFormulae[item.Name]; isService {
		// Homebrew leaves data directories behind, but people reasonably
		// assume otherwise, and a stopped service is easy to forget.
		warning = " Stored data under the Homebrew prefix is left in place."
	}

	return fmt.Sprintf("Run 'brew uninstall %s'?%s Press enter again to confirm, esc to cancel.", name, warning)
}

func (a Uninstall) Run(ctx context.Context, item detector.Item) <-chan string {
	name := formula(a.p, item)
	if name == "" {
		return closed(fmt.Sprintf("✗ could not work out which Homebrew formula owns %s", binaryName(item)))
	}

	// No --force and no --ignore-dependencies: if another formula depends on
	// this one, Homebrew refuses and says so, which is the right outcome. The
	// user can act on that themselves rather than having the tool override it.
	return run(ctx, fmt.Sprintf("%s uninstalled", item.Name), "brew", "uninstall", name)
}
