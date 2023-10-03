// Package ci is responsible for generating a Buildkite pipeline configuration. It is invoked by the
// gen-pipeline.go command.
package ci

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
	"github.com/sourcegraph/sourcegraph/enterprise/dev/ci/images"
	bk "github.com/sourcegraph/sourcegraph/enterprise/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/enterprise/dev/ci/internal/ci/changed"
	"github.com/sourcegraph/sourcegraph/enterprise/dev/ci/internal/ci/operations"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var legacyDockerImages = []string{
	"dind",
	"executor-vm",

	// See RFC 793, those images will be dropped in 5.1.x.
	"alpine-3.14",
	"codeinsights-db",
	"codeintel-db",
	"postgres-12-alpine",
}

// GeneratePipeline is the main pipeline generation function. It defines the build pipeline for each of the
// main CI cases, which are defined in the main switch statement in the function.
func GeneratePipeline(c Config) (*bk.Pipeline, error) {
	if err := c.ensureCommit(); err != nil {
		return nil, err
	}

	// Common build env
	env := map[string]string{
		// Build meta
		"BUILDKITE_PULL_REQUEST":             os.Getenv("BUILDKITE_PULL_REQUEST"),
		"BUILDKITE_PULL_REQUEST_BASE_BRANCH": os.Getenv("BUILDKITE_PULL_REQUEST_BASE_BRANCH"),
		"BUILDKITE_PULL_REQUEST_REPO":        os.Getenv("BUILDKITE_PULL_REQUEST_REPO"),
		"COMMIT_SHA":                         c.Commit,
		"DATE":                               c.Time.Format(time.RFC3339),
		"VERSION":                            c.Version,

		// Go flags
		"GO111MODULE": "on",

		// Additional flags
		"FORCE_COLOR": "3",
		// Add debug flags for scripts to consume
		"CI_DEBUG_PROFILE": strconv.FormatBool(c.MessageFlags.ProfilingEnabled),
		// Bump Node.js memory to prevent OOM crashes
		"NODE_OPTIONS": "--max_old_space_size=8192",

		// Bundlesize configuration: https://github.com/siddharthkp/bundlesize2#build-status-and-checks-for-github
		"CI_REPO_OWNER": "sourcegraph",
		"CI_REPO_NAME":  "sourcegraph",
		"CI_COMMIT_SHA": os.Getenv("BUILDKITE_COMMIT"),
		// $ in commit messages must be escaped to not attempt interpolation which will fail.
		"CI_COMMIT_MESSAGE": strings.ReplaceAll(os.Getenv("BUILDKITE_MESSAGE"), "$", "$$"),

		// HoneyComb dataset that stores build traces.
		"CI_BUILDEVENT_DATASET": "buildkite",
	}
	bk.FeatureFlags.ApplyEnv(env)

	// On release branches Percy must compare to the previous commit of the release branch, not main.
	if c.RunType.Is(runtype.ReleaseBranch, runtype.TaggedRelease) {
		env["PERCY_TARGET_BRANCH"] = c.Branch
		// When we are building a release, we do not want to cache the client bundle.
		//
		// This is a defensive measure, as caching the client bundle is tricky when it comes to invalidating it.
		// This makes sure that we're running integration tests on a fresh bundle and, the image
		// that 99% of our customers are using is exactly the same as the other deployments.
		env["SERVER_NO_CLIENT_BUNDLE_CACHE"] = "true"
	}

	// Build options for pipeline operations that spawn more build steps
	buildOptions := bk.BuildOptions{
		Message: os.Getenv("BUILDKITE_MESSAGE"),
		Commit:  c.Commit,
		Branch:  c.Branch,
		Env:     env,
	}

	// Test upgrades from mininum upgradeable Sourcegraph version - updated by release tool
	const minimumUpgradeableVersion = "5.1.0"

	// Set up operations that add steps to a pipeline.
	ops := operations.NewSet()

	// This statement outlines the pipeline steps for each CI case.
	//
	// PERF: Try to order steps such that slower steps are first.
	switch c.RunType {
	case runtype.BazelDo:
		// parse the commit message, looking for the bazel command to run
		var bzlCmd string
		scanner := bufio.NewScanner(strings.NewReader(env["CI_COMMIT_MESSAGE"]))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "!bazel") {
				bzlCmd = strings.TrimSpace(strings.TrimPrefix(line, "!bazel"))

				// sanitize the input
				if err := verifyBazelCommand(bzlCmd); err != nil {
					return nil, errors.Wrapf(err, "cannot generate bazel-do")
				}

				ops.Append(func(pipeline *bk.Pipeline) {
					pipeline.AddStep(":bazel::desktop_computer: bazel "+bzlCmd,
						bk.Key("bazel-do"),
						bk.Agent("queue", "bazel"),
						bk.Cmd(bazelCmd(bzlCmd)),
					)
				})
				break
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		}

		if bzlCmd == "" {
			return nil, errors.Newf("no bazel command was given")
		}
	case runtype.PullRequest:
		// First, we set up core test operations that apply both to PRs and to other run
		// types such as main.
		ops.Merge(CoreTestOperations(buildOptions, c.Diff, CoreTestOperationsOptions{
			MinimumUpgradeableVersion: minimumUpgradeableVersion,
			ForceReadyForReview:       c.MessageFlags.ForceReadyForReview,
			CreateBundleSizeDiff:      true,
		}))

		securityOps := operations.NewNamedSet("Security Scanning")
		securityOps.Append(sonarcloudScan())
		ops.Merge(securityOps)

		// Wolfi package and base images
		addWolfiOps(c, ops)

		// Now we set up conditional operations that only apply to pull requests.
		if c.Diff.Has(changed.Client) {
			// triggers a slow pipeline, currently only affects web. It's optional so we
			// set it up separately from CoreTestOperations
			ops.Merge(operations.NewNamedSet(operations.PipelineSetupSetName,
				triggerAsync(buildOptions)))
		}

	case runtype.ReleaseNightly:
		ops.Append(triggerReleaseBranchHealthchecks(minimumUpgradeableVersion))

	case runtype.BextReleaseBranch:
		// If this is a browser extension release branch, run the browser-extension tests and
		// builds.
		ops = operations.NewSet(
			addBrowserExtensionUnitTests,
			addBrowserExtensionIntegrationTests(0), // we pass 0 here as we don't have other pipeline steps to contribute to the resulting Percy build
			frontendTests,
			wait,
			addBrowserExtensionReleaseSteps)

	case runtype.VsceReleaseBranch:
		// If this is a vs code extension release branch, run the vscode-extension tests and release
		ops = operations.NewSet(
			addVsceTests,
			wait,
			addVsceReleaseSteps)

	case runtype.BextNightly:
		// If this is a browser extension nightly build, run the browser-extension tests and
		// e2e tests.
		ops = operations.NewSet(
			addBrowserExtensionUnitTests,
			recordBrowserExtensionIntegrationTests,
			frontendTests,
			wait,
			addBrowserExtensionE2ESteps)

	case runtype.VsceNightly:
		// If this is a VS Code extension nightly build, run the vsce-extension integration tests
		ops = operations.NewSet(
			addVsceTests,
		)

	case runtype.AppRelease:
		ops = operations.NewSet(addAppReleaseSteps(c, false))

	case runtype.AppInsiders:
		ops = operations.NewSet(addAppReleaseSteps(c, true))

	case runtype.CandidatesNoTest:
		imageBuildOps := operations.NewNamedSet("Image builds")
		imageBuildOps.Append(bazelBuildCandidateDockerImages(legacyDockerImages, c.Version, c.candidateImageTag(), c.RunType))
		ops.Merge(imageBuildOps)

		ops.Append(wait)

		// Add final artifacts
		publishOps := operations.NewNamedSet("Publish images")
		publishOps.Append(bazelPushImagesNoTest(c.Version))

		for _, dockerImage := range legacyDockerImages {
			publishOps.Append(publishFinalDockerImage(c, dockerImage))
		}
		ops.Merge(publishOps)

	case runtype.ImagePatch:
		// only build image for the specified image in the branch name
		// see https://handbook.sourcegraph.com/engineering/deployments#building-docker-images-for-a-specific-branch
		patchImage, err := c.RunType.Matcher().ExtractBranchArgument(c.Branch)
		if err != nil {
			panic(fmt.Sprintf("ExtractBranchArgument: %s", err))
		}
		if !contains(images.SourcegraphDockerImages, patchImage) {
			panic(fmt.Sprintf("no image %q found", patchImage))
		}

		ops = operations.NewSet(
			bazelBuildCandidateDockerImage(patchImage, c.Version, c.candidateImageTag(), c.RunType),
			trivyScanCandidateImage(patchImage, c.candidateImageTag()))
		// Test images
		ops.Merge(CoreTestOperations(buildOptions, changed.All, CoreTestOperationsOptions{
			MinimumUpgradeableVersion: minimumUpgradeableVersion,
		}))
		// Publish images after everything is done
		ops.Append(
			wait,
			publishFinalDockerImage(c, patchImage))

	case runtype.ImagePatchNoTest:
		// If this is a no-test branch, then run only the Docker build. No tests are run.
		patchImage, err := c.RunType.Matcher().ExtractBranchArgument(c.Branch)
		if err != nil {
			panic(fmt.Sprintf("ExtractBranchArgument: %s", err))
		}
		if !contains(images.SourcegraphDockerImages, patchImage) {
			panic(fmt.Sprintf("no image %q found", patchImage))
		}
		ops = operations.NewSet(
			bazelBuildCandidateDockerImage(patchImage, c.Version, c.candidateImageTag(), c.RunType),
			wait,
			publishFinalDockerImage(c, patchImage))
	case runtype.ExecutorPatchNoTest:
		executorVMImage := "executor-vm"
		ops = operations.NewSet(
			bazelBuildCandidateDockerImage(executorVMImage, c.Version, c.candidateImageTag(), c.RunType),
			trivyScanCandidateImage(executorVMImage, c.candidateImageTag()),
			buildExecutorVM(c, true),
			buildExecutorDockerMirror(c),
			buildExecutorBinary(c),
			wait,
			publishFinalDockerImage(c, executorVMImage),
			publishExecutorVM(c, true),
			publishExecutorDockerMirror(c),
			publishExecutorBinary(c),
		)

	default:
		// Slow async pipeline
		ops.Merge(operations.NewNamedSet(operations.PipelineSetupSetName,
			triggerAsync(buildOptions)))

		// Executor VM image
		skipHashCompare := c.MessageFlags.SkipHashCompare || c.RunType.Is(runtype.ReleaseBranch, runtype.TaggedRelease) || c.Diff.Has(changed.ExecutorVMImage)
		// Slow image builds
		imageBuildOps := operations.NewNamedSet("Image builds")
		imageBuildOps.Append(bazelBuildCandidateDockerImages(legacyDockerImages, c.Version, c.candidateImageTag(), c.RunType))

		if c.RunType.Is(runtype.MainDryRun, runtype.MainBranch, runtype.ReleaseBranch, runtype.TaggedRelease, runtype.WIPRelease) {
			imageBuildOps.Append(buildExecutorVM(c, skipHashCompare))
			imageBuildOps.Append(buildExecutorBinary(c))
			// TODO RFC 795 Add WIPRuntype
			if c.RunType.Is(runtype.ReleaseBranch, runtype.TaggedRelease) || c.Diff.Has(changed.ExecutorDockerRegistryMirror) {
				imageBuildOps.Append(buildExecutorDockerMirror(c))
			}
		}
		ops.Merge(imageBuildOps)

		// Core tests
		ops.Merge(CoreTestOperations(buildOptions, changed.All, CoreTestOperationsOptions{
			ChromaticShouldAutoAccept: c.RunType.Is(runtype.MainBranch, runtype.ReleaseBranch, runtype.TaggedRelease),
			MinimumUpgradeableVersion: minimumUpgradeableVersion,
			ForceReadyForReview:       c.MessageFlags.ForceReadyForReview,
			CacheBundleSize:           c.RunType.Is(runtype.MainBranch, runtype.MainDryRun),
			IsMainBranch:              true,
		}))

		// Security scanning - sonarcloud
		securityOps := operations.NewNamedSet("Security Scanning")
		securityOps.Append(sonarcloudScan())
		ops.Merge(securityOps)

		// Publish candidate images to dev registry
		publishOpsDev := operations.NewNamedSet("Publish candidate images")
		publishOpsDev.Append(bazelPushImagesCandidates(c.Version))
		ops.Merge(publishOpsDev)

		// End-to-end tests
		ops.Merge(operations.NewNamedSet("End-to-end tests",
			executorsE2E(c.candidateImageTag()),
			// testUpgrade(c.candidateImageTag(), minimumUpgradeableVersion),
		))

		// Wolfi package and base images
		addWolfiOps(c, ops)

		// All operations before this point are required
		ops.Append(wait)

		// Add final artifacts
		publishOps := operations.NewNamedSet("Publish images")
		// Add final artifacts
		for _, dockerImage := range legacyDockerImages {
			publishOps.Append(publishFinalDockerImage(c, dockerImage))
		}
		// Executor VM image
		if c.RunType.Is(runtype.MainBranch, runtype.TaggedRelease) {
			publishOps.Append(publishExecutorVM(c, skipHashCompare))
			publishOps.Append(publishExecutorBinary(c))
			if c.RunType.Is(runtype.TaggedRelease) || c.Diff.Has(changed.ExecutorDockerRegistryMirror) {
				publishOps.Append(publishExecutorDockerMirror(c))
			}
		}
		// Final Bazel images
		publishOps.Append(bazelPushImagesFinal(c.Version))
		ops.Merge(publishOps)
	}

	// Construct pipeline
	pipeline := &bk.Pipeline{
		Env: env,
		AfterEveryStepOpts: []bk.StepOpt{
			withDefaultTimeout,
			withAgentQueueDefaults,
			withAgentLostRetries,
		},
	}
	// Toggle profiling of each step
	if c.MessageFlags.ProfilingEnabled {
		pipeline.AfterEveryStepOpts = append(pipeline.AfterEveryStepOpts, withProfiling)
	}

	// Apply operations on pipeline
	ops.Apply(pipeline)

	// Validate generated pipeline have unique keys
	if err := pipeline.EnsureUniqueKeys(make(map[string]int)); err != nil {
		return nil, err
	}

	return pipeline, nil
}

