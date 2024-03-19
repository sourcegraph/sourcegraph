# Features

## Powerful, flexible queries

Sourcegraph code search performs full-text searches and supports both regular expression and exact queries. By default, Sourcegraph searches across all your repositories. Our search query syntax allows for advanced queries, such as searching over any branch or commit, narrowing searches by programming language or file pattern, and more.

See the [query syntax](../reference/queries.md) and [query reference](../reference/language.md) documentation for a comprehensive overview of supported syntax.

## Language-aware structural code search

Sourcegraph supports advanced code search for specifically matching patterns inside code structures, like function parameters and loop bodies.

See the [structural search](../reference/structural.md) documentation for a detailed explanation of this search mode.

## Commit diff search

Search over commit diffs using `type:diff` to see how your codebase has changed over time. This is often used to find changes to particular functions, classes, or areas of the codebase when debugging.

You can also search within commit diffs on multiple branches by specifying the branches in a `repo:` field after the `@` sign. After the `@`, separate Git refs with `:`, specify Git ref globs by prefixing them with `*`, and exclude a commit reachable from a ref by prefixing it with `^`. Diff searches can be further narrowed down with parameters that filter by author and time.

See the [query syntax](../reference/queries.md#keywords-diff-and-commit-searches-only) documentation for a comprehensive list of supported parameters.

## Commit message search

Searching over commit messages is supported in Sourcegraph by adding `type:commit` to your search query. Separately, you can also use the `message:"any string"` parameter to filter `type:diff` searches for a given commit message. Commit message searches can narrowed down further with filters such as author and time.

See our [query syntax](../reference/queries.md#diff-and-commit-searches-only) documentation for a comprehensive list of supported parameters.

## Symbol search

Searching for symbols makes it easier to find specific functions, variables, and more. Use the `type:symbol` filter to search for symbol results. Symbol results also appear in typeahead suggestions, so you can jump directly to symbols by name. When on an [indexed](../../admin/search.md#indexed-search) commit, it uses Zoekt. Otherwise it uses the [symbols service](../../code_navigation/explanations/features.md#symbol-search)

## Smart Search

Smart Search helps find search results that are likely to be more useful than showing "no results" by trying slight variations of a user's original query. Smart Search automatically tries alternative queries based on a handful of rules (we know how easy it is to get tripped up by query syntax). When a query alternative finds results, those results are shown immediately. Smart Search is activated by toggling the lightning bolt <span style="display:inline-flex; vertical-align:middle; margin:2px"><img style="width:20px; height:20px" src="https://storage.googleapis.com/sourcegraph-assets/sourcegraph.com/blog/2022/smart-search-bar-lightning.png"/></span> in the search bar, and is on by default. Smart Search is only enabled in the web application and its results view (Search APIs remain the same and are unaffected).

### Example

Take a query like `go buf byte parser`, for example. Normally, Sourcegraph will search for the string "go buf byte parser" with those tokens in that order. If there are **_no_** results, Smart Search attempts variations of the query. One rule applies a `lang:` filter to known languages. For example, `go` may refer to the `Go` language, so we convert this token to a `lang:Go` filter. Another rule relaxes the ordering on remaining tokens so that we search for `buf AND byte AND parser` anywhere in the file. Here's an example of what Smart Search looks like in action:

<img src="https://storage.googleapis.com/sourcegraph-assets/sourcegraph.com/blog/2022/smart-search-example.png" alt="Smart Search example"/>
<br />

Note that if the original query finds results (which depends on the code it runs on), Smart Search has no effect. Smart Search does not otherwise intervene or interfere with search queries if those queries return results, and Sourcegraph behaves as usual.

### Configuration options

It is sometimes useful to check for the _absence_ of results (we _want_ to see zero matches). In these cases, Smart Search can be disabled temporarily by toggling the lightning button in the search bar. To deactivate Smart Search by default, set `"search.defaultMode": "precise"` in settings.

It is not possible to customize Smart Search rules at this time. So far a small number of rules are enabled based on feedback and utility. They affect the following query properties:

- Separate patterns with `AND` (pattern order doesn't matter)
- Patterns as filters (e.g., apply `lang:` or `type:symbol`  filters based on keywords)
- Quotes in queries (run a literal search for quoted patterns)
- Patterns as Regular Expressions (check patterns for likely regular expression syntax)

## Saved searches

Saved searches let you save and describe search queries so you can easily monitor the results on an ongoing basis. You can create a saved search for anything, including diffs and commits across all branches of your repositories. Saved searches can be an early warning system for common problems in your code and a way to monitor best practices, the progress of refactors, etc.

See the [saved searches](../how-to/saved_searches.md) documentation for instructions for setting up and configuring saved searches.

## Suggestions

As you type a query, the menu below will contain suggestions based on the query. Use the keyboard or mouse to select a suggestion. For example, if your query is `repo:foo file:\.js$ hello`, the suggestions will consist of the list of files that match your query.

You can also type in the partial name of a repository or filename to quickly jump to it. For example, typing in just `foo` would show you a list of repositories (first) and files with names containing _foo_.

## Search contexts

Search contexts help you search the code you care about on Sourcegraph. A search context represents a set of repositories at specific revisions on a Sourcegraph instance that will be targeted by search queries by default.

Every search on Sourcegraph uses a search context. Search contexts can be defined with the contexts selector shown in the search input, or entered directly in a search query.

If you currently use version contexts, you can automatically [convert your existing version contexts to search contexts](../../admin/how-to/converting-version-contexts-to-search-contexts.md). We recommend migrating to search contexts for a more intuitive, powerful search experience and the latest improvements and updates.

See the [search contexts](../how-to/search_contexts.md) documentation to learn how to use and create search contexts.

## Fuzzy search <span class="badge badge-primary">experimental</span>

> NOTE: This feature is still in active development. If you have any feedback on how we can improve this feature, please [let us know](https://github.com/sourcegraph/sourcegraph/discussions/42874).

Use the fuzzy finder to quickly navigate to a repository, symbol, or file.

To open the fuzzy finder, press `Cmd+K` (macOS) or `Ctrl+K` (Linux/Windows) from any page. Use the dedicated Repos, Symbols, and Files tabs to search only for a repository, symbol, or file. Each tab has a dedicated shortcut:

- Repos: Cmd+I (macOS), Ctrl+K (Linux/Windows)
- Symbols: Cmd+O (macOS), Cmd+Shift+O (macOS Safari), Ctrl+O (Linux/Windows)
- Files: Cmd+P (macOS), Ctrl+P (Linux/Windows)

<img src="https://storage.googleapis.com/sourcegraph-assets/Fuzzy%20Finder%20-%20All.png" alt="Fuzzy search">

Use the "Searching everywhere" or "Searching in this repo" filter to determine whether to search for results only in the active repository or globally.

<img src="https://storage.googleapis.com/sourcegraph-assets/Fuzzy%20Finder%20-%20Search%20Scope.png" alt="Fuzzy search">

## Multi-branch indexing

<aside class="experimental">
<p>
<span style="margin-right:0.25rem;" class="badge badge-experimental">Experimental</span> Multi-branch indexing is in the experimental stage and must be enabled by a Sourcegraph site admin in site configuration.
</p>
</aside>

The most common branch to search is your default branch. To speed up this common operation, Sourcegraph maintains an index of the source code on your default branch. Some organizations have other branches that are regularly searched. To speed up search for those branches, Sourcegraph can be configured to index up to 64 branches per repository.

Your site admin can configure indexed branches in site configuration under the `experimentalFeatures.search.index.branches` setting. For example:

``` json
"experimentalFeatures": {
  "search.index.branches": {
   "github.com/sourcegraph/sourcegraph": ["3.15", "develop"],
   "github.com/sourcegraph/src-cli": ["next"]
  }
}
```

Indexing multiple branches will add additional resource requirements to Sourcegraph (particularly memory). The indexer will deduplicate documents between branches. So the size of your index will grow in relation to the number of unique documents. Refer to our [resource estimator](../../../admin/deploy/resource_estimator.md) to estimate whether additional resources are required.

> NOTE: The default branch (`HEAD`) is always indexed.

> NOTE: All revisions specified in version contexts are also indexed.
