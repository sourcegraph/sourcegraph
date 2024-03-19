# Continuous integration

<span class="badge badge-note">SOC2/GN-105</span> <span class="badge badge-note">SOC2/GN-106</span>

Sourcegraph uses a continuous integration (CI) and delivery tool, [Buildkite](#buildkite-pipelines), to help ensure a [consistent](#pipeline-health) build, test and deploy process. Software changes are systematically required to complete all steps within the continuous integration tool workflow prior to production deployment, in addition to being [peer reviewed](../pull_request_reviews.md).

Sourcegraph also maintains a variety of tooling on [GitHub Actions](#github-actions) for continuous integration and repository maintainence purposes.

> NOTE: To learn more about testing in particular, see our [testing principles](../testing_principles.md).

<div class="getting-started">
  <a href="#buildkite-pipelines" class="btn" alt="Buildkite pipelines">
   <span>★ Buildkite pipelines</span>
   <br>
   An introduction to Sourcegraph's Buildkite pipelines.
  </a>

  <a href="./development" class="btn" alt="Development">
   <span>Development</span>
   <br>
   Contribute changes to Sourcegraph's Buildkite pipelines!
  </a>
</div>

Run `sg ci docs` to see documentation about the CI pipeline steps.


## Buildkite pipelines

[Tests](../../how-to/testing.md) are automatically run in our [various Buildkite pipelines](https://buildkite.com/sourcegraph) when you push your changes (i.e. when you run `git push`) to the `sourcegraph/sourcegraph` GitHub repository.
Pipeline steps are generated on the fly using the [pipeline generator](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/tree/dev/ci) - a complete reference of all available pipeline types and steps is available from `sg ci docs`. To keep the repository tidy, consider deleting the branch after the pipeline has completed. The build results will be available even after the branch is deleted.

To see what checks will get run against your current branch, use [`sg`](../../setup/quickstart.md):

```sh
sg ci preview
```

You can also request builds manually for your builds using `sg ci build`. You'll find below a summary video about some useful `sg ci *` commands, to learn how move fast when interacting with the CI:

<div style="position: relative; padding-bottom: 82%; height: 0;">
  <iframe src="https://www.loom.com/embed/f451d05978b34d97bdc06d411aacc69d" frameborder="0" webkitallowfullscreen mozallowfullscreen allowfullscreen style="position: absolute; top: 0; left: 0; width: 100%; height: 100%;"></iframe>
</div>

To learn about making changes to our Buildkite pipelines, see [Pipeline development](./development.md).

### Pipeline steps

#### Soft failures

<span class="badge badge-note">SOC2/GN-106</span>

Many steps in Sourcegraph's Buildkite pipelines allow for [soft failures](https://buildkite.com/changelog/56-command-steps-can-now-be-made-to-soft-fail), which means that even if they fail they do not cause the entire build to be failed.

In the Buildkite UI, soft failures currently look like the following, with a _triangular_ warning sign (not to be mistaken for a hard failure!):

![soft fail in Buildkite UI](https://user-images.githubusercontent.com/23356519/150558751-d8e0da19-0b6f-4645-aa12-7547d375330f.png)

We use soft failures for the following reasons only:

- Steps that determine whether a subsequent step should run, where soft failures are the only technical way to communicate that a later step should be skipped in this manner using Buildkite.
  - Examples: [hash comparison steps that determine if a build should run](https://sourcegraph.com/search?q=context:%40sourcegraph/all+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Edev/ci/internal/ci/operations%5C.go+compare-hash.sh&patternType=literal)
- Regular analysis tasks, where soft failures serve as an monitoring indicator to warn the team responsible for fixing issues.
  - Examples: [image vulnerability scanning](#image-vulnerability-scanning), linting tasks for catching deprecation warnings
- Temporary exceptions to accommodate experimental or in-progress work.

You can find all usages of soft failures [with the following queries](https://sourcegraph.com/notebooks/Tm90ZWJvb2s6NTc=):

- [Soft failures in the Sourcegraph pipeline generator](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%7B...bk.SoftFail...%7D+OR+%28...bk.SoftFail...%29+count:all&patternType=structural)
- [Soft failures in Buildkite YAML pipelines](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/.*+soft_fail+lang:yaml+count:all&patternType=literal)

All other failures are hard failures.

#### Image vulnerability scanning

Our CI pipeline scans uses [Trivy](https://aquasecurity.github.io/trivy/) to scan our Docker images for security vulnerabilities.
Refer to `sg ci docs` to see what pipelines Trivy checks run in.

If there are any `HIGH` or `CRITICAL` severities in a Docker image that have a known fix:

1. The CI pipeline will create an annotation that contains links to reports that describe the vulnerabilities
2. The Trivy scanning step will [soft fail](#soft-failures). Note that soft failures **do not fail builds or block deployments**. They simply highlight the failing step for further analysis.

> NOTE: Our vulnerability management process (including this workflow) is under active development and in its early stages. All of the above is subject to change. See [https://github.com/sourcegraph/sourcegraph/pull/25756](https://github.com/sourcegraph/sourcegraph/pull/25756) for more context.

We also run [separate vulnerability scans for our infrastructure](https://handbook.sourcegraph.com/departments/product-engineering/engineering/cloud/security/checkov).

### Pipeline health

Maintaining [Buildkite pipeline](#buildkite-pipelines) health is a critical part of ensuring we ship a stable product—changes that make it to the `main` branch may be deployed to various Sourcegraph instances, and having a reliable and predictable pipeline is crucial to ensuring bugs do not make it to production environments.

To enable this, we [address flakes as they arise](#flakes) and mitigate the impacts of pipeline instability with [branch locks](#branch-locks).

> NOTE: Sourcegraph teammates should refer to the [CI incidents playbook](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/incidents/playbooks/ci#scenarios) for help managing issues with pipeline health.

#### Branch locks

> WARNING: **A red `main` build is not okay and must be fixed.** Learn more about our `main` branch policy in [Testing principles: Failures on the `main` branch](../testing_principles.md#failures-on-the-main-branch).

[`buildchecker`](#buildchecker) is a tool responding to periods of consecutive build failures on the `main` branch Sourcegraph Buildkite pipeline. If it detects a series of failures on the `main` branch, merges to `main` will be restricted to members of the Sourcegraph team who authored the failing commits until the issue is resolved—this is referred to as a "branch lock". When a build passes on `main` again, `buildchecker` will automatically unlock the branch.

**Authors of the most recent failed builds are responsible for investigating failures.** Please refer to the [Continuous integration playbook](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/incidents/playbooks/ci#build-has-failed-on-the-main-branch) for step-by-step guides on what to do in various scenarios.

#### Flakes

A *flake* is defined as a test or script that is unreliable or non-deterministic, i.e. it exhibits both a passing and a failing result with the same code. In other words: something that sometimes fails, but if you retry it enough times, it passes, *eventually*.

Tests are not the only thing that are flaky—flakes can also encompass [sporadic infrastructure issues](#flaky-infrastructure) and [unreliable steps](#flaky-steps).

##### Flaky tests

> WARNING: **We do not tolerate flaky tests of any kind.** Learn more about our flaky test policy in [Testing principles: Flaky tests](../testing_principles.md#flaky-tests).

Typical reasons why a test may be flaky:

- Race conditions or timing issues
- Caching or inconsistent state between tests
- Unreliable test infrastructure (such as CI)
- Reliance on third-party services that are inconsistent

**If a flaky test is discovered:**

1. Immediately use language-specific functionality to skip a test and open a PR to disable the test:

    - Go: [`testing.T.Skip`](https://pkg.go.dev/testing#hdr-Skipping)
    - Typescript: [`.skip()`](https://mochajs.org/#inclusive-tests)

   If the language or framework allows for a skip reason, include a link to the issue track re-enabling the test, or leave a docstring with a link.

2. Open an issue to investigate the flaky test (use the [flaky test issue template](https://github.com/sourcegraph/sourcegraph/issues/new/choose)), and assign it to the most likely owner.

##### Flaky steps

If a step is flaky we need to get the build back to reliable as soon as possible. If there is not already a discussion in `#buildkite-main` create one and link what step you take. Here are the recommended approaches in order:

1. Revert the PR if a recent change introduced the instability. Ping author.
2. Use `Skip` StepOpt when creating the step. Include reason and a link to context. This will still show the step on builds so we don't forget about it.

An example use of `Skip`:

```diff
--- a/dev/ci/internal/ci/operations.go
+++ b/dev/ci/internal/ci/operations.go
@@ -260,7 +260,9 @@ func addGoBuild(pipeline *bk.Pipeline) {
 func addDockerfileLint(pipeline *bk.Pipeline) {
        pipeline.AddStep(":docker: Lint",
                bk.Cmd("go run ./dev/sg lint -annotations docker"),
+               bk.Skip("2021-09-29 example message https://github.com/sourcegraph/sourcegraph/issues/123"),
        )
 }
```

> NOTE: If it's hard to make sure that the flake is fixed, another approach is to monitor the step wihout breaking the build, see [How to allow a CI step to fail without breaking the build and still receive a notification](../../how-to/ci_soft_failure_and_still_notify.md).

##### Assessing flaky client steps

See more information on how to assess flaky client steps [here](../../how-to/testing.md#assessing-flaky-client-steps).

##### Flaky infrastructure

If the [build or test infrastructure itself is flaky](https://handbook.sourcegraph.com/departments/product-engineering/engineering/enablement/dev-experience#build-pipeline-support), then [open an issue with the `team/devx` label](https://github.com/sourcegraph/sourcegraph/issues/new?labels=team/devx) and notify the [Developer Experience team](https://handbook.sourcegraph.com/departments/product-engineering/engineering/enablement/dev-experience#contact).

Also see [Buildkite infrastructure](#buildkite-infrastructure).

##### Flaky linters

Linters are all run through [`sg lint`], with linters defined in [`dev/sg/linters`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/dev/sg/linters).
If a linter is flaky, you can modify the `dev/sg/linters` package to disable the specific linter (or entire category of linters) with the `Enabled: disabled(...)` helper:

```diff
  {
    Name:        "svg",
    Description: "Check svg assets",
+   Enabled:     disabled("reported as unreliable"),
    Checks: []*linter{
      checkSVGCompression(),
    },
  },
```

### Pipeline development

See [Pipeline development](./development.md) to get started with contributing to our Buildkite pipelines!

### Deployment notifications

When a pull request is deployed, an automated notification will be posted in [#deployments-cloud](https://sourcegraph.slack.com/archives/C03BGBR796H). Notifications include a list of the pull-request that were shipped as well as a list of which services specifically were rolled out.

If you want to be explictly notified (through a Slack ping) when your pull request reaches _dotcom production_, add the label `notify-on-deploy`.

## GitHub Actions

### `buildchecker`

[![buildchecker](https://github.com/sourcegraph/sourcegraph/actions/workflows/buildchecker.yml/badge.svg)](https://github.com/sourcegraph/sourcegraph/actions/workflows/buildchecker.yml) [![buildchecker-history](https://github.com/sourcegraph/sourcegraph/actions/workflows/buildchecker-history.yml/badge.svg)](https://github.com/sourcegraph/sourcegraph/actions/workflows/buildchecker-history.yml)

[`buildchecker`](https://github.com/sourcegraph/sourcegraph/actions/workflows/buildchecker.yml), our [branch lock management tool](#branch-locks), runs in GitHub actions—see the [workflow specification](https://github.com/sourcegraph/sourcegraph/blob/main/.github/workflows/buildchecker.yml).

To learn more about `buildchecker`, refer to the [`buildchecker` source code and documentation](https://github.com/sourcegraph/sourcegraph/tree/main/dev/buildchecker).

### `pr-auditor`

[![pr-auditor](https://github.com/sourcegraph/sourcegraph/actions/workflows/pr-auditor.yml/badge.svg)](https://github.com/sourcegraph/sourcegraph/actions/workflows/pr-auditor.yml)

[`pr-auditor`](https://github.com/sourcegraph/sourcegraph/actions/workflows/pr-auditor.yml), our [PR audit tool](../testing_principles.md#policy), runs in GitHub actions—see the [workflow specification](https://github.com/sourcegraph/sourcegraph/blob/main/.github/workflows/pr-auditor.yml).

To learn more about `pr-auditor`, refer to the [`pr-auditor` source code and documentation](https://github.com/sourcegraph/sourcegraph/tree/main/dev/pr-auditor).

### Third-party licenses

[![Licenses Update](https://github.com/sourcegraph/sourcegraph/actions/workflows/licenses-update.yml/badge.svg)](https://github.com/sourcegraph/sourcegraph/actions/workflows/licenses-update.yml) [![Licenses Check](https://github.com/sourcegraph/sourcegraph/actions/workflows/licenses-check.yml/badge.svg)](https://github.com/sourcegraph/sourcegraph/actions/workflows/licenses-check.yml)

We use the [`license_finder`](https://github.com/pivotal/LicenseFinder) tool to check third-party dependencies for their licenses. It runs as a [GitHub Action on pull requests](https://github.com/sourcegraph/sourcegraph/actions?query=workflow%3A%22Licenses+Check%22), which will fail if one of the following occur:

- If the license for a dependency cannot be inferred. To resolve:
  - Use `license_finder licenses add <dep> <license>` to set the license manually
- If the license for a new or updated dependency is not on the list of approved licenses. To resolve, either:
  - Remove the dependency
  - Use `license_finder ignored_dependencies add <dep> --why="Some reason"` to ignore it
  - Use `license_finder permitted_licenses add <license> --why="Some reason"` to allow the offending license

The `license_finder` tool can be installed using `gem install license_finder`. You can run the script locally using:

```sh
# updates ThirdPartyLicenses.csv
./dev/licenses.sh

# runs the same check as the one used in CI, returning status 1
# if there are any unapproved dependencies ('action items')
LICENSE_CHECK=true ./dev/licenses.sh
```

The `./dev/licenses.sh` script will also output some `license_finder` configuration for debugging purposes—this configuration is based on the `doc/dependency_decisions.yml` file, which tracks decisions made about licenses and dependencies.

For more details, refer to the [`license_finder` documentation](https://github.com/pivotal/LicenseFinder#usage).