// withDefaultTimeout makes all command steps timeout after 60 minutes in case a buildkite
// agent got stuck / died.
func withDefaultTimeout(s *bk.Step) {
	// bk.Step is a union containing fields across all the different step types.
	// However, "timeout_in_minutes" only applies to the "command" step type.
	//
	// Testing the length of the "Command" field seems to be the most reliable way
	// of differentiating "command" steps from other step types without refactoring
	// everything.
	if len(s.Command) > 0 {
		if s.TimeoutInMinutes == "" {
			// Set the default value iff someone else hasn't set a custom one.
			s.TimeoutInMinutes = "60"
		}
	}
}

// withAgentQueueDefaults ensures all agents target a specific queue, and ensures they
// steps are configured appropriately to run on the queue
func withAgentQueueDefaults(s *bk.Step) {
	if len(s.Agents) == 0 || s.Agents["queue"] == "" {
		s.Agents["queue"] = bk.AgentQueueStateless
	}
}

// withProfiling wraps "time -v" around each command for CPU/RAM utilization information
func withProfiling(s *bk.Step) {
	var prefixed []string
	for _, cmd := range s.Command {
		prefixed = append(prefixed, fmt.Sprintf("env time -v %s", cmd))
	}
	s.Command = prefixed
}

// withAgentLostRetries insert automatic retries when the job has failed because it lost its agent.
//
// If the step has been marked as not retryable, the retry will be skipped.
func withAgentLostRetries(s *bk.Step) {
	if s.Retry != nil && s.Retry.Manual != nil && !s.Retry.Manual.Allowed {
		return
	}
	if s.Retry == nil {
		s.Retry = &bk.RetryOptions{}
	}
	if s.Retry.Automatic == nil {
		s.Retry.Automatic = []bk.AutomaticRetryOptions{}
	}
	s.Retry.Automatic = append(s.Retry.Automatic, bk.AutomaticRetryOptions{
		Limit:      1,
		ExitStatus: -1,
	})
}

// addWolfiOps adds operations to rebuild modified Wolfi packages and base images.
func addWolfiOps(c Config, ops *operations.Set) {
	// Rebuild Wolfi packages that have config changes
	var updatedPackages []string
	if c.Diff.Has(changed.WolfiPackages) {
		var packageOps *operations.Set
		packageOps, updatedPackages = WolfiPackagesOperations(c.ChangedFiles[changed.WolfiPackages])
		ops.Merge(packageOps)
	}

	// Rebuild Wolfi base images
	// Inspect package dependencies, and rebuild base images with updated packages
	_, imagesWithChangedPackages, err := GetDependenciesOfPackages(updatedPackages, "sourcegraph")
	if err != nil {
		panic(err)
	}
	// Rebuild base images with package changes AND with config changes
	imagesToRebuild := append(imagesWithChangedPackages, c.ChangedFiles[changed.WolfiBaseImages]...)
	imagesToRebuild = sortUniq(imagesToRebuild)

	if len(imagesToRebuild) > 0 {
		baseImageOps, _ := WolfiBaseImagesOperations(
			imagesToRebuild,
			c.Version,
			(len(updatedPackages) > 0),
		)
		ops.Merge(baseImageOps)
	}
}
