# Configure code graph data retention policies

<style>
img.screenshot {
  display: block;
  margin: 1em auto;
  max-width: 600px;
  margin-bottom: 0.5em;
  border: 1px solid lightgrey;
  border-radius: 10px;
}
</style>

Configure the retention policies of your code graph data to prevent old data from taking up valuable space in the database.

As code graph data ages, it gets less and less relevant. Users tend to need code navigation on newer commits, tagged commits, and in branches. The data providing intelligence for an old commit on the main development branch is unlikely to be used and only takes up valuable space in the database.

> NOTE: See the [best practices guide](./policies_resource_usage_best_practices.md) for additional details on how policies affect resource usage.

Each policy has several configurable options, including:

- The set of Git branches or tags to which the policy applies.
- How long to keep the associated code graph data.
- For branches, whether or not to consider the _tip_ of the branch only or all commits contained in that branch.

Note that Sourcegraph tracks cross-repository dependencies and will not delete any data that is referenced by another code graph data index. This means that code graph data for dependencies pinned to older versions or dependencies that have reached a steady state and no longer receive frequent updates will not be deleted.

All upload records will be periodically compared against global data retention policies and their target repository's data retention policies. Uploads on the tip of the default branch for a repository will never expire, regardless of age.

## Applying data retention policies globally

Site admins can create data retention policies that are applied to _all repositories_ on your Sourcegraph instance. To view and edit these policies, navigate to the code graph configuration in the site-admin dashboard.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/global-list.png" class="screenshot" alt="Global data retention policy configuration list page">

By default, there are three _protected_ policies that cannot be deleted or disabled. These policies refer to all tagged commits (associated data being kept for one year by default), the tip of all branches (associated data being kept for three months by default), and the HEAD of the default branch (associated data being kept forever by default) for all repositories. Protected policies cannot be deleted or disabled, but the retention length can be modified to suit your instance's usage patterns.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/global-protected.png" class="screenshot" alt="Protected global data retention policy edit page">

New policies can also be created to apply to the HEAD of the default branch or to apply to any arbitrary Git branch or tag pattern. For example, you may want to never expire data for any major version tags in your organization.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/retention-create.png" class="screenshot" alt="Global data retention policy configuration edit page">
<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/retention-post-create.png" class="screenshot" alt="Global data retention policy configuration created confirmation">

New policies can be created to apply to a set of repositories that are matched by name. For example, you may want to change the duration retention on branches that exist within a particular set of repositories (in this example, repositories in the same organization matching `scip-*`).

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/retention-create-repo-list.png" class="screenshot" alt="Global data retention policy with repository patterns configuration edit page">
<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/retention-post-create-repo-list.png" class="screenshot" alt="Global data retention policy with repository patterns configuration created confirmation">

## Applying data retention policies to a specific repository

Data retention policies can also be created on a per-repository basis as commit and merge workflows differ widely from project to project. To view and edit repository-specific policies, navigate to the code graph settings on the target repository's index page.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/sg-3.33/repository-page.png" class="screenshot" alt="Repository index page">

The settings page will show all policies that apply to the given repository, including repository-specific policies and global policies.

In this example, we create the _`:bestcoder:` branch retention policy_ that ensures all commits visible to the tip of any branch matching the pattern `ef/*` will not be removed regardless of age.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/retention-repo-create.png" class="screenshot" alt="Repository-specific data retention policy configuration edit page">
<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/retention-repo-post-create.png" class="screenshot" alt="Repository-specific data retention policy configuration created confirmation">
