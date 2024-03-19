# Re-executing batch specs multiple times

## Idempotency as goal

One goal behind the [design of Batch Changes](batch_changes_design.md) and the `src batch [apply|preview]` commands in the [Sourcegraph CLI](../../cli/index.md) is **idempotency**.

Re-applying the same batch spec produces the same batch change and changesets.

In practical terms: a user should be able to run `src batch apply -f my.batch.yaml` multiple times and, if `my.batch.yaml` didn't change, the batch change shouldn't change.

## Exceptions

That can only work, though, if the _inputs_ to the batch spec stay the same too.

Since a batch spec's [`on.repositoriesMatchingQuery`](../references/batch_spec_yaml_reference.md#on-repositoriesmatchingquery) uses Sourcegraph's search functionality to **dynamically** produce a list of repositories in which to execute the [`steps`](../references/batch_spec_yaml_reference.md#steps), the list of repositories might change between executions of `src batch apply`.

If that's the case, the `steps` need to be re-executed in any newly found repositories. Changesets in repositories that are not found anymore will be closed and archived in the batch change (see "[Updating a batch change](../how-tos/updating_a_batch_change.md)" for more details). In repositories that are unchanged [Sourcegraph CLI](../../cli/index.md) tries to use cached results to avoid having to re-execute the `steps`.

## Local caching

Whenever [Sourcegraph CLI](../../cli/index.md) re-executes the same batch spec it checks a _local cache_ to see if it already executed the same [`steps`](../references/batch_spec_yaml_reference.md#steps) in a given repository.

Whether a cached result can be used is dependent on multiple things:

1. the repository's default branch's revision didn't change (because if new commits have been pushed to the repository, re-executing the `steps` might lead to different results)
1. the `steps` themselves didn't change, including and all their inputs, such as [`steps.env`](../references/batch_spec_yaml_reference.md#environment-array)), and the `steps.run` field (which _can_ change between executions if it uses [templating](../references/batch_spec_templating.md) and is dynamically built from search results)

That also means that [Sourcegraph CLI](../../cli/index.md) can use cached results when re-executing _a changed batch spec_, as long as the changes didn't affect the `steps` and the results they produce. For example: if only the [`changesetTemplate.title`](../references/batch_spec_yaml_reference.md#changesettemplate-title) field has been changed, cached results can be used, since that field doesn't have any influence on the `steps` and their results.
