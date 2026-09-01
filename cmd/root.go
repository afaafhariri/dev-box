// Package cmd parses the command line and starts the app.
//
// Phase 1 has a single command and three flags, so it uses the standard
// library. Cobra (per the architecture doc) lands in Phase 2, when there are
// subcommands to justify the dependency.
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"devenv/internal/app"
)

// Version is the build version, overridable at link time:
//
//	go build -ldflags "-X devenv/cmd.Version=v0.1.0"
var Version = "dev"

// Execute parses args and runs the app, returning a process exit code.
func Execute(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("devenv", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.Usage = func() {
		fmt.Fprint(errOut, usage)
		fs.PrintDefaults()
	}

	var (
		asJSON      = fs.Bool("json", false, "print one scan as JSON and exit, instead of starting the TUI")
		format      = fs.String("format", "", "print one scan in this format and exit: json or markdown")
		refresh     = fs.Duration("refresh", 0, "rescan automatically on this interval while the TUI is open")
		listOnly    = fs.Bool("list", false, "list the registered detectors and exit")
		showVersion = fs.Bool("version", false, "print the devenv version and exit")
		timeout     = fs.Duration("timeout", 0, "per-command timeout for a single detector probe (overrides the config file)")
		configPath  = fs.String("config", "", "path to the config file (default ~/.config/devenv/config.toml)")
		noCache     = fs.Bool("no-cache", false, "ignore the disk cache for this run")
	)

	if err := fs.Parse(args); err != nil {
		// -h and --help are deliberate, successful invocations: the usage
		// text is what was asked for, not a complaint about it.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Fprintln(out, "devenv", Version)
		return 0
	}

	opts := app.DefaultOptions()
	opts.Format = *format
	if *asJSON && opts.Format == "" {
		opts.Format = "json"
	}
	opts.Refresh = *refresh
	opts.Timeout = *timeout
	opts.ConfigPath = *configPath
	opts.NoCache = *noCache
	opts.Out = out

	// A broken config file is reported, but the app still runs on defaults: a
	// typo should not lock anyone out of their own tool.
	a, err := app.New(opts)
	if err != nil {
		fmt.Fprintln(errOut, "devenv:", err)
	}

	if *listOnly {
		for _, name := range a.Detectors() {
			fmt.Fprintln(out, name)
		}
		return 0
	}

	if err := a.Run(ctx); err != nil {
		fmt.Fprintln(errOut, "devenv:", err)
		return 1
	}
	return 0
}

const usage = `devenv — see what your development environment is made of.

Usage:
  devenv [flags]

Flags:
`

// Main is the entrypoint helper used by main.go.
func Main() {
	os.Exit(Execute(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
