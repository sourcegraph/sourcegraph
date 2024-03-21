package ci

import (
	bk "github.com/sourcegraph/sourcegraph/dev/ci/internal/buildkite"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/changed"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/operations"
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
	ops.Append(
		bazelPrechecks(),
		triggerBackCompatTest(buildOpts),
		bazelGoModTidy(),
	)
	linterOps := operations.NewNamedSet("Linters and static analysis")
	if targets := changed.GetLinterTargets(diff); len(targets) > 0 {
		linterOps.Append(addSgLints(targets))
	}
	ops.Merge(linterOps)

	if diff.Has(changed.Client | changed.GraphQL) {
		// If there are any Graphql changes, they are impacting the client as well.
		clientChecks := operations.NewNamedSet("Client checks",
			clientChromaticTests(opts),
			addJetBrainsUnitTests, // ~2.5m
			addVsceTests,          // ~3.0m
			addStylelint,
		)
		ops.Merge(clientChecks)
	}

	return ops
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

func addJetBrainsUnitTests(pipeline *bk.Pipeline) {
	pipeline.AddStep(":java: Build (client/jetbrains)",
		withPnpmCache(),
		bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
		bk.Cmd("pnpm generate"),
		bk.Cmd("pnpm --filter @sourcegraph/jetbrains run build"),
	)
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

// Adds a Buildkite pipeline "Wait".
func wait(pipeline *bk.Pipeline) {
	pipeline.AddWait()
}
