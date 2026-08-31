// Package servers detects long-running services, and whether they are up.
package servers

import (
	"context"
	"fmt"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// infoFormat pulls the three facts worth showing out of a single docker info
// call. Separate calls would each pay the daemon round trip.
const infoFormat = "{{.ServerVersion}}|{{.ContainersRunning}}|{{.Images}}"

// Docker detects the Docker CLI and whether its daemon is reachable.
type Docker struct{ p probe.Prober }

// NewDocker returns a Docker detector reading through p.
func NewDocker(p probe.Prober) Docker { return Docker{p: p} }

func (d Docker) Name() string                { return "Docker" }
func (d Docker) Category() detector.Category { return detector.CategoryServer }

func (d Docker) Detect(ctx context.Context) (detector.Item, error) {
	path, err := d.p.LookPath("docker")
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	out, err := d.p.Run(ctx, "docker", "--version")
	if err != nil {
		return detector.Item{}, fmt.Errorf("docker --version: %w", err)
	}

	item := detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  parse.Version(out),
		Path:     path,
		// The CLI being installed says nothing about the daemon; assume down
		// until docker info proves otherwise.
		Status: detector.StatusStopped,
	}

	info, err := d.p.Run(ctx, "docker", "info", "--format", infoFormat)
	if err != nil {
		// The overwhelmingly common cause is "daemon not running", which is a
		// state to display, not an error to report.
		return item, nil
	}

	item.Status = detector.StatusRunning
	server, running, images := splitInfo(parse.FirstLine(info))
	item = item.WithMeta("server", server)
	item = item.WithMeta("containers", running)
	item = item.WithMeta("images", images)
	return item, nil
}

func splitInfo(line string) (server, running, images string) {
	parts := strings.Split(line, "|")
	get := func(i int) string {
		if i < len(parts) {
			return strings.TrimSpace(parts[i])
		}
		return ""
	}
	return get(0), get(1), get(2)
}
