package main

import (
	"context"
	"flag"
	"fmt"
	"sort"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/graph"
)

var summaryFlagSet = flag.NewFlagSet("depgraph summary", flag.ExitOnError)

var summaryCommand = &ffcli.Command{
	Name:       "summary",
	ShortUsage: "depgraph summary {package}",
	ShortHelp:  "Outputs a DOT-formatted graph of the given package dependency and dependents",
	FlagSet:    summaryFlagSet,
	Exec:       summary,
}

func summary(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("expected exactly one package")
	}
	pkg := args[0]

	graph, err := graph.Load()
	if err != nil {
		return err
	}

	dependencyMap := summaryTraverse(pkg, graph.Dependencies)
	dependencies := make([]string, 0, len(dependencyMap))
	for dependency := range dependencyMap {
		dependencies = append(dependencies, dependency)
	}
	sort.Strings(dependencies)

	dependentMap := summaryTraverse(pkg, graph.Dependents)
	dependents := make([]string, 0, len(dependentMap))
	for dependent := range dependentMap {
		dependents = append(dependents, dependent)
	}
	sort.Strings(dependents)

	fmt.Printf("Direct dependencies:\n")

	for _, dependency := range dependencies {
		if dependencyMap[dependency] {
			fmt.Printf("\t> %s\n", dependency)
		}
	}

	fmt.Printf("\n")
	fmt.Printf("Transitive dependencies:\n")

	for _, dependency := range dependencies {
		if !dependencyMap[dependency] {
			fmt.Printf("\t> %s\n", dependency)
		}
	}

	fmt.Printf("\n")
	fmt.Printf("Dependent commands:\n")

	for _, dependent := range dependents {
		if isMain(graph, dependent) {
			fmt.Printf("\t> %s\n", dependent)
		}
	}

	fmt.Printf("\n")
	fmt.Printf("Direct dependents:\n")

	for _, dependent := range dependents {
		if !isMain(graph, dependent) && dependentMap[dependent] {
			fmt.Printf("\t> %s\n", dependent)
		}
	}

	fmt.Printf("\n")
	fmt.Printf("Transitive dependents:\n")

	for _, dependent := range dependents {
		if !isMain(graph, dependent) && !dependentMap[dependent] {
			fmt.Printf("\t> %s\n", dependent)
		}
	}

	return nil
}

// summaryTraverse returns a set of packages related to teh given package via the given
// relation. Each package is returned with a boolean value indicating whether or not the
// relation is direct (true) or transitive (false).k
func summaryTraverse(pkg string, relation map[string][]string) map[string]bool {
	m := make(map[string]bool, len(relation[pkg]))
	for _, v := range relation[pkg] {
		m[v] = true
	}

outer:
	for {
		for k := range m {
			for _, v := range relation[k] {
				if _, ok := m[v]; ok {
					continue
				}

				m[v] = false
				continue outer
			}
		}

		break
	}

	return m
}

// isMain returns true if the given package declares "main" in the given package name map.
func isMain(graph *graph.DependencyGraph, pkg string) bool {
	for _, name := range graph.PackageNames[pkg] {
		if name == "main" {
			return true
		}
	}

	return false
}
