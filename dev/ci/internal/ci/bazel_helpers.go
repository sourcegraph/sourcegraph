package ci

import (
	"fmt"
	"strings"

	"github.com/grafana/regexp"

	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func bazelBuild(targets ...string) func(*bk.Pipeline) {
	cmds := []bk.StepOpt{
		bk.Key("bazel_build"),
		bk.Agent("queue", AspectWorkflows.QueueDefault),
	}
	cmd := bazelStampedCmd(fmt.Sprintf("build %s", strings.Join(targets, " ")))
	cmds = append(
		cmds,
		bk.Cmd(cmd),
		bk.Cmd(bazelStampedCmd("run //cmd/server:candidate_push")),
	)

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":bazel: Build ...",
			cmds...,
		)
	}
}

func bazelCmd(args ...string) string {
	genBazelRC, bazelrc := aspectBazelRC()
	pre := []string{
		genBazelRC,
		"bazel",
		fmt.Sprintf("--bazelrc=%s", bazelrc),
	}
	Cmd := append(pre, args...)
	return strings.Join(Cmd, " ")
}

func bazelStampedCmd(args ...string) string {
	genBazelRC, bazelrc := aspectBazelRC()
	pre := []string{
		genBazelRC,
		"bazel",
		fmt.Sprintf("--bazelrc=%s", bazelrc),
		fmt.Sprintf("--bazelrc=%s", ".aspect/bazelrc/ci.sourcegraph.bazelrc"),
	}
	post := []string{
		"--stamp",
		"--workspace_status_command=./dev/bazel_stamp_vars.sh",
	}

	cmd := append(pre, args...)
	cmd = append(cmd, post...)
	return strings.Join(cmd, " ")
}

// TODO(burmudar): do we remove this?
func bazelPrechecks() func(*bk.Pipeline) {
	cmds := []bk.StepOpt{
		bk.Key("bazel-prechecks"),
		bk.SoftFail(100),
		bk.Agent("queue", AspectWorkflows.QueueDefault),
		bk.ArtifactPaths("./sg"),
		bk.AnnotatedCmd("dev/ci/bazel-prechecks.sh", bk.AnnotatedCmdOpts{
			Annotations: &bk.AnnotationOpts{
				Type:         bk.AnnotationTypeError,
				IncludeNames: false,
			},
		}),
		// We want to build sg on a bazel agent, but without the overhead
		// of its own pipeline step. After pre-checks have passed seems
		// the most natural, as we then know that the bazel files are
		// up-to-date for building sg.

		// TODO(burmudar): maybe move this to be part of gen pipeline?
		bk.Cmd("dev/ci/bazel-build-sg.sh"),
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":bazel: Bazel prechecks & build `sg`",
			cmds...,
		)
	}
}

func bazelAnnouncef(format string, args ...any) bk.StepOpt {
	msg := fmt.Sprintf(format, args...)
	return bk.Cmd(fmt.Sprintf(`echo "--- :bazel: %s"`, msg))
}

var allowedBazelFlags = map[string]struct{}{
	"--runs_per_test":        {},
	"--nobuild":              {},
	"--local_test_jobs":      {},
	"--test_arg":             {},
	"--nocache_test_results": {},
	"--test_tag_filters":     {},
	"--test_timeout":         {},
	"--config":               {},
	"--test_output":          {},
	"--verbose_failures":     {},
}

var bazelFlagsRe = regexp.MustCompile(`--\w+`)

func verifyBazelCommand(command string) error {
	// check for shell escape mechanisms.
	bannedChars := []string{"`", "$", "(", ")", ";", "&", "|", "<", ">"}
	for _, c := range bannedChars {
		if strings.Contains(command, c) {
			return errors.Newf("unauthorized input for bazel command: %q", c)
		}
	}

	// check for command and targets
	strs := strings.Split(command, " ")
	if len(strs) < 2 {
		return errors.New("invalid command")
	}

	// command must be either build or test.
	switch strs[0] {
	case "build":
	case "test":
	default:
		return errors.Newf("disallowed bazel command: %q", strs[0])
	}

	// need at least one target.
	if !strings.HasPrefix(strs[1], "//") {
		return errors.New("misconstructed command, need at least one target")
	}

	// ensure flags are in the allow-list.
	matches := bazelFlagsRe.FindAllString(command, -1)
	for _, m := range matches {
		if _, ok := allowedBazelFlags[m]; !ok {
			return errors.Newf("disallowed bazel flag: %q", m)
		}
	}
	return nil
}
