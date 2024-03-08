package ci

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver"
	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
)

func bazelBuildExecutorVM(c Config, alwaysRebuild bool) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		imageFamily := executorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Key(candidateImageStepKey("executor.vm-image")),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.RFC795InternalRelease))),
		}

		cmd := bazelStampedCmd("run //cmd/executor/vm-image:ami.build")

		if !alwaysRebuild {
			stepOpts = append(stepOpts,
				// Soft-fail with code 222 if nothing has changed
				bk.SoftFail(222),
				bk.Cmd("if ! ./cmd/executor/ci-should-rebuild.sh; then exit 222; fi"))
		}
		stepOpts = append(stepOpts, bk.Cmd(cmd))
		pipeline.AddStep(":bazel::packer: :construction: Build executor image", stepOpts...)
	}
}

func bazelPublishExecutorVM(c Config, alwaysRebuild bool) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		// TODO(rfc795) because we don't have a real testbed for those, let's not push
		// those amis for now.
		if strings.Contains(c.Branch, "rfc795/v") {
			return
		}

		imageFamily := executorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.DependsOn(candidateImageStepKey("executor.vm-image")),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.RFC795InternalRelease))),
		}

		cmd := bazelStampedCmd("run //cmd/executor/vm-image:ami.push")

		if !alwaysRebuild {
			stepOpts = append(stepOpts,
				// Soft-fail with code 222 if nothing has changed
				bk.SoftFail(222),
				bk.Cmd("if ! ./cmd/executor/ci-should-rebuild.sh; then exit 222; fi"))
		}

		stepOpts = append(stepOpts, bk.Cmd(cmd))

		pipeline.AddStep(":bazel::packer: :white_check_mark: Publish executor image", stepOpts...)
	}
}

func bazelBuildExecutorDockerMirror(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		imageFamily := executorDockerMirrorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Key(candidateImageStepKey("executor-docker-miror.vm-image")),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.RFC795InternalRelease))),
			bk.Cmd(bazelStampedCmd("run //cmd/executor/docker-mirror:ami.build")),
		}
		pipeline.AddStep(":bazel::packer: :construction: Build docker registry mirror image", stepOpts...)
	}
}

func bazelPublishExecutorDockerMirror(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		// TODO(rfc795) because we don't have a real testbed for those, let's not push
		// those amis for now.
		if strings.Contains(c.Branch, "rfc795/v") {
			return
		}
		candidateBuildStep := candidateImageStepKey("executor-docker-miror.vm-image")
		imageFamily := executorDockerMirrorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.DependsOn(candidateBuildStep),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.RFC795InternalRelease))),
			bk.Cmd(bazelStampedCmd("run //cmd/executor/docker-mirror:ami.push")),
		}
		pipeline.AddStep(":bazel::packer: :white_check_mark: Publish docker registry mirror image", stepOpts...)
	}
}

func bazelPublishExecutorBinary(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		stepOpts := []bk.StepOpt{
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Env("VERSION", c.Version),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.RFC795InternalRelease))),
			bk.Cmd(bazelStampedCmd(`run //cmd/executor:binary.push`)),
		}
		pipeline.AddStep(":bazel::arrow_heading_up: Publish executor binary", stepOpts...)
	}
}

// executorDockerMirrorImageFamilyForConfig returns the image family to be used for the build.
// This defaults to `-nightly`, and will be `-$MAJOR-$MINOR` for a tagged release
// build.
func executorDockerMirrorImageFamilyForConfig(c Config) string {
	imageFamily := "sourcegraph-executors-docker-mirror-nightly"
	if c.RunType.Is(runtype.TaggedRelease, runtype.RFC795InternalRelease) {
		ver, err := semver.NewVersion(c.Version)
		if err != nil {
			panic("cannot parse version")
		}
		imageFamily = fmt.Sprintf("sourcegraph-executors-docker-mirror-%d-%d", ver.Major(), ver.Minor())
	}
	return imageFamily
}

// executorImageFamilyForConfig returns the image family to be used for the build.
// This defaults to `-nightly`, and will be `-$MAJOR-$MINOR` for a tagged release
// build.
func executorImageFamilyForConfig(c Config) string {
	imageFamily := "sourcegraph-executors-nightly"
	if c.RunType.Is(runtype.TaggedRelease, runtype.RFC795InternalRelease) {
		ver, err := semver.NewVersion(c.Version)
		if err != nil {
			panic("cannot parse version")
		}
		imageFamily = fmt.Sprintf("sourcegraph-executors-%d-%d", ver.Major(), ver.Minor())
	}
	return imageFamily
}

func executorsE2E(candidateTag string) operations.Operation {
	return func(p *bk.Pipeline) {
		p.AddStep(":bazel::docker::packer: Executors E2E",
			// Run tests against the candidate server image
			bk.DependsOn("bazel-push-images-candidate"),
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Env("CANDIDATE_VERSION", candidateTag),
			bk.Env("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:7080"),
			bk.Env("SOURCEGRAPH_SUDO_USER", "admin"),
			bk.Env("TEST_USER_EMAIL", "test@sourcegraph.com"),
			bk.Env("TEST_USER_PASSWORD", "supersecurepassword"),
			bk.Cmd("dev/ci/integration/executors/run.sh"),
			bk.ArtifactPaths("./*.log"),
		)
	}
}
