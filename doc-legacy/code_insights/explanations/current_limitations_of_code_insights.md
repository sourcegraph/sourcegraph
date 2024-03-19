# Current limitations of Code Insights

There are a few existing limitations.  

If you have strong feedback, please do [let us know](mailto:feedback@sourcegraph.com). 

_Limitations that are no longer current are [documented at the bottom](#older-versions-limitations) for the benefit of customers who have not yet upgraded._

## Insight chart position and size do not persist

You can resize and reorder charts on the dashboard for the purpose of taking a screenshot or presenting information, but that order will revert on a page refresh.

If the ordering of insights is important, you can remove and then re-add the insights in the order you'd like via the add/remove insights to dashboard flow. 

If the size is important, you can use the single-insight view page to consistently view an insight at a larger size, reachable by clicking the insight title or from the context three dots menu on the insight card under "Get shareable link". 

## Performance speed considerations for a data series running over many repositories

To accurately return historical data for insights running over many repositories, the backend service must run a large number of Sourcegraph searches. This means that unlike code insights running over just a few repositories, results are not returned instantly, but more often on the scale of 20-120 minutes, depending on:

* How many and how large the repositories you have connected to your instance
* The performance and resources of your Sourcegraph code insights instance in queries-per-second
* How well we can "compress" repositories so we don't need to run queries for each historic data point (e.g., if a repository hasn't changed in several months)

The number of insights you have does not affect the overall speed at which they run: it will take the same total time to run all of them whether or not you let each one finish before creating the next one. As of version 4.4.0 Insights prioritize completing the backfills for the insights that will complete the fastest.  In general this means that insights over many repositories will pause to allow insights over a few repositories to complete. 


## Creating insights over very large repositories (<3.42)

In some cases, depending on the size of the Sourcegraph instance and the size of the repo, you may see odd behavior or timeout errors if you try to create a code insight running over a single large repository. In this case, it's best to try: 

