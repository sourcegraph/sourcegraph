package main

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/graph"
	"github.com/sourcegraph/sourcegraph/dev/depgraph/internal/lints"
)

var lintFlagSet = flag.NewFlagSet("depgraph lint", flag.ExitOnError)
var lintCommand = &ffcli.Command{
	Name:       "lint",
	ShortUsage: "depgraph lint [pass...]",
	ShortHelp:  "Runs lint passes over the internal Go dependency graph",
	FlagSet:    lintFlagSet,
	Exec:       lint,
}

func lint(ctx context.Context, args []string) error {
	if len(args) == 0 {
		args = lints.DefaultLints
	}

	graph, err := graph.Load()
	if err != nil {
		return err
	}

	return lints.Run(graph, args)
}
