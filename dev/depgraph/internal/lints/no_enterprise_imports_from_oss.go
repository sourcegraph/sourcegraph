package lints

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

// NoEnterpriseImportsFromOSS returns an error for each non-enterprise package that
// imports an enterprise package.
func NoEnterpriseImportsFromOSS(graph *graph.DependencyGraph) []lintError {
	return mapPackageErrors(graph, func(pkg string) (lintError, bool) {
		if isEnterprise(pkg) {
			return lintError{}, false
		}

		var imports []string
		for _, dependency := range graph.Dependencies[pkg] {
			if isEnterprise(dependency) {
				imports = append(imports, dependency)
			}
		}
		if len(imports) == 0 {
			return lintError{}, false
		}

		return makeNoEnterpriseImportsFromOSSError(pkg, imports), true
	})
}

func makeNoEnterpriseImportsFromOSSError(pkg string, imports []string) lintError {
	items := make([]string, 0, len(imports))
	for _, importer := range imports {
		items = append(items, fmt.Sprintf("\t- %s", importer))
	}

	return lintError{
		pkg: pkg,
		message: []string{
			fmt.Sprintf("This package imports the following %d enterprise packages:\n%s", len(items), strings.Join(items, "\n")),
			"To resolve, move this package into enterprise/.",
		},
	}
}
