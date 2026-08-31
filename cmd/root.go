// Package cmd parses the command line and starts the app.
//
// Phase 1 has a single command and three flags, so it uses the standard
// library. Cobra (per the architecture doc) lands in Phase 2, when there are
// subcommands to justify the dependency.
package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

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
		listOnly    = fs.Bool("list", false, "list the registered detectors and exit")
		showVersion = fs.Bool("version", false, "print the devenv version and exit")
		timeout     = fs.Duration("timeout", 5*time.Second, "per-command timeout for a single detector probe")
	)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Fprintln(out, "devenv", Version)
		return 0
	}

	opts := app.DefaultOptions()
	opts.JSON = *asJSON
	opts.Timeout = *timeout
	opts.Out = out

	a := app.New(opts)

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
