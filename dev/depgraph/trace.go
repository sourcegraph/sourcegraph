package main

import (
	"context"
	"flag"
	"fmt"
	"sort"

	"github.com/peterbourgon/ff/v3/ffcli"

	depgraph "github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/visualization"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	traceFlagSet           = flag.NewFlagSet("depgraph trace", flag.ExitOnError)
	dependencyMaxDepthFlag = traceFlagSet.Int("dependency-max-depth", 1, "Show transitive dependencies up to this depth (default 1)")
	dependentMaxDepthFlag  = traceFlagSet.Int("dependent-max-depth", 1, "Show transitive dependents up to this depth (default 1)")
)

var traceCommand = &ffcli.Command{
	Name:       "trace",
	ShortUsage: "depgraph trace {package} [-dependency-max-depth=1] [-dependent-max-depth=1]",
	ShortHelp:  "Outputs a DOT-formatted graph of the given package dependency and dependents",
	FlagSet:    traceFlagSet,
	Exec:       trace,
}

func trace(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.Errorf("expected exactly one package")
	}
	pkg := args[0]

	root, err := findRoot()
	if err != nil {
		return err
	}

	graph, err := depgraph.Load(root)
	if err != nil {
		return err
	}
	if _, ok := graph.PackageNames[pkg]; !ok {
		return errors.Newf("pkg %q not found", pkg)
	}

	packages, dependencyEdges, dependentEdges := traceWalkGraph(graph, pkg, *dependencyMaxDepthFlag, *dependentMaxDepthFlag)
	fmt.Printf("%s\n", visualization.Dotify(packages, dependencyEdges, dependentEdges))
	return nil
}

// traceWalkGraph traverses the given dependency graph in both directions and returns a
// set of packages and edges (separated by traversal direction) forming the dependency
// graph around the given blessed package.
func traceWalkGraph(graph *depgraph.DependencyGraph, pkg string, dependencyMaxDepth, dependentMaxDepth int) (packages []string, dependencyEdges, dependentEdges map[string][]string) {
	dependencyPackages, dependencyEdges := traceTraverse(pkg, graph.Dependencies, dependencyMaxDepth)
	dependentPackages, dependentEdges := traceTraverse(pkg, graph.Dependents, dependentMaxDepth)
	return append(dependencyPackages, dependentPackages...), dependencyEdges, dependentEdges
}

// traceTraverse returns a set of packages and edges forming the dependency graph around
// the given blessed package using the given relation to traverse the dependency graph in
// one direction from the given package root.
func traceTraverse(pkg string, relation map[string][]string, maxDepth int) (packages []string, edges map[string][]string) {
	frontier := relation[pkg]
	packageMap := map[string]int{pkg: 0}
	edges = map[string][]string{pkg: relation[pkg]}

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		nextFrontier := []string{}
		for _, pkg := range frontier {
			if _, ok := packageMap[pkg]; ok {
				continue
			}
			packageMap[pkg] = depth

			edges[pkg] = append(edges[pkg], relation[pkg]...)
			nextFrontier = append(nextFrontier, relation[pkg]...)
		}

		frontier = nextFrontier
	}

	packages = make([]string, 0, len(packages))
	for k := range packageMap {
		packages = append(packages, k)
	}
	sort.Strings(packages)

	// Ensure we don't point to anything we don't have an explicit
	// vertex for. This can happen at the edge of the last frontier.
	pruneEdges(edges, packageMap)

	return packages, edges
}

// pruneEdges removes all references to a vertex that does not exist in the
// given vertex map. The edge map is modified in place.
func pruneEdges(edges map[string][]string, vertexMap map[string]int) {
	for edge, targets := range edges {
		edges[edge] = targets[:0]
		for _, target := range targets {
			if _, ok := vertexMap[target]; ok {
				edges[edge] = append(edges[edge], target)
			}
		}
	}
}
