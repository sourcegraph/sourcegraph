package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"

	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
	"github.com/sourcegraph/sourcegraph/enterprise/dev/ci/images"
	bk "github.com/sourcegraph/sourcegraph/enterprise/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/enterprise/dev/ci/internal/ci/changed"
	"github.com/sourcegraph/sourcegraph/enterprise/dev/ci/internal/ci/operations"
)

// CoreTestOperationsOptions should be used ONLY to adjust the behaviour of specific steps,
// e.g. by adding flags, and not as a condition for adding steps or commands.
type CoreTestOperationsOptions struct {
	// for clientChromaticTests
	ChromaticShouldAutoAccept bool
	MinimumUpgradeableVersion string
	ForceReadyForReview       bool

	CacheBundleSize      bool // for addWebAppEnterpriseBuild
	CreateBundleSizeDiff bool // for addWebAppEnterpriseBuild

	IsMainBranch bool
}

// CoreTestOperations is a core set of tests that should be run in most CI cases. More
// notably, this is what is used to define operations that run on PRs. Please read the
// following notes:
//
//   - opts should be used ONLY to adjust the behaviour of specific steps, e.g. by adding
//     flags and not as a condition for adding steps or commands.
//   - be careful not to add duplicate steps.
//
// If the conditions for the addition of an operation cannot be expressed using the above
// arguments, please add it to the switch case within `GeneratePipeline` instead.
func CoreTestOperations(buildOpts bk.BuildOptions, diff changed.Diff, opts CoreTestOperationsOptions) *operations.Set {
	// Base set
	ops := operations.NewSet()

	// If the only thing that has change is the Client Jetbrains, then we skip:
	// - BazelOperations
	// - Sg Lint
	if diff.Only(changed.ClientJetbrains) {
		return ops
	}

	// Simple, fast-ish linter checks
	ops.Append(BazelOperations(buildOpts, opts.IsMainBranch)...)
	linterOps := operations.NewNamedSet("Linters and static analysis")
	if targets := changed.GetLinterTargets(diff); len(targets) > 0 {
		linterOps.Append(addSgLints(targets))
	}
	ops.Merge(linterOps)

	if diff.Has(changed.Client | changed.GraphQL) {
		// If there are any Graphql changes, they are impacting the client as well.
		clientChecks := operations.NewNamedSet("Client checks",
			clientChromaticTests(opts),
			addWebAppEnterpriseBuild(opts),
			addJetBrainsUnitTests, // ~2.5m
			addVsceTests,          // ~3.0m
			addStylelint,
		)
		ops.Merge(clientChecks)
	}

	return ops
}

// addSgLints runs linters for the given targets.
func addSgLints(targets []string) func(pipeline *bk.Pipeline) {
	cmd := "go run ./dev/sg "

	if retryCount := os.Getenv("BUILDKITE_RETRY_COUNT"); retryCount != "" && retryCount != "0" {
		cmd = cmd + "-v "
	}

	var (
		branch = os.Getenv("BUILDKITE_BRANCH")
		tag    = os.Getenv("BUILDKITE_TAG")
		// evaluates what type of pipeline run this is
		runType = runtype.Compute(tag, branch, map[string]string{
			"BEXT_NIGHTLY":    os.Getenv("BEXT_NIGHTLY"),
			"RELEASE_NIGHTLY": os.Getenv("RELEASE_NIGHTLY"),
			"VSCE_NIGHTLY":    os.Getenv("VSCE_NIGHTLY"),
		})
	)

	formatCheck := ""
	if runType.Is(runtype.MainBranch) || runType.Is(runtype.MainDryRun) {
		formatCheck = "--skip-format-check "
	}

	cmd = cmd + "lint -annotations -fail-fast=false " + formatCheck + strings.Join(targets, " ")

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(":pineapple::lint-roller: Run sg lint",
			withPnpmCache(),
			bk.AnnotatedCmd(cmd, bk.AnnotatedCmdOpts{
				Annotations: &bk.AnnotationOpts{
					IncludeNames: true,
					Type:         bk.AnnotationTypeAuto,
				},
			}))
	}
}

// Adds the terraform scanner step.  This executes very quickly ~6s
// func addTerraformScan(pipeline *bk.Pipeline) {
//	pipeline.AddStep(":lock: Checkov Terraform scanning",
//		bk.Cmd("dev/ci/ci-checkov.sh"),
//		bk.SoftFail(222))
// }

// Adds Typescript check.
func addTypescriptCheck(pipeline *bk.Pipeline) {
	pipeline.AddStep(":typescript: Build TS",
		withPnpmCache(),
		bk.Cmd("dev/ci/pnpm-run.sh build-ts"))
}

