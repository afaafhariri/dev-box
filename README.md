# devenv

A terminal UI for seeing what your development environment is actually made of —
languages, runtimes, servers, AI tooling, and CLI tools — in one place.

The guiding principle is **read first, act on request**. On launch, `devenv`
scans your machine concurrently and shows what it finds. Nothing is installed,
started, or modified unless you ask for it.

```
devenv  Dev Environment Manager  7 of 7 found · 21:09:15 in 280ms

  LANGUAGES
▸ Go              1.25.6      installed  platform darwin/arm64
  Node.js         22.13.1     installed  npm 11.0.0
  Python          3.14.5      installed  pip 25.1.1

  SERVERS
  Docker          29.6.2      running    containers 3 · images 41
  Redis           7.2.4       stopped    port 6379

  AI / ML
  Ollama          0.20.5      running    models 2 · llama3:latest, mistral:7b

  TOOLS
  Git             2.54.0      installed

  ↑/↓ move · r rescan · q quit
```

## Status

Phase 1 of the architecture plan is complete: a concurrent scanner, a
thread-safe store, and a single list view. Phase 2 (tabbed pages, a detail
view, the action engine, disk caching, config file) is not started.

## Requirements

Go 1.25 or newer. macOS and Linux; detection strategies are POSIX-oriented and
Windows is not supported yet.

## Build and run

```sh
go run .                # start the TUI
go build -o devenv .    # build a binary
```

## Usage

| Flag | Effect |
| --- | --- |
| `--json` | Run one scan, print the result as JSON, and exit. No TUI. |
| `--list` | List the registered detectors and exit. |
| `--timeout` | Per-command timeout for a single probe (default `5s`). |
| `--version` | Print the version and exit. |

`--json` is the scripting and debugging path — it needs no terminal, so it also
works over SSH, in CI, and inside a pipe:

```sh
devenv --json | jq '.items[] | select(.status == "running") | .name'
```

### Keys

| Key | Action |
| --- | --- |
| `↑` / `k`, `↓` / `j` | Move the selection |
| `g` / `G` | Jump to top / bottom |
| `r` | Rescan |
| `a` | Show or hide items that were not found |
| `q` / `Esc` / `Ctrl-C` | Quit |

Items that are absent from your machine are hidden by default; the help line
tells you how many are being hidden.

## What it detects

| Category | Detectors |
| --- | --- |
| Languages | Go, Python (with pip), Node.js (with npm) |
| Servers | Docker, Redis |
| AI / ML | Ollama (with its model list) |
| Tools | Git |

Servers report `running` or `stopped` separately from being installed, so a
Docker CLI with a dead daemon reads as `stopped` rather than missing.

## Architecture

Five layers, each depending only on the one below it:

```
TUI         Bubble Tea model, view, key map, Lip Gloss styles
App         orchestrator: builds the prober, registers detectors, wires the engine
Core        detection engine — one goroutine per detector, context-cancellable
Store       thread-safe state, subscription channels
Detectors   one implementation per tool
```

Results stream into the store as each detector returns, so the UI renders the
fast probes without waiting for the slow ones.

### Layout

```
main.go                  entrypoint
cmd/root.go              flag parsing
internal/
  app/                   orchestrator, wires everything together
  probe/                 OS interactions (exec, PATH, files, ports) behind an interface
  detector/              Detector interface, Item type, registry, concurrent engine
  detectors/
    languages/           go, python, node
    servers/             docker, redis
    ai/                  ollama
    tools/               git
    parse/               shared version-string helpers
  store/                 in-memory state with subscriptions
  tui/                   Bubble Tea model, update, view, keys, styles
```

### Three interfaces

`Detector` is implemented once per tool and returns an `Item` describing what
was found. `Prober` is the only way a detector touches the operating system.
`Sink` is what the engine writes results to — the store implements it.

Detectors depend on `Prober` rather than `os/exec` directly, which is what makes
them testable: every detector test runs against an in-memory `probe.Fake` and
never touches the host machine.

## Adding a detector

Add one file under the right `internal/detectors/` sub-package:

```go
type Rust struct{ p probe.Prober }

func NewRust(p probe.Prober) Rust { return Rust{p: p} }

func (d Rust) Name() string                { return "Rust" }
func (d Rust) Category() detector.Category { return detector.CategoryLanguage }

func (d Rust) Detect(ctx context.Context) (detector.Item, error) {
	path, err := d.p.LookPath("rustc")
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	out, err := d.p.Run(ctx, "rustc", "--version")
	if err != nil {
		return detector.Item{}, fmt.Errorf("rustc --version: %w", err)
	}

	return detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Version:  parse.Version(out),
		Path:     path,
		Status:   detector.StatusInstalled,
	}, nil
}
```

Then add it to that package's `All` function. Nothing else needs to change.

Two conventions matter:

- Return `detector.ErrNotInstalled` when the tool is simply absent. Any other
  error means the probe itself failed, and is surfaced to the user as a reason
  rather than a silent absence.
- Gate anything slow behind a cheap check. Both Redis and Ollama dial their
  port before shelling out, because their CLIs block trying to reach a daemon
  that is not there — without the gate, every scan on a machine that lacks them
  would stall.

## Testing

```sh
go test ./...           # full suite
go test -race ./...     # the store and engine are concurrent; run this too
go test -cover ./...
```

Detector tests script a `probe.Fake` with canned command output, so they are
fast, hermetic, and identical on every machine.
