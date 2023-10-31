package main

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/grafana/regexp"
	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/bk"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"

	"github.com/sourcegraph/run"
)

var releaseRunFlags = []cli.Flag{
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
	&cli.BoolFlag{
		Name:  "pretend",
		Value: false,
		Usage: "Preview all the commands that would be performed",
	},
	&cli.StringFlag{
		Name:  "version",
		Value: "v6.6.666",
		Usage: "Force version",
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

var releaseCommand = &cli.Command{
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
					Usage: "todo",
					Subcommands: []*cli.Command{
						{
							Name:  "finalize",
							Usage: "Run internal release finalize steps",
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
					Usage: "TODO",
					Subcommands: []*cli.Command{
						{
							Name:  "finalize",
							Usage: "Run internal release finalize steps",
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
			Name:      "create",
			Usage:     "Create a release for a given product",
			UsageText: "sg release create --workdir [path] --type patch",
			Category:  category.Util,
			Flags:     releaseRunFlags,
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
			Usage:     "Promete an existing release to the public",
			UsageText: "sg release promote-to-public --workdir [path] --type patch",
			Category:  category.Util,
			Flags:     releaseRunFlags,
			Action: func(cctx *cli.Context) error {
				r, err := newReleaseRunnerFromCliContext(cctx)
				if err != nil {
					return err
				}
				return r.Promote(cctx.Context)
			},
		},
	},
}

func newReleaseRunnerFromCliContext(cctx *cli.Context) (*releaseRunner, error) {
	workdir := cctx.String("workdir")
	pretend := cctx.Bool("pretend")
	version := cctx.String("version")
	typ := cctx.String("type")
	inputs := cctx.String("inputs")

	if cctx.Bool("config-from-commit") {
		cmd := run.Cmd(cctx.Context, "git", "log", "-1")
		cmd.Dir(cctx.String("workdir"))
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

	return NewReleaseRunner(workdir, version, inputs, typ, pretend)
}

var buildNumberFlag = cli.StringFlag{
	Name:     "buildNumber",
	Usage:    "The buildkite build number to check for CVEs",
	Required: true,
	Aliases:  []string{"b"},
}

var referenceUriFlag = cli.StringFlag{
	Name:     "uri",
	Usage:    "A reference url that contains approved CVEs. Often a link to a handbook page eg: https://handbook.sourcegraph.com/departments/security/tooling/trivy/4-2-0/.",
	Required: true,
	Aliases:  []string{"u"},
}

var cvePattern = regexp.MustCompile(`<\w+>(CVE-\d+-\d+)<\/\w+>`)

func cveCheck(cmd *cli.Context) error {
	std.Out.WriteLine(output.Styledf(output.StylePending, "Checking release for approved CVEs..."))

	referenceUrl := referenceUriFlag.Get(cmd)
	number := buildNumberFlag.Get(cmd)

	client, err := bk.NewClient(cmd.Context, std.Out)
	if err != nil {
		return errors.Wrap(err, "bk.NewClient")
	}

	artifacts, err := client.ListArtifactsByBuildNumber(cmd.Context, "sourcegraph", number)
	if err != nil {
		return errors.Wrap(err, "unable to list artifacts by build number")
	}

	var refBuf bytes.Buffer
	err = downloadUrl(referenceUrl, &refBuf)
	if err != nil {
		return errors.Wrap(err, "unable to download reference url")
	}
	if refBuf.Len() == 0 {
		return errors.New("provided reference url does not have any contents")
	}

	var foundCVE []string
	var unapprovedCVE []string

	for _, artifact := range artifacts {
		name := *artifact.Filename
		url := *artifact.DownloadURL

		if strings.HasSuffix(*artifact.Filename, "security-report.html") {
			std.Out.WriteLine(output.Styledf(output.StylePending, "Checking security report: %s %s", name, url))

			var buf bytes.Buffer
			err = client.DownloadArtifact(artifact, &buf)
			if err != nil {
				return errors.Newf("failed to download security artifact %q at %s: %w", name, url, err)
			}

			foundCVE = append(foundCVE, extractCVEs(cvePattern, buf.String())...)
		}
	}
	unapprovedCVE = findUnapprovedCVEs(foundCVE, refBuf.String())

	std.Out.WriteLine(output.Styledf(output.StyleBold, "Found %d CVEs in the build", len(foundCVE)))
	if verbose {
		for _, s := range foundCVE {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "%s", s))
		}
	}
	if len(unapprovedCVE) > 0 {
		std.Out.WriteLine(output.Styledf(output.StyleWarning, "Unable to match CVEs"))
		for _, s := range unapprovedCVE {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "%s", s))
		}
	} else {
		std.Out.WriteLine(output.Styledf(output.StyleSuccess, "All CVEs approved!"))
	}

	return nil
}

func findUnapprovedCVEs(all []string, referenceDocument string) []string {
	var unapproved []string
	for _, cve := range all {
		if !strings.Contains(referenceDocument, cve) {
			unapproved = append(unapproved, cve)
		}
	}
	return unapproved
}

func extractCVEs(pattern *regexp.Regexp, document string) []string {
	var found []string
	matches := pattern.FindAllStringSubmatch(document, -1)
	for _, match := range matches {
		cve := strings.TrimSpace(match[1])
		found = append(found, cve)
	}
	return found
}

func downloadUrl(uri string, w io.Writer) (err error) {
	std.Out.WriteLine(output.Styledf(output.StylePending, "Downloading url: %s", uri))
	resp, err := http.Get(uri)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
