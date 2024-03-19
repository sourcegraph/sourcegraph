package ci

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/google/uuid"
	"github.com/grafana/regexp"
	sgrun "github.com/sourcegraph/run"
	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/bk"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/loki"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/open"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/repo"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/usershell"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/completions"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/exit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var previewCommand = &cli.Command{
	Name:    "preview",
	Aliases: []string{"plan"},
	Usage:   "Preview the pipeline that would be run against the currently checked out branch",
	Flags: []cli.Flag{
		&ciBranchFlag,
		&cli.StringFlag{
			Name:  "format",
			Usage: "Output format for the preview (one of 'markdown', 'json', or 'yaml')",
			Value: "markdown",
		},
	},
	Action: func(cmd *cli.Context) error {
		std.Out.WriteLine(output.Styled(output.StyleSuggestion,
			"If the current branch were to be pushed, the following pipeline would be run:"))

		target, err := getBuildTarget(cmd)
		if err != nil {
			return err
		}
		if target.targetType != buildTargetTypeBranch {
			// Should never happen because we only register the branch flag
			return errors.New("target is not a branch")
		}

		message, err := run.TrimResult(run.GitCmd("show", "--format=%B"))
		if err != nil {
			return err
		}

		var previewCmd *sgrun.Command
		env := map[string]string{
			"BUILDKITE_BRANCH":  target.target, // this must be a branch
			"BUILDKITE_MESSAGE": message,
		}
		switch cmd.String("format") {
		case "markdown":
			previewCmd = usershell.Command(cmd.Context, "go run ./dev/ci/gen-pipeline.go -preview").
				Env(env)
			out, err := root.Run(previewCmd).String()
			if err != nil {
				return err
			}
			return std.Out.WriteMarkdown(out)
		case "json":
			previewCmd = usershell.Command(cmd.Context, "go run ./dev/ci/gen-pipeline.go").
				Env(env)
			out, err := root.Run(previewCmd).String()
			if err != nil {
				return err
			}
			return std.Out.WriteCode("json", out)
		case "yaml":
			previewCmd = usershell.Command(cmd.Context, "go run ./dev/ci/gen-pipeline.go -yaml").
				Env(env)
			out, err := root.Run(previewCmd).String()
			if err != nil {
				return err
			}
			return std.Out.WriteCode("yaml", out)
		default:
			return errors.Newf("unsupported format type: %q", cmd.String("format"))
		}
	},
}

var bazelCommand = &cli.Command{
	Name:      "bazel",
	Usage:     "Fires a CI build running a given bazel command",
	ArgsUsage: "[--web|--wait] [test|build] <target1> <target2> ... <bazel flags>",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "wait",
			Usage: "Wait until build completion and then print logs for the Bazel command",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "web",
			Usage: "Print the web URL for the build and return immediately",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  "staged",
			Usage: "Perform the build/test including the current staged files",
			Value: false,
		},
	},
	Action: func(cmd *cli.Context) (err error) {
		args := cmd.Args().Slice()

		if !cmd.Bool("staged") {
			out, err := run.GitCmd("diff", "--cached")
			if err != nil {
				return err
			}

			if out != "" {
				return errors.New("You have staged changes, aborting.")
			}
		}

		branch := fmt.Sprintf("bazel-do/%s", uuid.NewString())
		_, err = run.GitCmd("checkout", "-b", branch)
		if err != nil {
			return err
		}
		_, err = run.GitCmd("commit", "--allow-empty", "-m", fmt.Sprintf("!bazel %s", strings.Join(args, " ")))
		if err != nil {
			return err
		}
		_, err = run.GitCmd("push", "origin", branch)
		if err != nil {
			return err
		}
		commit, err := run.TrimResult(run.GitCmd("rev-parse", "HEAD"))
		if err != nil {
			return err
		}

		if cmd.Bool("staged") {
			// restore the changes we've commited so theyre back in the staging area
			// when we checkout the original branch again.
			_, err = run.GitCmd("reset", "--soft", "HEAD~1")
			if err != nil {
				return err
			}
		}

		_, err = run.GitCmd("checkout", "-")
		if err != nil {
			return err
		}
		_, err = run.GitCmd("branch", "-D", branch)
		if err != nil {
			return err
		}

		// give buildkite some time to kick off the build so that we can find it later on
		time.Sleep(10 * time.Second)
		client, err := bk.NewClient(cmd.Context, std.Out)
		if err != nil {
			return err
		}
		build, err := client.TriggerBuild(cmd.Context, "sourcegraph", branch, commit, bk.WithEnvVar("DISABLE_ASPECT_WORKFLOWS", "true"))
		if err != nil {
			return err
		}

		if cmd.Bool("web") {
			if err := open.URL(*build.WebURL); err != nil {
				std.Out.WriteWarningf("failed to open build in browser: %s", err)
			}
		}

		if cmd.Bool("wait") {
			pending := std.Out.Pending(output.Styledf(output.StylePending, "Waiting for %d jobs...", len(build.Jobs)))
			err = statusTicker(cmd.Context, fetchJobs(cmd.Context, client, &build, pending))
			if err != nil {
				return err
			}

			std.Out.WriteLine(output.Styledf(output.StylePending, "Fetching logs for %s ...", *build.WebURL))
			options := bk.ExportLogsOpts{
				JobStepKey: "bazel-do",
			}
			logs, err := client.ExportLogs(cmd.Context, "sourcegraph", *build.Number, options)
			if err != nil {
				return err
			}
			if len(logs) == 0 {
				std.Out.WriteLine(output.Line("", output.StyleSuggestion,
					fmt.Sprintf("No logs found matching the given parameters (job: %q, state: %q).", options.JobQuery, options.State)))
				return nil
			}

			for _, entry := range logs {
				std.Out.Write(*entry.Content)
			}

			pending.Destroy()
			if err != nil {
				return err
			}
		}
		std.Out.WriteLine(output.Styledf(output.StyleBold, "Build URL: %s", *build.WebURL))
		return nil
	},
}

