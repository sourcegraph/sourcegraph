# Enable code navigation auto-indexing

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is in beta for self-hosted customers.
</p>
</aside>

This how-to explains how to turn on [auto-indexing](../explanations/auto_indexing.md) on your Sourcegraph instance to enable [precise code navigation](../explanations/precise_code_navigation.md).

## Deploy executors

First, [deploy the executor service](../../../../admin/executors/deploy_executors.md) targeting your Sourcegraph instance. This will provide the necessary compute resources that clone the target Git repository, securely analyze the code to produce a code graph data index, then upload that index to your Sourcegraph instance for processing.

## Enable index job scheduling

Next, enable the precise code navigation auto-indexing feature by enabling the following feature flag in your Sourcegraph instance's site configuration.

```yaml
{
  "codeIntelAutoIndexing.enabled": true
}
```

This step will control the scheduling of indexing jobs which are made available to the executors deployed in the previous step.

## Configure auto-indexing policies

Once auto-indexing has been enabled, [create auto-indexing policies](configure_auto_indexing.md) to control the set of repositories and commits that are eligible for indexing. Note that policies only select repositories and branches/tags/commits that _can_ be indexed, not _how_ they are indexed. The _how_ is handled in the next section.

## Configure auto-indexing jobs

For repository and commit pairs that are marked as eligible for indexing, the index job is either [inferred from the project](../explanations/auto_indexing_inference.md), or [explicitly configured](./configure_auto_indexing.md#explicit-index-job-configuration) on a per-repository basis.

## Tune the index scheduler

The frequency of index job scheduling can be tuned via the following environment variables read by `worker` service containers running the [`codeintel-auto-indexing`](../../../admin/workers.md#codeintel-auto-indexing) task.

**`PRECISE_CODE_INTEL_AUTO_INDEXING_TASK_INTERVAL`**: The frequency with which to run periodic codeintel auto-indexing tasks. Default is every 2 minutes.

**`PRECISE_CODE_INTEL_AUTO_INDEXING_REPOSITORY_PROCESS_DELAY`**: The minimum frequency that the same repository can be considered for auto-index scheduling. Default is every 24 hours.

**`PRECISE_CODE_INTEL_AUTO_INDEXING_REPOSITORY_BATCH_SIZE`**: The number of repositories to consider for auto-indexing scheduling at a time. Default is 2500.

**`PRECISE_CODE_INTEL_AUTO_INDEX_MAXIMUM_REPOSITORIES_INSPECTED_PER_SECOND`**: The maximum number of repositories inspected for auto-indexing per second. Set to zero to disable limit. Default is 0.

## Access to private repositories and packages

Auto-indexing jobs run as Docker containers on the Executors 
infrastructure, and by default don't have any knowledge about 
the private repositories or package registries your builds might use.

The type of information you will need to provide depends on the language and 
ecosystem, but the mechanism is the same - [adding a secret to Executors "Code Graph" 
namespace](../../../../admin/executors/executor_secrets.md).

See the [language-specific instructions](./configure_auto_indexing.md#private-repositories-and-packages-configuration)
