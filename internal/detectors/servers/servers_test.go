package servers

import (
	"context"
	"errors"
	"testing"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

func dockerFake() *probe.Fake {
	return &probe.Fake{
		Paths: map[string]string{"docker": "/usr/local/bin/docker"},
		Commands: map[string]probe.FakeResult{
			"docker --version":                   {Out: "Docker version 27.0.3, build 7d4bcd8\n"},
			"docker info --format " + infoFormat: {Out: "27.0.3|3|41\n"},
		},
	}
}

func TestDockerRunning(t *testing.T) {
	item, err := NewDocker(dockerFake()).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Status != detector.StatusRunning {
		t.Errorf("Status = %q, want running", item.Status)
	}
	if item.Version != "27.0.3" {
		t.Errorf("Version = %q, want 27.0.3", item.Version)
	}
	if got := item.Meta["containers"]; got != "3" {
		t.Errorf("Meta[containers] = %q, want 3", got)
	}
	if got := item.Meta["images"]; got != "41" {
		t.Errorf("Meta[images] = %q, want 41", got)
	}
}

func TestDockerInstalledButDaemonDown(t *testing.T) {
	p := dockerFake()
	// The CLI is present but the daemon is not answering.
	delete(p.Commands, "docker info --format "+infoFormat)

	item, err := NewDocker(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v, want a stopped item and no error", err)
	}
	if item.Status != detector.StatusStopped {
		t.Errorf("Status = %q, want stopped", item.Status)
	}
	if item.Version != "27.0.3" {
		t.Errorf("Version = %q — version should survive a down daemon", item.Version)
	}
}

func TestDockerNotInstalled(t *testing.T) {
	_, err := NewDocker(&probe.Fake{}).Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}

func TestRedisRunning(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{
			"redis-server": "/opt/homebrew/bin/redis-server",
			"redis-cli":    "/opt/homebrew/bin/redis-cli",
		},
		Commands: map[string]probe.FakeResult{
			"redis-server --version": {Out: "Redis server v=7.2.4 sha=00000000:0 malloc=libc bits=64 build=abc\n"},
			"redis-cli -p 6379 ping": {Out: "PONG\n"},
		},
		Ports: map[string]bool{"127.0.0.1:6379": true},
	}

	item, err := NewRedis(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Status != detector.StatusRunning {
		t.Errorf("Status = %q, want running", item.Status)
	}
	if item.Version != "7.2.4" {
		t.Errorf("Version = %q, want 7.2.4", item.Version)
	}
	if got := item.Meta["ping"]; got != "PONG" {
		t.Errorf("Meta[ping] = %q, want PONG", got)
	}
}

func TestRedisInstalledNotRunning(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"redis-server": "/opt/homebrew/bin/redis-server"},
		Commands: map[string]probe.FakeResult{
			"redis-server --version": {Out: "Redis server v=7.2.4 sha=00000000:0\n"},
		},
	}

	item, err := NewRedis(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Status != detector.StatusStopped {
		t.Errorf("Status = %q, want stopped", item.Status)
	}
	// The ping must not run when nothing is listening: it would block on a
	// connect timeout for every scan on every machine without Redis.
	if p.Ran("redis-cli", "-p", "6379", "ping") {
		t.Error("pinged redis with the port closed")
	}
}

func TestRedisCLIOnly(t *testing.T) {
	p := &probe.Fake{
		Paths: map[string]string{"redis-cli": "/opt/homebrew/bin/redis-cli"},
		Commands: map[string]probe.FakeResult{
			"redis-cli --version": {Out: "redis-cli 7.2.4\n"},
		},
	}

	item, err := NewRedis(p).Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if item.Version != "7.2.4" {
		t.Errorf("Version = %q, want 7.2.4", item.Version)
	}
}

func TestRedisNotInstalled(t *testing.T) {
	_, err := NewRedis(&probe.Fake{}).Detect(context.Background())
	if !errors.Is(err, detector.ErrNotInstalled) {
		t.Fatalf("error = %v, want ErrNotInstalled", err)
	}
}
