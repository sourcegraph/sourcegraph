# Code Insights repository scope

A Code Insight runs on a list of repositories as specified on insight creation. The scope options are:

* Manually specify a list of repositories
* Run the insight over all repositories (Sourcegraph 3.31.1+).
  * From Sourcegraph 4.5 this is achieved through a `repo:.*` search query.
* Run the insight over repositories as returned from a repository search query (Sourcegraph 4.5+)
  * This replaces the "Run your insight over all repositories" checkbox in prior versions

## Using the repository search query box

![Code Insights repository search box with a query for repo:sourcegraph](https://storage.googleapis.com/sourcegraph-assets/docs/images/code_insights/create_insight_repo_selection.png)

The repository search query box allows you to search for repositories on your Sourcegraph instance using standard Sourcegraph `repo` filters, as well as boolean operators.

Some example use cases might be:
* I want my insight to run over all the repositories in the `sourcegraph` organisation, but not the `handbook` repository

```sgquery
repo:sourcegraph/* -repo:sourcegraph/handbook$
```

* I want my insight to run over all my repositories that have `github-actions`
```sgquery
repo:has.file(github-actions)
```

The repository search box functions as the Sourcegraph search box, so it will suggest repository names as you type.

### Refining and previewing your repositories

After writing your repository search query, the Code Insights creation UI will display how many repositories the query has resolved.

If this number is unexpected, you can preview the repositories using the `Preview results` link.

> NOTE: Repositories will include archived repositories and forks by default.

## How is the list of repositories resolved?

If you are using a search query to define the list of repositories to run your insight over, then:

1. On insight creation the list of repositories that matches your search query is fetched. All the historical searches that are used to backfill your insight are ran against this unchanged list of repositories.
2. Every newer point added to your insight will then fetch results using (your_repo_search_query) (your_insight_search_query), so the repositories will be resolved against the global state of your instance.
    * For example, if you have a repo query for `repo:docs or repo:handbook` and a query for `lang:Markdown TODO`, every point will fetch results for the following query:

```sgquery
(repo:docs or repo:handbook) (lang:Markdown TODO)
```


