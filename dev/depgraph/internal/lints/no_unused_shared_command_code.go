package lints

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

// NoUnusedSharedCommandCode returns an error for each non-private package within
// a command that is imported only by private packages within the same command.
func NoUnusedSharedCommandCode(graph *graph.DependencyGraph) []lintError {
	return mapPackageErrors(graph, func(pkg string) (lintError, bool) {
		if isMain(graph.PackageNames, pkg) || containingCommand(pkg) == "" || isCommandPrivate(pkg) {
			// Not shared command code
			return lintError{}, false
		}

		if len(graph.Dependents[pkg]) == 0 {
			// Caught by NoDeadPackages
			return lintError{}, false
		}

		for _, dependent := range graph.Dependents[pkg] {
			if containingCommand(dependent) != containingCommand(pkg) {
				// Caught by NoReachingIntoCommands
				return lintError{}, false
			}

			if !isEnterprise(pkg) && isEnterprise(dependent) {
				// ok: imported from enterprise version of command
				return lintError{}, false
			}

			if !isCommandPrivate(dependent) && !isMain(graph.PackageNames, dependent) {
				// ok: imported from non-internal non-main code in same command
				return lintError{}, false
			}
		}

		prefix := containingCommandPrefix(pkg)

		return lintError{
			pkg: pkg,
			message: []string{
				fmt.Sprintf("This package is imported exclusively by internal code within %s.", prefix),
				fmt.Sprintf("To resolve, move this package into %s/internal.", prefix),
			},
		}, true
	})
}