func addStylelint(pipeline *bk.Pipeline) {
	pipeline.AddStep(":stylelint: Stylelint (all)",
		withPnpmCache(),
		bk.Cmd("dev/ci/pnpm-run.sh lint:css:all"))
}

func addWebAppTests(opts CoreTestOperationsOptions) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		// Webapp tests
		pipeline.AddStep(":jest::globe_with_meridians: Test (client/web)",
			withPnpmCache(),
			bk.AnnotatedCmd("dev/ci/pnpm-test.sh client/web", bk.AnnotatedCmdOpts{
				TestReports: &bk.TestReportOpts{
					TestSuiteKeyVariableName: "BUILDKITE_ANALYTICS_FRONTEND_UNIT_TEST_SUITE_API_KEY",
				},
			}),
			bk.Cmd("dev/ci/codecov.sh -c -F typescript -F unit"))
	}
}

// Webapp enterprise build
func addWebAppEnterpriseBuild(opts CoreTestOperationsOptions) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		commit := os.Getenv("BUILDKITE_COMMIT")

		cmds := []bk.StepOpt{
			withPnpmCache(),
			bk.Cmd("dev/ci/pnpm-build.sh client/web"),
			bk.Env("NODE_ENV", "production"),
			bk.Env("CHECK_BUNDLESIZE", "1"),
			// Emit a stats.json file for bundle size diffs
			bk.Env("WEBPACK_EXPORT_STATS", "true"),
		}

		if opts.CacheBundleSize {
			cmds = append(cmds, withBundleSizeCache(commit))
		}

		if opts.CreateBundleSizeDiff {
			cmds = append(cmds, bk.Cmd("pnpm --filter @sourcegraph/web run report-bundle-diff"))
		}

		pipeline.AddStep(":webpack::globe_with_meridians::moneybag: Enterprise build", cmds...)
	}
}

var browsers = []string{"chrome"}

func getParallelTestCount(webParallelTestCount int) int {
	return webParallelTestCount + len(browsers)
}

// Builds and tests the VS Code extensions.
func addVsceTests(pipeline *bk.Pipeline) {
	pipeline.AddStep(
		":vscode: Tests for VS Code extension",
		withPnpmCache(),
		bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
		bk.Cmd("pnpm generate"),
		bk.Cmd("pnpm --filter @sourcegraph/vscode run build:test"),
		// TODO: fix integrations tests and re-enable: https://github.com/sourcegraph/sourcegraph/issues/40891
		// bk.Cmd("pnpm --filter @sourcegraph/vscode run test-integration --verbose"),
		// bk.AutomaticRetry(1),
	)
}

func addBrowserExtensionIntegrationTests(parallelTestCount int) operations.Operation {
	testCount := getParallelTestCount(parallelTestCount)
	return func(pipeline *bk.Pipeline) {
		for _, browser := range browsers {
			pipeline.AddStep(
				fmt.Sprintf(":%s: Puppeteer tests for %s extension", browser, browser),
				withPnpmCache(),
				bk.Env("EXTENSION_PERMISSIONS_ALL_URLS", "true"),
				bk.Env("BROWSER", browser),
				bk.Env("LOG_BROWSER_CONSOLE", "false"),
				bk.Env("SOURCEGRAPH_BASE_URL", "https://sourcegraph.com"),
				bk.Env("POLLYJS_MODE", "replay"), // ensure that we use existing recordings
				bk.Env("PERCY_ON", "true"),
				bk.Env("PERCY_PARALLEL_TOTAL", strconv.Itoa(testCount)),
				bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
				bk.Cmd("pnpm --filter @sourcegraph/browser run build"),
				bk.Cmd("pnpm run cover-browser-integration"),
				bk.Cmd("pnpm nyc report -r json"),
				bk.Cmd("dev/ci/codecov.sh -c -F typescript -F integration"),
				bk.ArtifactPaths("./puppeteer/*.png"),
			)
		}
	}
}

func recordBrowserExtensionIntegrationTests(pipeline *bk.Pipeline) {
	for _, browser := range browsers {
		pipeline.AddStep(
			fmt.Sprintf(":%s: Puppeteer tests for %s extension", browser, browser),
			withPnpmCache(),
			bk.Env("EXTENSION_PERMISSIONS_ALL_URLS", "true"),
			bk.Env("BROWSER", browser),
			bk.Env("LOG_BROWSER_CONSOLE", "false"),
			bk.Env("SOURCEGRAPH_BASE_URL", "https://sourcegraph.com"),
			bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
			bk.Cmd("pnpm --filter @sourcegraph/browser run build"),
			bk.Cmd("pnpm --filter @sourcegraph/browser run record-integration"),
			// Retry may help in case if command failed due to hitting the rate limit or similar kind of error on the code host:
			// https://docs.github.com/en/rest/reference/search#rate-limit
			bk.AutomaticRetry(1),
			bk.ArtifactPaths("./puppeteer/*.png"),
		)
	}
}

