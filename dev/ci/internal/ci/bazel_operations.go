package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/dev/ci/images"
	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func BazelOperations(buildOpts bk.BuildOptions, isMain bool) []operations.Operation {
	ops := []operations.Operation{}
	ops = append(ops, bazelPrechecks())
	if isMain {
		ops = append(ops, bazelTest("//...", "//client/web:test", "//testing:codeintel_integration_test", "//testing:grpc_backend_integration_test"))
	} else {
		ops = append(ops, bazelTest("//...", "//client/web:test"))
	}

	ops = append(ops, triggerBackCompatTest(buildOpts), bazelGoModTidy())
	return ops
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

// Used in default run type
func bazelPushImagesCandidates(version string) func(*bk.Pipeline) {
	return bazelPushImagesCmd(version, true, "bazel-tests")
}

// Used in default run type
func bazelPushImagesFinal(version string) func(*bk.Pipeline) {
	return bazelPushImagesCmd(version, false, "bazel-tests")
}

// Used in CandidateNoTest run type
func bazelPushImagesNoTest(version string) func(*bk.Pipeline) {
	return bazelPushImagesCmd(version, false, "pipeline-gen")
}

func bazelPushImagesCmd(version string, isCandidate bool, depKey string) func(*bk.Pipeline) {
	stepName := ":bazel::docker: Push final images"
	stepKey := "bazel-push-images"
	candidate := ""

	if isCandidate {
		stepName = ":bazel::docker: Push candidate Images"
		stepKey = stepKey + "-candidate"
		candidate = "true"
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(stepName,
			bk.Agent("queue", "bazel"),
			bk.DependsOn(depKey),
			bk.Key(stepKey),
			bk.Env("PUSH_VERSION", version),
			bk.Env("CANDIDATE_ONLY", candidate),
			bazelApplyPrecheckChanges(),
			bk.Cmd(bazelStampedCmd(`build $$(bazel query 'kind("oci_push rule", //...)')`)),
			bk.Cmd("./dev/ci/push_all.sh"),
		)
	}
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

// bazelAnalysisPhase only runs the analasys phase, ensure that the buildfiles
// are correct, but do not actually build anything.
func bazelAnalysisPhase() func(*bk.Pipeline) {
	cmd := bazelCmd(
		"build",
		"--nobuild", // this is the key flag to enable this.
		"//...",
	)

	cmds := []bk.StepOpt{
		bk.Key("bazel-analysis"),
		bk.Agent("queue", "bazel"),
		bk.Cmd(cmd),
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":bazel: Analysis phase",
			cmds...,
		)
	}
}

func bazelPrechecks() func(*bk.Pipeline) {
	cmds := []bk.StepOpt{
		bk.Key("bazel-prechecks"),
		bk.SoftFail(100),
		bk.Agent("queue", "bazel"),
		bk.ArtifactPaths("./bazel-configure.diff"),
		bk.AnnotatedCmd("dev/ci/bazel-prechecks.sh", bk.AnnotatedCmdOpts{
			Annotations: &bk.AnnotationOpts{
				Type:         bk.AnnotationTypeError,
				IncludeNames: false,
			},
		}),
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":bazel: Perform bazel prechecks",
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

	// bazel build //client/web:bundle is very resource hungry and often crashes when ran along other targets
	// so we run it first to avoid failing builds midway.
	cmds = append(cmds,
		bazelAnnouncef("bazel build //client/web:bundle"),
		bk.Cmd(bazelCmd("build //client/web:bundle")),
	)

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

func triggerBackCompatTest(buildOpts bk.BuildOptions) func(*bk.Pipeline) {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddTrigger(":bazel::snail: Async BackCompat Tests", "sourcegraph-backcompat",
			bk.Key("trigger-backcompat"),
			bk.DependsOn("bazel-prechecks"),
			bk.AllowDependencyFailure(),
			bk.Build(buildOpts),
		)
	}
}

func bazelGoModTidy() func(*bk.Pipeline) {
	cmds := []bk.StepOpt{
		bk.Agent("queue", "bazel"),
		bk.Key("bazel-go-mod"),
		bk.Cmd("./dev/ci/bazel-gomodtidy.sh"),
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":bazel::broom: Go mod tidy", cmds...)
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

// Keep: allows building an array of images on one agent. Useful for streamlining and rules_oci in the future.
func bazelBuildCandidateDockerImages(apps []string, version string, tag string, rt runtype.RunType) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		cmds := []bk.StepOpt{}

		cmds = append(cmds,
			bk.Key(candidateImageStepKey(apps[0])),
			bk.Env("DOCKER_BAZEL", "true"),
			bk.Env("DOCKER_BUILDKIT", "1"),
			bk.Env("VERSION", version),
			bk.Agent("queue", "bazel"),
		)

		// Allow all build scripts to emit info annotations
		// TODO(JH) probably remove
		buildAnnotationOptions := bk.AnnotatedCmdOpts{
			Annotations: &bk.AnnotationOpts{
				Type:         bk.AnnotationTypeInfo,
				IncludeNames: true,
			},
		}

		for _, app := range apps {
			image := strings.ReplaceAll(app, "/", "-")
			localImage := "sourcegraph/" + image + ":" + version

			// Add Sentry environment variables if we are building off main branch
			// to enable building the webapp with source maps enabled
			if rt.Is(runtype.MainDryRun) && app == "frontend" {
				cmds = append(cmds,
					bk.Env("SENTRY_UPLOAD_SOURCE_MAPS", "1"),
					bk.Env("SENTRY_ORGANIZATION", "sourcegraph"),
					bk.Env("SENTRY_PROJECT", "sourcegraph-dot-com"),
				)
			}

			cmds = append(cmds,
				bk.Cmd(fmt.Sprintf(`echo "--- Building candidate %s image..."`, app)),
				bk.Cmd("export IMAGE='"+localImage+"'"),
			)

			if _, err := os.Stat(filepath.Join("docker-images", app)); err == nil {
				// Building Docker image located under $REPO_ROOT/docker-images/
				buildScriptPath := filepath.Join("docker-images", app, "build.sh")
				_, err := os.Stat(filepath.Join("docker-images", app, "build-bazel.sh"))
				if err == nil {
					// If the file exists.
					buildScriptPath = filepath.Join("docker-images", app, "build-bazel.sh")
				}

				cmds = append(cmds,
					bk.Cmd("ls -lah "+buildScriptPath),
					bk.Cmd(buildScriptPath),
				)
			} else if _, err := os.Stat(filepath.Join("client", app)); err == nil {
				// Building Docker image located under $REPO_ROOT/client/
				cmds = append(cmds, bk.AnnotatedCmd("client/"+app+"/build.sh", buildAnnotationOptions))
			} else {
				// Building Docker images located under $REPO_ROOT/cmd/
				cmdDir := func() string {
					folder := app
					if app == "blobstore2" {
						// experiment: cmd/blobstore is a Go rewrite of docker-images/blobstore. While
						// it is incomplete, we do not want cmd/blobstore/Dockerfile to get published
						// under the same name.
						// https://github.com/sourcegraph/sourcegraph/issues/45594
						// TODO(blobstore): remove this when making Go blobstore the default
						folder = "blobstore"
					}

					return "cmd/" + folder
				}()
				buildScriptPath := filepath.Join(cmdDir, "build.sh")
				_, err := os.Stat(filepath.Join(cmdDir, "build-bazel.sh"))
				if err == nil {
					// If the file exists.
					buildScriptPath = filepath.Join(cmdDir, "build-bazel.sh")
				}
				cmds = append(cmds, bk.AnnotatedCmd(buildScriptPath, buildAnnotationOptions))
			}

			devImage := images.DevRegistryImage(app, tag)
			cmds = append(cmds,
				bk.Cmd(fmt.Sprintf(`echo "--- Tagging and Pushing candidate %s image..."`, app)),
				// Retag the local image for dev registry
				bk.Cmd(fmt.Sprintf("docker tag %s %s", localImage, devImage)),
				// Publish tagged image
				bk.Cmd(fmt.Sprintf("docker push %s || exit 10", devImage)),
				// Retry in case of flakes when pushing
				// bk.AutomaticRetryStatus(3, 10),
				// Retry in case of flakes when pushing
				// bk.AutomaticRetryStatus(3, 222),
			)
		}
		pipeline.AddStep(":bazel::docker: :construction: Build Docker images", cmds...)
	}
}

func bazelBuildCandidateDockerImage(app string, version string, tag string, rt runtype.RunType) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		cmds := []bk.StepOpt{}
		cmds = append(cmds,
			bk.Key(candidateImageStepKey(app)),
			bk.Env("DOCKER_BAZEL", "true"),
			bk.Env("VERSION", version),
			bk.Agent("queue", "bazel"),
		)

		// Allow all build scripts to emit info annotations
		// TODO(JH) probably remove
		buildAnnotationOptions := bk.AnnotatedCmdOpts{
			Annotations: &bk.AnnotationOpts{
				Type:         bk.AnnotationTypeInfo,
				IncludeNames: true,
			},
		}

		image := strings.ReplaceAll(app, "/", "-")
		localImage := "sourcegraph/" + image + ":" + version

		// Add Sentry environment variables if we are building off main branch
		// to enable building the webapp with source maps enabled
		if rt.Is(runtype.MainDryRun) && app == "frontend" {
			cmds = append(cmds,
				bk.Env("SENTRY_UPLOAD_SOURCE_MAPS", "1"),
				bk.Env("SENTRY_ORGANIZATION", "sourcegraph"),
				bk.Env("SENTRY_PROJECT", "sourcegraph-dot-com"),
			)
		}

		cmds = append(cmds,
			bk.Cmd(fmt.Sprintf(`echo "--- Building candidate %s image..."`, app)),
			bk.Cmd("export IMAGE='"+localImage+"'"),
		)

		if _, err := os.Stat(filepath.Join("docker-images", app)); err == nil {
			// Building Docker image located under $REPO_ROOT/docker-images/
			buildScriptPath := filepath.Join("docker-images", app, "build.sh")
			_, err := os.Stat(filepath.Join("docker-images", app, "build-bazel.sh"))
			if err == nil {
				// If the file exists.
				buildScriptPath = filepath.Join("docker-images", app, "build-bazel.sh")
			}

			cmds = append(cmds,
				bk.Cmd("ls -lah "+buildScriptPath),
				bk.Cmd(buildScriptPath),
			)
		} else if _, err := os.Stat(filepath.Join("client", app)); err == nil {
			// Building Docker image located under $REPO_ROOT/client/
			cmds = append(cmds, bk.AnnotatedCmd("client/"+app+"/build.sh", buildAnnotationOptions))
		} else {
			// Building Docker images located under $REPO_ROOT/cmd/
			cmdDir := func() string {
				folder := app
				if app == "blobstore2" {
					// experiment: cmd/blobstore is a Go rewrite of docker-images/blobstore. While
					// it is incomplete, we do not want cmd/blobstore/Dockerfile to get published
					// under the same name.
					// https://github.com/sourcegraph/sourcegraph/issues/45594
					// TODO(blobstore): remove this when making Go blobstore the default
					folder = "blobstore"
				}

				return "cmd/" + folder
			}()
			buildScriptPath := filepath.Join(cmdDir, "build.sh")
			_, err := os.Stat(filepath.Join(cmdDir, "build-bazel.sh"))
			if err == nil {
				// If the file exists.
				buildScriptPath = filepath.Join(cmdDir, "build-bazel.sh")
			}
			cmds = append(cmds, bk.AnnotatedCmd(buildScriptPath, buildAnnotationOptions))
		}

		devImage := images.DevRegistryImage(app, tag)
		cmds = append(cmds,
			bk.Cmd(fmt.Sprintf(`echo "--- Tagging and Pushing candidate %s image..."`, app)),
			// Retag the local image for dev registry
			bk.Cmd(fmt.Sprintf("docker tag %s %s", localImage, devImage)),
			// Publish tagged image
			bk.Cmd(fmt.Sprintf("docker push %s || exit 10", devImage)),
			// Retry in case of flakes when pushing
			// bk.AutomaticRetryStatus(3, 10),
			// Retry in case of flakes when pushing
			// bk.AutomaticRetryStatus(3, 222),
		)
		pipeline.AddStep(fmt.Sprintf(":bazel::docker: :construction: Build %s", app), cmds...)
	}
}

