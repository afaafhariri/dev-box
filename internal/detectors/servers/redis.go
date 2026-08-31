package servers

import (
	"context"
	"fmt"
	"strconv"

	"devenv/internal/detector"
	"devenv/internal/detectors/parse"
	"devenv/internal/probe"
)

// redisPort is the default Redis port. Phase 2 makes this configurable.
const redisPort = 6379

// Redis detects a Redis installation and whether a server is answering.
type Redis struct{ p probe.Prober }

// NewRedis returns a Redis detector reading through p.
func NewRedis(p probe.Prober) Redis { return Redis{p: p} }

func (d Redis) Name() string                { return "Redis" }
func (d Redis) Category() detector.Category { return detector.CategoryServer }

func (d Redis) Detect(ctx context.Context) (detector.Item, error) {
	// redis-server is the install; redis-cli alone still means Redis is usable
	// here, typically against a remote or containerised server.
	bin, path, err := firstBinary(d.p, []string{"redis-server", "redis-cli"})
	if err != nil {
		return detector.Item{}, err
	}

	out, err := d.p.Run(ctx, bin, "--version")
	if err != nil {
		return detector.Item{}, fmt.Errorf("%s --version: %w", bin, err)
	}

	item := detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  parse.Version(out), // "Redis server v=7.2.4 sha=..."
		Path:     path,
		Status:   detector.StatusStopped,
	}
	item = item.WithMeta("port", strconv.Itoa(redisPort))

	// A TCP dial is far cheaper than shelling out, so it gates the ping.
	if !d.p.PortOpen(ctx, "127.0.0.1", redisPort) {
		return item, nil
	}
	item.Status = detector.StatusRunning

	// Something is listening; confirm it actually speaks Redis.
	if _, err := d.p.LookPath("redis-cli"); err == nil {
		if pong, err := d.p.Run(ctx, "redis-cli", "-p", strconv.Itoa(redisPort), "ping"); err == nil {
			item = item.WithMeta("ping", parse.FirstLine(pong))
		}
	}
	return item, nil
}
