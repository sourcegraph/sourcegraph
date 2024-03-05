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
		bk.Agent("queue", "bazel"),
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

func bazelTest(targets ...string) func(*bk.Pipeline) {
	cmds := []bk.StepOpt{
		bk.DependsOn("bazel-prechecks"),
		bk.AllowDependencyFailure(),
		bk.Agent("queue", "bazel"),
		bk.Key("bazel-tests"),
		bk.ArtifactPaths("./bazel-testlogs/cmd/embeddings/shared/shared_test/*.log", "./command.profile.gz"),
		bk.AutomaticRetry(1), // TODO @jhchabran flaky stuff are breaking builds
	}

	// Test commands
	bazelTestCmds := []bk.StepOpt{}

	cmds = append(cmds, bazelApplyPrecheckChanges())

	for _, target := range targets {
		cmd := bazelCmd(fmt.Sprintf("test %s", target))
		bazelTestCmds = append(bazelTestCmds,
			bazelAnnouncef("bazel test %s", target),
			bk.Cmd(cmd))
	}
	cmds = append(cmds, bazelTestCmds...)

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":bazel: Tests",
			cmds...,
		)
	}
}

func bazelTestWithDepends(optional bool, dependsOn string, targets ...string) func(*bk.Pipeline) {
	cmds := []bk.StepOpt{
		bk.Agent("queue", "bazel"),
	}

	bazelCmd := bazelCmd(fmt.Sprintf("test %s", strings.Join(targets, " ")))
	cmds = append(cmds, bk.Cmd(bazelCmd))
	cmds = append(cmds, bk.DependsOn(dependsOn))

	return func(pipeline *bk.Pipeline) {
		if optional {
			cmds = append(cmds, bk.SoftFail())
		}
		pipeline.AddStep(":bazel: Tests",
			cmds...,
		)
	}
}

func bazelCmd(args ...string) string {
	pre := []string{
		"bazel",
		"--bazelrc=.bazelrc",
		"--bazelrc=.aspect/bazelrc/ci.bazelrc",
		"--bazelrc=.aspect/bazelrc/ci.sourcegraph.bazelrc",
	}
	Cmd := append(pre, args...)
	return strings.Join(Cmd, " ")
}

func bazelStampedCmd(args ...string) string {
	pre := []string{
		"bazel",
		"--bazelrc=.bazelrc",
		"--bazelrc=.aspect/bazelrc/ci.bazelrc",
		"--bazelrc=.aspect/bazelrc/ci.sourcegraph.bazelrc",
	}
	post := []string{
		"--stamp",
		"--workspace_status_command=./dev/bazel_stamp_vars.sh",
	}

	cmd := append(pre, args...)
	cmd = append(cmd, post...)
	return strings.Join(cmd, " ")
}

func bazelPrechecks() func(*bk.Pipeline) {
	cmds := []bk.StepOpt{
		bk.Key("bazel-prechecks"),
		bk.SoftFail(100),
		bk.Agent("queue", "bazel"),
		bk.ArtifactPaths("./bazel-configure.diff", "./sg"),
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

func bazelApplyPrecheckChanges() bk.StepOpt {
	return bk.Cmd("dev/ci/bazel-prechecks-apply.sh")
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
	"--subcommands":          {},
}

var bazelFlagsRe = regexp.MustCompile(`--\w+`)

func verifyBazelCommand(command string) error {
	// check for shell escape mechanisms.
	if strings.Contains(command, ";") {
		return errors.New("unauthorized input for bazel command: ';'")
	}
	if strings.Contains(command, "&") {
		return errors.New("unauthorized input for bazel command: '&'")
	}
	if strings.Contains(command, "|") {
		return errors.New("unauthorized input for bazel command: '|'")
	}
	if strings.Contains(command, "$") {
		return errors.New("unauthorized input for bazel command: '$'")
	}
	if strings.Contains(command, "`") {
		return errors.New("unauthorized input for bazel command: '`'")
	}
	if strings.Contains(command, ">") {
		return errors.New("unauthorized input for bazel command: '>'")
	}
	if strings.Contains(command, "<") {
		return errors.New("unauthorized input for bazel command: '<'")
	}
	if strings.Contains(command, "(") {
		return errors.New("unauthorized input for bazel command: '('")
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
	case "aquery":
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
