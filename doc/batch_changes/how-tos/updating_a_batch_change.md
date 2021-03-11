# Updating a batch change

Updating a batch change works by applying a batch spec to an **existing** batch change in the same namespace.

Since batch changes are uniquely identified by their name (the `name` property in the batch spec) and the namespace in which they were created, you can edit any other part of a batch spec and apply it again.

When a new batch spec is applied to an existing batch change the existing batch change is updated and its changesets are updated to match the new desired state.

## Requirements

To update a changeset, you need:

1. [admin permissions for the batch change](../explanations/permissions_in_batch_changes.md#permission-levels-for-batch-changes),
1. write access to the changeset's repository (on the code host), and
1. a personal access token [configured in Sourcegraph for your code host(s)](configuring_user_credentials.md).

For more information, see [Code host interactions in Batch Changes](../explanations/permissions_in_batch_changes.md#code-host-interactions-in-batch-changes).

## Preview and apply a new batch spec

In order to update a batch change after previewing the changes, do the following:

1. Edit the [batch spec](../references/batch_spec_yaml_reference.md) with which you created the batch change to include the changes you want to make to the batch change. For example, change [the commit message in the `changesetTemplate`](../references/batch_spec_yaml_reference.md#changesettemplate-commit-message), or add a new changeset id [to the importedChangesets](https://docs.sourcegraph.com/batch-changes/references/batch_spec_yaml_reference#importchangesets), or [modify the repositoriesMatchingQuery](https://docs.sourcegraph.com/batch-changes/references/batch_spec_yaml_reference#on-repositoriesmatchingquery) to return different search results.
1. Use the [Sourcegraph CLI (`src`)](https://github.com/sourcegraph/src-cli) to execute and preview the batch spec.

    <pre><code>src batch preview -f <em>YOUR_BATCH_SPEC.batch.yaml</em></code></pre>
1. Open on the URL that's printed to preview the changes that will be made by applying the new batch spec.
1. Click **Apply spec** to update the batch change.

All of the changesets on your code host will be updated to the desired state that was shown in the preview.

## Apply a new batch spec directly

In order to update a batch change directly, without preview, do the following:

1. Edit the [batch spec](../references/batch_spec_yaml_reference.md) with which you created the batch change to include the changes you want to make to the batch change.
1. Use the [Sourcegraph CLI (`src`)](https://github.com/sourcegraph/src-cli) to execute, upload, and the batch spec.

    <pre><code>src batch apply -f <em>YOUR_BATCH_SPEC.batch.yaml</em></code></pre>

The new batch spec will be applied directly and the batch change and its changesets will be updated.

## How batch change updates are processed

Changes in the batch spec that affect the batch change itself, such as the [`description`](../references/batch_spec_yaml_reference.md#description), are applied directly when you apply the new batch spec.

Changes that affect the changesets are processed asynchronously to update the changeset to its new desired state. Different fields are processed differently.

Here are some examples:

- When the diff or attributes that affect the resulting commit of a changeset directly (such as the [`changesetTemplate.commit.message`](../references/batch_spec_yaml_reference.md#changesettemplate-commit-message) or the [`changesetTemplate.commit.author`](../references/batch_spec_yaml_reference.md#changesettemplate-commit-author)) and the changeset has been published, the commit on the code host will be overwritten by a new commit that includes the updated diff.
- When the [`changesetTemplate.title`](../references/batch_spec_yaml_reference.md#changesettemplate-title) or the [`changesetTemplate.body`](../references/batch_spec_yaml_reference.md#changesettemplate-commit-author) are changed and the changeset has been published, the changeset on the code host will be updated accordingly.
- When the [`changesetTemplate.branch`](../references/batch_spec_yaml_reference.md#changesettemplate-title) is changed after the changeset has been published on the code host, the existing changeset will be closed on the code host and new one, with the new branch, will be created.
- When the batch spec is changed in such a way that no diff is produced in a repository in which the batch change already created and published a changeset, the existing changeset will be closed on the code host and detached from the batch change.

See the "[Batch Changes design](../explanations/batch_changes_design.md)" doc for more information on the declarative nature of the Batch Changes system.

## Updating a batch change to change its scope

### Increasing the number of changesets

You can gradually increase the number of repositories to which a batch change applies by modifying the entries in the [`on`](../references/batch_spec_yaml_reference.md#on) property of the batch spec.

That means you can start with an `on` property like this in your batch spec:

```yaml
# [...]

# Find all repositories that contain a README.md file, in the GitHub my-company org.
on:
  - repositoriesMatchingQuery: file:README.md repo:github.com/my-company

# [...]
```

After you applied that batch spec, you can extend the scope of batch change by changing the `on` property to result in more repositories:

```yaml
# [...]

# Find all repositories that contain a README.md file, in the GitHub my-company and my-company-ci org.
on:
  - repositoriesMatchingQuery: file:README.md repo:github.com/my-company|github.com/my-company-ci

# [...]
```

The updated [`repo:` keyword](../../code_search/reference/queries.md#keywords-all-searches) in the search query will result in more repositories being returned by the search.

If you apply the updated batch spec new changesets will be created for each additional repository.

### Decreasing the number of changesets

You can also decrease the number of repositories to which a batch change applies the by modifying the entries in the [`on`](../references/batch_spec_yaml_reference.md#on) property.

If you, for example, started with this batch spec

```yaml
# [...]

# Find all repositories that contain a README.md file, in the GitHub my-company org.
on:
  - repositoriesMatchingQuery: file:README.md repo:github.com/my-company

# [...]
```

and applied it and [published changesets](publishing_changesets.md) and then change it to this

```yaml
# [...]

# Find all repositories that contain a README.md file, in the GitHub my-company org.
on:
  - repositoriesMatchingQuery: file:README.md repo:github.com/my-company/my-one-repository

# [...]
```

and apply it, then all the changesets that were published in repositories other than `my-one-repository` _will be closed on the code host and detached from the batch change_.
