# FAQ

This is a compilation of some common questions about Batch Changes.

### What happens if my batch change creation breaks down at 900 changesets out of 1,000? Do I have to re-run it all again?
Batch Changes' default behavior is to stop if creating the diff on a repo errors. You can choose to ignore errors instead by adding the [`-skip-errors`](../../cli/references/batch/preview.md) flag to the `src batch preview` command.

### Can we close a batch change and still leave the changesets open?
Yes. There is a confirmation page that shows you all the actions that will occur on the various changesets in the batch change after you close it. Open changesets will be marked 'Kept open', which means that batch change won't alter them. See [closing a batch change](../how-tos/closing_or_deleting_a_batch_change.md#closing-a-batch-change).

### How scalable is Batch Changes? How many changesets can I create?
Batch Changes can create tens of thousands of changesets. This is something we run testing on internally.
Known limitations:

- Since diffs are created locally by running a docker container, performance depends on the capacity of your machine. See [How `src` executes a batch spec](../explanations/how_src_executes_a_batch_spec.md).
- Batch Changes creates changesets in parallel locally. You can set up the maximum number of parallel jobs with [`-j`](../../cli/references/batch/apply.md)
- Manipulating (commenting, notifying users, etc) changesets at that scale can be clumsy. This is a major area of work for future releases.

### How long does it take to create a batch change?
A rule of thumb:

