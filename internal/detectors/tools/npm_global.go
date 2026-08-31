package tools

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"devenv/internal/detector"
	"devenv/internal/probe"
)

// maxListedPackages caps how many package names go into Meta.
const maxListedPackages = 5

// NpmGlobals reports the globally installed npm packages, which are otherwise
// invisible and easy to forget.
type NpmGlobals struct{ p probe.Prober }

// NewNpmGlobals returns an npm globals detector reading through p.
func NewNpmGlobals(p probe.Prober) NpmGlobals { return NpmGlobals{p: p} }

func (d NpmGlobals) Name() string                { return "npm globals" }
func (d NpmGlobals) Category() detector.Category { return detector.CategoryTool }

func (d NpmGlobals) Detect(ctx context.Context) (detector.Item, error) {
	path, err := d.p.LookPath("npm")
	if err != nil {
		return detector.Item{}, detector.ErrNotInstalled
	}

	// --json is parsed rather than the human table, which changes shape
	// between npm versions. --depth=0 keeps it to top-level installs.
	out, err := d.p.Run(ctx, "npm", "ls", "-g", "--depth=0", "--json")
	if err != nil && out == "" {
		return detector.Item{}, err
	}

	names := parseGlobalPackages(out)

	item := detector.Item{
		Name:     d.Name(),
		Category: d.Category(),
		Path:     path,
		Status:   detector.StatusInstalled,
	}
	item = item.WithMeta("packages", strconv.Itoa(len(names)))
	if len(names) > 0 {
		shown := names
		if len(shown) > maxListedPackages {
			shown = shown[:maxListedPackages]
		}
		item = item.WithMeta("packageList", strings.Join(shown, ", "))
	}
	return item, nil
}

// npmList is the shape of "npm ls -g --json" output.
type npmList struct {
	Dependencies map[string]struct {
		Version string `json:"version"`
	} `json:"dependencies"`
}

// parseGlobalPackages reads the dependency names, sorted for a stable display.
// npm reports itself as a global package; that is true, and hiding it would
// misstate the count.
func parseGlobalPackages(out string) []string {
	var list npmList
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil
	}

	names := make([]string, 0, len(list.Dependencies))
	for name := range list.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