func addBrowserExtensionUnitTests(pipeline *bk.Pipeline) {
	pipeline.AddStep(":jest::chrome: Test (client/browser)",
		withPnpmCache(),
		bk.AnnotatedCmd("dev/ci/pnpm-test.sh client/browser", bk.AnnotatedCmdOpts{
			TestReports: &bk.TestReportOpts{
				TestSuiteKeyVariableName: "BUILDKITE_ANALYTICS_FRONTEND_UNIT_TEST_SUITE_API_KEY",
			},
		}),
		bk.Cmd("dev/ci/codecov.sh -c -F typescript -F unit"))
}

func addJetBrainsUnitTests(pipeline *bk.Pipeline) {
	pipeline.AddStep(":java: Build (client/jetbrains)",
		withPnpmCache(),
		bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
		bk.Cmd("pnpm generate"),
		bk.Cmd("pnpm --filter @sourcegraph/jetbrains run build"),
	)
}

func clientIntegrationTests(pipeline *bk.Pipeline) {
	chunkSize := 2
	prepStepKey := "puppeteer:prep"
	// TODO check with Valery about this. Because we're running stateless agents,
	// this runs on a fresh instance and the hooks are not present at all, which
	// breaks the step.
	// skipGitCloneStep := bk.Plugin("uber-workflow/run-without-clone", "")

	// Build web application used for integration tests to share it between multiple parallel steps.
	pipeline.AddStep(":puppeteer::electric_plug: Puppeteer tests prep",
		withPnpmCache(),
		bk.Key(prepStepKey),
		bk.Env("NODE_ENV", "production"),
		bk.Env("INTEGRATION_TESTS", "true"),
		bk.Env("COVERAGE_INSTRUMENT", "true"),
		bk.Cmd("dev/ci/pnpm-build.sh client/web"),
		bk.Cmd("dev/ci/create-client-artifact.sh"))

	// Chunk web integration tests to save time via parallel execution.
	chunkedTestFiles := getChunkedWebIntegrationFileNames(chunkSize)
	chunkCount := len(chunkedTestFiles)
	parallelTestCount := getParallelTestCount(chunkCount)

	addBrowserExtensionIntegrationTests(chunkCount)(pipeline)

	// Add pipeline step for each chunk of web integrations files.
	for i, chunkTestFiles := range chunkedTestFiles {
		stepLabel := fmt.Sprintf(":puppeteer::electric_plug: Puppeteer tests chunk #%s", fmt.Sprint(i+1))

		pipeline.AddStep(stepLabel,
			withPnpmCache(),
			bk.DependsOn(prepStepKey),
			bk.DisableManualRetry("The Percy build is not finalized if one of the concurrent agents fails. To retry correctly, restart the entire pipeline."),
			bk.Env("PERCY_ON", "true"),
			// If PERCY_PARALLEL_TOTAL is set, the API will wait for that many finalized builds to finalize the Percy build.
			// https://docs.percy.io/docs/parallel-test-suites#how-it-works
			bk.Env("PERCY_PARALLEL_TOTAL", strconv.Itoa(parallelTestCount)),
			bk.Cmd(fmt.Sprintf(`dev/ci/pnpm-web-integration.sh "%s"`, chunkTestFiles)),
			bk.ArtifactPaths("./puppeteer/*.png"))
	}
}

func clientChromaticTests(opts CoreTestOperationsOptions) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		stepOpts := []bk.StepOpt{
			withPnpmCache(),
			bk.AutomaticRetry(3),
			bk.Cmd("./dev/ci/pnpm-install-with-retry.sh"),
			bk.Cmd("pnpm gulp generate"),
			bk.Env("MINIFY", "1"),
		}

		// Upload storybook to Chromatic
		chromaticCommand := "pnpm chromatic --exit-zero-on-changes --exit-once-uploaded --build-script-name=storybook:build"
		if opts.ChromaticShouldAutoAccept {
			chromaticCommand += " --auto-accept-changes"
		} else {
			// Unless we plan on automatically accepting these changes, we only run this
			// step on ready-for-review pull requests.
			stepOpts = append(stepOpts, bk.IfReadyForReview(opts.ForceReadyForReview))
			chromaticCommand += " | ./dev/ci/post-chromatic.sh"
		}

		pipeline.AddStep(":chromatic: Upload Storybook to Chromatic",
			append(stepOpts, bk.Cmd(chromaticCommand))...)
	}
}

