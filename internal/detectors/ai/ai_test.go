package ai

import (
	"context"
	"errors"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

const listOutput = `NAME                    ID              SIZE      MODIFIED
llama3:latest           365c0bd3c000    4.7 GB    2 days ago
mistral:7b              61e88e884507    4.1 GB    3 weeks ago
`

func TestOllamaRunningWithModels(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"ollama": "/usr/local/bin/ollama"},
		Commands: map[string]probe.FakeResult{
			"ollama --version": {Out: "ollama version is 0.3.9\n"},
			"ollama list":      {Out: listOutput},
		},
		Ports: map[string]bool{"127.0.0.1:11434": true},
	}

	item, err := NewOllama(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Status != detector.StatusRunning {
		t.Errorf("Status = %q, want running", item.Status)
	}
	if item.Version != "0.3.9" {
		t.Errorf("Version = %q, want 0.3.9", item.Version)
	}
	if got := item.Meta["models"]; got != "2" {
		t.Errorf("Meta[models] = %q, want 2", got)
	}
	if got := item.Meta["modelList"]; got != "llama3:latest, mistral:7b" {
		t.Errorf("Meta[modelList] = %q", got)
	}
}

func TestOllamaInstalledDaemonDown(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"ollama": "/usr/local/bin/ollama"},
		Commands: map[string]probe.FakeResult{
			// With the daemon down the CLI warns and exits non-zero, but still
			// prints its own version.
			"ollama --version": {
				Out: "Warning: could not connect to a running Ollama instance\nollama version is 0.3.9\n",
				Err: errors.New("exit status 1"),
			},
		},
	}

	item, err := NewOllama(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Status != detector.StatusStopped {
		t.Errorf("Status = %q, want stopped", item.Status)
	}
	if item.Version != "0.3.9" {
		t.Errorf("Version = %q, want 0.3.9 despite the warning line", item.Version)
	}
	// "ollama list" blocks on the daemon, so it must not run when the port is
	// closed.
	if p.Ran("ollama", "list") {
		t.Error("ran 'ollama list' with the daemon down")
	}
}

func TestOllamaNotInstalled(t *testing.T) {
	_, err := NewOllama(&probe.Fake{}).Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func TestParseModelsSkipsHeader(t *testing.T) {
	got := parseModels(listOutput)
	want := []string{"llama3:latest", "mistral:7b"}

	if len(got) != len(want) {
		t.Fatalf("parseModels() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("model %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseModelsEmpty(t *testing.T) {
	if got := parseModels("NAME  ID  SIZE  MODIFIED\n"); len(got) != 0 {
		t.Errorf("parseModels() = %v, want empty", got)
	}
}
