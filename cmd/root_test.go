package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run executes the CLI with args, returning its exit code and streams.
func run(t *testing.T, args ...string) (code int, out, errOut string) {
	t.Helper()

	// Keep every run inside its own config directory, so tests never read or
	// write the developer's real cache.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var stdout, stderr strings.Builder
	code = Execute(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestVersionFlag(t *testing.T) {
	code, out, _ := run(t, "--version")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(out, "devenv ") {
		t.Errorf("output = %q, want the version", out)
	}
}

func TestListFlag(t *testing.T) {
	code, out, _ := run(t, "--list")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	names := strings.Fields(out)
	if len(names) < 30 {
		t.Errorf("listed %d detectors, want the full catalog", len(names))
	}
	if !strings.Contains(out, "Docker") {
		t.Error("the catalog is missing Docker")
	}
}

func TestJSONFlagProducesValidJSON(t *testing.T) {
	code, out, _ := run(t, "--json")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	var payload struct {
		ScannedAt string `json:"scannedAt"`
		Items     []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Error("no items in the scan")
	}
}

func TestMarkdownFormat(t *testing.T) {
	code, out, _ := run(t, "--format", "markdown")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(out, "# Development environment") {
		t.Errorf("output does not look like Markdown:\n%s", firstLines(out, 3))
	}
}

func TestUnknownFormatIsRejected(t *testing.T) {
	code, _, errOut := run(t, "--format", "yaml")

	if code == 0 {
		t.Error("exit code = 0 for an unknown format")
	}
	if !strings.Contains(errOut, "yaml") {
		t.Errorf("stderr = %q, want the bad format named", errOut)
	}
}

func TestUnknownFlagIsRejected(t *testing.T) {
	if code, _, _ := run(t, "--nonsense"); code != 2 {
		t.Errorf("exit code = %d, want 2 for a usage error", code)
	}
}

func TestDisabledDetectorsAreNotRegistered(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	config := filepath.Join(dir, "config.toml")
	writeFile(t, config, "[detectors]\ndisabled = [\"Docker\", \"Redis\"]\n")

	var out, errOut strings.Builder
	Execute(context.Background(), []string{"--config", config, "--list"}, &out, &errOut)

	if strings.Contains(out.String(), "Docker") {
		t.Error("a disabled detector was still registered")
	}
	if !strings.Contains(out.String(), "Git") {
		t.Error("disabling two detectors removed the others too")
	}
}

func TestMalformedConfigIsReportedButNotFatal(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.toml")
	writeFile(t, config, "[scan\n")

	var out, errOut strings.Builder
	code := Execute(context.Background(), []string{"--config", config, "--list"}, &out, &errOut)

	// A typo must not lock anyone out of their own tool.
	if code != 0 {
		t.Errorf("exit code = %d, want the run to continue on defaults", code)
	}
	if errOut.Len() == 0 {
		t.Error("the broken config was not reported")
	}
	if !strings.Contains(out.String(), "Git") {
		t.Error("no detectors registered after a bad config")
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHelpExitsSuccessfully(t *testing.T) {
	// Asking for help is a deliberate, successful invocation, not a usage
	// error — scripts and shells read the exit code.
	for _, flag := range []string{"-h", "--help"} {
		code, _, errOut := run(t, flag)
		if code != 0 {
			t.Errorf("%s exit code = %d, want 0", flag, code)
		}
		if !strings.Contains(errOut, "Usage:") {
			t.Errorf("%s printed no usage text", flag)
		}
	}
}
