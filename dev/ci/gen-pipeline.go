// gen-pipeline.go generates a Buildkite YAML file that tests the entire
// Sourcegraph application and writes it to stdout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/grafana/regexp"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/changed"
	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
	"github.com/sourcegraph/sourcegraph/internal/hostname"
)

var preview bool
var wantYaml bool
var docs bool

func init() {
	flag.BoolVar(&preview, "preview", false, "Preview the pipeline steps")
	flag.BoolVar(&wantYaml, "yaml", false, "Use YAML instead of JSON")
	flag.BoolVar(&docs, "docs", false, "Render generated documentation")
}

func main() {
	flag.Parse()

	liblog := log.Init(log.Resource{
		Name:       "buildkite-ci",
		Version:    "",
		InstanceID: hostname.Get(),
	}, log.NewSentrySinkWith(
		log.SentrySink{
			ClientOptions: sentry.ClientOptions{
				Dsn:        os.Getenv("CI_SENTRY_DSN"),
				SampleRate: 1, //send all
			},
		},
	))
	defer liblog.Sync()

	logger := log.Scoped("gen-pipeline")

	if docs {
		renderPipelineDocs(logger, os.Stdout)
		return
	}

	config := ci.NewConfig(time.Now())

	pipeline, err := ci.GeneratePipeline(config)
	if err != nil {
		logger.Fatal("failed to generate pipeline", log.Error(err))
	}

	if preview {
		previewPipeline(os.Stdout, config, pipeline)
		return
	}

	if wantYaml {
		_, err = pipeline.WriteYAMLTo(os.Stdout)
	} else {
		_, err = pipeline.WriteJSONTo(os.Stdout)
	}
	if err != nil {
		logger.Fatal("failed to write pipeline out to stdout", log.Error(err), log.Bool("wantYaml", wantYaml))
	}
}

func isAspectWorkflowsEnabled() bool {
	value, ok := os.LookupEnv("DISABLE_ASPECT_WORKFLOWS")
	if ok && value == "true" {
		return false
	}

	return true
}

func previewPipeline(w io.Writer, c ci.Config, pipeline *buildkite.Pipeline) {
	fmt.Fprintf(w, "- **Detected run type:** %s\n", c.RunType.String())
	fmt.Fprintf(w, "- **Detected diffs:** %s\n", c.Diff.String())
	fmt.Fprintf(w, "- **Computed variables:**\n")
	fmt.Fprintf(w, "  - VERSION=%s\n", c.Version)
	fmt.Fprintf(w, "- **Computed build steps:**\n")

	if c.RunType != runtype.BazelDo && isAspectWorkflowsEnabled() {
		// The reason we hard code the Aspect steps here is because we have no control over the Aspect steps
		// that get generated, so we rather just specify that there will be Aspect Workflow steps instead of
		// running the risk of hardcoding all the steps and the rendered steps getting out of sync with what
		// is ACTUALLY running on the agent.
		steps := []any{&buildkite.Pipeline{
			Group: buildkite.Group{Group: "Aspect Workflow specific steps"},
			Steps: []any{&buildkite.Step{Label: "🤖 Generated steps that include Buildifier, Gazelle, Test and Integration/E2E tests"}},
		}}
		// Flipping the order so that the Aspect steps appear first
		pipeline.Steps = append(steps, pipeline.Steps...)
	}
	printPipeline(w, "", pipeline)
}

func printPipeline(w io.Writer, prefix string, pipeline *buildkite.Pipeline) {
	if pipeline.Group.Group != "" {
		fmt.Fprintf(w, "%s- **%s**\n", prefix, pipeline.Group.Group)
	}
	for _, raw := range pipeline.Steps {
		switch v := raw.(type) {
		case *buildkite.Step:
			printStep(w, prefix, v)
		case *buildkite.Pipeline:
			printPipeline(w, prefix+"\t", v)
		}
	}
}

func printStep(w io.Writer, prefix string, step *buildkite.Step) {
	fmt.Fprintf(w, "%s\t- %s", prefix, step.Label)
	switch {
	case len(step.DependsOn) > 5:
		fmt.Fprintf(w, " → _depends on %s, ... (%d more steps)_", strings.Join(step.DependsOn[0:5], ", "), len(step.DependsOn)-5)
	case len(step.DependsOn) > 0:
		fmt.Fprintf(w, " → _depends on %s_", strings.Join(step.DependsOn, " "))
	}
	fmt.Fprintln(w)
}

var emojiRegexp = regexp.MustCompile(`:(\S*):`)

func trimEmoji(s string) string {
	return strings.TrimSpace(emojiRegexp.ReplaceAllString(s, ""))
}

