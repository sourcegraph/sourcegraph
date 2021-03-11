package lints

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

// NoLooseCommands returns an error for each main package not declared in a known command root.
func NoLooseCommands(graph *graph.DependencyGraph) []lintError {
	return mapPackageErrors(graph, func(pkg string) (lintError, bool) {
		if !isMain(graph.PackageNames, pkg) || cmdPattern.MatchString(pkg) {
			return lintError{}, false
		}

		var prefix string
		if isEnterprise(pkg) {
			prefix = "enterprise/"
		}

		return lintError{
			pkg: pkg,
			message: []string{
				"This package is a binary entrypoint outside of the expected command root.",
				fmt.Sprintf("To resolve, move this package into %sdev/ or %scmd/.", prefix, prefix),
			},
		}, true
	})
}