- measure the time it takes to run your change container on a typical repository
- multiply by the number of repositories
- divide by the number of changeset creation jobs that will be ran in parallel set by the [`-j`](../../cli/references/batch/apply.md) CLI flag. It defaults to GOMAXPROCS, [roughly](https://golang.org/pkg/runtime/#NumCPU) the number of available cores.

Note: If you run memory-intensive jobs, you might need to reduce the number of parallel job executions. You can run `docker stats` locally to get an idea of memory usage.

### My batch change does not open changesets on all the repositories it should. Why?
- Do you have enough permissions? Batch Changes will error on the repositories you don’t have access to. See [Repository permissions for Batch Changes](../explanations/permissions_in_batch_changes.md).
- Does your `repositoriesMatchingQuery` contain all the necessary flags? If you copied the query from the sourcegraph UI, note that some flags are represented as buttons (case sensitivity, regex, structural search), and do not appear in the query unless you use the copy query button.
- Are the files you are trying to change in your repository's `.gitignore`? Batch Changes respects .gitignore files when creating the diff.

### Can I create tickets or issues along with Batch Changes?
Batch Changes does not support a declarative syntax for issues or tickets.
However, [steps](../references/batch_spec_yaml_reference.md#steps-run) can be used to run any container. Some users have built scripts to create tickets at each apply:

- [Jira tickets](https://github.com/sourcegraph/batch-change-examples/blob/main/ticketing-systems/jira-tickets/README.md)
- [GitHub issues](https://github.com/sourcegraph/batch-change-examples/blob/main/ticketing-systems/github-issues/README.md)

### What happens to the preview page if the batch spec is not applied?
Unapplied batch specs are removed from the database after 7 days.

### Can I pull containers from private container registries in a batch change?
Yes. When [executing a batch spec](../explanations/how_src_executes_a_batch_spec.md), `src` will attempt to pull missing docker images. If you are logged into the private container registry, it will pull from it. Also see [`steps.container`](batch_spec_yaml_reference.md#steps-container). Within the spec, if `docker pull` points to your private registry from the command line, it will work as expected. 

However, outside of the spec, `src` pulls an image from Docker Hub when running in volume workspace mode. This is the default on macOS, so you will need to use one of the following three workarounds:

1. Run `src` with the `-workspace bind` flag. This will be slower, but will prevent `src` from pulling the image.
2. If you have a way of replicating trusted images onto your private registry, you can replicate [our image](https://hub.docker.com/r/sourcegraph/src-batch-change-volume-workspace) to your private registry. Ensure that the replicated image has the same tags, or this will fail.
3. If you have the ability to ad hoc pull images from public Docker Hub, you can run `docker pull -a sourcegraph/src-batch-change-volume-workspace` to pull the image and its tags.

> NOTE: If you choose to replicate or pull the Docker image, you should ensure that it is frequently synchronized, as a new tag is pushed each time `src` is released.

### What tool can I use for changing/refactoring `<programming-language>`?

Batch Changes supports any tool that can run in a container and changes file contents on disk. You can use the tool/script that works for your stack or build your own, but here is a list of [examples](https://github.com/sourcegraph/batch-change-examples) to get started.
Common language agnostic starting points:

- `sed`, [`yq`](https://github.com/mikefarah/yq), `awk` are common utilities for changing text
- [comby](https://comby.dev/docs/overview) is a language-aware structural code search and replace tool. It can match expressions and function blocks, and is great for more complex changes.

### Why can't I run steps with different container user IDs in the same batch change?

This is an artifact of [how Batch Changes executes batch specs](../explanations/how_src_executes_a_batch_spec.md). Consider this partial spec:

```yaml
steps:
  - run: /do-it.sh
    container: my-alpine-running-as-root

  - run: /do-it.sh
    container: my-alpine-running-as-uid-1000

  - run: /do-it.sh
    container: my-alpine-running-as-uid-500
```

Files created by the first step will be owned by UID 0 and (by default) have 0644 permissions, which means that the subsequent steps will be unable to modify or delete those files, as they are running as different, unprivileged users.

Even if the first step is replaced by one that runs as UID 1000, the same scenario will occur when the final step runs as UID 500: files created by the previous steps cannot be modified or deleted.

In theory, it's possible to run the first _n_ steps in a batch spec as an unprivileged user, and then run the last _n_ steps as root, but we don't recommend this due to the likelihood that later changes may cause issues. We strongly recommend only using containers that run as the same user in a single batch spec.

### How can I use [GitHub expression syntax](https://docs.github.com/en/actions/reference/context-and-expression-syntax-for-github-actions) (`${{ }}` literally) in my batch spec?

To tell Sourcegraph not to evaluate `${{ }}` like a normal [template delimiter](batch_spec_templating.md), you can quote it and wrap it in a second set of `${{ }}` like so:

```
${{ "${{ leave me alone! }}" }}
```

Keep in mind the context in which the inner `${{ }}` will be evaluated and be sure to escape characters as is appropriate. Check out the cheat sheet for an [example](batch_spec_cheat_sheet.md#write-a-github-actions-workflow-that-includes-github-expression-syntax) within a shell script.

### How is commit author determined for commits produced from Batch Changes?

Commit author is determined at the time of running `src batch [apply|preview]`. If no [author](./batch_spec_yaml_reference.md#changesettemplate-commit-author) key is defined in the batch spec, `src` will try to use the git config values for `user.name` and `user.email` from your local environment, or "batch-changes@sourcegraph.com" if no user is set.

### Why is the checkbox on my changeset disabled when I'm previewing a batch change?

Since Sourcegraph 3.31, it is possible to publish many types of changeset when previewing a batch change by modifying the publication state for the changeset directly from the UI (see ["Publishing changesets"](../how-tos/publishing_changesets.md#from-the-preview)). However, not every changeset can be published by Sourcegraph. By hovering over your changeset's disabled checkbox, you can see the reason why that specific changeset is not currently publishable. The most common reasons include:

- The changeset is already published (we cannot unpublish a changeset, or convert it back to a draft).
- The changeset's publication state is being controlled from your batch spec file (i.e. you have the [`published` flag set in your batch spec](batch_spec_yaml_reference.md#changesettemplate-published)); the batch spec takes precedence over the UI.
- You do not have permission to publish to the repository the changeset would be opened against.
- The changeset was imported (and was therefore already published by someone or something else).

The changeset may also be in a state that we cannot currently publish from: for example, because a previous push to the code host failed (in which case you should re-apply the batch change), or if you are actively detaching the changeset from your batch change.

### Why do my changesets take a long time to sync?
Have you [set up webhooks](requirements.md#batch-changes-effect-on-code-host-rate-limits)?

### Why has my changeset been archived?

When re-running a batch spec on an existing batch change, the scope of repositories affected may change if you modify your `on` statement or if Sourcegraph simply finds a different set of results than it did last time. If the new batch spec no longer matches a repository that Sourcegraph has already published a changeset for, that changeset will be closed on the codehost and marked as *archived* in the batch change when you apply the new batch spec. You will be able to see these actions from the preview screen before you apply the batch spec. Archived changesets are still associated with the batch change, but they will appear under the "Archived" tab on the batch change page instead.

See our [how-to guide](../how-tos/updating_a_batch_change.md#removing-changesets) to learn more about archiving changesets, including how to unarchive a changeset and how to remove a changeset from the batch change entirely.

### Why is my changeset read-only?

Unmerged changesets on repositories that have been archived on the code host will move into a *Read-Only* state, which reflects that they cannot be modified any further on the code host. Re-applying the batch change will result in no operations being performed on those changesets, even if they would otherwise be updated. The only exception is that changesets that would be [archived](#why-has-my-changeset-been-archived) due to the `on` statement or search results changing will still be archived.

If the repository is unarchived, Batch Changes will move the changeset back into its previous state the next time Sourcegraph syncs the repository.

### Why do I get different results counts when I run the same search query as a normal search vs. for my `repositoriesMatchingQuery` in a batch spec?

By default, a normal Sourcegraph search will return the total number of _matches_ for a given query, counting matches in the same file or repository as separate results. However, when you use the search query in your batch spec, the results are grouped based on the repository (or "workspace", if you're [working with monorepos](../how-tos/creating_changesets_per_project_in_monorepos.md))  they belong to, giving you the total number of _repositories_ (or _workspaces_) that match the query. This is because Batch Changes produces one changeset for each matching repository (or workspace).

So, if you have a search query that returns 10 results in a single repo, the batch spec will only return 1 result for that repo. This is the equivalent of supplying the `select:repo` aggregator parameter to your search query.

### Why do I get fewer changes in my changeset diff when I run a batch spec than there are results when I run the same search query?

Sourcegraph search shows you results on any repositories that you have read access to. However, Sourcegraph and Batch Changes do not know which repositories you have _write_ access to. This disparity most often stems from not having write access to one or more of the repositories where your search query returns results. Consider asking an admin to set up a [global service account token](../how-tos/configuring_credentials.md#global-service-account-tokens) if it's important that your batch change updates all matching repositories.

### Why is my batch change preview hanging?

When working with `src`, there are occurences where applying your batch spec might get stuck on a particular step. More so in the `Determining workspace type` step. The `Determining workspace type` is a simple step that decides if bind or volume modes should be used based on the command line flags, and the OS and architecture. If volume mode is used (which is default on Mac OS), then `src` will attempt to pull the `sourcegraph/src-batch-change-volume-workspace` Docker image from docker hub since that's required for the batch spec to be executed. The "hanging" is typically is caused by the local machine's CLI state. Restarting your computer and applying the batch spec again should fix this. 

### Can I create a batch change and use a team's namespace so that the team owns the batch change?

Yes, you can create a batch change under a team's namespace so that the team owns and manages the batch change. Here are the steps to achieve this:

1. Create an [organization](../../admin/organizations.md) on Sourcegraph for your team.
1. Add all members of your team to the organization.
1. When creating the batch change, select the organization's namespace instead of your personal namespace. This can be done via the UI or using the `-namespace` flag with `src batch preview/apply`.
1. The batch change will now be created under the organization's namespace.
1. All members of the organization (your team) will have admin permissions to manage the batch change.

So by using an organization's namespace, you can create a batch change that is owned and editable by the entire team, not just yourself.