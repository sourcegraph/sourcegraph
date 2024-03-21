package ci

import (
	"fmt"
	"strconv"

	"github.com/Masterminds/semver"

	"github.com/sourcegraph/sourcegraph/dev/ci/images"
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
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.InternalRelease))),
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
		imageFamily := executorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.DependsOn(candidateImageStepKey("executor.vm-image")),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.InternalRelease))),
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
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.InternalRelease))),
			bk.Cmd(bazelStampedCmd("run //cmd/executor/docker-mirror:ami.build")),
		}
		pipeline.AddStep(":bazel::packer: :construction: Build docker registry mirror image", stepOpts...)
	}
}

func bazelPublishExecutorDockerMirror(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		candidateBuildStep := candidateImageStepKey("executor-docker-miror.vm-image")
		imageFamily := executorDockerMirrorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.DependsOn(candidateBuildStep),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.InternalRelease))),
			bk.Env("RELEASE_INTERNAL", strconv.FormatBool(c.RunType.Is(runtype.InternalRelease))),
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
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease, runtype.InternalRelease))),
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
	if c.RunType.Is(runtype.TaggedRelease) {
		ver, err := semver.NewVersion(c.Version)
		if err != nil {
			panic("cannot parse version")
		}
		imageFamily = fmt.Sprintf("sourcegraph-executors-docker-mirror-%d-%d", ver.Major(), ver.Minor())
	}

	if c.RunType.Is(runtype.InternalRelease) {
		ver, err := semver.NewVersion(c.Version)
		if err != nil {
			panic("cannot parse version")
		}
		imageFamily = fmt.Sprintf("sourcegraph-executors-internal-docker-mirror-%d-%d-%d", ver.Major(), ver.Minor(), ver.Patch())
	}
	return imageFamily
}

// executorImageFamilyForConfig returns the image family to be used for the build.
// This defaults to `-nightly`, and will be `-$MAJOR-$MINOR` for a tagged release
// build.
func executorImageFamilyForConfig(c Config) string {
	imageFamily := "sourcegraph-executors-nightly"
	if c.RunType.Is(runtype.TaggedRelease) {
		ver, err := semver.NewVersion(c.Version)
		if err != nil {
			panic("cannot parse version")
		}
		imageFamily = fmt.Sprintf("sourcegraph-executors-%d-%d", ver.Major(), ver.Minor())
	}
	if c.RunType.Is(runtype.InternalRelease) {
		ver, err := semver.NewVersion(c.Version)
		if err != nil {
			panic("cannot parse version")
		}
		imageFamily = fmt.Sprintf("sourcegraph-executors-internal-%d-%d-%d", ver.Major(), ver.Minor(), ver.Patch())
	}
	return imageFamily
}

func executorsE2E(c Config) operations.Operation {
	registry := images.SourcegraphDockerDevRegistry
	if c.RunType.Is(runtype.CloudEphemeral) {
		registry = images.CloudEphemeralRegistry
	}

	return func(p *bk.Pipeline) {
		p.AddStep(":bazel::docker::packer: Executors E2E",
			bk.DependsOn("bazel-push-images-candidate"),
			bk.Agent("queue", AspectWorkflows.QueueDefault),
			bk.Env("REGISTRY", registry),
			bk.Env("CANDIDATE_VERSION", c.candidateImageTag()),
			bk.Env("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:7080"),
			bk.Env("SOURCEGRAPH_SUDO_USER", "admin"),
			bk.Env("TEST_USER_EMAIL", "test@sourcegraph.com"),
			bk.Env("TEST_USER_PASSWORD", "supersecurepassword"),
			bk.Cmd("dev/ci/integration/executors/run.sh"),
			bk.ArtifactPaths("./*.log")) // Run tests against the candidate server image
	}
}
