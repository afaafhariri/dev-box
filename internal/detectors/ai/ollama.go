// Package ai detects local AI/ML infrastructure.
package ai

import (
	"context"
	"strconv"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// ollamaPort is the port the Ollama daemon serves on.
const ollamaPort = 11434

// maxListedModels caps how many model names go into Meta, so a machine with
// thirty models does not produce an unreadable row.
const maxListedModels = 5

// Ollama detects the Ollama runtime and the models it holds.
type Ollama struct{ p probe.Prober }

// NewOllama returns an Ollama detector reading through p.
func NewOllama(p probe.Prober) Ollama { return Ollama{p: p} }

func (d Ollama) Name() string                { return "Ollama" }
func (d Ollama) Category() detector.Category { return detector.CategoryAI }

func (d Ollama) Detect(ctx context.Context) (detector.Item, error) {
	path, err := d.p.LookPath("ollama")
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	// "ollama --version" prints a warning line and exits non-zero when the
	// daemon is down, but still reports the client version — so the output is
	// worth reading either way.
	out, _ := d.p.Run(ctx, "ollama", "--version")
	version := parse.Version(out)

	item := detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  version,
		Path:     path,
		Status:   detector.StatusStopped,
	}
	item = item.WithMeta("port", strconv.Itoa(ollamaPort))

	// "ollama list" blocks trying to reach the daemon, so only run it once a
	// dial has shown something is listening.
	if !d.p.PortOpen(ctx, "127.0.0.1", ollamaPort) {
		return item, nil
	}
	item.Status = detector.StatusRunning

	list, err := d.p.Run(ctx, "ollama", "list")
	if err != nil {
		return item, nil
	}

	models := parseModels(list)
	item = item.WithMeta("models", strconv.Itoa(len(models)))
	if len(models) > 0 {
		shown := models
		if len(shown) > maxListedModels {
			shown = shown[:maxListedModels]
		}
		item = item.WithMeta("modelList", strings.Join(shown, ", "))
	}
	return item, nil
}

// parseModels reads the NAME column of "ollama list" output:
//
//	NAME               ID            SIZE      MODIFIED
//	llama3:latest      365c0bd3c000  4.7 GB    2 days ago
func parseModels(out string) []string {
	var models []string
	for i, line := range parse.Lines(out) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if i == 0 && strings.EqualFold(fields[0], "NAME") {
			continue
		}
		models = append(models, fields[0])
	}
	return models
}
