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

func TestCUDAReportsDriverAndToolkit(t *testing.T) {
	// Two things travel under the name CUDA: nvidia-smi reports the driver,
	// nvcc the toolkit. The toolkit release is the number people mean.
	p := &probe.Fake{
		Paths: map[string]string{"nvidia-smi": "/usr/bin/nvidia-smi", "nvcc": "/usr/local/cuda/bin/nvcc"},
		Commands: map[string]probe.FakeResult{
			"nvidia-smi --query-gpu=driver_version,name --format=csv,noheader": {
				Out: "550.54.15, NVIDIA GeForce RTX 4090\n",
			},
			"nvcc --version": {
				Out: "nvcc: NVIDIA (R) Cuda compiler driver\nCuda compilation tools, release 12.4, V12.4.131\n",
			},
		},
	}

	item, err := NewCUDA(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Meta["driver"] != "550.54.15" {
		t.Errorf("Meta[driver] = %q, want 550.54.15", item.Meta["driver"])
	}
	if item.Meta["gpu"] != "NVIDIA GeForce RTX 4090" {
		t.Errorf("Meta[gpu] = %q", item.Meta["gpu"])
	}
	if item.Meta["toolkit"] == "" {
		t.Error("Meta[toolkit] not set with nvcc installed")
	}
}

func TestCUDAWithoutTheToolkit(t *testing.T) {
	// A driver with no toolkit is normal on a machine that only runs models.
	p := &probe.Fake{
		Paths: map[string]string{"nvidia-smi": "/usr/bin/nvidia-smi"},
		Commands: map[string]probe.FakeResult{
			"nvidia-smi --query-gpu=driver_version,name --format=csv,noheader": {
				Out: "550.54.15, NVIDIA GeForce RTX 4090\n",
			},
		},
	}

	item, err := NewCUDA(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "550.54.15" {
		t.Errorf("Version = %q, want the driver version when there is no toolkit", item.Version)
	}
	if _, ok := item.Meta["toolkit"]; ok {
		t.Error("Meta[toolkit] set with no nvcc installed")
	}
}

func TestCUDANotPresent(t *testing.T) {
	// The ordinary case on any machine without an NVIDIA GPU.
	if _, err := NewCUDA(&probe.Fake{}).Detect(context.Background()); !errors.Is(err, detector.ErrNotInstalled) {
		t.Errorf("error = %v, want ErrNotInstalled", err)
	}
}

func TestGPUName(t *testing.T) {
	tests := []struct{ row, want string }{
		{"550.54.15, NVIDIA GeForce RTX 4090", "NVIDIA GeForce RTX 4090"},
		{"550.54.15", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := gpuName(tt.row); got != tt.want {
			t.Errorf("gpuName(%q) = %q, want %q", tt.row, got, tt.want)
		}
	}
}

func TestPyPackageDetect(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"pip3": "/opt/homebrew/bin/pip3"},
		Commands: map[string]probe.FakeResult{
			"pip3 show torch": {Out: "Name: torch\nVersion: 2.3.1\nLocation: /opt/homebrew/lib/python3.12/site-packages\n"},
		},
	}

	item, err := NewPyPackage(p, "PyTorch", "torch").Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "2.3.1" {
		t.Errorf("Version = %q, want 2.3.1", item.Version)
	}
	if item.Path != "/opt/homebrew/lib/python3.12/site-packages" {
		t.Errorf("Path = %q", item.Path)
	}
	if item.Meta["dist"] != "torch" {
		t.Errorf("Meta[dist] = %q, want torch", item.Meta["dist"])
	}
	if item.Category != detector.CategoryAI {
		t.Errorf("Category = %q, want ai", item.Category)
	}
}

func TestPyPackageNotInstalled(t *testing.T) {
	// pip show exits non-zero for a package that is absent, which is the
	// ordinary case here rather than a failure.
	p := &probe.Fake{Paths: map[string]string{"pip3": "/opt/homebrew/bin/pip3"}}

	_, err := NewPyPackage(p, "PyTorch", "torch").Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Errorf("error = %v, want ErrNotInstalled", err)
	}
}

func TestPyPackageWithoutPip(t *testing.T) {
	_, err := NewPyPackage(&probe.Fake{}, "PyTorch", "torch").Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Errorf("error = %v, want ErrNotInstalled", err)
	}
}

func TestShowField(t *testing.T) {
	const out = "Name: torch\nVersion: 2.3.1\nSummary: Tensors and neural networks\n"

	if got := showField(out, "Version"); got != "2.3.1" {
		t.Errorf("showField(Version) = %q", got)
	}
	if got := showField(out, "Missing"); got != "" {
		t.Errorf("showField(Missing) = %q, want empty", got)
	}
}

func TestAllAIDetectorsAreInTheAICategory(t *testing.T) {
	for _, d := range All(&probe.Fake{}) {
		if d.Category() != detector.CategoryAI {
			t.Errorf("%s has category %q, want ai", d.Name(), d.Category())
		}
	}
}
