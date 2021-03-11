package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

var (
	endpoint = env.Get("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:3080", "Sourcegraph frontend endpoint")
	token    = env.Get("SOURCEGRAPH_SUDO_TOKEN", "", "Access token")

	// Flags
	indexDir                    string
	numConcurrentUploads        int
	numConcurrentRequests       int
	checkQueryResult            bool
	queryReferencesOfReferences bool

	// Entrypoints
	commands = map[string]func() error{
		"upload": uploadCommand,
		"query":  queryCommand,
	}
)

func main() {
	flag.StringVar(&indexDir, "indexDir", "./testdata/indexes", "The location of the testdata directory") // Assumes running from this directory
	flag.IntVar(&numConcurrentUploads, "numConcurrentUploads", 5, "The maximum number of concurrent uploads")
	flag.IntVar(&numConcurrentRequests, "numConcurrentRequests", 5, "The maximum number of concurrent requests")
	flag.BoolVar(&checkQueryResult, "checkQueryResult", true, "Whether to confirm query results are correct")
	flag.BoolVar(&queryReferencesOfReferences, "queryReferencesOfReferences", false, "Whether to perform reference operations on test case references")

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "subcommand (one of %s) is required\n", commandNameList())
		os.Exit(1)
	}

	command, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "subcommand (one of %s) is required\n", commandNameList())
		os.Exit(1)
	}

	if err := flag.CommandLine.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

	if err := command(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}

}

// commandNameList returns a comma-separated list of valid command names.
func commandNameList() string {
	var commandNames []string
	for name := range commands {
		commandNames = append(commandNames, name)
	}

	var parts []string
	for i, name := range commandNames {
		if i == len(commandNames)-1 {
			name = fmt.Sprintf("or %s", name)
		}
		parts = append(parts, name)
	}

	return strings.Join(parts, ", ")
}
