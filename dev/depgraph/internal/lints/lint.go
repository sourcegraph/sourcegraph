package lints

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

type Lint func(graph *graph.DependencyGraph) []lintError

type lintError struct {
	pkg     string
	message []string
}

var lintsByName = map[string]Lint{
	"NoBinarySpecificSharedCode": NoBinarySpecificSharedCode,
	"NoDeadPackages":             NoDeadPackages,
	"NoEnterpriseImportsFromOSS": NoEnterpriseImportsFromOSS,
	"NoLooseCommands":            NoLooseCommands,
	"NoReachingIntoCommands":     NoReachingIntoCommands,
	"NoUnusedSharedCommandCode":  NoUnusedSharedCommandCode,
}

var DefaultLints []string

func init() {
	for name := range lintsByName {
		DefaultLints = append(DefaultLints, name)
	}
}

// Run runs the lint passes with the given names using the given graph. The lint
// violations will be formatted as a non-nil error value.
func Run(graph *graph.DependencyGraph, names []string) error {
	lints := make([]Lint, 0, len(names))
	for _, name := range names {
		lint, ok := lintsByName[name]
		if !ok {
			return fmt.Errorf("unknown lint '%s'", name)
		}

		lints = append(lints, lint)
	}

	var errors []lintError
	for _, lint := range lints {
		errors = append(errors, lint(graph)...)
	}

	return formatErrors(errors)
}

// maxNumErrors is the maxmum number of errors that will be displayed at once.
const maxNumErrors = 500

// formatErrors returns an error value that is formatted to display the given lint
// errors. If there were no lint errors, this function will return nil.
func formatErrors(errors []lintError) error {
	if len(errors) == 0 {
		return nil
	}

	sort.Slice(errors, func(i, j int) bool {
		return errors[i].pkg < errors[j].pkg || (errors[i].pkg == errors[j].pkg && strings.Join(errors[i].message, "\n") < strings.Join(errors[j].message, "\n"))
	})

	preamble := fmt.Sprintf("%d lint violations", len(errors))

	if len(errors) > maxNumErrors {
		errors = errors[:maxNumErrors]
		preamble += fmt.Sprintf(" (showing %d)", len(errors))
	}

	items := make([]string, 0, len(errors))
	for i, err := range errors {
		pkg := err.pkg
		if pkg == "" {
			pkg = "<root>"
		}

		items = append(items, fmt.Sprintf("%3d. %s\n     %s\n", i+1, pkg, strings.Join(err.message, "\n     ")))
	}

	return fmt.Errorf("%s:\n\n%s", preamble, strings.Join(items, "\n"))
}
