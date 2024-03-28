# searcher

Provides on-demand unindexed search for repositories. It scans through a git archive fetched from gitserver to find results, similar in nature to `git grep`.

This service should be scaled up the more on-demand searches that need to be done at once. For a search the frontend will scatter the search for each repo@commit across the replicas. The frontend will then gather the results. Like gitserver this is an IO and compute bound service. However, its state is just a disk cache which can be lost at anytime without being detrimental.

[Life of a search query](../../doc/dev/background-information/architecture/life-of-a-search-query.md)
Hello World
