# Configure Embeddings

<p class="subtitle">Learn how you can configure embeddings with Cody for better code context.</p>

Embeddings will not be configured for any repository unless an admin takes an action. There are two ways to configure an embedding:

1. Embeddings policies
2. Schedule embeddings jobs

## Policies

The recommended way to configure embeddings is by using policies. Embeddings policies define which repositories are automatically scheduled for embedding. These policies are configured through your admin dashboard and will be automatically updated based on the update interval.

### Create an embeddings policy

Admins can create embeddings policies from the website's admin page. Navigate to your site admin. Then go to **Cody > Embedding Policies** from the left navigation menu

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/embeddings/embedding-policies.png" class="screenshot" alt="Embedding policies page">

Here, you'll see a list of all existing embedding policies. Click the **Create new global policy** button to create a new policy and enter a policy name and click **Add repository pattern** to define a pattern.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/embeddings/new-policy-create.png" class="screenshot" alt="Create new global policy">

The pattern should match exactly, unless you use an asterisk `*`, which matches any sequence of zero or more characters. Finally, click on **Create policy**. The new policy will be shown in the list of policies.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/embeddings/new-policy-saved.png" class="screenshot" alt="policy-saved">

Once a policy is created, it is active. To deactivate a policy, you can delete it. All embeddings jobs are listed under **Cody > Embeddings Jobs**.

### How pattern matching works?

Let's walk through some examples to see how the pattern works.

#### Example 1

- Original: `github.com/sourcegraph/sourcegraph`
- Pattern: `github.com/sourcegraph/*`
- Result: The pattern matches the original and all other repositories under the `github.com/sourcegraph/` organization.

#### Example 2

- Original: `github.com/sourcegraph/sourcegraph`
- Pattern: `github.com/sourcegraph/sourcegraph`
- Result: The pattern matches only the original repository link.

#### Example 3

- Original: `github.com/sourcegraph/sourcegraph`
- Pattern: `*sourcegraph*`
- Result: The pattern matches the original and any repository, from any code host, with the word `sourcegraph` in it.

In all the above examples, the defined policy will be applied to all repositories that match the pattern.
If you choose not to define a pattern, the policy will be applied to up to [embeddings.policyRepositoryMatchLimit](./usage-and-limits.md#configure-global-policy-match-limit) repositories.

It's recommended embedding repositories that are only being actively developed,
as embedding all repositories without discrimination can consume significant resources without necessarily providing better context for Cody.

### Lifecycle of an embeddings policy

Every 15 minutes, a worker process checks the embeddings policies and resolves them into a list of repositories to index.

Another worker then creates a new index job for each repository and queues it for processing.
A repository cannot be queued for processing if:

- It is already queued or being processed
- A job for the same repository and the same revision already completed successfully
- If another job for the same repository has been queued for processing within the [embeddings.MinimumInterval](./../embeddings.md#minimum-time-interval-between-automatically-scheduled-embeddings) time window

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/embeddings/embeddings-jobs.png" class="screenshot" alt="policy-saved">

## Schedule embeddings jobs

Admins can manually schedule one-off embeddings jobs for specific repositories from the website admin. These embeddings will not be automatically updated.

- Go to **Cody > Embeddings Jobs** from the left navigation menu of your site's admin
- Type your repository name in the search bar you want to index. You can select multiple repositories
- Finally, click the **Schedule Embedding** button

The new jobs will be shown in the list of jobs below. The initial status of the jobs will be `QUEUED` and will change to `COMPLETED` once the job is ready.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/embeddings/schedule-one-off-jobs.png" class="screenshot" alt="schedule one-off embeddings jobs">

Whether created manually or through a policy, embeddings will be generated incrementally if [incremental updates](./../embeddings.md#incremental-embeddings) are enabled.

> NOTE: Generating embeddings sends code snippets to a third-party language party provider. By enabling Cody, you agree to the [Cody Notice and Usage Policy](https://sourcegraph.com/terms/cody-notice).
