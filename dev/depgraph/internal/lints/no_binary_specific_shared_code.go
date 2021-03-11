package lints

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

// NoBinarySpecificSharedCode returns an error for each shared package that is used
// by a single command.
func NoBinarySpecificSharedCode(graph *graph.DependencyGraph) []lintError {
	return mapPackageErrors(graph, func(pkg string) (lintError, bool) {
		if containingCommand(pkg) != "" || isLibrary(pkg) {
			// Not shared code
			return lintError{}, false
		}

		allInternal := true
		allEnterprise := true
		dependentCommands := map[string]struct{}{}
		for _, dependent := range graph.Dependents[pkg] {
			if !isCommandPrivate(dependent) {
				allInternal = false
			}
			if !isEnterprise(dependent) {
				allEnterprise = false
			}

			dependentCommands[containingCommand(dependent)] = struct{}{}
		}
		if len(dependentCommands) != 1 {
			// Not a single import
			return lintError{}, false
		}

		var importer string
		for cmd := range dependentCommands {
			importer = cmd
		}
		if importer == "" {
			// Only imported by other internal packages
			return lintError{}, false
		}

		var target string
		for _, importer := range graph.Dependents[pkg] {
			target = containingCommandPrefix(importer)
		}
		if allInternal {
			target += "/internal"
		}
		if !allEnterprise {
			target = strings.TrimPrefix(target, "enterprise/")
		}

		return lintError{
			pkg: pkg,
			message: []string{
				fmt.Sprintf("This package is used exclusively by %s.", importer),
				fmt.Sprintf("To resolve, move this package to %s/.", target),
			},
		}, true
	})
}