// Adds the frontend tests (without the web app and browser extension tests).
func frontendTests(pipeline *bk.Pipeline) {
	// Shared tests
	pipeline.AddStep(":jest: Test (all)",
		withPnpmCache(),
		bk.AnnotatedCmd("dev/ci/pnpm-test.sh --testPathIgnorePatterns client/web client/browser", bk.AnnotatedCmdOpts{
			TestReports: &bk.TestReportOpts{
				TestSuiteKeyVariableName: "BUILDKITE_ANALYTICS_FRONTEND_UNIT_TEST_SUITE_API_KEY",
			},
		}),
		bk.Cmd("dev/ci/codecov.sh -c -F typescript -F unit"))
}

func addBrowserExtensionE2ESteps(pipeline *bk.Pipeline) {
	for _, browser := range []string{"chrome"} {
		// Run e2e tests
		pipeline.AddStep(fmt.Sprintf(":%s: E2E for %s extension", browser, browser),
			withPnpmCache(),
			bk.Env("EXTENSION_PERMISSIONS_ALL_URLS", "true"),
			bk.Env("BROWSER", browser),
			bk.Env("LOG_BROWSER_CONSOLE", "true"),
			bk.Env("SOURCEGRAPH_BASE_URL", "https://sourcegraph.com"),
			bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
			bk.Cmd("pnpm --filter @sourcegraph/browser run build"),
			bk.Cmd("pnpm mocha ./client/browser/src/end-to-end/github.test.ts ./client/browser/src/end-to-end/gitlab.test.ts"),
			bk.ArtifactPaths("./puppeteer/*.png"))
	}
}

// Release the browser extension.
func addBrowserExtensionReleaseSteps(pipeline *bk.Pipeline) {
	addBrowserExtensionE2ESteps(pipeline)

	pipeline.AddWait()

	// Release to the Chrome Webstore
	pipeline.AddStep(":rocket::chrome: Extension release",
		withPnpmCache(),
		bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
		bk.Cmd("pnpm --filter @sourcegraph/browser run build"),
		bk.Cmd("pnpm --filter @sourcegraph/browser release:chrome"))

	// Build and self sign the FF add-on and upload it to a storage bucket
	pipeline.AddStep(":rocket::firefox: Extension release",
		withPnpmCache(),
		bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
		bk.Cmd("pnpm --filter @sourcegraph/browser release:firefox"))

	// Release to npm
	pipeline.AddStep(":rocket::npm: npm Release",
		withPnpmCache(),
		bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
		bk.Cmd("pnpm --filter @sourcegraph/browser run build"),
		bk.Cmd("pnpm --filter @sourcegraph/browser release:npm"))
}

// Release the VS Code extension.
func addVsceReleaseSteps(pipeline *bk.Pipeline) {
	// Publish extension to the VS Code Marketplace
	pipeline.AddStep(":vscode: Extension release",
		withPnpmCache(),
		bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
		bk.Cmd("pnpm generate"),
		bk.Cmd("pnpm --filter @sourcegraph/vscode run release"))
}

// Release a snapshot of App.
func addAppReleaseSteps(c Config, insiders bool) operations.Operation {
	// The version scheme we use for App is one of:
	//
	// * yyyy.mm.dd+$BUILDNUM.$COMMIT
	// * yyyy.mm.dd-insiders+$BUILDNUM.$COMMIT
	//
	// We do not follow the Sourcegraph enterprise versioning scheme, because Cody App is
	// released much more frequently than the enterprise versions by nature of being a desktop
	// app.
	//
	// Also note that goreleaser requires the version is semver-compatible.
	insidersStr := ""
	if insiders {
		insidersStr = "-insiders"
	}
	version := fmt.Sprintf("%s%s+%d.%.6s", c.Time.Format("2006.01.02"), insidersStr, c.BuildNumber, c.Commit)

	return func(pipeline *bk.Pipeline) {
		// Release App (.zip/.deb/.rpm to Google Cloud Storage, new tap for Homebrew, etc.).
		pipeline.AddStep(":desktop_computer: App release",
			withPnpmCache(),
			bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
			bk.Env("VERSION", version),
			bk.Cmd("enterprise/dev/ci/scripts/release-app.sh"))
	}
}

