package release

import (
	"fmt"
	"os"
	"strings"

	"github.com/sourcegraph/run"
	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// releaseBaseFlags are the flags that are common to all subcommands of the release command.
// In particular, the version flag is not included in that list, because while it's required
// for create and promote-to-public, it's not for the others (to allow --config-from-commit).
var releaseBaseFlags = []cli.Flag{
	&cli.StringFlag{
		Name:  "workdir",
		Value: ".",
		Usage: "Set the working directory to load release scripts from",
	},
	&cli.StringFlag{
		Name:  "type",
		Value: "patch",
		Usage: "Select release type: major, minor, patch",
	},
	&cli.StringFlag{
		Name:  "branch",
		Usage: "Branch to create release from, usually `main` or `5.3` if you're cutting a patch release",
	},
	&cli.BoolFlag{
		Name:  "pretend",
		Value: false,
		Usage: "Preview all the commands that would be performed",
	},
	&cli.StringFlag{
		Name:  "inputs",
		Usage: "Set inputs to use for a given release, ex: --input=server=v5.2.404040,foobar=ffefe",
	},
	&cli.BoolFlag{
		Name:  "config-from-commit",
		Value: false,
		Usage: "Infer run configuration from last commit instead of flags.",
	},
}

// releaseRunFlags are the flags for the release run * subcommands. Version is optional here, because
// we can also use --infer-from-commit.
var releaseRunFlags = append(releaseBaseFlags, &cli.StringFlag{
	Name:  "version",
	Usage: "Force version",
})

// releaseCreatePromoteFlags are the flags for the release create and promote-to-public subcommands, Version
// is required here, because it makes no sense to create a release without one.
//
// TODO https://github.com/sourcegraph/sourcegraph/issues/61077 to add the "auto" value that ask
// the releaseregistry to provide the version number.
var releaseCreatePromoteFlags = append(releaseBaseFlags, &cli.StringFlag{
	Name:     "version",
	Usage:    "Force version (required)",
	Required: true,
})

var Command = &cli.Command{
	Name:     "release",
	Usage:    "Sourcegraph release utilities",
	Category: category.Util,
	Subcommands: []*cli.Command{
		{
			Name:     "cve-check",
			Usage:    "Check all CVEs found in a buildkite build against a set of preapproved CVEs for a release",
			Category: category.Util,
			Action:   cveCheck,
			Flags: []cli.Flag{
				&buildNumberFlag,
				&referenceUriFlag,
			},
			UsageText: `sg release cve-check -u https://handbook.sourcegraph.com/departments/security/tooling/trivy/4-2-0/ -b 184191`,
		},
		{
			Name:     "run",
			Usage:    "Run steps defined in release manifest. Those are meant to be run in CI",
			Category: category.Util,
			Subcommands: []*cli.Command{
				{
					Name:  "test",
					Flags: releaseRunFlags,
					Usage: "Run test steps as defined in the release manifest",
					Action: func(cctx *cli.Context) error {
						r, err := newReleaseRunnerFromCliContext(cctx)
						if err != nil {
							return err
						}
						return r.Test(cctx.Context)
					},
				},
				{
					Name:  "internal",
					Usage: "Run manifest defined steps (internal releases)",
					Subcommands: []*cli.Command{
						{
							Name:  "finalize",
							Usage: "Run manifest defined finalize step for internal releases",
							Flags: releaseRunFlags,
							Action: func(cctx *cli.Context) error {
								r, err := newReleaseRunnerFromCliContext(cctx)
								if err != nil {
									return err
								}
								return r.InternalFinalize(cctx.Context)
							},
						},
					},
				},
				{
					Name:  "promote-to-public",
					Usage: "Run manifest defined steps (public releases)",
					Subcommands: []*cli.Command{
						{
							Name:  "finalize",
							Usage: "Run manifest defined finalize step for public releases",
							Flags: releaseRunFlags,
							Action: func(cctx *cli.Context) error {
								r, err := newReleaseRunnerFromCliContext(cctx)
								if err != nil {
									return err
								}
								return r.PromoteFinalize(cctx.Context)
							},
						},
					},
				},
			},
		},
		{
			Name:        "create",
			Usage:       "Create a release for a given product",
			Description: "See https://go/releases",
			UsageText:   "sg release create --workdir [path-to-folder-with-manifest] --version vX.Y.Z",
			Category:    category.Util,
			Flags:       releaseCreatePromoteFlags,
			Action: func(cctx *cli.Context) error {
				r, err := newReleaseRunnerFromCliContext(cctx)
				if err != nil {
					return err
				}
				return r.CreateRelease(cctx.Context)
			},
		},
		{
			Name:      "promote-to-public",
			Usage:     "Promote an internal release to the public",
			UsageText: "sg release promote-to-public --workdir [path-to-folder-with-manifest] --version vX.Y.Z",
			Category:  category.Util,
			Flags:     releaseCreatePromoteFlags,
			Action: func(cctx *cli.Context) error {
				r, err := newReleaseRunnerFromCliContext(cctx)
				if err != nil {
					return err
				}
				return r.Promote(cctx.Context)
			},
		},
		{
			Name:      "calendar",
			Usage:     "Generate a calendar events for releases",
			Aliases:   []string{"cal"},
			Category:  category.Util,
			UsageText: "sg release calendar --config /path/to/calendar/config",
			Action:    generateCalendarEvents,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "config",
					Required: true,
					Usage:    "Path to the calendar config file",
					Action: func(ctx *cli.Context, s string) error {
						if _, err := os.Stat(s); err != nil {
							return errors.Newf("config file %q does not exist", s)
						}
						return nil
					},
				},
			},
		},
	},
}

func newReleaseRunnerFromCliContext(cctx *cli.Context) (*releaseRunner, error) {
	if cctx.Bool("config-from-commit") && cctx.String("version") != "" {
		return nil, errors.New("You cannot use --config-from-commit and --version at the same time")
	}

	if !cctx.Bool("config-from-commit") && cctx.String("version") == "" {
		return nil, errors.New("You must provide a version by specifying either --version or --config-from-commit")
	}

	workdir := cctx.String("workdir")
	pretend := cctx.Bool("pretend")
	// Normalize the version string, to prevent issues where this was given with the wrong convention
	// which requires a full rebuild.
	version := fmt.Sprintf("v%s", strings.TrimPrefix(cctx.String("version"), "v"))
	typ := cctx.String("type")
	inputs := cctx.String("inputs")
	branch := cctx.String("branch")

	if cctx.Bool("config-from-commit") {
		cmd := run.Cmd(cctx.Context, "git", "log", "-1")
		cmd.Dir(workdir)
		lines, err := cmd.Run().Lines()
		if err != nil {
			return nil, err
		}

		// config dump is always the last line.
		configRaw := lines[len(lines)-1]
		if !strings.HasPrefix(strings.TrimSpace(configRaw), "{") {
			return nil, errors.New("Trying to infer config from last commit, but did not find serialized config")
		}
		rc, err := parseReleaseConfig(configRaw)
		if err != nil {
			return nil, err
		}

		version = rc.Version
		typ = rc.Type
		inputs = rc.Inputs
	}

	return NewReleaseRunner(cctx.Context, workdir, version, inputs, typ, branch, pretend)
}
