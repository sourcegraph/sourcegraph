package lints

import (
	"regexp"
	"sort"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
)

var (
	enterprisePrefixPattern         = regexp.MustCompile(`^(?:enterprise/)`)
	optionalEnterprisePrefixPattern = regexp.MustCompile(`^(?:enterprise/)?`)
	publicLibraryPrefixPattern      = regexp.MustCompile(optionalEnterprisePrefixPattern.String() + "(?:lib)/")
	cmdPrefixPattern                = regexp.MustCompile(optionalEnterprisePrefixPattern.String() + `(?:cmd|dev)/([^/]+)`)
	cmdPattern                      = regexp.MustCompile(cmdPrefixPattern.String() + `$`)
	cmdInternalPrefixPattern        = regexp.MustCompile(cmdPrefixPattern.String() + "/internal")
)

// isEnterprise returns true if the given path is in the enterprise directory.
func isEnterprise(path string) bool { return enterprisePrefixPattern.MatchString(path) }

// isLibray returns true if the given path is publicly importable.
func isLibrary(path string) bool { return publicLibraryPrefixPattern.MatchString(path) }

// IsCommandPrivate returns true if the given path is in the internal directory of its command.
func isCommandPrivate(path string) bool { return cmdInternalPrefixPattern.MatchString(path) }

// containingCommandPrefix returns the path of the command the given path resides in, if any.
// This method returns the same value for packages composing the same binary, regardless if
// it's part of the OSS or enterprise definition, and different values for different binaries
// and shared code.
func containingCommandPrefix(path string) string {
	if match := cmdPrefixPattern.FindStringSubmatch(path); len(match) > 0 {
		return match[0]
	}

	return ""
}

// containingCommand returns the name of the command the given path resides in, if any. This
// method returns the same value for packages composing the same binary, regardless if it's
// part of the OSS or enterprise definition, and different values for different binaries and
// shared code.
func containingCommand(path string) string {
	if match := cmdPrefixPattern.FindStringSubmatch(path); len(match) > 0 {
		return match[1]
	}

	return ""
}

// isMain returns true if the given package declares "main" in the given package name map.
func isMain(packageNames map[string][]string, pkg string) bool {
	for _, name := range packageNames[pkg] {
		if name == "main" {
			return true
		}
	}

	return false
}

// mapPackageErrors aggregates errors from the given function invoked on each package in
// the given graph.
func mapPackageErrors(graph *graph.DependencyGraph, fn func(pkg string) (lintError, bool)) []lintError {
	var errors []lintError
	for _, pkg := range graph.Packages {
		if err, ok := fn(pkg); ok {
			errors = append(errors, err)
		}
	}

	return errors
}

// allDependencies returns an ordered list of transitive dependencies of the given package.
func allDependencies(graph *graph.DependencyGraph, pkg string) []string {
	dependencyMap := map[string]struct{}{}

	var recur func(pkg string)
	recur = func(pkg string) {
		for _, dependency := range graph.Dependencies[pkg] {
			if _, ok := dependencyMap[dependency]; ok {
				continue
			}

			dependencyMap[dependency] = struct{}{}
			recur(dependency)
		}
	}
	recur(pkg)

	dependencies := make([]string, 0, len(dependencyMap))
	for dependency := range dependencyMap {
		dependencies = append(dependencies, dependency)
	}
	sort.Strings(dependencies)

	return dependencies
}

// allDependents returns an ordered list of transitive dependents of the given package.
func allDependents(graph *graph.DependencyGraph, pkg string) []string {
	dependentsMap := map[string]struct{}{}

	var recur func(pkg string)
	recur = func(pkg string) {
		for _, dependent := range graph.Dependents[pkg] {
			if _, ok := dependentsMap[dependent]; ok {
				continue
			}

			dependentsMap[dependent] = struct{}{}
			recur(dependent)
		}
	}
	recur(pkg)

	dependents := make([]string, 0, len(dependentsMap))
	for dependent := range dependentsMap {
		dependents = append(dependents, dependent)
	}
	sort.Strings(dependents)

	return dependents
}
