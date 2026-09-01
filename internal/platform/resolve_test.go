package platform

import (
	"context"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

const home = "/Users/test"

// resolverWith builds a resolver for an OS, with symlinks that resolve to
// themselves unless stated.
func resolverWith(os OS, links map[string]string) *Resolver {
	return NewResolverFor(&probe.Fake{HomeDir: home, Links: links}, os)
}

func installed(path string) detector.Item {
	return detector.Item{
		Name: "Thing", Category: detector.CategoryTool,
		Status: detector.StatusInstalled, Path: path, Version: "1.0.0",
	}
}

func TestResolveMacOS(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		resolvesTo  string
		wantManager string
		wantPackage string
	}{
		{
			name:       "homebrew formula from the cellar path",
			path:       "/opt/homebrew/bin/psql",
			resolvesTo: "/opt/homebrew/Cellar/postgresql@15/15.19/bin/psql",
			// The formula is not the command name, which is the whole reason
			// this is resolved rather than guessed.
			wantManager: Homebrew, wantPackage: "postgresql@15",
		},
		{
			name:        "intel homebrew prefix",
			path:        "/usr/local/Cellar/redis/7.2.4/bin/redis-server",
			wantManager: Homebrew, wantPackage: "redis",
		},
		{
			name:        "nvm-managed node carries its version",
			path:        home + "/.nvm/versions/node/v22.13.1/bin/node",
			wantManager: NVM, wantPackage: "v22.13.1",
		},
		{
			name:        "pyenv-managed python",
			path:        home + "/.pyenv/versions/3.12.4/bin/python3",
			wantManager: Pyenv, wantPackage: "3.12.4",
		},
		{
			name:        "rbenv-managed ruby",
			path:        home + "/.rbenv/versions/3.3.1/bin/ruby",
			wantManager: Rbenv, wantPackage: "3.3.1",
		},
		{
			name:        "rustup toolchain",
			path:        home + "/.rustup/toolchains/stable-aarch64-apple-darwin/bin/rustc",
			wantManager: Rustup, wantPackage: "stable-aarch64-apple-darwin",
		},
		{
			name:        "system ruby belongs to the OS",
			path:        "/usr/bin/ruby",
			wantManager: System, wantPackage: "ruby",
		},
		{
			name:        "system java stub",
			path:        "/usr/bin/java",
			wantManager: System, wantPackage: "java",
		},
		{
			name:        "go from the official tarball is a manual install",
			path:        "/usr/local/go/bin/go",
			wantManager: Manual, wantPackage: "/usr/local/go/bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := map[string]string{}
			if tt.resolvesTo != "" {
				links[tt.path] = tt.resolvesTo
			}

			got := resolverWith(Darwin, links).Resolve(context.Background(), installed(tt.path))

			if got.ManagedBy != tt.wantManager {
				t.Errorf("ManagedBy = %q, want %q", got.ManagedBy, tt.wantManager)
			}
			if got.PackageID != tt.wantPackage {
				t.Errorf("PackageID = %q, want %q", got.PackageID, tt.wantPackage)
			}
		})
	}
}

func TestResolveLinuxUsesThePackageDatabase(t *testing.T) {
	// On Linux /usr/bin holds packaged software, so the distribution database
	// is the authority rather than a path rule.
	p := &probe.Fake{
		HomeDir: "/home/test",
		Paths:   map[string]string{"dpkg": "/usr/bin/dpkg"},
		Commands: map[string]probe.FakeResult{
			"dpkg -S /usr/bin/git": {Out: "git: /usr/bin/git\n"},
		},
	}

	got := NewResolverFor(p, Linux).Resolve(context.Background(), installed("/usr/bin/git"))

	if got.ManagedBy != APT {
		t.Errorf("ManagedBy = %q, want %q", got.ManagedBy, APT)
	}
	if got.PackageID != "git" {
		t.Errorf("PackageID = %q, want git", got.PackageID)
	}
}

func TestResolveLinuxRPM(t *testing.T) {
	p := &probe.Fake{
		HomeDir: "/home/test",
		Paths:   map[string]string{"rpm": "/usr/bin/rpm"},
		Commands: map[string]probe.FakeResult{
			"rpm -qf --queryformat %{NAME} /usr/bin/git": {Out: "git"},
		},
	}

	got := NewResolverFor(p, Linux).Resolve(context.Background(), installed("/usr/bin/git"))
	if got.ManagedBy != DNF || got.PackageID != "git" {
		t.Errorf("got (%q, %q), want (%q, git)", got.ManagedBy, got.PackageID, DNF)
	}
}

