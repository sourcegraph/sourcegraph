# How to find monitoring

This guide documents how to find monitoring within Sourcegraph's source code.
Sourcegraph employees should also refer to the [handbook's monitoring section](https://handbook.sourcegraph.com/engineering/observability/monitoring) for Sourcegraph-specific documentation.
The [developing observability page](../background-information/observability/index.md) contains relevant documentation as well.

> NOTE: For how to *use* Sourcegraph's observability and an overview of our observability features, refer to the [observability for site administrators documentation](../../admin/observability/index.md).

## Alerts

Alerts are defined in the [`monitoring/definitions` package](https://k8s.sgdev.org/github.com/sourcegraph/sourcegraph/-/tree/monitoring/definitions)—for example, [querying for definitions of `Warning` or `Critical`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:monitoring/definitions+Warning:+:%5B_%5Cn%5D+OR+Critical:+:%5B_%5Cn%5D&patternType=structural) will surface all Sourcegraph alerts.

## Metrics

You can use Sourcegraph itself to search for metrics definitions—for example, by [querying for usages of `prometheus.HistogramOpts`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+prometheus.HistogramOpts%7B+:%5B_%5D+%7D+&patternType=structural).

Sometimes the metrics are hard to find because their name declarations are not literal strings, but are concatenated in code from variables.
In these cases you can try a specialized tool called [`promgrep`](https://github.com/sourcegraph/promgrep) to find them.

```sh
go get github.com/sourcegraph/promgrep
# in the root `sourcegraph/sourcegraph` source directory
promgrep <some_partial_metric_name> # no arguments lists all declared metrics
```
