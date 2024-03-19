package main

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/urfave/cli/v2"
	"golang.org/x/exp/maps"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/completions"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type bzlgenTarget struct {
	order  int
	cmd    string
	args   []string
	env    []string
	protip string
}

var bzlgenTargets = map[string]bzlgenTarget{
	"builds": {
		order: 1,
		cmd:   "run",
		args:  []string{"//:gazelle"},
	},
	"godeps": {
		cmd:  "run",
		args: []string{"//:gazelle-update-repos"},
	},
	"rustdeps": {
		cmd:    "sync",
		args:   []string{"--only=crate_index"},
		env:    []string{"CARGO_BAZEL_REPIN=1"},
		protip: "run with CARGO_BAZEL_ISOLATED=0 for faster (but less sandboxed) repinning.",
	},
}

var bazelCommand = &cli.Command{
	Name:            "bazel",
	Aliases:         []string{"bz"},
	SkipFlagParsing: true,
	HideHelpCommand: true,
	Usage:           "Proxies the bazel CLI with custom commands for local dev convenience",
	Category:        category.Dev,
	Action: func(ctx *cli.Context) error {
		if slices.Equal(ctx.Args().Slice(), []string{"help"}) || slices.Equal(ctx.Args().Slice(), []string{"--help"}) || slices.Equal(ctx.Args().Slice(), []string{"-h"}) {
			fmt.Println("Additional commands from sg:")
			fmt.Println("  configure           Wrappers around some commands to generate various files required by Bazel")
		}

		cmd := exec.CommandContext(ctx.Context, "bazel", ctx.Args().Slice()...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	},
	Subcommands: []*cli.Command{
		{
			Name:  "configure",
			Usage: "Wrappers around some commands to generate various files required by Bazel",
			UsageText: `sg bazel configure [category...]

Available categories:
	- builds: updates BUILD.bazel files for Go & Typescript targets.
	- godeps: updates the bazel Go dependency targets based on go.mod changes.
	- rustdeps: updates the cargo bazel lockfile.
	- all: catch-all for all of the above

If no categories are referenced, then 'builds' is assumed as the default.`,
			BashComplete: completions.CompleteArgs(func() (options []string) {
				return append(maps.Keys(bzlgenTargets), "all")
			}),
			Before: func(ctx *cli.Context) error {
				for _, arg := range ctx.Args().Slice() {
					if _, ok := bzlgenTargets[arg]; !ok && arg != "all" {
						cli.HandleExitCoder(errors.Errorf("category doesn't exist %q, run `sg bazel configure --help` for full info.", arg))
						cli.ShowSubcommandHelpAndExit(ctx, 1)
						return nil
					}
				}
				return nil
			},
			Action: func(ctx *cli.Context) error {
				var categories []bzlgenTarget
				var categoryNames []string
				if slices.Contains(ctx.Args().Slice(), "all") {
					categories = maps.Values(bzlgenTargets)
					categoryNames = maps.Keys(bzlgenTargets)
				} else if ctx.NArg() == 0 {
					categories = []bzlgenTarget{bzlgenTargets["builds"]}
					categoryNames = []string{"builds"}
				} else {
					for i := range ctx.NArg() {
						categories = append(categories, bzlgenTargets[ctx.Args().Get(i)])
						categoryNames = append(categoryNames, ctx.Args().Get(i))
					}
				}

				slices.SortFunc(categories, func(a, b bzlgenTarget) int {
					return cmp.Compare(a.order, b.order)
				})

				std.Out.WriteLine(output.Emojif(output.EmojiAsterisk, "Invoking the following Bazel generating categories: %s", strings.Join(categoryNames, ", ")))

				for _, c := range categories {
					std.Out.WriteNoticef("running command %q", strings.Join(append([]string{"bazel", c.cmd}, c.args...), " "))
					if c.protip != "" {
						std.Out.WriteLine(output.Emojif(output.EmojiLightbulb, "pro-tip: %s", c.protip))
					}

					args := append([]string{c.cmd, "--noshow_progress"}, c.args...)
					cmd := exec.CommandContext(ctx.Context, "bazel", args...)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Env = c.env
					cmd.Env = append(cmd.Env, os.Environ()...)

					err := cmd.Run()
					var exitErr *exec.ExitError
					if errors.As(err, &exitErr) && exitErr.ExitCode() == 110 {
						return nil
					} else if err != nil {
						return err
					}
				}
				return nil
			},
		},
	},
}