// Tag and push final Docker image for the service defined by `app`
// after the e2e tests pass.
//
// It requires Config as an argument because published images require a lot of metadata.
func bazelPublishFinalDockerImage(c Config, apps []string) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		cmds := []bk.StepOpt{}
		cmds = append(cmds, bk.Agent("queue", "bazel"))

		for _, app := range apps {

			devImage := images.DevRegistryImage(app, "")
			publishImage := images.PublishedRegistryImage(app, "")

			var imgs []string
			for _, image := range []string{publishImage, devImage} {
				// TODO RFC 795
				if c.RunType.Is(runtype.WIPRelease) {
					if image != publishImage {
						imgs = append(imgs, fmt.Sprintf("%s:%s", image, c.Branch))
					}
					continue
				}

				if app != "server" || c.RunType.Is(runtype.TaggedRelease, runtype.ImagePatch, runtype.ImagePatchNoTest) {
					imgs = append(imgs, fmt.Sprintf("%s:%s", image, c.Version))
				}

				if app == "server" && c.RunType.Is(runtype.ReleaseBranch) {
					imgs = append(imgs, fmt.Sprintf("%s:%s-insiders", image, c.Branch))
				}

				if c.RunType.Is(runtype.MainBranch) {
					imgs = append(imgs, fmt.Sprintf("%s:insiders", image))
				}
			}

			// these tags are pushed to our dev registry, and are only
			// used internally
			for _, tag := range []string{
				c.Version,
				c.Commit,
				c.shortCommit(),
				fmt.Sprintf("%s_%s_%d", c.shortCommit(), c.Time.Format("2006-01-02"), c.BuildNumber),
				fmt.Sprintf("%s_%d", c.shortCommit(), c.BuildNumber),
				fmt.Sprintf("%s_%d", c.Commit, c.BuildNumber),
				strconv.Itoa(c.BuildNumber),
			} {
				internalImage := fmt.Sprintf("%s:%s", devImage, tag)
				imgs = append(imgs, internalImage)
			}

			candidateImage := fmt.Sprintf("%s:%s", devImage, c.candidateImageTag())
			cmds = append(cmds, bk.Cmd(fmt.Sprintf("./dev/ci/docker-publish.sh %s %s", candidateImage, strings.Join(imgs, " "))))
		}
		pipeline.AddStep(":docker: :truck: Publish images", cmds...)
		// This step just pulls a prebuild image and pushes it to some registries. The
		// only possible failure here is a registry flake, so we retry a few times.
		bk.AutomaticRetry(3)
	}
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
