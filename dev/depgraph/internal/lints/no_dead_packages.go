package lints

import (
	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

// NoDeadPackages returns an error for any package that is not importable from outside the
// repository and is not imported (transitively) by a main package.
func NoDeadPackages(graph *graph.DependencyGraph) []lintError {
	return mapPackageErrors(graph, func(pkg string) (lintError, bool) {
		if isMain(graph.PackageNames, pkg) || isLibrary(pkg) {
			return lintError{}, false
		}

		for _, dependent := range allDependents(graph, pkg) {
			if isMain(graph.PackageNames, dependent) {
				return lintError{}, false
			}
		}

		return lintError{
			pkg: pkg,
			message: []string{
				"This package is not accessible to any repository-external project.",
				"This package is not imported by any binary defined in this repository.",
				"To resolve, delete this package (not including any existing child package).",
			},
		}, true
	})
}