// Adds a Buildkite pipeline "Wait".
func wait(pipeline *bk.Pipeline) {
	pipeline.AddWait()
}

// Trigger the async pipeline to run. See pipeline.async.yaml.
func triggerAsync(buildOptions bk.BuildOptions) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddTrigger(":snail: Trigger async", "sourcegraph-async",
			bk.Key("trigger:async"),
			bk.Async(true),
			bk.Build(buildOptions),
		)
	}
}

func triggerReleaseBranchHealthchecks(minimumUpgradeableVersion string) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		version := semver.MustParse(minimumUpgradeableVersion)

		// HACK: we can't just subtract a single minor version once we roll over to 4.0,
		// so hard-code the previous minor version.
		previousMinorVersion := fmt.Sprintf("%d.%d", version.Major(), version.Minor()-1)
		if version.Major() == 4 && version.Minor() == 0 {
			previousMinorVersion = "3.43"
		} else if version.Major() == 5 && version.Minor() == 0 {
			previousMinorVersion = "4.5"
		}

		for _, branch := range []string{
			// Most recent major.minor
			fmt.Sprintf("%d.%d", version.Major(), version.Minor()),
			previousMinorVersion,
		} {
			name := fmt.Sprintf(":stethoscope: Trigger %s release branch healthcheck build", branch)
			pipeline.AddTrigger(name, "sourcegraph",
				bk.Async(false),
				bk.Build(bk.BuildOptions{
					Branch:  branch,
					Message: time.Now().Format(time.RFC1123) + " healthcheck build",
				}),
			)
		}
	}
}

func codeIntelQA(candidateTag string) operations.Operation {
	return func(p *bk.Pipeline) {
		p.AddStep(":bazel::docker::brain: Code Intel QA",
			bk.SlackStepNotify(&bk.SlackStepNotifyConfigPayload{
				Message:     ":alert: :noemi-handwriting: Code Intel QA Flake detected <@Noah S-C>",
				ChannelName: "code-intel-buildkite",
				Conditions: bk.SlackStepNotifyPayloadConditions{
					Failed: true,
				},
			}),
			// Run tests against the candidate server image
			bk.DependsOn(candidateImageStepKey("server")),
			bk.Agent("queue", "bazel"),
			bk.Env("CANDIDATE_VERSION", candidateTag),
			bk.Env("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:7080"),
			bk.Env("SOURCEGRAPH_SUDO_USER", "admin"),
			bk.Env("TEST_USER_EMAIL", "test@sourcegraph.com"),
			bk.Env("TEST_USER_PASSWORD", "supersecurepassword"),
			bk.Cmd("dev/ci/integration/code-intel/run.sh"),
			bk.ArtifactPaths("./*.log"),
			bk.SoftFail(1))
	}
}

func executorsE2E(candidateTag string) operations.Operation {
	return func(p *bk.Pipeline) {
		p.AddStep(":bazel::docker::packer: Executors E2E",
			// Run tests against the candidate server image
			bk.DependsOn("bazel-push-images-candidate"),
			bk.Agent("queue", "bazel"),
			bk.Env("CANDIDATE_VERSION", candidateTag),
			bk.Env("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:7080"),
			bk.Env("SOURCEGRAPH_SUDO_USER", "admin"),
			bk.Env("TEST_USER_EMAIL", "test@sourcegraph.com"),
			bk.Env("TEST_USER_PASSWORD", "supersecurepassword"),
			// See enterprise/dev/ci/integration/executors/docker-compose.yaml
			// This enable the executor to reach the dind container
			// for docker commands.
			bk.Env("DOCKER_GATEWAY_HOST", "172.17.0.1"),
			bk.Cmd("enterprise/dev/ci/integration/executors/run.sh"),
			bk.ArtifactPaths("./*.log"),
		)
	}
}

func serverQA(candidateTag string) operations.Operation {
	return func(p *bk.Pipeline) {
		p.AddStep(":docker::chromium: Sourcegraph QA",
			// Run tests against the candidate server image
			bk.DependsOn(candidateImageStepKey("server")),
			bk.Env("CANDIDATE_VERSION", candidateTag),
			bk.Env("DISPLAY", ":99"),
			bk.Env("LOG_STATUS_MESSAGES", "true"),
			bk.Env("NO_CLEANUP", "false"),
			bk.Env("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:7080"),
			bk.Env("SOURCEGRAPH_SUDO_USER", "admin"),
			bk.Env("TEST_USER_EMAIL", "test@sourcegraph.com"),
			bk.Env("TEST_USER_PASSWORD", "supersecurepassword"),
			bk.Env("INCLUDE_ADMIN_ONBOARDING", "false"),
			bk.AnnotatedCmd("dev/ci/integration/qa/run.sh", bk.AnnotatedCmdOpts{
				Annotations: &bk.AnnotationOpts{},
			}),
			bk.ArtifactPaths("./*.png", "./*.mp4", "./*.log"))
	}
}

