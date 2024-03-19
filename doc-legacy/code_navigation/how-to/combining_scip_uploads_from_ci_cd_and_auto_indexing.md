# Combining SCIP uploads from CI/CD and auto-indexing

Sourcegraph Enterprise instances can serve many profiles of repository size and build complexity, and therefore provides multiple methods precise SCIP index data for your team's repositories. We currently support:

1. Uploading [SCIP data yourself](./index_other_languages.md#4-upload-lsif-data) directly from an already-configured build or continuous integration server, as well as
2. [Auto-indexing](../explanations/auto_indexing.md), which schedules index creation within the Sourcegraph instance.

There is nothing preventing users from mix-and-matching these methods (we do it ourselves), but we do have some tips for doing so successfully.

First and foremost, try to that auto-indexing is disabled on repositories that will receive manually configured uploads. This can be done by ensuring that no [auto-indexing policies](./configure_auto_indexing.md#configure-auto-indexing-policies) exist for the target repo. Even if one is not created explicitly for this repo, it may still be covered under a global policy with a matching repository pattern.

If covering policies can't be changed to exclude a repo (or if you want _partially_ auto-indexed coverage), then the repository can be [explicitly configured](./configure_auto_indexing.md#explicit-index-job-configuration) with a set of auto-indexing jobs to schedule. Simply delete the job configuration that conflicts the explicitly configured project (the directory and indexer's target language should match).

Once an explicit set of jobs are configured, auto-inference will not update it. If the repository shape changes frequently (new index targets are added or removed). Therefore, this configuration should be manually refreshed as necessary.
