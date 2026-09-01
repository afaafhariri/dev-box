package platform

import (
	"strings"
	"testing"
)

func TestCommandFillsTheTemplate(t *testing.T) {
	got := Command([]string{"brew", "uninstall", "{}"}, "postgresql@15")
	want := []string{"brew", "uninstall", "postgresql@15"}

	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("Command() = %v, want %v", got, want)
	}
}

func TestServiceCommandFillsPackageAndVerb(t *testing.T) {
	got := ServiceCommand([]string{"systemctl", "--user", "{verb}", "{}"}, "redis", "start")

	if strings.Join(got, " ") != "systemctl --user start redis" {
		t.Errorf("ServiceCommand() = %v", got)
	}
}

func TestElevationRequiringManagersAreHintOnly(t *testing.T) {
	// devenv runs actions with no stdin, so a sudo password prompt would hang
	// with no way to answer it. These must advise, never execute.
	for _, name := range []string{APT, DNF, Pacman, Snap, WinGet, Chocolatey} {
		m, ok := Lookup(name)
		if !ok {
			t.Fatalf("%s is not a known manager", name)
		}
		if m.CanUninstall() || m.CanUpgrade() {
			t.Errorf("%s would be executed by devenv despite needing elevation", name)
		}
		if m.Hint == "" {
			t.Errorf("%s offers no guidance in place of acting", name)
		}
	}
}

func TestPerUserManagersCanBeDriven(t *testing.T) {
	// These install into the user's own tree, so no elevation is involved.
	for _, name := range []string{Homebrew, Scoop} {
		m, _ := Lookup(name)
		if !m.CanUninstall() || !m.CanUpgrade() {
			t.Errorf("%s should be drivable without elevation", name)
		}
	}
}

func TestSystemIsProtected(t *testing.T) {
	m, _ := Lookup(System)

	if !m.Protected {
		t.Fatal("system installs are not marked protected")
	}
	if m.CanUninstall() || m.CanUpgrade() || m.CanService() {
		t.Error("a protected manager still permits actions")
	}
}

func TestVersionManagersAdviseTheirOwnCommand(t *testing.T) {
	tests := []struct{ manager, pkg, want string }{
		{NVM, "22.13.1", "nvm uninstall 22.13.1"},
		{Pyenv, "3.12.4", "pyenv uninstall 3.12.4"},
		{Rbenv, "3.3.1", "rbenv uninstall 3.3.1"},
		{Mise, "node@22", "mise uninstall node@22"},
		{Rustup, "stable", "rustup toolchain uninstall stable"},
	}

	for _, tt := range tests {
		m, ok := Lookup(tt.manager)
		if !ok {
			t.Fatalf("%s is not a known manager", tt.manager)
		}
		if got := m.Advice(tt.pkg); !strings.Contains(got, tt.want) {
			t.Errorf("%s advice = %q, want it to mention %q", tt.manager, got, tt.want)
		}
	}
}

func TestServiceTemplatePerOS(t *testing.T) {
	brew, _ := Lookup(Homebrew)
	apt, _ := Lookup(APT)

	if got := ServiceTemplate(Darwin, brew); strings.Join(got, " ") != "brew services {verb} {}" {
		t.Errorf("darwin template = %v", got)
	}
	// A distribution package has no template of its own, so systemd drives it.
	if got := ServiceTemplate(Linux, apt); !strings.Contains(strings.Join(got, " "), "systemctl") {
		t.Errorf("linux template = %v, want systemctl", got)
	}
	// Homebrew on Linux still uses brew services.
	if got := ServiceTemplate(Linux, brew); !strings.Contains(strings.Join(got, " "), "brew") {
		t.Errorf("linuxbrew template = %v, want brew services", got)
	}
	// Windows service control needs elevation, so it is not attempted.
	if got := ServiceTemplate(Windows, brew); len(got) != 0 {
		t.Errorf("windows template = %v, want none", got)
	}
}

func TestUnknownManagerIsReported(t *testing.T) {
	if _, ok := Lookup("Nonesuch"); ok {
		t.Error("Lookup() accepted an unknown manager")
	}
}

func TestAdviceIsDroppedWithoutAPackageID(t *testing.T) {
	// nvm itself has no version, so "run 'nvm uninstall '" would be worse
	// than saying nothing.
	m, _ := Lookup(NVM)

	if got := m.Advice(""); got != "" {
		t.Errorf("Advice(\"\") = %q, want empty", got)
	}
	if got := m.Advice("22.13.1"); got == "" {
		t.Error("Advice() dropped a hint that had everything it needed")
	}
}

func TestProtectedAdviceSurvivesWithoutAPackageID(t *testing.T) {
	// This hint names no package, so it stands on its own.
	m, _ := Lookup(System)
	if got := m.Advice(""); got == "" {
		t.Error("the system warning was dropped")
	}
}
