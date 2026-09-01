package platform

import (
	"context"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

// Resolver works out which manager owns an install.
//
// It runs during a scan, once per item, so the result can be stored and every
// later question — can this be uninstalled? what should the user run instead?
// — is answered from memory rather than by probing again.
type Resolver struct {
	p  probe.Prober
	os OS
	// updates, when set, marks items whose manager reports a newer version.
	updates *Updates
	// queriers are the OS package databases available on this machine, tried
	// in order for a path the prefix rules did not claim.
	queriers []querier
}

// querier asks an OS package database which package owns a path.
type querier struct {
	manager string
	bin     string
	args    []string
	// parse pulls the package name out of the tool's answer.
	parse func(out string) string
}

// NewResolver returns a resolver for the current OS.
func NewResolver(p probe.Prober) *Resolver {
	return NewResolverFor(p, Current())
}

// NewResolverFor returns a resolver for a specific OS, which is what makes
// the per-platform rules testable from any machine.
func NewResolverFor(p probe.Prober, os OS) *Resolver {
	r := &Resolver{p: p, os: os}

	if os == Linux {
		// Only register databases that actually exist here, so a scan never
		// shells out to a package manager this distribution does not have.
		for _, q := range linuxQueriers() {
			if _, err := p.LookPath(q.bin); err == nil {
				r.queriers = append(r.queriers, q)
			}
		}
	}
	return r
}

// WithUpdates attaches an update check, whose results tag items as the scan
// resolves them.
func (r *Resolver) WithUpdates(u *Updates) *Resolver {
	r.updates = u
	return r
}

func linuxQueriers() []querier {
	return []querier{
		{
			manager: APT, bin: "dpkg", args: []string{"-S", "{}"},
			// "coreutils: /usr/bin/ls"
			parse: func(out string) string {
				name, _, found := strings.Cut(firstLine(out), ":")
				if !found {
					return ""
				}
				return strings.TrimSpace(name)
			},
		},
		{
			manager: DNF, bin: "rpm", args: []string{"-qf", "--queryformat", "%{NAME}", "{}"},
			parse: func(out string) string { return strings.TrimSpace(firstLine(out)) },
		},
		{
			manager: Pacman, bin: "pacman", args: []string{"-Qo", "{}"},
			// "/usr/bin/ls is owned by coreutils 9.5-1"
			parse: func(out string) string {
				_, rest, found := strings.Cut(firstLine(out), " is owned by ")
				if !found {
					return ""
				}
				fields := strings.Fields(rest)
				if len(fields) == 0 {
					return ""
				}
				return fields[0]
			},
		},
	}
}

// Resolve tags an item with the manager that owns it and the identifier that
// manager knows it by.
func (r *Resolver) Resolve(ctx context.Context, item detector.Item) detector.Item {
	if item.Path == "" || !item.Status.Found() {
		return item
	}

	// Follow symlinks first: package managers put links on PATH and the real
	// files in their own tree, and only the real path identifies the owner.
	resolved := item.Path
	if target, err := r.p.Resolve(item.Path); err == nil {
		resolved = target
	}

	switch manager, pkg := r.byPath(resolved, item); {
	case manager != "":
		item.ManagedBy, item.PackageID = manager, pkg
	default:
		if manager, pkg := r.byPackageDatabase(ctx, resolved); manager != "" {
			item.ManagedBy, item.PackageID = manager, pkg
		} else {
			// Nothing claims it: an install someone put there by hand.
			item.ManagedBy, item.PackageID = Manual, dirOf(resolved)
		}
	}

	if r.updates != nil {
		item.UpdateAvail = r.updates.Outdated(item)
	}
	return item
}

// byPath identifies an owner from where the software lives. Every one of
// these tools has a fixed, documented layout, which makes the path a reliable
// answer and a free one.
func (r *Resolver) byPath(path string, item detector.Item) (manager, pkg string) {
	home := r.p.Home()

	// Version managers first: their trees sit inside the home directory and
	// would otherwise be mistaken for manual installs.
	for _, vm := range []struct {
		manager string
		dirs    []string
		version func(path string) string
	}{
		{NVM, []string{".nvm/versions/node", ".nvm"}, versionAfter(".nvm/versions/node")},
		{Pyenv, []string{".pyenv/versions", ".pyenv"}, versionAfter(".pyenv/versions")},
		{Rbenv, []string{".rbenv/versions", ".rbenv"}, versionAfter(".rbenv/versions")},
		{SDKMAN, []string{".sdkman/candidates", ".sdkman"}, versionAfter(".sdkman/candidates")},
		{Mise, []string{".local/share/mise", ".local/share/rtx"}, versionAfter("installs")},
		{ASDF, []string{".asdf/installs", ".asdf"}, versionAfter(".asdf/installs")},
		{Rustup, []string{".rustup/toolchains", ".cargo", ".rustup"}, versionAfter(".rustup/toolchains")},
	} {
		for _, dir := range vm.dirs {
			prefix := dir
			if home != "" {
				prefix = slash(home) + "/" + dir
			}
			if !within(path, prefix) {
				continue
			}
			if v := vm.version(path); v != "" {
				return vm.manager, v
			}
			return vm.manager, item.Version
		}
	}

	// Homebrew, on macOS and Linux alike.
	if isBrewPath(path) {
		if formula := formulaFromPath(path); formula != "" {
			return Homebrew, formula
		}
		return Homebrew, ""
	}

	switch r.os {
	case Windows:
		if home != "" && within(path, slash(home)+"/scoop") {
			return Scoop, segmentAfter(path, "apps")
		}
		if within(path, `C:\ProgramData\chocolatey`) {
			return Chocolatey, baseOf(dirOf(path))
		}
	case Linux:
		if within(path, "/snap/") || within(path, "/var/lib/snapd/snap") {
			return Snap, segmentAfter(path, "snap")
		}
	}

	// Operating system territory. On macOS these are Apple's own, and on
	// Linux the package database below gives a better answer for /usr/bin, so
	// this only claims the paths no distribution packages.
	if r.isSystemPath(path) {
		return System, baseOf(path)
	}
	return "", ""
}

// isSystemPath reports whether a path belongs to the operating system itself.
func (r *Resolver) isSystemPath(path string) bool {
	switch r.os {
	case Darwin:
		for _, prefix := range []string{"/usr/bin/", "/bin/", "/sbin/", "/usr/sbin/", "/usr/libexec/", "/System/"} {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	case Linux:
		// Only the paths no package manager owns; the database is asked first
		// for everything under /usr/bin.
		for _, prefix := range []string{"/boot/", "/proc/", "/sys/"} {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	case Windows:
		for _, prefix := range []string{`C:\Windows\`, `C:\Program Files\WindowsApps\`} {
			if strings.HasPrefix(slash(path), slash(prefix)) {
				return true
			}
		}
	}
	return false
}

// byPackageDatabase asks the distribution which package owns a path. This is
// the authoritative answer on Linux, where /usr/bin holds packaged software
// rather than untouchable system files.
func (r *Resolver) byPackageDatabase(ctx context.Context, path string) (manager, pkg string) {
	for _, q := range r.queriers {
		out, err := r.p.Run(ctx, q.bin, Command(q.args, path)...)
		if err != nil {
			continue
		}
		if name := q.parse(out); name != "" {
			return q.manager, name
		}
	}
	return "", ""
}

// brewPrefixes are the standard Homebrew roots: Apple silicon, Intel macOS,
// and Linuxbrew.
var brewPrefixes = []string{
	"/opt/homebrew/",
	"/usr/local/Cellar/",
	"/usr/local/opt/",
	"/home/linuxbrew/",
}

func isBrewPath(path string) bool {
	for _, prefix := range brewPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// formulaFromPath reads the formula name out of a resolved Homebrew path:
//
//	/opt/homebrew/Cellar/postgresql@15/15.19/bin/psql -> postgresql@15
//
// The last marker is the one that counts: the Apple silicon prefix is itself
// /opt/homebrew, so "/opt/homebrew/opt/node@20/bin/node" contains "/opt/"
// twice and only the second names a formula.
func formulaFromPath(path string) string {
	for _, marker := range []string{"/Cellar/", "/opt/"} {
		i := strings.LastIndex(path, marker)
		if i < 0 {
			continue
		}
		name, _, _ := strings.Cut(path[i+len(marker):], "/")
		if name != "" && name != "bin" && name != "homebrew" {
			return name
		}
	}
	return ""
}

// slash normalises Windows separators, so the same rules apply whichever host
// the resolver is describing rather than whichever host it is compiled for.
func slash(p string) string { return strings.ReplaceAll(p, `\`, "/") }

// within reports whether path lies inside dir.
func within(path, dir string) bool {
	path, dir = slash(path), strings.TrimSuffix(slash(dir), "/")
	return path == dir || strings.HasPrefix(path, dir+"/")
}

// dirOf is filepath.Dir without the host's separator baked in.
func dirOf(path string) string {
	normalised := slash(path)
	i := strings.LastIndex(normalised, "/")
	if i <= 0 {
		return normalised
	}
	return normalised[:i]
}

// baseOf is filepath.Base without the host's separator baked in.
func baseOf(path string) string {
	normalised := strings.TrimSuffix(slash(path), "/")
	i := strings.LastIndex(normalised, "/")
	if i < 0 {
		return normalised
	}
	return normalised[i+1:]
}

// versionAfter builds a function reading the path segment following a marker,
// which is where every version manager puts the version number.
func versionAfter(marker string) func(string) string {
	return func(path string) string { return segmentAfter(path, lastSegment(marker)) }
}

// segmentAfter returns the path segment following the named one.
func segmentAfter(path, marker string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for i, part := range parts {
		if part == marker && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func lastSegment(p string) string {
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(line)
}
