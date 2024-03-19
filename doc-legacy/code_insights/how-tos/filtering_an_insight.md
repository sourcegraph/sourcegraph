# Filtering a code insight

This how-to assumes that you already have [created some search insights](../quickstart.md).

> NOTE: filters are not yet available on language statistics insights.

### 1. Click the filter icon button for the insight you want to filter

While viewing the insight you want to filter, click the filter icon in the upper right corner of the insight. This will open the filter popover. 

### 2. Filter to include or exclude repositories using a regular expression

The filter popover gives you two inputs that allow you to to filter the repositories contributing to the insight, either via inclusion with `repo:` or exclusion with `-repo:`. 

Enter a regular expression for repository names that you'd like to include or exclude. Inclusion filters limit your insight to show data from only given repository matches. Exclusion filters filter out results from the repository matches. ([More details on how repo filters are applied](../explanations/code_insights_filters.md#repo-repo-filters).)

The regular expression to filter for a repository works the same as the [`repo:` filter in the search box](../../code_search/reference/queries.md#keywords-all-searches).

Examples:

| Pattern | Explanation |
|---------|-------------|
| `^github\.com/sourcegraph/sourcegraph$` | Filter the specific repository `github.com/sourcegraph/sourcegraph` |
| `^github\.com/sourcegraph/(sourcegraph\|about\|docsite)$` | Filter the specific repositories `github.com/sourcegraph/sourcegraph`, `github.com/sourcegraph/about` and `github.com/sourcegraph/docsite` |
| `^github\.com/sourcegraph/go-` | Filter all repositories that start with `github.com/sourcegraph/go-` |
| `service` | Filter all repositories that contain the word `service` in their name |
| `\.js$` | Filter all repositories that end in `.js` |

### 3. Or, filter to include a reusable group of repositories using a query-based search context

In the `context:` field of the filter panel, you can use a [query-based search context](../../code_search/how-to/search_contexts.md#beta-query-based-search-contexts) to filter your insights to only results matching repositories that match the query-based context's `repo:` keyword. 

First, if you haven't already created a search context, create a [query-based search context](../../code_search/how-to/search_contexts.md#beta-query-based-search-contexts). You can define any group of repos using the syntax `repo:(^github\.com/sourcegraph/sourcegraph$|^github\.com/sourcegraph/about$...)`. 

Then, in the context field, reference the search context's name, usually `@owner/name-of-context`. 

([More details on using search contexts as filters, and currently supported search keywords](../explanations/code_insights_filters.md#context-query-based-search-context-filters).)

### 4. Close the filter panel 

You can safely click outside the panel to close it and your filters will remain (until a page reload).

A dot next to the filter icon indicates that an insight currently has filters applied.

### 5. Edit or reset filters

Click the filter button again to edit or reset filters. 

### 6. Save your filters as defaults on this insight (optional)

By default, the filter will only be visible to you locally and will reset after a page reload.

To persist the filter so that anyone who views the insight will have the filters applied by default, click "Save/update default filters".

### 7. Save as new view (optional)

If you don't want to modify the insight for all future viewers, but still want to preserve your filtered view, you can select "save as new view." 

This will fork the insight and create a separate chart on the dashboard with these filters applied by default. The original insight will be unmodified, and future edits to either insight will be independent. 

