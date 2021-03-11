package lints

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

// NoReachingIntoCommands returns an error for each shared package that imports a package
// from a command. This includes reaching into cmd/X from another cmd, or from shared code.
func NoReachingIntoCommands(graph *graph.DependencyGraph) []lintError {
	violations := map[string][]string{}
	for _, pkg := range graph.Packages {
		for _, dependency := range graph.Dependencies[pkg] {
			if cmd := containingCommand(dependency); cmd != "" && cmd != containingCommand(pkg) {
				violations[dependency] = append(violations[dependency], pkg)
			}
		}
	}

	errors := make([]lintError, 0, len(violations))
	for imported, importers := range violations {
		errors = append(errors, makeReachingIntoCommandError(imported, importers))
	}

	return errors
}

func makeReachingIntoCommandError(imported string, importers []string) lintError {
	items := make([]string, 0, len(importers))
	for _, importer := range importers {
		items = append(items, fmt.Sprintf("\t- %s", importer))
	}

	allEnterprise := true
	for _, importer := range importers {
		if !isEnterprise(importer) {
			allEnterprise = false
		}
	}

	target := "internal"
	if allEnterprise {
		target = "enterprise/" + target
	}

	return lintError{
		pkg: imported,
		message: []string{
			fmt.Sprintf("The following %d packages import this package across a command boundary.", len(items)),
			strings.Join(items, "\n"),
			fmt.Sprintf("To resolve, move this package to %s/.", target),
		},
	}
}