var statusCommand = &cli.Command{
	Name:    "status",
	Aliases: []string{"st"},
	Usage:   "Get the status of the CI run associated with the currently checked out branch",
	Flags: append(ciTargetFlags,
		&cli.BoolFlag{
			Name:  "wait",
			Usage: "Wait by blocking until the build is finished",
		},
		&cli.BoolFlag{
			Name:    "web",
			Aliases: []string{"view", "w"},
			Usage:   "Open build page in web browser (--view is DEPRECATED and will be removed in the future)",
		}),
	Action: func(cmd *cli.Context) error {
		client, err := bk.NewClient(cmd.Context, std.Out)
		if err != nil {
			return err
		}
		target, err := getBuildTarget(cmd)
		if err != nil {
			return err
		}

		// Just support main pipeline for now
		build, err := target.GetBuild(cmd.Context, client)
		if err != nil {
			return err
		}

		// Print a high level overview, and jump into a browser
		printBuildOverview(build)
		if cmd.Bool("view") {
			if err := open.URL(*build.WebURL); err != nil {
				std.Out.WriteWarningf("failed to open build in browser: %s", err)
			}
		}

		// If we are waiting and unfinished, poll for a build
		if cmd.Bool("wait") && build.FinishedAt == nil {
			if build.Branch == nil {
				return errors.Newf("build %d not associated with a branch", *build.Number)
			}

			pending := std.Out.Pending(output.Styledf(output.StylePending, "Waiting for %d jobs...", len(build.Jobs)))
			err := statusTicker(cmd.Context, fetchJobs(cmd.Context, client, &build, pending))
			pending.Destroy()
			if err != nil {
				return err
			}
		}

		// lets get annotations (if any) for the build
		var annotations bk.JobAnnotations
		annotations, err = client.GetJobAnnotationsByBuildNumber(cmd.Context, "sourcegraph", strconv.Itoa(*build.Number))
		if err != nil {
			return errors.Newf("failed to get annotations for build %d: %w", *build.Number, err)
		}

		// render resutls
		failed := printBuildResults(build, annotations, cmd.Bool("wait"))

		// If we're not on a specific branch and not asking for a specific build,
		// warn if build commit is not your local copy - we are building an
		// unknown revision.
		if !target.fromFlag && target.targetType == buildTargetTypeBranch {
			commit, err := run.GitCmd("rev-parse", "HEAD")
			if err != nil {
				return err
			}
			commit = strings.TrimSpace(commit)
			if commit != *build.Commit {
				std.Out.WriteLine(output.Linef("⚠️", output.StyleSuggestion,
					"The currently checked out commit %q does not match the commit of the build found, %q.\nHave you pushed your most recent changes yet?",
					commit, *build.Commit))
			}
		}

		if failed {
			std.Out.WriteLine(output.Linef(output.EmojiLightbulb, output.StyleSuggestion,
				"Some jobs have failed - try using 'sg ci logs' to see what went wrong, or go to the build page: %s", *build.WebURL))
		}

		return nil
	},
}

