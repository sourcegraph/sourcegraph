package main

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/completions"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

func init() {
	postInitHooks = append(postInitHooks, func(cmd *cli.Context) {
		// Create 'sg test' help text after flag (and config) initialization
		testCommand.Description = constructTestCmdLongHelp()
	})
}

var testCommand = &cli.Command{
	Name:      "test",
	ArgsUsage: "<testsuite>",
	Usage:     "Run the given test suite",
	UsageText: `
# Run different test suites:
sg test backend
sg test backend-integration
sg test client
sg test web-e2e

# List available test suites:
sg test -help

# Arguments are passed along to the command
sg test backend-integration -run TestSearch
`,
	Category: category.Dev,
	BashComplete: completions.CompleteArgs(func() (options []string) {
		config, _ := getConfig()
		if config == nil {
			return
		}
		for name := range config.Tests {
			options = append(options, name)
		}
		return
	}),
	Action: testExec,
}

func testExec(ctx *cli.Context) error {
	config, err := getConfig()
	if err != nil {
		return err
	}

	args := ctx.Args().Slice()
	if len(args) == 0 {
		std.Out.WriteLine(output.Styled(output.StyleWarning, "No test suite specified"))
		return flag.ErrHelp
	}

	cmd, ok := config.Tests[args[0]]
	if !ok {
		std.Out.WriteLine(output.Styledf(output.StyleWarning, "ERROR: test suite %q not found :(", args[0]))
		return flag.ErrHelp
	}

	return run.Test(ctx.Context, newSGTestCommand(*cmd, args[1:]), config.Env)
}

func constructTestCmdLongHelp() string {
	var out strings.Builder

	fmt.Fprintf(&out, "Testsuites are defined in sg configuration.")

	// Attempt to parse config to list available testsuites, but don't fail on
	// error, because we should never error when the user wants --help output.
	config, err := getConfig()
	if err != nil {
		out.Write([]byte("\n"))
		// Do not treat error message as a format string
		std.NewOutput(&out, false).WriteWarningf("%s", err.Error())
		return out.String()
	}

	fmt.Fprintf(&out, "\n\n")
	fmt.Fprintf(&out, "Available testsuites in `%s`:\n", configFile)
	fmt.Fprintf(&out, "\n")

	var names []string
	for name := range config.Tests {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprint(&out, "* "+strings.Join(names, "\n* "))

	return out.String()
}

type sgTestCommand struct {
	run.Command
	args []string
}

// Ovrrides the GetExecCmd method with a custom implementation to construct the command
// using CLI-passed arguments
func (test sgTestCommand) GetExecCmd(ctx context.Context) (*exec.Cmd, error) {
	cmdArgs := []string{test.Command.Cmd}
	if len(test.args) != 0 {
		cmdArgs = append(cmdArgs, test.args...)
	} else {
		cmdArgs = append(cmdArgs, test.Command.DefaultArgs)
	}

	return exec.CommandContext(ctx, "bash", "-c", strings.Join(cmdArgs, " ")), nil
}

func newSGTestCommand(cmd run.Command, args []string) sgTestCommand {
	return sgTestCommand{
		Command: cmd,
		args:    args,
	}
}
