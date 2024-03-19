package ci

import (
	"fmt"
	"strconv"

	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
)

func addStylelint(pipeline *bk.Pipeline) {
	pipeline.AddStep(":stylelint: Stylelint (all)",
		withPnpmCache(),
		bk.Cmd("dev/ci/pnpm-run.sh lint:css:all"))
}

var browsers = []string{"chrome"}

func getParallelTestCount(webParallelTestCount int) int {
	return webParallelTestCount + len(browsers)
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
				bk.Cmd("pnpm run test-browser-integration"),
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

func clientChromaticTests(opts CoreTestOperationsOptions) operations.Operation {
	return func(pipeline *bk.Pipeline) {
		stepOpts := []bk.StepOpt{
			withPnpmCache(),
			bk.AutomaticRetry(3),
			bk.Cmd("./dev/ci/pnpm-install-with-retry.sh"),
			bk.Cmd("pnpm run generate"),
			bk.Env("MINIFY", "1"),
		}

		// Upload storybook to Chromatic
		//
		// We run chromatic through `run-chromatic` because the script detects whether a build is being retried
		// and then adds the flag `--force-rebuild`. We need to do this because Chromatic fails when running on
		// the same commit.
		chromaticCommand := "./dev/ci/run-chromatic.sh --exit-zero-on-changes --exit-once-uploaded"
		if opts.ChromaticShouldAutoAccept {
			// have to specify a value. Value can either be a branch glob or a boolean
			// see https://www.chromatic.com/docs/cli/
			chromaticCommand += " --auto-accept-changes true"
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
