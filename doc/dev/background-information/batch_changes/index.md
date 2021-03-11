# Developing batch changes

## What are batch changes?

Before diving into the technical part of batch changes, make sure to read up on what batch changes are, what they're not and what we want them to be.

1. Start by looking at the [batch changes description on about.sourcegraph.com](https://about.sourcegraph.com).
1. Read through the [batch changes documentation](../../../batch_changes/index.md).
	- Especially: "[Batch changes design doc](../../../batch_changes/explanations/batch_changes_design.md)" and "[How src executes a batch spec](../../../batch_changes/explanations/how_src_executes_a_batch_spec.md)".

## Starting up your environment

1. Run `./enterprise/dev/start.sh` and wait until all repositories are cloned.
1. Create [your first batch change](../../../batch_changes/quickstart.md). **Remember:** If you create a batch change, you're opening real PRs on GitHub. Make sure only [testing repositories](#github-testing-account) are affected. If you create a large batch change, it takes a while to preview/create but also helps a lot with finding bugs/errors, etc.

## Glossary

The batch changes feature introduces a lot of new names, GraphQL queries and mutations and database tables. This section tries to explain the most common names and provide a mapping between the GraphQL types and their internal counterpart in the Go backend.

<!-- depends-on-source: ~/internal/batches/batch_change.go, ~/internal/batches/batch_spec.go, etc -->

| GraphQL type        | Go type              | Database table     | Description |
| ------------------- | -------------------- | -------------------| ----------- |
| `BatchChange`       | `batches.BatchChange`    | `batch_changes`    | A batch change is a collection of changesets. The central entity. |
| `ChangesetSpec`     | `batches.ChangesetSpec`  | `changeset_specs`  | A changeset spec describes the desired state of a changeset. |
| `BatchSpec`         | `batches.BatchSpec`      | `batch_specs`      | A batch spec describes the desired state of a batch change. |
| `ExternalChangeset` | `batches.Changeset`      | `changesets`       | Changeset is the unified name for pull requests/merge requests/etc. on code hosts.        |
| `ChangesetEvent`    | `batches.ChangesetEvent` | `changeset_events` | A changeset event is an event on a code host, e.g. a comment or a review on a pull request on GitHub. They are created by syncing the changesets from the code host on a regular basis and by accepting webhook events and turning them into changeset events. |

## Database layout

<!-- TODO(mrnugget): Outdated
<!-- <object data="/dev/background-information/batch_changes/batch_changes_database_layout.svg" type="image/svg+xml" style="width:100%; max-width: 800px"> -->
<!-- </object> -->
<!--  -->
<!-- (To re-generate the diagram from the `batch_changes_database_layout.dot` file with Graphviz, run: `dot -Tsvg -o batch_changes_database_layout.svg batch_changes_database_layout.dot`.) -->

## Diving into the code as a backend developer

1. Read through `./cmd/frontend/graphqlbackend/batch_changes.go` to get an overview of the batch changes GraphQL API.
1. Read through `./internal/batches/*.go` to see all batch changes related type definitions.
1. Compare that with the GraphQL definitions in `./cmd/frontend/graphqlbackend/schema.graphql`.
1. Start reading through `./enterprise/internal/batches/resolvers/resolver.go` to see how the main mutations are implemented (look at `CreateBatchChange` and `ApplyBatchChange` to see how the two main operations are implemented).
1. Then start from the other end, `enterprise/cmd/repo-updater/main.go`. `enterpriseInit()` creates two sets of batch change goroutines:
  1. `batches.NewSyncRegistry` creates a pool of _syncers_ to pull changes from code hosts.
  2. `batches.RunWorkers` creates a set of _reconciler_ workers to push changes to code hosts as batch changes are applied.

## GitHub testing account

Batch changes create changesets (PRs) on code hosts. If you are not part of the Sourcegraph organization, we recommend you create dummy projects to safely test changes on so you do not spam real repositories with your tests. If you _are_ part of the Sourcegraph organization, we have an account set up for this purpose.

To use this account, follow these steps:

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
