package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func main() {
	if err := mainErr(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

var rootFlagSet = flag.NewFlagSet("depgraph", flag.ExitOnError)
var rootCommand = &ffcli.Command{
	ShortUsage: "depgraph [flags] <subcommand>",
	FlagSet:    rootFlagSet,
	Subcommands: []*ffcli.Command{
		summaryCommand,
		traceCommand,
		traceInternalCommand,
		lintCommand,
	},
}

func mainErr() error {
	if err := rootCommand.Parse(os.Args[1:]); err != nil {
		return err
	}

	return rootCommand.Run(context.Background())
}
