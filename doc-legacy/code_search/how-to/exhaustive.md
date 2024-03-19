# Exhaustive Search (count:all)

Exhaustive search is a search that returns the complete set of every result matching an expression. Sourcegraph's search is optimized for fast interactive searching, and as such, there are time and match limits which can stop a search before it is exhaustive. To remove the limits, add `count:all` to your search query.

Exhaustive search is often needed when you want to solve security, compliance, code health, and other automated use cases based on the output of a search.

## Slow queries

A query can be slow due to not using the index (commit/diff, non-indexed branches); poorly using the index (complicated regular expression); or having a very large result set. This is a common concern with the exhaustive use cases, and we expect Sourcegraph to still return accurate results.

For a `count:all` query you will always get accurate statistics (eg number of matches) once the query is complete. If a repository is not searched (eg not cloned) it will be reported directly.

Over time the priority of a query is reduced. This is to ensure that we have the capacity to answer interactive queries quickly, while still allowing slow queries to run to completion.

### Timeouts

There are two sources of timeouts in a `count:all` query:

- A timeout in the HTTP load balancer in front of Sourcegraph (nginx/ELB/Cloudflare/etc). Your admin will likely need to increase timeouts for Sourcegraph endpoints. In particular the `.api/search/stream` path. This uses [SSE](https://en.wikipedia.org/wiki/Server-sent_events), so your reverse proxy may have specific support for these requests.
- A maximum timeout enforced by Sourcegraph. Your admin may need to increase the site configuration value (default 60s) with the following setting:

```json
"search.limits": {
    "maxTimeoutSeconds": 60,
  },
```

### Large result sets

The Sourcegraph webapp will only display up to 500 results (however will continue to display accurate statistics). If you need to process more than 500 results, please use the [Sourcegraph CLI](https://github.com/sourcegraph/src-cli). For now, you will need to pass in the `-stream` flag to efficiently get large result sets.

## Limitations

### Missing on Sourcegraph.com

This is a specific limitation for Sourcegraph.com and does not apply to customer instances. Sourcegraph.com does not keep a copy of all open source code it has ever discovered. However, it remembers that it discovered it. This leads to the repository showing up as "missing" when your repository filter includes it.

### Non-indexed backends

A search is unindexed if you are searching non-indexed branches or using diff/commit search. Using a non-indexed backend and searching all code in a large instance can take 10min+. This is likely much higher than any configured timeouts. See the [Timeouts](#timeouts) section on how to configure this use case.

Currently, our non-indexed backends do not use the same scheduling logic as indexed backends. This means concurrent slow non-indexed searches will impact resources of interactive searches.

An unindexed search can under-report result counts. This is due to limits on the number of results reported per file. See [#18298](https://github.com/sourcegraph/sourcegraph/issues/18298).
