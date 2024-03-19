# Code Insights

<style>

.markdown-body h2 {
  margin-top: 2em;
}

.markdown-body ul {
  list-style:none;
  padding-left: 1em;
}

.markdown-body ul li {
  margin: 0.5em 0;
}

.markdown-body ul li:before {
  content: '';
  display: inline-block;
  height: 1.2em;
  width: 1em;
  background-size: contain;
  background-repeat: no-repeat;
  background-image: url(code_monitoring/file-icon.svg);
  margin-right: 0.5em;
  margin-bottom: -0.29em;
}

body.theme-dark .markdown-body ul li:before {
  filter: invert(50%);
}

</style>

<p class="subtitle">Anything you can search, you can track and analyze</p>

<img src="https://sourcegraphstatic.com/docs/images/code_insights/insights_index_light.png" class="screenshot theme-light-only" />
<img src="https://sourcegraphstatic.com/docs/images/code_insights/insights_index_dark.png" class="screenshot theme-dark-only" />

<p class="lead">Code Insights reveals high-level information about your codebase, based on both how your code changes over time and its current state.</p>

Code Insights is based on our universal code search, making it precise and configurable. Track anything that can be expressed with a Sourcegraph search query: migrations, package use, version adoption, code smells, codebase size and much more, across 1,000s of repositories. Code Insights will backfill years of historical data from your version control, so you can immediately reveal current trends and baselines. 

<div class="cta-group">
<a class="btn btn-primary" href="quickstart">★ Quickstart</a>
<a class="btn" href="language_insight_quickstart">Language Insight Quickstart</a>
<a href="https://www.youtube.com/watch?v=fMCUJQHfbUA" class="btn" alt="Watch the Code Insights demo video">Demo video</a>
<a class="btn" href="references/requirements">Requirements</a>
</div>

## [Explanations](explanations/index.md)

- [Administration and security of Code Insights](explanations/administration_and_security_of_code_insights.md)
- [Automatically generated data series for version or pattern tracking](explanations/automatically_generated_data_series.md)
- [Code Insights filters](explanations/code_insights_filters.md)
- [Current limitations of Code Insights](explanations/current_limitations_of_code_insights.md)
- [Search-screen search results aggregations](explanations/search_results_aggregations.md)
- [Viewing code insights](explanations/viewing_code_insights.md)
- [Data retention](explanations/data_retention.md)

## [How-tos](how-tos/index.md)

- [Creating a dashboard of code insights](how-tos/creating_a_custom_dashboard_of_code_insights.md)
- [Filtering an insight](how-tos/filtering_an_insight.md)
- [Troubleshooting](how-tos/Troubleshooting.md)

## [References](references/index.md)

- [Common use cases and recipes](references/common_use_cases.md)
- [Common reasons code insights may not match search results](references/common_reasons_code_insights_may_not_match_search_results.md)
- [Incomplete data errors](references/incomplete_data_points.md)
- [Licensing and limited access](references/license.md)
- [Managing code insights with the API](../api/graphql/managing-code-insights-with-api.md)
- [Requirements](references/requirements.md)
- [Code Insight repository scope](references/repository_scope.md)

