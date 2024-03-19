# Developing batch changes

## Getting started

Welcome, new batch change developer! This section will give you a rough overview of what Batch Changes is and how it works.

> NOTE: Never hesitate to ask in [`#batch-changes-internal`](https://sourcegraph.slack.com/archives/C01LJ9DK8ES) for help!

### What are batch changes?

Before diving into the technical part of batch changes, make sure to read up on what batch changes are, what they're not and what we want them to be:

1. Look at the [batch changes product page](https://sourcegraph.com).
1. Watch the 2min [demo video](https://www.youtube.com/watch?v=GKyHYqH6ggY)

Next: **create your first batch change!**

### Creating a batch change locally

> NOTE: **Make sure your local development environment is set up** by going through the "[Getting started](https://docs.sourcegraph.com/dev/getting-started)" guide.

1. Since Batch Changes is an enterprise-only feature, make sure to start your local environment with `sg start` (which defaults to `sg start enterprise`).
1. Go through the [Quickstart for Batch Changes](https://docs.sourcegraph.com/batch_changes/quickstart) to create a batch change in your local environment. See "[Testing repositories](#testing-repositories)" for a list of repositories in which you can safely publish changesets.
1. Now combine what you just did with some background information by reading the following:

- [Batch Changes architecture overview](https://docs.sourcegraph.com/dev/background-information/architecture#batch-changes)
- [How src executes a batch spec](https://docs.sourcegraph.com/batch_changes/explanations/how_src_executes_a_batch_spec)
- [Batch Changes design](https://docs.sourcegraph.com/batch_changes/explanations/batch_changes_design)

### Code walkthrough

To give you a rough overview where each part of the code lives, let's take a look at **which code gets executed** when you

1. run `src batch preview -f your-batch-spec.yaml`
1. click on the preview link
1. click **Apply** to publish changesets on the code hosts

It starts in [`src-cli`](https://github.com/sourcegraph/src-cli):

1. `src batch preview` starts [the "preview" command in `src-cli`](https://github.com/sourcegraph/src-cli/blob/6cbaba6d47761b5f5041ed285aea686bf5b266c3/cmd/src/batch_preview.go)
1. That executes your batch spec, which means it [parses it, validates it, resolves the namespace, prepares the docker images, and checks which workspaces are required](https://github.com/sourcegraph/src-cli/blob/6cbaba6d47761b5f5041ed285aea686bf5b266c3/cmd/src/batch_common.go#L187:6)
1. Then, for each repository (or [workspace in each repository](https://docs.sourcegraph.com/batch_changes/how-tos/creating_changesets_per_project_in_monorepos)), it [runs the `steps` in the batch spec](https://github.com/sourcegraph/src-cli/blob/6cbaba6d47761b5f5041ed285aea686bf5b266c3/internal/batches/run_steps.go#L54) by downloading a repository archive, creating a workspace in which to execute the `steps`, and then starting the Docker containers.
1. If changes were produced in a repository, these changes are turned into a `ChangesetSpec` (a specification of what a changeset should look like on the code host—title, body, commit, etc.) and [uploaded to the Sourcegraph instance](https://github.com/sourcegraph/src-cli/blob/6cbaba6d47761b5f5041ed285aea686bf5b266c3/cmd/src/batch_common.go#L297-L324)
1. `src batch preview`'s last step is then to [create a `BatchSpec` on the Sourcegraph instance](https://github.com/sourcegraph/src-cli/blob/6cbaba6d47761b5f5041ed285aea686bf5b266c3/cmd/src/batch_common.go#L331-L336), which is a collection of the `ChangesetSpec`s that you can then preview or apply

When you then click the "Preview the batch change" link that `src-cli` printed, you'll land on the preview page in the web frontend:

1. The [`BatchChangePreviewPage` component](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/client/web/src/enterprise/batches/preview/BatchChangePreviewPage.tsx#L43) then sends a GraphQL request to the backend to [query the `BatchSpecByID`](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/client/web/src/enterprise/batches/preview/backend.ts#L93-L107).
1. Once you hit the **Apply** button, the component [uses the `applyBatchChange`](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/client/web/src/enterprise/batches/preview/backend.ts#L140-L159) to apply the batch spec and create a batch change.
1. You're then redirected to the [`BatchChangeDetailsPage` component](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/client/web/src/enterprise/batches/detail/BatchChangeDetailsPage.tsx#L65) that shows you you're newly-created batch change.

In the backend, all Batch Changes related GraphQL queries and mutations start in the [`Resolver` package](https://github.com/sourcegraph/sourcegraph/blob/8b99439e21aaa000443382f03f92e532b0445858/enterprise/cmd/frontend/internal/batches/resolvers/resolver.go):

1. The [`CreateChangesetSpec`](https://github.com/sourcegraph/sourcegraph/blob/8b99439e21aaa000443382f03f92e532b0445858/enterprise/cmd/frontend/internal/batches/resolvers/resolver.go#L545) and [`CreateBatchSpec`](https://github.com/sourcegraph/sourcegraph/blob/8b99439e21aaa000443382f03f92e532b0445858/enterprise/cmd/frontend/internal/batches/resolvers/resolver.go#L489) mutations that `src-cli` called to create the changeset and batch specs are defined here.
1. When you clicked **Apply** the [`ApplyBatchChange` resolver](https://github.com/sourcegraph/sourcegraph/blob/8b99439e21aaa000443382f03f92e532b0445858/enterprise/cmd/frontend/internal/batches/resolvers/resolver.go#L404) was executed to create the batch change.
1. Most of that doesn't happen in the resolver layer, but in the service layer: [here is the `(*Service).ApplyBatchChange` method](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/service/service_apply_batch_change.go#L48:19) that talks to the database to create an entry in the `batch_changes` table.
1. The most important thing that happens in `(*Service).ApplyBatchChange` is that [it calls the `rewirer`](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/service/service_apply_batch_change.go#L119-L135) to wire the entries in the `changesets` table to the correct `changeset_specs`.
1. Once that is done, the `changesets` are created or updated to point to the new `changeset_specs` that you created with `src-cli`.

After that you can look at your new batch change in the UI while the rest happens asynchronously in the background:

1. In a background process (which is started in (`enterprise/cmd/repo-updater`](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/cmd/repo-updater/main.go#L58)) [a `worker` is running](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/background/background.go#L19) that monitors the `changesets` the table.
1. Once a `changeset` has been rewired to a new `changeset_spec` and reset, this worker, called the [`Reconciler`](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/reconciler/reconciler.go#L24:6), fetches the changeset from the database and "reconciles" its current state (not published yet) with its desired state ("published on code host X, with this diff, that title and this body")
1. To do that, the `Reconciler` looks at the changeset's current and previous `ChangesetSpec` [to determine a plan](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/reconciler/reconciler.go#L65-L68) for what it should do ("publish", "push a commit", "update title", etc.)
1. Once it has the plan, it hands over to the [`Executor`](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/reconciler/executor.go#L28:6) which executes the plan.
1. To push a commit to the code host, the `Executor` [sends a request](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/reconciler/executor.go#L462:20) to the [`gitserver` service](https://docs.sourcegraph.com/dev/background-information/architecture#code-syncing)
1. To create or update a pull request or merge request on the code host it [builds a `ChangesetSource`](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/enterprise/internal/batches/reconciler/executor.go#L149) which is a wrapper around the GitHub, Bitbucket Server, Bitbucket Data Center and GitLab HTTP clients.

While that is going on in the background the [`BatchChangeDetailsPage` component is polling the GraphQL](https://github.com/sourcegraph/sourcegraph/blob/e7f26c0d7bc965892669a5fc9835ec65211943aa/client/web/src/enterprise/batches/detail/BatchChangeDetailsPage.tsx#L87-L90) to get the current state of the Batch Change and its changesets.

Once all instances of the `Reconciler` worker are done determining plans and executing them, you'll see that your changesets have been published on the code hosts.

## Glossary

Batch changes introduce a lot of new names, GraphQL queries & mutations, and database tables. This section tries to explain the most common names and provide a mapping between the GraphQL types and their internal counterpart in the Go backend.

| GraphQL type        | Go type                  | Database table     | Description                                                                                                                                                                                                                                                    |
| ------------------- | ------------------------ | ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Changeset`         | `batches.Changeset`      | `changesets`       | A changeset is a generic abstraction for pull requests and merge requests.                                                                                                                                                                                     |
| `BatchChange`       | `batches.BatchChange`    | `batch_changes`    | A batch change is a collection of changesets. The central entity.                                                                                                                                                                                              |
| `BatchSpec`         | `batches.BatchSpec`      | `batch_specs`      | A batch spec describes the desired state of a single batch change.                                                                                                                                                                                                    |
| `ChangesetSpec`     | `batches.ChangesetSpec`  | `changeset_specs`  | A changeset spec describes the desired state of a single changeset.                                                                                                                                                                                                   |
| `ExternalChangeset` | `batches.Changeset`      | `changesets`       | Changeset is the unified name for pull requests/merge requests/etc. on code hosts.                                                                                                                                                                             |
| `ChangesetEvent`    | `batches.ChangesetEvent` | `changeset_events` | A changeset event is an event on a code host, e.g. a comment or a review on a pull request on GitHub. They are created by syncing the changesets from the code host on a regular basis and by accepting webhook events and turning them into changeset events. |

## Structure of the Go backend code

The following is a list of Go packages in the [`sourcegraph/sourcegraph`](https://github.com/sourcegraph/sourcegraph) repository and short explanations of what each package does:

- `enterprise/internal/batches/types`:

    Type definitions of common `batches` types, such as `BatchChange`, `BatchSpec`, `Changeset`, etc. A few helper functions and methods, but no real business logic.
- `enterprise/internal/batches`:

    The hook `InitBackgroundJobs` injects Batch Changes code into `enterprise/repo-updater`. This is the "glue" in "glue code".
- `enterprise/internal/batches/background`

    Another bit of glue code that starts background goroutines: the changeset reconciler, the stuck-reconciler resetter, the old-changeset-spec expirer.
- `enterprise/internal/batches/rewirer`:

    The `ChangesetRewirer` maps existing/new changesets to the matching `ChangesetSpecs` when a user applies a batch spec.
- `enterprise/internal/batches/state`:

    All the logic concerned with calculating a changesets state at a given point in time, taking into account its current state, past events synced from regular code host APIs, and events received via webhooks.
- `enterprise/internal/batches/search`:

    Parsing text-field input for changeset searches and turning them into database-queryable structures.
- `enterprise/internal/batches/search/syntax`:

    The old Sourcegraph-search-query parser we inherited from the search team a week or two back (the plan is _not_ to keep it, but switch to the new one when we have time)
- `cmd/frontend/internal/batches/resolvers`:

    The GraphQL resolvers that are injected into the `enterprise/frontend` in `cmd/frontend/internal/batches/init.go`. They mostly concern themselves with input/argument parsing/validation, (bulk-)reading (and paginating) from the database via the `batches/store`, but delegate most business logic to `batches/service`.
- `cmd/frontend/internal/batches/resolvers/apitest`:

    A package that helps with testing the resolvers by defining types that match the GraphQL schema.
- `enterprise/internal/batches/testing`:

    Common testing helpers we use across `enterprise/internal/batches/*` to create test data in the database, verify test output, etc.
- `enterprise/internal/batches/reconciler`:

    The `reconciler` is what gets kicked off by the `workerutil.Worker` initialised in `batches/background` when a `changeset` is enqueued. It's the heart of the declarative model of batches: compares changeset specs, creates execution plans, executes those.
- `enterprise/internal/batches/syncer`:

    This contains everything related to "sync changeset data from the code host to sourcegraph". The `Syncer` is started in the background, keeps state in memory (rate limit per external service), and syncs changesets either periodically (according to heuristics) or when directly enqueued from the `resolvers`.
- `enterprise/internal/batches/service`:

    This is what's often called the "service layer" in web architectures and contains a lot of the business logic: creating a batch change and validating whether the user can create one, applying new batch specs, calling the `rewirer`, deleting batch changes, closing batch changes, etc.
- `cmd/frontend/internal/batches/webhooks`:

    These `webhooks` endpoints are injected by `InitFrontend` into the `frontend` and implement the `cmd/frontend/webhooks` interfaces.
- `enterprise/internal/batches/store`:

    This is the batch changes `Store` that takes `enterprise/internal/batches/types` types and writes/reads them to/from the database. This contains everything related to SQL and database persistence, even some complex business logic queries.
- `enterprise/internal/batches/sources`:

    This package contains the abstraction layer of code host APIs that live in [`internal/extsvc/*`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/internal/extsvc). It provides a generalized interface [`ChangesetSource`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/enterprise/internal/batches/sources/common.go#L40:6) and implementations for each of our supported code hosts.

## Diving into the code as a backend developer

1. Read through [`./cmd/frontend/graphqlbackend/batches.go`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/graphqlbackend/batches.go) to get an overview of the batch changes GraphQL API.
1. Read through [`./enterprise/internal/batches/types/*.go`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/enterprise/internal/batches/types) to see all batch changes related type definitions.
1. Compare that with the GraphQL definitions in `./cmd/frontend/graphqlbackend/batches.graphql`.
1. Start reading through `./enterprise/internal/batches/resolvers/resolver.go` to see how the main mutations are implemented (look at `CreateBatchChange` and `ApplyBatchChange` to see how the two main operations are implemented).
1. Then start from the other end, `enterprise/cmd/repo-updater/main.go`. `enterpriseInit()` creates two sets of batch change goroutines:
1. `batches.NewSyncRegistry` creates a pool of _syncers_ to pull changes from code hosts.
1. `batches.RunWorkers` creates a set of _reconciler_ workers to push changes to code hosts as batch changes are applied.

## Testing repositories

Batch changes create changesets (PRs) on code hosts. For testing Batch Changes locally we recommend to use the following repositories:

- The [sourcegraph-testing GitHub organization](https://github.com/sourcegraph-testing) contains testing repositories in which you can open pull requests.
- We have an `automation-testing` repository that exists on [Github](https://github.com/sourcegraph/automation-testing), [Bitbucket Server](https://bitbucket.sgdev.org/projects/SOUR/repos/automation-testing/), and [GitLab](https://gitlab.sgdev.org/sourcegraph/automation-testing)
- The GitHub user `sd9` was specifically created to be used for testing Batch Changes. See "[GitHub testing account](#github-testing-account)" for details.

If you're lacking permissions to publish changesets in one of these repositories, feel free to reach out to a team member.

### GitHub testing account

To use the `sd9` GitHub testing account:

1. Find the GitHub `sd9` user in 1Password
2. Copy the `Campaigns Testing Token`
3. Change your `dev-private/enterprise/dev/external-services-config.json` to only contain a GitHub config with the token, like this:

```json
{
  "GITHUB": [
    {
      "authorization": {},
      "url": "https://github.com",
      "token": "<TOKEN>",
      "repositoryQuery": ["affiliated"]
    }
  ]
}
```

## Batch Spec examples

Take a look at the following links to see some examples of batch changes and the batch specs that produced them:

- [sourcegraph/batch-change-examples](https://github.com/sourcegraph/batch-change-examples)
- [k8s.sgdev.org/batch-changes](https://k8s.sgdev.org/batch-changes)
- [Batch Changes tutorials](https://docs.sourcegraph.com/batch_changes/tutorials)

## Server-side execution

### Database tables

There are currently (Sept '21) four tables at the heart of the server-side execution of batch specs:

**`batch_specs`**. These are the `batch_specs` we already have, but in server-side mode they are created through a special mutation that also creates a `batch_spec_resolution_job`, see below.

**`batch_spec_resolution_jobs`**. These are [worker jobs](../workers.md) that are created through the GraphQL when a user wants to kick of a server-side execution. Once a `batch_spec_resolution_job` is created a worker will pick them up, load the corresponding `batch_spec` and resolve its `on` part into `RepoWorkspaces`: a combination of repository, commit, path, steps, branch, etc. For each `RepoWorkspace` they create a `batch_spec_workspace` in the database.

**`batch_spec_workspace`**. Each `batch_spec_workspace` represents a unit of work for a [`src batch exec`](https://github.com/sourcegraph/src-cli/pull/608) invocation inside the executor. Once `src batch exec` has successfully executed, these `batch_spec_workspaces` will contain references to `changeset_specs` and those in turn will be updated to point to the `batch_spec` that kicked all of this off.

**`batch_spec_workspace_execution_jobs`**. These are the worker jobs that get picked up the executor and lead to `src batch exec` being called. Each `batch_spec_workspace_execution_job` points to one `batch_spec_workspace`. This extra table lets us separate the workspace _data_ from the _execution_ of `src batch exec`. Separation of these two tables is the result of us running into tricky concurrency problems where workers were modifying table rows that the GraphQL layer was reading (or even modifying).

Here's a diagram of their relationship:

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/dev/diagram.png">
