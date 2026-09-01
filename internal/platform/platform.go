// Package platform knows which tool owns an installed piece of software, and
// what can be done about it on this operating system.
//
// This is the difference between "no actions available" and an explanation.
// A Node.js under ~/.nvm cannot be touched by Homebrew, and /usr/bin/ruby must
// not be touched at all — in both cases the useful thing to tell someone is
// who does own it and what to run instead.
package platform

import (
	"runtime"
	"sort"
	"strings"
)

// OS is the operating system devenv is running on.
type OS string

const (
	Darwin  OS = "darwin"
	Linux   OS = "linux"
	Windows OS = "windows"
)

// Current is the OS of this build.
func Current() OS { return OS(runtime.GOOS) }

// Manager is whatever installed a piece of software and is responsible for
// removing or updating it.
type Manager struct {
	// Name is the display name, and the key items are tagged with.
	Name string

	// Upgrade and Uninstall are argv templates, with "{}" standing in for the
	// package identifier. An empty template means devenv will not run that
	// operation itself — see Hint.
	Upgrade   []string
	Uninstall []string

	// Service, when set, is the argv template for controlling a service, with
	// "{}" for the package and "{verb}" for start/stop/restart.
	Service []string

	// Outdated, when set, lists the manager's out-of-date packages. It takes
	// no package argument: asking once per scan is far cheaper than asking
	// once per item.
	Outdated []string

	// Hint is what to tell the user when devenv will not act for them. "{}"
	// is replaced by the package identifier.
	Hint string

	// Protected marks installs that must never be removed, whatever the user
	// asks: they belong to the operating system.
	Protected bool
}

// CanUninstall reports whether devenv will run an uninstall for this manager.
func (m Manager) CanUninstall() bool { return !m.Protected && len(m.Uninstall) > 0 }

// CanUpgrade reports whether devenv will run an upgrade for this manager.
func (m Manager) CanUpgrade() bool { return !m.Protected && len(m.Upgrade) > 0 }

// CanService reports whether devenv can start and stop this as a service.
func (m Manager) CanService() bool { return !m.Protected && len(m.Service) > 0 }

// CanCheckUpdates reports whether this manager can list out-of-date packages.
func (m Manager) CanCheckUpdates() bool { return len(m.Outdated) > 0 }