func testUpgrade(candidateTag, minimumUpgradeableVersion string) operations.Operation {
	return func(p *bk.Pipeline) {
		p.AddStep(":docker::arrow_double_up: Sourcegraph Upgrade",
			// Run tests against the candidate server image
			bk.DependsOn("bazel-push-images-candidate"),
			bk.Env("CANDIDATE_VERSION", candidateTag),
			bk.Env("MINIMUM_UPGRADEABLE_VERSION", minimumUpgradeableVersion),
			bk.Env("DISPLAY", ":99"),
			bk.Env("LOG_STATUS_MESSAGES", "true"),
			bk.Env("NO_CLEANUP", "false"),
			bk.Env("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:7080"),
			bk.Env("SOURCEGRAPH_SUDO_USER", "admin"),
			bk.Env("TEST_USER_EMAIL", "test@sourcegraph.com"),
			bk.Env("TEST_USER_PASSWORD", "supersecurepassword"),
			bk.Env("INCLUDE_ADMIN_ONBOARDING", "false"),
			bk.Cmd("dev/ci/integration/upgrade/run.sh"),
			bk.ArtifactPaths("./*.png", "./*.mp4", "./*.log"))
	}
}

func clusterQA(candidateTag string) operations.Operation {
	var dependencies []bk.StepOpt
	for _, image := range images.DeploySourcegraphDockerImages {
		dependencies = append(dependencies, bk.DependsOn(candidateImageStepKey(image)))
	}
	return func(p *bk.Pipeline) {
		p.AddStep(":k8s: Sourcegraph Cluster (deploy-sourcegraph) QA", append(dependencies,
			bk.Env("CANDIDATE_VERSION", candidateTag),
			bk.Env("DOCKER_CLUSTER_IMAGES_TXT", strings.Join(images.DeploySourcegraphDockerImages, "\n")),
			bk.Env("NO_CLEANUP", "false"),
			bk.Env("SOURCEGRAPH_BASE_URL", "http://127.0.0.1:7080"),
			bk.Env("SOURCEGRAPH_SUDO_USER", "admin"),
			bk.Env("TEST_USER_EMAIL", "test@sourcegraph.com"),
			bk.Env("TEST_USER_PASSWORD", "supersecurepassword"),
			bk.Env("INCLUDE_ADMIN_ONBOARDING", "false"),
			bk.Cmd("./dev/ci/integration/cluster/run.sh"),
			bk.ArtifactPaths("./*.png", "./*.mp4", "./*.log"),
		)...)
	}
}

// candidateImageStepKey is the key for the given app (see the `images` package). Useful for
// adding dependencies on a step.
func candidateImageStepKey(app string) string {
	return strings.ReplaceAll(app, ".", "-") + ":candidate"
}

