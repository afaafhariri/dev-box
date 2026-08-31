# devenv

A terminal UI for seeing what your development environment is actually made of —
languages, runtimes, servers, AI tooling, and CLI tools — in one place.

The guiding principle is **read first, act on request**. On launch, `devenv`
scans your machine concurrently and shows what it finds. Nothing is installed,
started, or modified unless you ask for it.

```
devenv  Dev Environment Manager  10 of 11 found · 21:09:15 in 280ms
  1 Dashboard · 2 Languages · 3 Servers · 4 AI · 5 Tools · 6 Managers

  LANGUAGES
▸ Go              1.25.6      installed  platform darwin/arm64
  Node.js         22.13.1     installed  npm 11.0.0
  Python          3.14.5      installed  pip 25.1.1

  SERVERS
  Docker          29.6.2      running    containers 3 · images 41
  PostgreSQL      15.19       running    port 5432
  Redis           7.2.4       stopped    port 6379

  AI / ML
  Ollama          0.20.5      running    models 2 · llama3:latest, mistral:7b

  TOOLS
  Git             2.54.0      installed  /usr/bin/git
  Homebrew        6.0.19      installed  /opt/homebrew/bin/brew

  VERSION MANAGERS
  nvm             —           installed  /Users/you/.nvm

  ↑/↓ move · tab page · enter detail · r rescan · a show 1 missing · q quit
```

`Enter` opens the detail view, where the actions that apply right now are one
keypress away and their output streams in as they run:

```
devenv  Dev Environment Manager  10 of 11 found · 21:09:15 in 280ms
  1 Dashboard · 2 Languages · 3 Servers · 4 AI · 5 Tools · 6 Managers

  Redis  stopped

  category    Servers
  version     7.2.4
  path        /opt/homebrew/bin/redis-server
  detected    21:09:15
  port        6379

  ▸ Start    Upgrade

  OUTPUT
  $ brew services start redis
  ==> Successfully started `redis` (label: homebrew.mxcl.redis)

  ←/→ action · enter run · esc back · q quit
```

## Status

Phases 1 and 2 of the architecture plan are complete: a concurrent scanner over
36 detectors, a thread-safe store with a disk cache, tabbed pages with a detail
view, an action engine, and a TOML config file. Phase 3 (fuzzy search,
update-available detection, background refresh, Markdown export) is not
started.

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
| `--timeout` | Per-command timeout for a single probe. Overrides the config file. |
| `--config` | Config file path (default `~/.config/devenv/config.toml`). |
| `--no-cache` | Ignore the disk cache for this run. |
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
| `Tab` / `Shift-Tab` | Next / previous page |
| `1` – `6` | Jump straight to a page |
| `Enter` | Open the detail view; in it, run the selected action |
| `Esc` | Leave the detail view |
| `←` / `→` | Choose an action in the detail view |
| `g` / `G` | Jump to top / bottom |
| `r` | Rescan |
| `a` | Show or hide items that were not found |
| `q` / `Ctrl-C` | Quit |

Items that are absent from your machine are hidden by default; the help line
tells you how many are being hidden.

### Pages

Six tabs: **Dashboard** (everything, grouped by category), then one page each
for **Languages**, **Servers**, **AI**, **Tools**, and **Managers**.

`Enter` on any item opens its detail view — every field, the full metadata, and
the actions that apply to it right now.

### Actions

Actions are always explicit. Nothing here runs as part of a scan; the app reads
your machine on its own and changes it only when you ask.

| Action | Offered for |
| --- | --- |
| Start / Stop / Restart | A Homebrew-managed service, matching its current state |
| Launch app | Docker and Ollama on macOS, when stopped |
| Upgrade | Anything installed under a Homebrew prefix |
| Uninstall | Anything installed under a Homebrew prefix whose formula can be identified |

Output streams into a pane in the detail view as the action runs, and a rescan
follows automatically so the new state is confirmed rather than assumed.

Three deliberate limits:

**Homebrew only.** Every action is gated on a Homebrew install path, and none
are registered at all when `brew` is absent. Upgrading a pyenv-managed Python
or uninstalling an nvm-managed Node through `brew` would fight the tool that
really manages it.

**The formula is resolved, never guessed.** A binary on `PATH` is a symlink into
the Cellar, and that is what names the owning formula:

```
/opt/homebrew/bin/psql -> /opt/homebrew/Cellar/postgresql@15/15.19/bin/psql
```

So PostgreSQL resolves to `postgresql@15`, not `psql` or `postgresql` — both of
which would fail, and one of which could act on a different package. If no
formula can be established, the action is not offered.

**Uninstall asks first.** `Enter` runs the selected action and `←`/`→` move
between them, which is far too easy a way to remove something. Uninstall instead
puts the exact command up for confirmation, and needs a second `Enter`:

