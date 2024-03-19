# Code Insights filters

Code insights filters allow you to filter a search insight to a subset of repositories. Each search insight has a filter icon that opens the filtering options. 

Filters take effect immediately, without needing to re-process the data series of the insight.

> NOTE: filters are not yet available on language statistics insights. 

## Filter options

### `repo:` Repo filters

You can include or exclude repositories from your insight using regular expressions, the same way you can with the `repo:` filter in [Sourcegraph searches](../../code_search/reference/queries.md#keywords-all-searches).
Only repository regex expressions are supported: unlike search, you cannot specify repository revisions using the syntax `repo:regexp-pattern@rev`. Predicate filters like `repo:has.path(...)` are also not supported. 

For inclusion, only repositories that have a repository name matching the regular expression will be counted.

For exclusion, only repositories that have a repository name **not** matching the regular expression will be counted.

If you combine both filters, the inclusion pattern will be applied first, then the exclusion pattern.

### `context:` Query-based search context filters 

You can use a [query-based search context](../../code_search/how-to/search_contexts.md#beta-query-based-search-contexts) to filter your insights to only results matching repositories that match the `repo:` or `-repo:` filter of the query-based context. 

You can use this to filter multiple insights to a group of repositories that need only be maintained in one location. When you update a context that's being used as a filter, the next time you load the page, the filtered insight will reflect the updated context. 
As with explicit `repo:` filters, only repository regex expressions are allowed: you cannot specify repository revisions or predicate filters.

At this time, all other filter keywords are not yet supported – only the `repo:` and `-repo:` keywords of a context are recognized. When creating your context, you can define any group of repos using the syntax `repo:(^github\.com/sourcegraph/sourcegraph$|^github\.com/sourcegraph/about$...)`. 

### Setting more than one filter 

Filters are intersected. This means if you use the search context `context:` filter that narrows your insight down from all repositories to some repo A, repo B, and repo C, and then you use the `repo:` filter that's set only to repo A, your insight will show only results from repo A. Similarly, if you were to use the same `context:` and then also use the `-repo:` exclusion filter set to repo C, your insight would show results from repo A and repo B. 

### Other filtering options

We're currently exploring additional filters that would be valuable. If you have feedback about a particular filter you'd like for code insights, we would [love to hear your feedback](mailto:feedback@sourcegraph.com).

## Filter persistance and sharing

### Filters are temporary by default

By default, filters are temporary (present until you refresh the page or switch to a different dashboard) and local (no one else can see them). You can modify filters without affecting what others see, unless or until you save the filters.

### Saving filters as defaults

You can set filters to be persistent, or default, even after a page reload on a code insights in two ways:

1. Create a filter and click "save/update default filters": this will save the filters so they persist for all viewers of this insight, on any dashboard page the insight appears. 
1. Create a filter and "save as new view": this will create a new insight chart with your filter applied as the default filter on that insight. It will _not_ also save these filters to the existing insight unless you also select "save/update default filters". 

### Filter indicator 

Insights that have any filters applied will have a small dot on their filter icon. The filter can always be reset by opening the filter panel again and clicking "Reset" above each filter – but note that, like any filter edit, you must also then "update default filters" to save that reset state if you want it to persist. 

### Saving a filter as a new view

When you create a filter and "save as new view," it will create a new insight chart with this filter saved as the default filters. Except the title and filters, all other configuration options will be cloned from the original insight. 

Filters and all other configuration options for the newly created view and the original insight are indpendent. Editing the forked insight (e.g. changing the data series query) will not change the original insight.
