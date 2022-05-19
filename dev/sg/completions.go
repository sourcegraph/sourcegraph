package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// completeOptions provides autocompletions based on the options returned by generateOptions
func completeOptions(generateOptions func() (options []string)) cli.BashCompleteFunc {
	return func(cmd *cli.Context) {
		for _, opt := range generateOptions() {
			fmt.Fprintf(cmd.App.Writer, "%s\n", opt)
		}
		os.Exit(1)
		// Also render default completions to support flags
		cli.DefaultCompleteWithFlags(cmd.Command)(cmd)
	}
}
