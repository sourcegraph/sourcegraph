# Introduction to code navigation

There are 2 types of code navigation that Sourcegraph supports: search-based and precise.

[**Search-based code navigation**](./search_based_code_navigation.md) works out of the box with all of the most popular programming languages.

**Precise code navigation** is an opt-in feature that is enabled in your admin settings and requires you to upload indexes for each repository to your Sourcegraph instance. Once setup is complete on Sourcegraph, precise code navigation is available for use across popular development tools:

- On the Sourcegraph web UI
- On code files on your code host, via [integrations](../../../integration/index.md)
- On diffs in your code review tool, via integrations
- Via the [Sourcegraph API](https://docs.sourcegraph.com/api/graphql)

## Search-based vs. precise

[Search-based code navigation](search_based_code_navigation.md) is available by default on all Sourcegraph instances and provides fuzzy code navigation using a combination of ctags and search. It is great for immediate access to code navigation features, but due to its dependence on text-based search its results are considered imprecise.

[Precise code navigation](precise_code_navigation.md) returns metadata from a knowledge graph that is generated through code analysis. The precomputation step is used to generate the graph results in lookups that are fast and have a high degree of accuracy.

To learn more about how to work with each type of code navigation, visit the [search-based](./search_based_code_navigation.md) and [precise](./precise_code_navigation.md) sections.
