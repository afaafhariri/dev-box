package ai

import (
	"context"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/detectors/simple"
	"devenv/internal/probe"
)

// NewCUDA detects an NVIDIA GPU stack.
//
// Two things travel under the name "CUDA": the driver (nvidia-smi) and the
// toolkit (nvcc). The driver is what decides whether anything can run, so it
// is the primary signal, with the toolkit version reported alongside it.
func NewCUDA(p probe.Prober) detector.Detector {
	return simple.Tool{
		ItemName: "CUDA",
		Cat:      detector.CategoryAI,
		Bins:     []string{"nvidia-smi"},
		Args:     []string{"--query-gpu=driver_version,name", "--format=csv,noheader"},
		P:        p,
		Enrich:   cudaDetail,
	}
}

// cudaDetail splits the nvidia-smi CSV row into a driver version and a GPU
// name, then adds the toolkit version if nvcc is installed.
func cudaDetail(ctx context.Context, p probe.Prober, item detector.Item, out string) detector.Item {
	// The version parsed from the CSV row is the driver version; label it as
	// such rather than leaving it to be read as a CUDA release number.
	if driver := item.Version; driver != "" {
		item = item.WithMeta("driver", driver)
	}

	item = item.WithMeta("gpu", gpuName(out))

	if _, err := p.LookPath("nvcc"); err == nil {
		if out, err := p.Run(ctx, "nvcc", "--version"); err == nil {
			// "Cuda compilation tools, release 12.4, V12.4.131"
			item.Version = parse.Version(out)
			item = item.WithMeta("toolkit", item.Version)
		}
	}
	return item
}

// gpuName pulls the GPU model out of an "nvidia-smi --format=csv,noheader" row
// such as "550.54.15, NVIDIA GeForce RTX 4090".
func gpuName(row string) string {
	_, name, found := strings.Cut(parse.FirstLine(row), ",")
	if !found {
		return ""
	}
	return strings.TrimSpace(name)
}