// Build a candidate docker image that will re-tagged with the final
// tags once the e2e tests pass.
//
// Version is the actual version of the code, and
func buildCandidateDockerImage(app, version, tag string, uploadSourcemaps bool) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		image := strings.ReplaceAll(app, "/", "-")
		localImage := "sourcegraph/" + image + ":" + version

		cmds := []bk.StepOpt{
			bk.Key(candidateImageStepKey(app)),
			bk.Cmd(fmt.Sprintf(`echo "Building candidate %s image..."`, app)),
			bk.Env("DOCKER_BUILDKIT", "1"),
			bk.Env("IMAGE", localImage),
			bk.Env("VERSION", version),
		}

		// Add Sentry environment variables if we are building off main branch
		// to enable building the webapp with source maps enabled
		if uploadSourcemaps {
			cmds = append(cmds,
				bk.Env("SENTRY_UPLOAD_SOURCE_MAPS", "1"),
				bk.Env("SENTRY_ORGANIZATION", "sourcegraph"),
				bk.Env("SENTRY_PROJECT", "sourcegraph-dot-com"),
			)
		}

		// Allow all build scripts to emit info annotations
		buildAnnotationOptions := bk.AnnotatedCmdOpts{
			Annotations: &bk.AnnotationOpts{
				Type:         bk.AnnotationTypeInfo,
				IncludeNames: true,
			},
		}

		if _, err := os.Stat(filepath.Join("docker-images", app)); err == nil {
			// Building Docker image located under $REPO_ROOT/docker-images/
			cmds = append(cmds,
				bk.Cmd("ls -lah "+filepath.Join("docker-images", app, "build.sh")),
				bk.Cmd(filepath.Join("docker-images", app, "build.sh")))
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
				// If /enterprise/cmd/... does not exist, build just /cmd/... instead.
				if _, err := os.Stat(filepath.Join("enterprise/cmd", folder)); err != nil {
					return "cmd/" + folder
				}
				return "enterprise/cmd/" + folder
			}()
			preBuildScript := cmdDir + "/pre-build.sh"
			if _, err := os.Stat(preBuildScript); err == nil {
				// Allow all
				cmds = append(cmds, bk.AnnotatedCmd(preBuildScript, buildAnnotationOptions))
			}
			cmds = append(cmds, bk.AnnotatedCmd(cmdDir+"/build.sh", buildAnnotationOptions))
		}

		devImage := images.DevRegistryImage(app, tag)
		cmds = append(cmds,
			// Retag the local image for dev registry
			bk.Cmd(fmt.Sprintf("docker tag %s %s", localImage, devImage)),
			// Publish tagged image
			bk.Cmd(fmt.Sprintf("docker push %s || exit 10", devImage)),
			// Retry in case of flakes when pushing
			bk.AutomaticRetryStatus(3, 10),
			// Retry in case of flakes when pushing
			bk.AutomaticRetryStatus(3, 222),
		)
		pipeline.AddStep(fmt.Sprintf(":docker: :construction: Build %s", app), cmds...)
	}
}

// Run a Sonarcloud scanning step in Buildkite
func sonarcloudScan() operations.Operation {
	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(
			"Sonarcloud Scan",
			bk.Cmd("dev/ci/sonarcloud-scan.sh"),
		)
	}

}

// Ask trivy, a security scanning tool, to scan the candidate image
// specified by "app" and "tag".
func trivyScanCandidateImage(app, tag string) operations.Operation {
	// hack to prevent trivy scanes of blobstore and server images due to timeouts,
	// even with extended deadlines
	if app == "blobstore" || app == "server" {
		return func(pipeline *bk.Pipeline) {
			// no-op
		}
	}

	image := images.DevRegistryImage(app, tag)

	// This is the special exit code that we tell trivy to use
	// if it finds a vulnerability. This is also used to soft-fail
	// this step.
	vulnerabilityExitCode := 27

	// For most images, waiting on the server is fine. But with the recent migration to Bazel,
	// this can lead to confusing failures. This will be completely refactored soon.
	//
	// See https://github.com/sourcegraph/sourcegraph/issues/52833 for the ticket tracking
	// the cleanup once we're out of the dual building process.
	dependsOnImage := candidateImageStepKey("server")
	if app == "syntax-highlighter" {
		dependsOnImage = candidateImageStepKey("syntax-highlighter")
	}
	if app == "symbols" {
		dependsOnImage = candidateImageStepKey("symbols")
	}

	return func(pipeline *bk.Pipeline) {
		pipeline.AddStep(fmt.Sprintf(":trivy: :docker: :mag: Scan %s", app),
			// These are the first images in the arrays we use to build images
			bk.DependsOn(candidateImageStepKey("alpine-3.14")),
			bk.DependsOn(candidateImageStepKey("batcheshelper")),
			bk.DependsOn(dependsOnImage),
			bk.Cmd(fmt.Sprintf("docker pull %s", image)),

			// have trivy use a shorter name in its output
			bk.Cmd(fmt.Sprintf("docker tag %s %s", image, app)),

			bk.Env("IMAGE", app),
			bk.Env("VULNERABILITY_EXIT_CODE", fmt.Sprintf("%d", vulnerabilityExitCode)),
			bk.ArtifactPaths("./*-security-report.html"),
			bk.SoftFail(vulnerabilityExitCode),
			bk.AutomaticRetryStatus(1, 1), // exit status 1 is what happens this flakes on container pulling

			bk.AnnotatedCmd("./dev/ci/trivy/trivy-scan-high-critical.sh", bk.AnnotatedCmdOpts{
				Annotations: &bk.AnnotationOpts{
					Type:            bk.AnnotationTypeWarning,
					MultiJobContext: "docker-security-scans",
				},
			}))
	}
}