// Managers returns every known manager, for callers that need to sweep them.
func Managers() []Manager {
	out := make([]Manager, 0, len(managers))
	for _, m := range managers {
		out = append(out, m)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// Command fills a template with the package identifier.
func Command(template []string, pkg string) []string {
	out := make([]string, 0, len(template))
	for _, arg := range template {
		out = append(out, strings.ReplaceAll(arg, "{}", pkg))
	}
	return out
}

// ServiceCommand fills a service template with the package and a verb.
func ServiceCommand(template []string, pkg, verb string) []string {
	out := make([]string, 0, len(template))
	for _, arg := range template {
		arg = strings.ReplaceAll(arg, "{}", pkg)
		out = append(out, strings.ReplaceAll(arg, "{verb}", verb))
	}
	return out
}

// Advice is the Hint with the package identifier filled in.
//
// A hint that names a package is useless without one — "run 'nvm uninstall '"
// is worse than saying nothing — so those are dropped rather than rendered
// half-empty.
func (m Manager) Advice(pkg string) string {
	if m.Hint == "" {
		return ""
	}
	if pkg == "" && strings.Contains(m.Hint, "{}") {
		return ""
	}
	return strings.ReplaceAll(m.Hint, "{}", pkg)
}

// Manager names. Items carry these strings, so they are part of the JSON
// output and the disk cache format.
const (
	Homebrew   = "Homebrew"
	APT        = "APT"
	DNF        = "DNF"
	Pacman     = "pacman"
	Snap       = "Snap"
	Scoop      = "Scoop"
	Chocolatey = "Chocolatey"
	WinGet     = "WinGet"
	NVM        = "nvm"
	Pyenv      = "pyenv"
	Rbenv      = "rbenv"
	SDKMAN     = "SDKMAN!"
	Mise       = "mise"
	ASDF       = "asdf"
	Rustup     = "rustup"
	System     = "system"
	Manual     = "manual install"
	Container  = "container"
)

// managers is every manager devenv understands.
//
// Package managers that need root are deliberately hint-only. devenv runs
// actions with no stdin, so a sudo password prompt would hang with no way to
// answer it — printing the command the user should run is the honest option.
var managers = map[string]Manager{
	Homebrew: {
		Name:      Homebrew,
		Upgrade:   []string{"brew", "upgrade", "{}"},
		Uninstall: []string{"brew", "uninstall", "{}"},
		Service:   []string{"brew", "services", "{verb}", "{}"},
		// --quiet prints one formula per line and, unlike plain "outdated",
		// does not hit the network for a fresh index.
		Outdated: []string{"brew", "outdated", "--quiet"},
	},
	Scoop: {
		// Scoop installs per-user and needs no elevation.
		Name:      Scoop,
		Upgrade:   []string{"scoop", "update", "{}"},
		Uninstall: []string{"scoop", "uninstall", "{}"},
		Outdated:  []string{"scoop", "status"},
	},
	APT: {
		Name: APT,
		Hint: "installed by APT — run 'sudo apt-get remove {}' to remove it",
	},
	DNF: {
		Name: DNF,
		Hint: "installed by DNF — run 'sudo dnf remove {}' to remove it",
	},
	Pacman: {
		Name: Pacman,
		Hint: "installed by pacman — run 'sudo pacman -R {}' to remove it",
	},
	Snap: {
		Name: Snap,
		Hint: "installed as a snap — run 'sudo snap remove {}' to remove it",
	},
	WinGet: {
		Name: WinGet,
		Hint: "installed by WinGet — run 'winget uninstall {}' in an elevated prompt",
	},
	Chocolatey: {
		Name: Chocolatey,
		Hint: "installed by Chocolatey — run 'choco uninstall {}' in an elevated prompt",
	},
	NVM: {
		Name: NVM,
		Hint: "managed by nvm — run 'nvm uninstall {}' to remove this version",
	},
	Pyenv: {
		Name: Pyenv,
		Hint: "managed by pyenv — run 'pyenv uninstall {}' to remove this version",
	},
	Rbenv: {
		Name: Rbenv,
		Hint: "managed by rbenv — run 'rbenv uninstall {}' to remove this version",
	},
	SDKMAN: {
		Name: SDKMAN,
		Hint: "managed by SDKMAN! — run 'sdk uninstall <candidate> {}' to remove this version",
	},
	Mise: {
		Name: Mise,
		Hint: "managed by mise — run 'mise uninstall {}' to remove this version",
	},
	ASDF: {
		Name: ASDF,
		Hint: "managed by asdf — run 'asdf uninstall {}' to remove this version",
	},
	Rustup: {
		Name: Rustup,
		Hint: "managed by rustup — run 'rustup toolchain uninstall {}', or 'rustup self uninstall' for all of it",
	},
	System: {
		Name:      System,
		Protected: true,
		Hint:      "part of the operating system — removing it would break other software",
	},
	Manual: {
		Name: Manual,
		Hint: "installed manually — remove its directory ({}) by hand",
	},
	Container: {
		Name: Container,
		Hint: "provided by a container runtime rather than installed on the host",
	},
}

// Lookup returns the manager with a name, and whether it is known.
func Lookup(name string) (Manager, bool) {
	m, ok := managers[name]
	return m, ok
}

// systemdService is the Linux service template. systemctl --user needs no
// elevation; system units do, and the hint covers those.
var systemdService = []string{"systemctl", "--user", "{verb}", "{}"}

// ServiceTemplate is how services are controlled on an OS, given the manager
// that owns the install.
func ServiceTemplate(os OS, manager Manager) []string {
	switch os {
	case Darwin:
		// Homebrew's services command wraps launchd and needs no elevation.
		return manager.Service
	case Linux:
		if len(manager.Service) > 0 {
			return manager.Service
		}
		return systemdService
	default:
		// Windows service control requires elevation, so devenv does not
		// attempt it.
		return nil
	}
}
