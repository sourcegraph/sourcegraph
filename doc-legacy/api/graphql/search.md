# Sourcegraph search GraphQL API

This page adds some additional depth and context to Sourcegraph's search GraphQL API.

## Search Pagination in GraphQL API

Sourcegraph does not support pagination for search results when using the GraphQL search API due to the dynamic nature of search queries. The order of results may vary each time you run a search, making traditional pagination unreliable.

Instead, we recommend using the [stream search API](../stream_api/index.md) for scenarios where you need to run a long query and receive continuous results. This enables you to execute long-running queries.

## `src` CLI usage (easier than GraphQL)

Putting together a comprehensive GraphQL search query can be difficult. For this reason, we created the [`src` CLI tool](https://sourcegraph.com/github.com/sourcegraph/src-cli) which allows you to simply run a search query and get the JSON results without constructing the GraphQL query:

```
export SRC_ENDPOINT=https://sourcegraph.com
export SRC_ACCESS_TOKEN=secret

src search -json 'repo:pallets/flask error'
```

You can then consume the JSON output directly, add `--get-curl` to get a `curl` execution line, and more. See [the `src` CLI tool](https://sourcegraph.com/github.com/sourcegraph/src-cli) for more details.