func TestResolveLinuxPacman(t *testing.T) {
	p := &probe.Fake{
		HomeDir: "/home/test",
		Paths:   map[string]string{"pacman": "/usr/bin/pacman"},
		Commands: map[string]probe.FakeResult{
			"pacman -Qo /usr/bin/git": {Out: "/usr/bin/git is owned by git 2.45.2-1\n"},
		},
	}

	got := NewResolverFor(p, Linux).Resolve(context.Background(), installed("/usr/bin/git"))
	if got.ManagedBy != Pacman || got.PackageID != "git" {
		t.Errorf("got (%q, %q), want (%q, git)", got.ManagedBy, got.PackageID, Pacman)
	}
}

func TestResolveLinuxSkipsAbsentPackageManagers(t *testing.T) {
	// A scan must never shell out to a package manager the machine lacks.
	p := &probe.Fake{HomeDir: "/home/test"}

	got := NewResolverFor(p, Linux).Resolve(context.Background(), installed("/usr/bin/git"))

	if len(p.Calls) != 0 {
		t.Errorf("ran %v with no package manager installed", p.Calls)
	}
	if got.ManagedBy != Manual {
		t.Errorf("ManagedBy = %q, want %q when nothing claims the path", got.ManagedBy, Manual)
	}
}

func TestResolveLinuxbrew(t *testing.T) {
	p := &probe.Fake{
		HomeDir: "/home/test",
		Links: map[string]string{
			"/home/linuxbrew/.linuxbrew/bin/gh": "/home/linuxbrew/.linuxbrew/Cellar/gh/2.52.0/bin/gh",
		},
	}

	got := NewResolverFor(p, Linux).Resolve(context.Background(), installed("/home/linuxbrew/.linuxbrew/bin/gh"))
	if got.ManagedBy != Homebrew || got.PackageID != "gh" {
		t.Errorf("got (%q, %q), want (Homebrew, gh)", got.ManagedBy, got.PackageID)
	}
}

func TestResolveLinuxSnap(t *testing.T) {
	p := &probe.Fake{HomeDir: "/home/test"}

	got := NewResolverFor(p, Linux).Resolve(context.Background(), installed("/snap/bin/code"))
	if got.ManagedBy != Snap {
		t.Errorf("ManagedBy = %q, want %q", got.ManagedBy, Snap)
	}
}

func TestResolveWindows(t *testing.T) {
	p := &probe.Fake{HomeDir: `C:\Users\test`}

	got := NewResolverFor(p, Windows).Resolve(context.Background(),
		installed(`C:\Users\test\scoop\apps\git\current\bin\git.exe`))
	if got.ManagedBy != Scoop {
		t.Errorf("ManagedBy = %q, want %q", got.ManagedBy, Scoop)
	}
	if got.PackageID != "git" {
		t.Errorf("PackageID = %q, want git", got.PackageID)
	}

	system := NewResolverFor(p, Windows).Resolve(context.Background(), installed(`C:\Windows\System32\where.exe`))
	if system.ManagedBy != System {
		t.Errorf("ManagedBy = %q, want %q for a Windows system path", system.ManagedBy, System)
	}
}

func TestResolveSkipsItemsWithNothingToGoOn(t *testing.T) {
	r := resolverWith(Darwin, nil)

	// Not found, so there is no install to attribute.
	absent := detector.Item{Name: "Rust", Status: detector.StatusNotFound}
	if got := r.Resolve(context.Background(), absent); got.ManagedBy != "" {
		t.Errorf("ManagedBy = %q, want empty for an absent item", got.ManagedBy)
	}

	// Found by directory rather than binary, with no path recorded.
	noPath := detector.Item{Name: "nvm", Status: detector.StatusInstalled}
	if got := r.Resolve(context.Background(), noPath); got.ManagedBy != "" {
		t.Errorf("ManagedBy = %q, want empty with no path", got.ManagedBy)
	}
}

func TestFormulaFromPath(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/opt/homebrew/Cellar/postgresql@15/15.19/bin/psql", "postgresql@15"},
		{"/usr/local/Cellar/redis/7.2.4/bin/redis-server", "redis"},
		// The Apple silicon prefix is itself /opt/homebrew, so the marker
		// appears twice and only the last one names a formula.
		{"/opt/homebrew/opt/node@20/bin/node", "node@20"},
		{"/usr/bin/git", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := formulaFromPath(tt.path); got != tt.want {
			t.Errorf("formulaFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
