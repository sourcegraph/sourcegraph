# Continuous integration development

This document covers information about contributing to [Sourcegraph's continuous integration tools](./index.md).

## Pipeline generator

The source code of [Sourcegraph's Buildkite pipelines](./index.md#buildkite-pipelines) generator is in [`/dev/ci`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/tree/dev/ci).
Internally, the pipeline generator determines what gets run over contributions based on:

1. [Run types](#run-types), determined by branch naming conventions, tags, and environment variables
2. [Diff types](#diff-types), determined by what files have been changed in a given branch

The above factors are then used to determine the appropriate [operations](#operations), composed of [step options](#step-options), that translate into steps in the resulting pipeline.

If you are looking to modify the pipeline, some good rules of thumbs for which construct to look at for implementing something are:

- Adding a new check? Try a new [operation](#operations) or additional [step options](#step-options).
- Adding a set of changes to run when particular files are changed? Start with a new or updated [diff type](#diff-types).
- Adding an entirely new pipeline type for the `sourcegraph/sourcegraph` repository? Take a look at how [run types](#run-types) are implemented.
- Does your check or test need a secret? Take a look at [how to manage secrets](#managing-secrets).

> WARNING: Sourcegraph's pipeline generator and its generated output are under the [Sourcegraph Enterprise license](https://github.com/sourcegraph/sourcegraph/blob/main/LICENSE.enterprise).

### Run types

> NOTE: A full reference of what our existing run types do is available from `sg ci docs`.

<div class="embed">
  <iframe src="https://sourcegraph.com/embed/notebooks/Tm90ZWJvb2s6MTU5"
    style="width:100%;height:720px" frameborder="0" sandbox="allow-scripts allow-same-origin allow-popups">
  </iframe>
</div>

### Diff types

<div class="embed">
  <iframe src="https://sourcegraph.com/embed/notebooks/Tm90ZWJvb2s6MTYw"
    style="width:100%;height:720px" frameborder="0" sandbox="allow-scripts allow-same-origin allow-popups">
  </iframe>
</div>

### Operations

<div class="embed">
  <iframe src="https://sourcegraph.com/embed/notebooks/Tm90ZWJvb2s6MTYx"
    style="width:100%;height:720px" frameborder="0" sandbox="allow-scripts allow-same-origin allow-popups">
  </iframe>
</div>

#### Developing PR checks

To create a new check that can run on pull requests on relevant files, refer to how [diff types](#diff-types) work to get started.

Then, you can add a new check to [`CoreTestOperations`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Edev/ci/internal/ci+CoreTestOperations+type:symbol+&patternType=literal).
Make sure to follow the best practices outlined in docstring.

For more advanced pipelines, see [Run types](#run-types).

### Step options

Each [operation](#operations) is composed of steps that are built via step options, defined as [implementations of the `StepOpt` interface](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/ci/internal/buildkite/buildkite.go?L229:6#tab=implementations_go). The core step option is `Cmd`, which defines a command to run when added to a pipeline via `AddStep`:

```go
func addGoBuild(pipeline *bk.Pipeline) {
  pipeline.AddStep(":go: Build",
    bk.Cmd("./dev/ci/go-build.sh"),
  )
}
```

> NOTE: More details coming soon!

#### Creating annotations

Annotations are used to present the viewer notices about the build and they get rendered in the Buildkite UI as well as when one executes `sg ci status`.
The pipeline generator provides an API for this that, at a high level, works like this:

1. In your script, leave a file in `./annotations`:

  ```sh
  if [ $EXIT_CODE -ne 0 ]; then
    echo -e "$OUT" >./annotations/docsite
  fi
  ```

2. In your pipeline operation, replace the usual `bk.Cmd` with `bk.AnnotatedCmd`:

  ```go
    pipeline.AddStep(":memo: Check and build docsite",
      bk.AnnotatedCmd("./dev/check/docsite.sh", bk.AnnotatedCmdOpts{
        Annotations: &bk.AnnotationOpts{},
      }))
  ```

3. That's it!

Linters implemented in `sg` automatically generate annotations with the `sg lint --annotations` flag.

Part of the annotation that gets generated also includes a link to view the job output and, if the build is on the main branch, a link to view the job logs on Grafana.

If you don't include a file extension in the annotation file, then the contents of the file are rendered terminal output.
An annotation can be rendered as Markdown instead by using the `.md` extension, for example:

```sh
echo -e "$OUT" >./annotations/docsite.md
```

For more details about best practices and additional features and capabilities, please refer to [the `bk.AnnotatedCmd` docstring](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Edev/ci/internal/buildkite+AnnotatedCmd+type:symbol&patternType=literal).

#### Caching build artefacts

For caching artefacts in steps to speed up steps, see [How to cache CI artefacts](../../how-to/cache_ci_artefacts.md).

Cached artefacts are *automatically expired after 30 days* (by an object lifecycle policy on the bucket).

### Observability

> NOTE: Sourcegraph teammates should refer to the [CI incidents playbook](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/incidents/playbooks/ci#scenarios) for help managing issues with [pipeline health](./index.md#pipeline-health).

#### Failure logs

Every failure in the `sourcegraph/sourcegraph` CI pipeline for `main` also [uploads logs using `sg` to Loki](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/upload-build-logs.sh).
We do not publish data for successful builds or branch builds (for those, you can refer to our [build traces](https://docs.sourcegraph.com/dev/background-information/ci/development#pipeline-command-tracing)).

For a brief overview, check out the [CI dashboard](https://sourcegraph.grafana.net/d/iBBWbxFnk/ci?orgId=1), which is a set of graphs based on the contents of uploaded logs.

Some annotations also have a link "View Grafana logs" which will take one to Grafana cloud with a pre-populated query to view the log output of a failure (if any). For more about querying logs, refer to the handbook page: [Grafana Cloud - CI logs](https://handbook.sourcegraph.com/departments/engineering/dev/tools/observability/cloud/#ci-logs).

#### Pipeline command tracing

Every successful build of the `sourcegraph/sourcegraph` repository comes with an annotation pointing at the full trace of the build on [Honeycomb.io](https://honeycomb.io).
See the [Buildkite board on Honeycomb](https://ui.honeycomb.io/sourcegraph/board/sqPvYj5BXNy/Buildkite) for an overview.

Individual commands are tracked from the perspective of a given [step](#step-options):

```go
  pipeline.AddStep(":memo: Check and build docsite", /* ... */)
```

Will result in a single trace span for the `./dev/check/docsite.sh` script. But the following will have individual trace spans for each `pnpm` commands:

```go
  pipeline.AddStep(fmt.Sprintf(":%s: Puppeteer tests for %s extension", browser, browser),
    // ...
    bk.Cmd("pnpm install --frozen-lockfile --fetch-timeout 60000"),
    bk.Cmd("pnpm --filter @sourcegraph/browser -s run build"),
```

Therefore, it's beneficial for tracing purposes to split the step in multiple commands, if possible.

### Buildkite infrastructure

Our continuous integration system is composed of two parts, a central server controled by Buildkite and agents that are operated by Sourcegraph within our own infrastructure.
In order to provide strong isolation across builds, to prevent a previous build to create any effect on the next one, our agents are stateless jobs.

When a build is dispatched by Buildkite, each individual job will be assigned to an agent in a pristine state. Each agent will execute its assigned job, automatically report back to Buildkite and finally shuts itself down. A fresh agent will then be created and will stand in line for the next job.

This means that our agents are totally **stateless**, exactly like the runners used in GitHub actions.

Also see [Flaky infrastructure](#flaky-infrastructure), [Continous integration infrastructure](https://handbook.sourcegraph.com/departments/product-engineering/engineering/tools/infrastructure/ci), and the [Continuous integration changelog](https://handbook.sourcegraph.com/departments/product-engineering/engineering/tools/infrastructure/ci/changelog).

#### Pipeline setup

To set up Buildkite to use the rendered pipeline, add the following step in the [pipeline settings](https://buildkite.com/sourcegraph/sourcegraph/settings):

```shell
go run ./dev/ci/gen-pipeline.go | buildkite-agent pipeline upload
```

#### Managing secrets

The term _secret_ refers to authentication credentials like passwords, API keys, tokens, etc. which are used to access a particular service. To add a secret:

1. Use Google Cloud Secret manager to add it to [the `sourcegraph-ci` project](https://console.cloud.google.com/security/secret-manager?project=sourcegraph-ci).
2. Inject it at deployment time as an environment variable in the CI agents via adding it to [the Buildkite GSM configuration](https://github.com/sourcegraph/infrastructure/blob/main/buildkite/kubernetes/gsm-secrets.tf).
3. Run `terraform apply` in [the `buildkite/kubernetes/` folder](https://github.com/sourcegraph/infrastructure/tree/main/buildkite/kubernetes). It will make it sure the secret is available in the nodes environment.
4. A Kubernetes Job gets dispatched onto a node, we need to tell Kubernetes to make the variable available to the Job by updating the [Job manifest](https://github.com/sourcegraph/infrastructure/blob/main/buildkite/kubernetes/buildkite-agent-stateless/buildkite-agent.Job.yaml) for the buildkite stateless agent.
5. Run `kubectl -f apply buildkite-agent-stateless/buildkite-agent.Job.yaml` in the `buildkite/kubernetes` folder.

**Note**: Jobs are created dynamically and it might take a while for the new Job manifest to be picked up.

Our CI pipeline must never leak secrets:

1. Use an environment variable name with one of the following suffixes to ensure it gets redacted in the logs: `*_PASSWORD, *_SECRET, *_TOKEN, *_ACCESS_KEY, *_SECRET_KEY, *_CREDENTIALS`
2. While environment variables can be assigned when declaring steps, they should never be used for secrets, because they won't get redacted, even if they match one of the above patterns.

#### Creating scheduled builds

You can schedule builds with build schedules, which automatically create builds at the specified intervals. They are useful to create, for example, nightly builds.

1. Go to `Pipeline Settings` in buildkite and then click `New Schedule`

![new schedule](https://user-images.githubusercontent.com/68532117/165358554-85e48dd0-379c-4461-aef7-09e1cd058569.png)

2. Complete the form to create a new build where you can define the intervals with the `Cron Interval` field. Check out the [Buildkite Docs](https://buildkite.com/docs/pipelines/scheduled-builds#schedule-intervals-predefined-intervals) to see a list of predefined intervals.

![cron interval](https://user-images.githubusercontent.com/68532117/165358933-a27e4293-a363-4a77-84d7-a3ce67f743d2.png)

> NOTE: You can also inject custom environment variables, for example, to trigger a custom [Run Type](#run-types).
