# Precise code navigation

<style>
  .video-container {
    position: relative;
    padding-bottom: 56.25%; /* 16:9 */
    height: 0;
  }
  .video-container iframe {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
  }
</style>

Precise code navigation relies on
[SCIP](https://github.com/sourcegraph/scip) (SCIP Code Intelligence Protocol) and
[LSIF](https://github.com/Microsoft/language-server-protocol/blob/master/indexFormat/specification.md)
(Language Server Index Format) data to deliver precomputed code navigation. It provides fast and highly accurate code navigation but needs to be periodically generated and uploaded to your Sourcegraph instance. Precise code navigation is an opt-in feature: repositories for which you have not uploaded indexes will continue to use the search-based code navigation.

## Getting started

See the [how-to guides](../how-to/index.md) to get started with precise code navigation.

## Cross-repository code navigation

Cross-repository code navigation works out-of-the-box when both the dependent repository and the dependency repository have indexes _at the correct commits or versions_.
We are working on relaxing this constraint so that nearest-commit functionality works on a cross-repository basis as well.

When the current repository has an index and a dependent doesn't,
the missing precise results will be supplemented with imprecise search-based code navigation.
This also applies when both repositories have indexes, but for a different set of versions.
For example, if repository A@v1 depends on B@v2,
then we will get precise cross-repository intelligence when we have indexes for both A@v1 and B@v2,
but would not get a precise result we instead have indexes for A@v1 and B@v1.

## Why are my results sometimes incorrect?

If an index is not found for a particular file in a repository, Sourcegraph will fall back to search-based code navigation.
You may occasionally see results from [search-based code navigation](search_based_code_navigation.md) even when you have uploaded an index.
This can happen in the following scenarios:

- The line containing the symbol was created or edited between the nearest indexed commit and the commit being browsed.
- The _Find references_ panel may include search-based results, but only after all of the precise results have been displayed. This ensures every symbol has useful code navigation.

## More about SCIP

- [Writing an SCIP indexer](writing_an_indexer.md)
<!-- - [Adding LSIF to many repositories](../how-to/adding_lsif_to_many_repos.md) -->
