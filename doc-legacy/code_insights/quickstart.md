# Quickstart for Code Insights

Get started and create your first [code insight](index.md) in 5 minutes or less.

## Introduction

In this guide, you'll create a Sourcegraph code insight that tracks the number of `TODOs` that appear in parts of your codebase.

For more information about Code Insights see the [Code Insights](index.md) documentation.

<img src="https://sourcegraphstatic.com/docs/images/code_insights/quickstart_TODOs_insight_dark.png" class="screenshot">

## Requirements

- You are a Sourcegraph enterprise customer. (Want code insights but aren't enterprise? [Let us know](mailto:feedback@sourcegraph.com).)
- Your Sourcegraph instance has at least 1 repository. (See "[Quickstart](../index.md#quick-install)" on how to setup a Sourcegraph instance.)
- Your Sourcegraph instance is deployed via [Docker Compose](../admin/deploy/docker-compose/index.md) or [Kubernetes](../admin/deploy/kubernetes/index.md). 
- You are running Sourcegraph version 3.31.1 (August 2021 release) or later.
    - Note: If you're on Sourcegraph version 3.24 to 3.28, you can instead follow [this gist](https://gist.github.com/Joelkw/f0582b164578aabc3ac936dee43f23e0) to create an insight. Due to the early stage of the product, it's more likely you'll run into trouble, though, so we recommend that you either upgrade your Sourcegraph or reach out to your Sourcegraph reps for help.

## Enable Code Insights

### 1. [If need be] Enable the experimental feature flag

**If you're on Sourcegraph version 3.31 or earlier** and you don't already see the "Insights" item in your main nav bar, you'll need to add the following to either your Sourcegraph user settings `sourcegraph.example.com/users/[username]/settings` or organization settings `sourcegraph.example.com/organizations/[your_org]/settings` (and _not_ in your site configuration settings):

```json
"experimentalFeatures": { "codeInsights": true },
 "extensions": {
      "sourcegraph/code-stats-insights": true,
      "sourcegraph/search-insights": true
 }
```

If you put this in your organization settings, everyone on your Sourcegraph insights will be able to see the "Insights" navbar menu item and create their own code insights. If you put the flag in your user settings, only you will have those abilities.

(Enabling code insights organization-wide doesn't mean that other users can automatically see the code insights you create, however – you can control that visibility per individual insight.)

### 2. Visit your example.sourcegraph.com/insights page and select "+ Create insight"

### 3. On the insight type selection page, select "Track changes"

This creates a code insight tracking an arbitrary input that you could run a Sourcegraph search for.

Your other options are to:

* [create a "Detect and track" insight](explanations/automatically_generated_data_series.md) to automatically generate your data series according to capture group matching (3.35+).
* [create a language-based insight](language_insight_quickstart.md) to show you language breakdown in your repositories.

### 4. Select the repositories you want to search

If you want an insight running over a specific amount of repositories, enter the repositories in the repository URL format, like `github.com/sourcegraph/sourcegraph`. 
Separate multiple repositories with a comma. The form field will validate that you've entered the repository correctly.

If you want to run an insight over all repositories, instead check the box to do so (available in Sourcegraph 3.31.1 and later). If you otherwise want to exclude specific repositories, you can do so after creating the insight by using filters (step 10). 

From Sourcegraph 4.5 you are able to define the repositories you want your insight to run over using a Sourcegraph search query. [Read the reference to learn more](references/repository_scope.md).


### 5. Define a data series to track the incidence of `TODO`

A data series becomes a line on your graph.

You can **Name** this data series something like `TODOs count`.

To track the incidence of TODOs, you can set your **Search query** to be simply `TODO`.

You can also select the color of your data series.

### 6. Add a title to the insight

Enter a descriptive **Title** for the chart, like `Count of TODOs in [repository name]`.

### 7. Set the distance between data points

Code insights give you twelve datapoints for each data series on insight creation. Setting it to one month means you'll see the results over the last year.

### 8. Click "create code insight" and view your insight.

You'll be taken to the `example.sourcegraph.com/insights` page and can view your insight.

### 9. Filter your insight to explore it further 

Click the filter button in the top right of an insight card to open the filters panel. This allows you to filter the insight down to a subset of repositories through inclusion or exclusion using regular expressions.
For more details, see [How to filter an insight](./how-tos/filtering_an_insight.md).