1. Create the insight, but check the box to "run over all repositories." (This sends the Insight backfilling jobs to the backend Sourcegraph instance worker which will handle them datapoint-by-datapoint. Running over an individual repository otherwise currently runs the jobs in bulk to generate its live preview.)
2. After the insight has finished running, [filter the insight](code_insights_filters.md#filter-options) to the specific repo you originally wanted to use. The filter resolves instantly. 

If this does not solve your problem, please reach out directly to your Sourcegraph contact or in your shared slack channel, as there are experimental solutions we are currently working on to further improve our handling of large repositories. 

## Accuracy considerations for an insight query returning a large result set

If you create an insight with a search query that returns a large result set that exceeds the search timeout (generally when there are over 1,000,000 results), non-historical data points may report undercounted numbers. This behaviour is tracked in [this issue](https://github.com/sourcegraph/sourcegraph/issues/37859). This is because non-historical data points are recorded with a global search query as opposed to per-repo queries we run for backfilling. For a large result set (e.g. a query for `test` with millions of results) the global query will be disadvantaged by the global search timeout. You can find more information on search timeouts in the [docs](https://docs.sourcegraph.com/code_search/how-to/exhaustive#timeouts). 

You can determine if this issue may be affecting your query by just running the query in the Search UI on `/search` with a `count:all` – if your search is returning `x results in 60s` (or the upper limit max timeout is configured to) then the search will time out on insights as well. Note that the duration could be more or less `60s`, e.g. you could encounter `60.02s` as well. 

In this case, you may want to try:

* Using a more granular query
* Changing your site configuration so that the timeout is increased, provided your instance setup allows it. [More information on timeouts](https://docs.sourcegraph.com/code_search/how-to/exhaustive#timeouts).

## General scale limitations 

Code Insights is disabled by default on single-docker deployment methods.

There are a few factors to consider with respect to scale and expected performance.

1. General permissiveness - instances that are more open (users can see most repos) will perform better than instances that are more restricted. Insights have been tested with users having up to 100k restricted repositories.
2. Number of repositories - Code Insights is well tested with insights running over ~35,000 repositories. It is recommended that users should scope their insights to the smallest set of repositories needed.  Users should expect at least linear degradation as repository count grows in both time to calculate insights, and render performance.
3. Large monorepos - Code Insights allocates a fixed amount of time for each query, so large repositories that cause query timeouts will likely not have exhaustive (and therefore accurate) results. As of version 4.4.0 we added visibility to this state via an icon on the insight. Prior to the 4.4.0 version a heuristic indicator for if this is a problem is seeing values "jump" (either a significant increase or decrease) between the backfilled datapoints on creation and the up-to-date datatpoints added after creation. 
4. High cardinality capture groups - When using a capture group insight, high cardinality matches (for example 1000 distinct matches per repository) will cause significant increase in loading times of charts. It is possible to exceed request timeouts if there are too many distinct matches.
5. Concurrent usage
  1. If there are many insight creators the insights will take longer to calculate.
  2. If there are more insight viewers loading times of charts may be impacted.


## Creating insights over specific branches and revisions

Code Insights does not yet support running over specific revisions. 

## VCS limitations

Code Insights only supports git based repositories and does not support perforce repositories that have sub-repo permissions enabled.

## Feature parity limitations 

### Features currently available only on insights over all your repositories

* **[Filtering insights](code_insights_filters.md)**: available in 3.41+ ~~we do not yet allow filtering for insights that run over explicitly defined lists of repositories, except for "detect and track" insights.~~

### Features currently available only on insights over explicitly defined repository lists

Because these insights need to run dramatically fewer queries than insights over thousands of repositories, you will have access to a number of features not _yet_ supported for insights over all repositories. These are: 

* **Live previews**: showing the preview of your insight in real time
* **[Released] Dynamic x-axis ranges**: available in 3.35+ ~~set a custom amount of historical data you care about~~
* **[Released] Editing data series queries after creation**: available in 3.35+ ~~for insights over all repositories, you must make a new insight if you wish to run a different query~~
* **[Released] "Diff click"**: available in 3.36+ ~~click a datapoint on your insight and be taken to a diff search showing any changes contributing to the difference between a datapoint and the prior one~~

> NOTE: many of the above-listed features will become available for insights over all repositories as well. The above list is ordererd top-down, where items on the top of the list will arrive roughly sooner than items on the bottom. 

### Limitations specific to "Detect and track patterns" insights (automatically generated data series)

Please see [Current limitations of automatically generated data series](automatically_generated_data_series.md#current-limitations).

## In certain cases, chart datapoints don't match the result count of a Sourcegraph search

There are currently a few subtle differences in how code insights and Sourcegraph web app searches handle defaults when searching over all repositories. Refer to [Common reasons code insights may not match search results](../references/common_reasons_code_insights_may_not_match_search_results.md). 

## Known bugs

Known bugs we plan to fix are tracked in our [GitHub repository here](https://github.com/sourcegraph/sourcegraph/issues?q=is%3Aopen+is%3Aissue+label%3Abug+label%3Ateam%2Fcode-insights). 

## Older versions' limitations

### Version 3.30 (July 2021) or older

#### Search-based Code Insights can only run over ~50-70 repositories 

Because this version of the prototype runs on frontend API calls to Sourcegraph searches, it may run slowly (or possibly timeout) if you're using it over many repositories or with many data series for each insight. 

#### The max match count is 5,000 matches per repository 

The current limit on searching over historical versions of repositories, which is an unindexed search, is 5,000 results per repository. If there are more than 5,000 matches, the search stops and returns a count of 5,000, and the code insight graph will calculate the overall chart using 5,000 as the match count for that repository. (This means if you query over two repositories and one of them hits this limit, the value shown on the graph will be 5,000 + [the match count in the other repository]). 

_This limit was lifted in the August 2021 release of Sourcegraph 3.31_