// Tag and push final Docker image for the service defined by `app`
// after the e2e tests pass.
//
// It requires Config as an argument because published images require a lot of metadata.
func publishFinalDockerImage(c Config, app string) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		devImage := images.DevRegistryImage(app, "")
		publishImage := images.PublishedRegistryImage(app, "")

		var imgs []string
		for _, image := range []string{publishImage, devImage} {
			// TODO RFC 795
			if c.RunType.Is(runtype.WIPRelease) {
				if image != publishImage {
					imgs = append(imgs, fmt.Sprintf("%s:wip_insiders", image))
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
		cmd := fmt.Sprintf("./dev/ci/docker-publish.sh %s %s", candidateImage, strings.Join(imgs, " "))

		pipeline.AddStep(fmt.Sprintf(":docker: :truck: %s", app),
			// This step just pulls a prebuild image and pushes it to some registries. The
			// only possible failure here is a registry flake, so we retry a few times.
			bk.AutomaticRetry(3),
			bk.Cmd(cmd))
	}
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
	return imageFamily
}

// ~15m (building executor base VM)
func buildExecutorVM(c Config, skipHashCompare bool) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		imageFamily := executorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Key(candidateImageStepKey("executor.vm-image")),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease))),
		}
		if !skipHashCompare {
			compareHashScript := "./enterprise/dev/ci/scripts/compare-hash.sh"
			stepOpts = append(stepOpts,
				// Soft-fail with code 222 if nothing has changed
				bk.SoftFail(222),
				bk.Cmd(fmt.Sprintf("%s ./cmd/executor/hash.sh", compareHashScript)))
		}
		stepOpts = append(stepOpts,
			bk.Cmd("./cmd/executor/vm-image/build.sh"))

		pipeline.AddStep(":packer: :construction: Build executor image", stepOpts...)
	}
}

func buildExecutorBinary(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		stepOpts := []bk.StepOpt{
			bk.Key(candidateImageStepKey("executor.binary")),
			bk.Env("VERSION", c.Version),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease))),
		}
		stepOpts = append(stepOpts,
			bk.Cmd("./cmd/executor/build_binary.sh"))

		pipeline.AddStep(":construction: Build executor binary", stepOpts...)
	}
}

func publishExecutorVM(c Config, skipHashCompare bool) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		candidateBuildStep := candidateImageStepKey("executor.vm-image")
		imageFamily := executorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.DependsOn(candidateBuildStep),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease))),
		}
		if !skipHashCompare {
			// Publish iff not soft-failed on previous step
			checkDependencySoftFailScript := "./enterprise/dev/ci/scripts/check-dependency-soft-fail.sh"
			stepOpts = append(stepOpts,
				// Soft-fail with code 222 if nothing has changed
				bk.SoftFail(222),
				bk.Cmd(fmt.Sprintf("%s %s", checkDependencySoftFailScript, candidateBuildStep)))
		}
		stepOpts = append(stepOpts,
			bk.Cmd("./cmd/executor/vm-image/release.sh"))

		pipeline.AddStep(":packer: :white_check_mark: Publish executor image", stepOpts...)
	}
}

func publishExecutorBinary(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		candidateBuildStep := candidateImageStepKey("executor.binary")
		stepOpts := []bk.StepOpt{
			bk.DependsOn(candidateBuildStep),
			bk.Env("VERSION", c.Version),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease))),
		}
		stepOpts = append(stepOpts,
			bk.Cmd("./cmd/executor/release_binary.sh"))

		pipeline.AddStep(":white_check_mark: Publish executor binary", stepOpts...)
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
	return imageFamily
}

// ~15m (building executor docker mirror base VM)
func buildExecutorDockerMirror(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		imageFamily := executorDockerMirrorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.Key(candidateImageStepKey("executor-docker-miror.vm-image")),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease))),
		}
		stepOpts = append(stepOpts,
			bk.Cmd("./cmd/executor/docker-mirror/build.sh"))

		pipeline.AddStep(":packer: :construction: Build docker registry mirror image", stepOpts...)
	}
}

func publishExecutorDockerMirror(c Config) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		candidateBuildStep := candidateImageStepKey("executor-docker-miror.vm-image")
		imageFamily := executorDockerMirrorImageFamilyForConfig(c)
		stepOpts := []bk.StepOpt{
			bk.DependsOn(candidateBuildStep),
			bk.Env("VERSION", c.Version),
			bk.Env("IMAGE_FAMILY", imageFamily),
			bk.Env("EXECUTOR_IS_TAGGED_RELEASE", strconv.FormatBool(c.RunType.Is(runtype.TaggedRelease))),
		}
		stepOpts = append(stepOpts,
			bk.Cmd("./cmd/executor/docker-mirror/release.sh"))

		pipeline.AddStep(":packer: :white_check_mark: Publish docker registry mirror image", stepOpts...)
	}
}