var buildCommand = &cli.Command{
	Name:      "build",
	ArgsUsage: "[runtype] <argument>",
	Usage:     "Manually request a build for the currently checked out commit and branch (e.g. to trigger builds on forks or with special run types)",
	Description: fmt.Sprintf(`
Optionally provide a run type to build with.

This command is useful when:

- you want to trigger a build with a particular run type, such as 'main-dry-run'
- triggering builds for PRs from forks (such as those from external contributors), which do not trigger Buildkite builds automatically for security reasons (we do not want to run insecure code on our infrastructure by default!)

Supported run types when providing an argument for 'sg ci build [runtype]':

* %s

For run types that require branch arguments, you will be prompted for an argument, or you
can provide it directly (for example, 'sg ci build [runtype] <argument>').`,
		strings.Join(getAllowedBuildTypeArgs(), "\n* ")),
	UsageText: `
# Start a main-dry-run build
sg ci build main-dry-run

# Publish a custom image build
sg ci build docker-images-patch

# Publish a custom Prometheus image build without running tests
sg ci build docker-images-patch-notest prometheus

# Publish all images without testing
sg ci build docker-images-candidates-notest
`,
	BashComplete: completions.CompleteArgs(getAllowedBuildTypeArgs),
	Flags: []cli.Flag{
		&ciPipelineFlag,
		&cli.StringFlag{
			Name:    "commit",
			Aliases: []string{"c"},
			Usage:   "`commit` from the current branch to build (defaults to current commit)",
		},
	},
	Action: func(cmd *cli.Context) error {
		ctx := cmd.Context
		client, err := bk.NewClient(ctx, std.Out)
		if err != nil {
			return err
		}

		branch, err := run.TrimResult(run.GitCmd("branch", "--show-current"))
		if err != nil {
			return err
		}

		commit := cmd.String("commit")
		if commit == "" {
			commit, err = run.TrimResult(run.GitCmd("rev-parse", "HEAD"))
			if err != nil {
				return err
			}
		}

		rt := runtype.PullRequest
		// 🚨 SECURITY: We do a simple check to see if commit is in origin, this is
		// non blocking but we ask for confirmation to double check that the user
		// is aware that potentially unknown code is going to get run on our infra.
		if !repo.HasCommit(ctx, commit) {
			std.Out.WriteLine(output.Linef(output.EmojiWarning, output.StyleReset,
				"Commit %q not found in in local 'origin/' branches - you might be triggering a build for a fork. Make sure all code has been reviewed before continuing.",
				commit))
			response, err := open.Prompt("Continue? (yes/no)")
			if err != nil {
				return err
			}
			if response != "yes" {
				return errors.New("Cancelling request.")
			}
			branch = fmt.Sprintf("ext_%s", commit)
			rt = runtype.ManuallyTriggered
		}

		if cmd.NArg() > 0 {
			rt = runtype.Compute("", fmt.Sprintf("%s/%s", cmd.Args().First(), branch), nil)
			// If a special runtype is not detected then the argument was invalid
			if rt == runtype.PullRequest {
				std.Out.WriteFailuref("Unsupported runtype %q", cmd.Args().First())
				std.Out.Writef("Supported runtypes:\n\n\t%s\n\nSee 'sg ci docs' to learn more.", strings.Join(getAllowedBuildTypeArgs(), ", "))
				return exit.NewEmptyExitErr(1)
			}
		}
		if rt != runtype.PullRequest {
			m := rt.Matcher()
			if m.BranchArgumentRequired {
				var branchArg string
				if cmd.NArg() >= 2 {
					branchArg = cmd.Args().Get(1)
				} else {
					std.Out.Write("This run type requires a branch path argument.")
					branchArg, err = open.Prompt("Enter your argument input:")
					if err != nil {
						return err
					}
				}
				branch = fmt.Sprintf("%s/%s", branchArg, branch)
			}

			branch = fmt.Sprintf("%s%s", rt.Matcher().Branch, branch)
			block := std.Out.Block(output.Line("", output.StylePending, fmt.Sprintf("Pushing %s to %s...", commit, branch)))
			gitOutput, err := run.GitCmd("push", "origin", fmt.Sprintf("%s:refs/heads/%s", commit, branch), "--force")
			if err != nil {
				return err
			}
			block.WriteLine(output.Line("", output.StyleSuggestion, strings.TrimSpace(gitOutput)))
			block.Close()
		}

		var (
			pipeline = ciPipelineFlag.Get(cmd)
			build    *buildkite.Build
		)
		if rt != runtype.PullRequest {
			pollTicker := time.NewTicker(5 * time.Second)
			std.Out.WriteLine(output.Styledf(output.StylePending, "Polling for build for branch %s at %s...", branch, commit))
			for range 30 {
				// attempt to fetch the new build - it might take some time for the hooks so we will
				// retry up to 30 times (roughly 30 seconds)
				if build != nil && build.Commit != nil && *build.Commit == commit {
					break
				}
				<-pollTicker.C
				build, err = client.GetMostRecentBuild(ctx, pipeline, branch)
				if err != nil {
					return errors.Wrap(err, "GetMostRecentBuild")
				}
			}
		} else {
			std.Out.WriteLine(output.Styledf(output.StylePending, "Requesting build for branch %q at %q...", branch, commit))
			build, err = client.TriggerBuild(ctx, pipeline, branch, commit)
			if err != nil {
				return errors.Newf("failed to trigger build for branch %q at %q: %w", branch, commit, err)
			}
		}

		std.Out.WriteLine(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Created build: %s", *build.WebURL))
		return nil
	},
}

var logsCommand = &cli.Command{
	Name:  "logs",
	Usage: "Get logs from CI builds (e.g. to grep locally)",
	Description: `Get logs from CI builds, and output them in stdout or push them to Loki. By default only gets failed jobs - to change this, use the '--state' flag.

The '--job' flag can be used to narrow down the logs returned - you can provide either the ID, or part of the name of the job you want to see logs for.

To send logs to a Loki instance, you can provide --out=http://127.0.0.1:3100 after spinning up an instance with 'sg run loki grafana'.
From there, you can start exploring logs with the Grafana explore panel.
`,
	Flags: append(ciTargetFlags,
		&cli.StringFlag{
			Name:    "job",
			Aliases: []string{"j"},
			Usage:   "ID or name of the job to export logs for",
		},
		&cli.StringFlag{
			Name:    "state",
			Aliases: []string{"s"},
			Usage:   "Job `state` to export logs for (provide an empty value for all states)",
			Value:   "failed",
		},
		&cli.StringFlag{
			Name:    "out",
			Aliases: []string{"o"},
			Usage: fmt.Sprintf("Output `format`: one of [%s], or a URL pointing to a Loki instance, such as %s",
				strings.Join([]string{ciLogsOutTerminal, ciLogsOutSimple, ciLogsOutJSON}, "|"), loki.DefaultLokiURL),
			Value: ciLogsOutTerminal,
		},
		&cli.StringFlag{
			Name:  "overwrite-state",
			Usage: "`state` to overwrite the job state metadata",
		},
	),
	Action: func(cmd *cli.Context) error {
		ctx := cmd.Context
		client, err := bk.NewClient(ctx, std.Out)
		if err != nil {
			return err
		}

		target, err := getBuildTarget(cmd)
		if err != nil {
			return err
		}

		build, err := target.GetBuild(ctx, client)
		if err != nil {
			return err
		}
		std.Out.WriteLine(output.Styledf(output.StylePending, "Fetching logs for %s ...",
			*build.WebURL))

		options := bk.ExportLogsOpts{
			JobQuery: cmd.String("job"),
			State:    cmd.String("state"),
		}
		logs, err := client.ExportLogs(ctx, "sourcegraph", *build.Number, options)
		if err != nil {
			return err
		}
		if len(logs) == 0 {
			std.Out.WriteLine(output.Line("", output.StyleSuggestion,
				fmt.Sprintf("No logs found matching the given parameters (job: %q, state: %q).", options.JobQuery, options.State)))
			return nil
		}

		logsOut := cmd.String("out")
		switch logsOut {
		case ciLogsOutTerminal, ciLogsOutSimple:
			// Buildkite's timestamp thingo causes log lines to not render in terminal
			bkTimestamp := regexp.MustCompile(`\x1b_bk;t=\d{13}\x07`) // \x1b is ESC, \x07 is BEL
			for _, log := range logs {
				block := std.Out.Block(output.Linef(output.EmojiInfo, output.StyleUnderline, "%s",
					*log.JobMeta.Name))
				content := bkTimestamp.ReplaceAllString(*log.Content, "")
				if logsOut == ciLogsOutSimple {
					content = bk.CleanANSI(content)
				}
				block.Write(content)
				block.Close()
			}
			std.Out.WriteLine(output.Styledf(output.StyleSuccess, "Found and output logs for %d jobs.", len(logs)))

		case ciLogsOutJSON:
			for _, log := range logs {
				if logsOut != "" {
					failed := logsOut
					log.JobMeta.State = &failed
				}
				stream, err := loki.NewStreamFromJobLogs(log)
				if err != nil {
					return errors.Newf("build %d job %s: NewStreamFromJobLogs: %s", log.JobMeta.Build, log.JobMeta.Job, err)
				}
				b, err := json.MarshalIndent(stream, "", "\t")
				if err != nil {
					return errors.Newf("build %d job %s: Marshal: %s", log.JobMeta.Build, log.JobMeta.Job, err)
				}
				std.Out.Write(string(b))
			}

		default:
			lokiURL, err := url.Parse(logsOut)
			if err != nil {
				return errors.Newf("invalid Loki target: %w", err)
			}
			lokiClient := loki.NewLokiClient(lokiURL)
			std.Out.WriteLine(output.Styledf(output.StylePending, "Pushing to Loki instance at %q", lokiURL.Host))

			var (
				pushedEntries int
				pushedStreams int
				pushErrs      []string
				pending       = std.Out.Pending(output.Styled(output.StylePending, "Processing logs..."))
			)
			for i, log := range logs {
				job := log.JobMeta.Job
				if log.JobMeta.Label != nil {
					job = fmt.Sprintf("%q (%s)", *log.JobMeta.Label, log.JobMeta.Job)
				}
				overwriteState := cmd.String("overwrite-state")
				if overwriteState != "" {
					failed := overwriteState
					log.JobMeta.State = &failed
				}

				pending.Updatef("Processing build %d job %s (%d/%d)...",
					log.JobMeta.Build, job, i, len(logs))
				stream, err := loki.NewStreamFromJobLogs(log)
				if err != nil {
					pushErrs = append(pushErrs, fmt.Sprintf("build %d job %s: %s",
						log.JobMeta.Build, job, err))
					continue
				}

				// Set buildkite metadata if available
				if ciBranch := os.Getenv("BUILDKITE_BRANCH"); ciBranch != "" {
					stream.Stream.Branch = ciBranch
				}
				if ciQueue := os.Getenv("BUILDKITE_AGENT_META_DATA_QUEUE"); ciQueue != "" {
					stream.Stream.Queue = ciQueue
				}

				err = lokiClient.PushStreams(ctx, []*loki.Stream{stream})
				if err != nil {
					pushErrs = append(pushErrs, fmt.Sprintf("build %d job %q: %s",
						log.JobMeta.Build, job, err))
					continue
				}

				pushedEntries += len(stream.Values)
				pushedStreams += 1
			}

			if pushedEntries > 0 {
				pending.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess,
					"Pushed %d entries from %d streams to Loki", pushedEntries, pushedStreams))
			} else {
				pending.Destroy()
			}

			if pushErrs != nil {
				failedStreams := len(logs) - pushedStreams
				std.Out.WriteLine(output.Linef(output.EmojiFailure, output.StyleWarning,
					"Failed to push %d streams: \n - %s", failedStreams, strings.Join(pushErrs, "\n - ")))
				if failedStreams == len(logs) {
					return errors.New("failed to push all logs")
				}
			}
		}

		return nil
	},
}

var docsCommand = &cli.Command{
	Name:  "docs",
	Usage: "Render reference documentation for build pipeline types",
	Action: func(ctx *cli.Context) error {
		cmd := exec.Command("go", "run", "./dev/ci/gen-pipeline.go", "-docs")
		out, err := run.InRoot(cmd)
		if err != nil {
			return err
		}
		return std.Out.WriteMarkdown(out)
	},
}

var openCommand = &cli.Command{
	Name:      "open",
	ArgsUsage: "[pipeline]",
	Usage:     "Open Sourcegraph's Buildkite page in browser",
	Action: func(ctx *cli.Context) error {
		buildkiteURL := fmt.Sprintf("https://buildkite.com/%s", bk.BuildkiteOrg)
		args := ctx.Args().Slice()
		if len(args) > 0 && args[0] != "" {
			pipeline := args[0]
			buildkiteURL += fmt.Sprintf("/%s", pipeline)
		}
		return open.URL(buildkiteURL)
	},
}