```
    Stop    Restart    Upgrade  ▸ Uninstall

  Run 'brew uninstall postgresql@15'? Stored data under the Homebrew prefix is
  left in place. Press enter again to confirm, esc to cancel.

  enter confirm · esc cancel · q quit
```

`Esc` cancels without leaving the page, and moving to another action clears the
pending confirmation so a `yes` can never land on something you did not read.
Uninstall runs without `--force` or `--ignore-dependencies`: if another formula
depends on this one, Homebrew refuses and says so, which is the right outcome.

## Configuration

Everything is optional. With no config file you get the defaults.

```toml
# ~/.config/devenv/config.toml

[scan]
timeout   = "5s"    # per-command timeout for a single probe
cache_ttl = "5m"    # how long a cached scan is considered fresh
no_cache  = false   # set true to disable the disk cache entirely

[detectors]
disabled = ["Caddy", "Gradle"]   # names from `devenv --list`, case-insensitive
```

A malformed config is reported on stderr and then ignored, so a typo cannot
lock you out of your own tool.

### Cache

The last scan is written to `~/.config/devenv/cache.json`, so launching again
renders instantly from that snapshot while a fresh scan runs behind it. The
header says `cached 3m ago · rescanning` while that is happening — showing
stale data without saying so is the difference between stale and wrong.

Both paths honour `XDG_CONFIG_HOME`. The cache is written atomically, and a
corrupt or older-format file is ignored rather than crashed on.

## What it detects

36 detectors across five categories:

| Category | Detectors |
| --- | --- |
| Languages | Go, Python (with pip), Node.js (with npm), Java, Rust (with toolchain), Ruby |
| Servers | Docker, Redis, PostgreSQL, MySQL, MongoDB, Nginx, Caddy |
| AI / ML | Ollama (with its model list), CUDA, PyTorch, TensorFlow, Hugging Face |
| Tools | Git, GitHub CLI, kubectl, Helm, Make, CMake, Cargo, Homebrew, pnpm, Yarn, Maven, Gradle, npm globals |
| Managers | nvm, pyenv, sdkman, mise, rbenv |

Servers report `running` or `stopped` separately from being installed, so a
Docker CLI with a dead daemon reads as `stopped` rather than missing.

Nginx and Caddy are the exception: they report only what is installed. Port 80
is shared by anything that wants it, so a dial there would claim "running" for
whatever else happened to be listening.

## Architecture

Five layers, each depending only on the one below it:

```
TUI         Bubble Tea root model, page sub-models, key map, Lip Gloss theme
App         orchestrator: loads config, registers detectors, wires engine to store and cache
Core        detection engine and action engine — context-cancellable throughout
Store       thread-safe state, subscription channels, JSON disk cache
Detectors   one implementation per tool
```

Results stream into the store as each detector returns, so the UI renders the
fast probes without waiting for the slow ones.

### Layout

```
main.go                  entrypoint
cmd/root.go              flag parsing
config/                  TOML config loading
internal/
  app/                   orchestrator, wires everything together
  probe/                 OS interactions (exec, PATH, files, ports) behind an interface
  detector/              Detector interface, Item type, registry, concurrent engine
  action/                Action interface, executor, the concrete actions
  detectors/
    simple/              reusable detector shapes: Tool, Service, Manager
    languages/           go, python, node, java, rust, ruby
    servers/             docker, redis, postgres, mysql, mongodb, nginx
    ai/                  ollama, cuda, pip-installed ML packages
    tools/               git and the CLI/package-manager catalog
    managers/            nvm, pyenv, sdkman, mise, rbenv
    parse/               shared version-string helpers
  store/                 in-memory state, subscriptions, disk cache
  tui/                   root model, update, view, keys
    pages/               list and detail sub-models
    theme/               colour tokens and Lip Gloss styles
```

### Four interfaces

`Detector` is implemented once per tool and returns an `Item` describing what
was found. `Prober` is the only way a detector touches the operating system.
`Sink` is what the engine writes results to — the store implements it. `Action`
is one operation a user can trigger, returning a channel of output lines.

Detectors depend on `Prober` rather than `os/exec` directly, which is what makes
them testable: every detector test runs against an in-memory `probe.Fake` and
never touches the host machine.

## Adding a detector

Most tools are "run a binary, parse a version", which `internal/detectors/simple`
already covers — those are a few lines:

```go
// NewGH detects the GitHub CLI.
func NewGH(p probe.Prober) detector.Detector {
	return simple.Tool{
		ItemName: "GitHub CLI",
		Cat:      detector.CategoryTool,
		Bins:     []string{"gh"},
		P:        p,
	}
}
```

`simple.Service` adds a port liveness check on top of that, and
`simple.Manager` finds version managers by their install directory, since most
are shell functions with no binary at all.

For anything with genuinely bespoke logic, implement `detector.Detector`
directly. Add one file under the right `internal/detectors/` sub-package:

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
