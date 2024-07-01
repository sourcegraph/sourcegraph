package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/analytics"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/interrupt"
)

// addAnalyticsHooks wraps command actions with analytics hooks. We reconstruct commandPath
// ourselves because the library's state (and hence .FullName()) seems to get a bit funky.
//
// It also handles watching for panics and formatting them in a useful manner.
func addAnalyticsHooks(commandPath []string, commands []*cli.Command) {
	for _, command := range commands {
		fullCommandPath := append(commandPath, command.Name)
		if len(command.Subcommands) > 0 {
			addAnalyticsHooks(fullCommandPath, command.Subcommands)
		}

		// No action to perform analytics on
		if command.Action == nil {
			continue
		}

		// Set up analytics hook for command
		fullCommand := strings.Join(fullCommandPath, " ")

		// Wrap action with analytics
		wrappedAction := command.Action
		command.Action = func(cmd *cli.Context) (actionErr error) {
			cmdFlags := make(map[string][]string)
			for _, parent := range cmd.Lineage() {
				cmdFlags[parent.Command.Name] = parent.LocalFlagNames()
			}
			cmd.Context = analytics.NewInvocation(cmd.Context, cmd.App.Version, map[string]any{
				"command": fullCommand,
				"flags":   cmdFlags,
				"args":    cmd.Args().Slice(),
				"nargs":   cmd.NArg(),
			})

			// Make sure analytics are persisted before exit (interrupts or panics)
			defer func() {
				if p := recover(); p != nil {
					// Render a more elegant message
					std.Out.WriteWarningf("Encountered panic - please open an issue with the command output:\n\t%s",
						sgBugReportTemplate)
					message := fmt.Sprintf("%v:\n%s", p, getRelevantStack("addAnalyticsHooks"))
					actionErr = cli.Exit(message, 1)

					// Log event
					analytics.AddMeta(cmd.Context, map[string]any{
						"panic": actionErr,
					})
				}
			}()
			interrupt.Register(func() {
				analytics.InvocationCancelled(cmd.Context)
			})

			// Call the underlying action
			actionErr = wrappedAction(cmd)

			// Capture analytics post-run
			if actionErr != nil {
				analytics.InvocationFailed(cmd.Context, actionErr)
			} else {
				analytics.InvocationSucceeded(cmd.Context)
			}

			return actionErr
		}
	}
}

// getRelevantStack generates a stacktrace that encapsulates the relevant parts of a
// stacktrace for user-friendly reading.
func getRelevantStack(excludeFunctions ...string) string {
	callers := make([]uintptr, 32)
	n := runtime.Callers(3, callers) // recover -> getRelevantStack -> runtime.Callers
	frames := runtime.CallersFrames(callers[:n])

	var stack strings.Builder
	for {
		frame, next := frames.Next()

		var excludedFunction bool
		for _, e := range excludeFunctions {
			if strings.Contains(frame.Function, e) {
				excludedFunction = true
				break
			}
		}

		// Only include frames from sg and things that are not excluded.
		if !strings.Contains(frame.File, "dev/sg/") || excludedFunction {
			if !next {
				break
			}
			continue
		}

		stack.WriteString(frame.Function)
		stack.WriteByte('\n')
		stack.WriteString(fmt.Sprintf("\t%s:%d\n", frame.File, frame.Line))
		if !next {
			break
		}
	}

	return stack.String()
}