func renderPipelineDocs(logger log.Logger, w io.Writer) {
	fmt.Fprintln(w, "# Pipeline types reference")
	fmt.Fprintln(w, "\nThis is a reference outlining what CI pipelines we generate under different conditions.")
	fmt.Fprintln(w, "\nTo preview the pipeline for your branch, use `sg ci preview`.")
	fmt.Fprintln(w, "\nFor a higher-level overview, please refer to the [continuous integration docs](https://sourcegraph.com/docs/dev/background-information/ci).")

	fmt.Fprintln(w, "\n## Run types")

	// Introduce pull request pipelines first
	fmt.Fprintf(w, "\n### %s\n\n", runtype.PullRequest.String())
	fmt.Fprintln(w, "The default run type.")
	changed.ForEachDiffType(func(diff changed.Diff) {
		pipeline, err := ci.GeneratePipeline(ci.Config{
			RunType: runtype.PullRequest,
			Diff:    diff,
		})
		if err != nil {
			logger.Fatal("generating pipeline for diff", log.Error(err), log.Uint32("diff", uint32(diff)))
		}
		fmt.Fprintf(w, "\n- Pipeline for `%s` changes:\n", diff)
		for _, raw := range pipeline.Steps {
			printStepSummary(w, "  ", raw)
		}
	})

	// Introduce the others
	for rt := runtype.PullRequest + 1; rt < runtype.None; rt += 1 {
		fmt.Fprintf(w, "\n### %s\n\n", rt.String())
		if m := rt.Matcher(); m == nil {
			fmt.Fprintln(w, "No matcher defined")
		} else {
			conditions := []string{}
			if m.Branch != "" {
				matchName := fmt.Sprintf("`%s`", m.Branch)
				if m.BranchRegexp {
					matchName += " (regexp match)"
				}
				if m.BranchExact {
					matchName += " (exact match)"
				}
				conditions = append(conditions, fmt.Sprintf("branches matching %s", matchName))
				if m.BranchArgumentRequired {
					conditions = append(conditions, "requires a branch argument in the second branch path segment")
				}
			}
			if m.TagPrefix != "" {
				conditions = append(conditions, fmt.Sprintf("tags starting with `%s`", m.TagPrefix))
			}
			if len(m.EnvIncludes) > 0 {
				env, _ := json.Marshal(m.EnvIncludes)
				conditions = append(conditions, fmt.Sprintf("environment including `%s`", string(env)))
			}
			fmt.Fprintf(w, "The run type for %s.\n", strings.Join(conditions, ", "))

			// We currently support 'sg ci build' commands for certain branch matcher types
			if m.IsBranchPrefixMatcher() {
				fmt.Fprintf(w, "You can create a build of this run type for your changes using:\n\n```sh\nsg ci build %s\n```\n",
					strings.TrimRight(m.Branch, "/"))
			}

			// Don't generate a preview for more complicated branch types, since we don't
			// know what arguments to provide as a sample in advance.
			if m.BranchArgumentRequired || rt.Is(runtype.BazelDo) {
				continue
			}

			// Generate a sample pipeline with all changes. If it panics just don't bother
			// generating a sample for now - we should have other tests to ensure this
			// does not happen.
			func() {
				defer func() {
					if err := recover(); err != nil {
						fmt.Fprintf(w, "\n<!--\n%+v\n-->\n", err)
					}
				}()

				pipeline, err := ci.GeneratePipeline(ci.Config{
					RunType: rt,
					Branch:  m.Branch,
					// Let generated reference docs be a subset of steps that are
					// guaranteed to be in the pipeline, rather than a superset, which
					// can be surprising.
					//
					// In the future we might want to be more clever about this to
					// generate more accurate docs for runtypes that run conditional steps.
					Diff: changed.None,
					// Make sure version parsing works.
					Version: "v1.1.1",
				})
				if err != nil {
					logger.Fatal("generating pipeline for RunType", log.String("runType", rt.String()), log.Error(err))
				}
				fmt.Fprint(w, "\nBase pipeline (more steps might be included based on branch changes):\n\n")
				for _, raw := range pipeline.Steps {
					printStepSummary(w, "", raw)
				}
			}()
		}
	}
}

func printStepSummary(w io.Writer, indent string, rawStep any) {
	switch v := rawStep.(type) {
	case *buildkite.Step:
		fmt.Fprintf(w, "%s- %s\n", indent, trimEmoji(v.Label))
	case *buildkite.Pipeline:
		var steps []string
		for _, step := range v.Steps {
			s, ok := step.(*buildkite.Step)
			if ok {
				steps = append(steps, trimEmoji(s.Label))
			}
		}
		fmt.Fprintf(w, "%s- **%s**: %s\n", indent, v.Group.Group, strings.Join(steps, ", "))
	}
}
