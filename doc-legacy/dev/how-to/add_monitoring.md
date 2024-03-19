# How to add monitoring

This guide documents how to add monitoring to Sourcegraph's source code.
Sourcegraph employees should also refer to the [handbook's monitoring section](https://handbook.sourcegraph.com/engineering/observability/monitoring) for Sourcegraph-specific documentation.
The [developing observability page](../background-information/observability/index.md) contains relevant documentation as well.

> NOTE: For how to *use* Sourcegraph's observability and an overview of our observability features, refer to the [observability for site administrators documentation](../../admin/observability/index.md).

## Metrics

Service-side, metrics should be made available over HTTP for Prometheus to scrape.
By default, Prometheus expects metrics to be exported on `$SERVICEPORT/metrics`—for example, run your local Sourcegraph dev server and metrics should be available on `http://localhost:$SERVICEPORT/metrics`.
How this is configured varies across the various [Sourcegraph deployment options](../../admin/deploy/index.md)—see [tracking a new service](#tracking-a-new-service).

### Tracking a new service

In [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph), Prometheus uses the Kubernetes API to discover endpoints to scrape. Just add the following annotations to your service definition:

```yaml
metadata:
  annotations:
    prometheus.io/port: "$SERVICEPORT" # replace with the port your service runs on
    sourcegraph.prometheus/scrape: "true"
```

In [deploy-sourcegraph-docker](https://github.com/sourcegraph/deploy-sourcegraph-docker), Prometheus relies on targets defined in the [`prometheus_targets`](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/prometheus/prometheus_targets.yml) configuration file—you will need to add your service here.

## Alerts, dashboards, and documentation

Creating alerts, dashboards, and documentation for monitoring is powered by the Sourcegraph monitoring generator, which requires monitorings to be defined in our [monitoring definitions package](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/monitoring/definitions).
The monitoring generator provides [a lot of features and integrations with the Sourcegraph monitoring ecosystem](../background-information/observability/monitoring-generator.md#features) for free.

This section documents how to use develop monitoring definitions for a Sourcegraph service.
To get started, you should read:

- the [Sourcegraph monitoring pillars](https://handbook.sourcegraph.com/engineering/observability/monitoring_pillars) for some of the principles we try to uphold when developing monitoring
- relevant [reference documentation for the monitoring generator](../background-information/observability/monitoring-generator.md)

### Set up an observable

Monitoring is build around "observables"—something you wish to observe.
The generator API exposes this concept through the [`Observable` type](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/docs/monitoring/monitoring#Observable).

You can decide where to put your new observable by looking for an existing dashboard that your information should go in.
Think "when this number shows something bad, which service logs are likely to be most relevant?".
If you are just editing an existing observable,

Existing dashboards can be viewed by either:

- Visiting Grafana on an existing Sourcegraph instance that you have site admin permissions for, e.g. `example.sourcegraph.com/-/debug/grafana`—see the [metrics for site administrators documentation](../../admin/observability/metrics.md) for more details.
- [Running the monitoring stack locally](../how-to/monitoring_local_dev.md)

Once you have found a home for your observable, open that service's monitoring definition (e.g. `monitoring/frontend.go`, `monitoring/git_server.go`) in your editor.
Declare your [`Observable`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24%40master+file:%5Emonitoring/+type+Observable&patternType=literal) by:

- adding it to [an existing `Row` in the file](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@64aa473/-/blob/monitoring/frontend.go#L12-43)
- adding a new [`Row`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24%40master+file:%5Emonitoring/+type+Row&patternType=literal)
- adding a new [`Group`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24%40master+file:%5Emonitoring/+type+Group&patternType=literal) entirely

Here's an example `Observable` that we will use throughout this guide to get you started:

```go
{
  Name:        "some_metric_behaviour",
  Description: "some behaviour of a metric",
}
```

### Write a query

Use the Grafana Explore page on a Sourcegraph instance where you have site administrator access (`/-/debug/grafana/explore`) to start writing your Prometheus query.

```diff
{
    Name:        "some_metric_behaviour",
-   Description: "some behaviour of a metric",
+   Description: "some behaviour of a metric over 5m",
+   Query:       `histogram_quantile(0.99, sum by (le)(rate(search_request_duration{status="success}[5m])))`,
}
```

Make sure to update your description to reflect the query you end up with where relevant.

### Configure panel options

Panel options can be used to customize the visualization of your observable in Grafana.
This step is optional, but highly recommended.

There are not many panel options (intentionally) to keep things simple.
The primary thing you'll use is to change the Grafana display from plain numbers to a unit like seconds:

```diff
{
    Name:        "some_metric_behaviour",
    Description: "some behaviour of a metric over 5m",
    Query:       `histogram_quantile(0.99, sum by (le)(rate(search_request_duration{status="success}[5m])))`,
+   Panel:       monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
}
```

The default `monitoring.Panel()` configures a panel for your observable using recommended defaults, and provides a set of recommended customization options through [`ObservablePanel`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/docs/monitoring/monitoring#ObservablePanel).

Additional customizations can be made to your observable's panel using [`ObservablePanel.With()`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/docs/monitoring/monitoring#ObservablePanel.With) and [`ObservablePanelOption`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/docs/monitoring/monitoring#ObservablePanelOption).

### Add an alert

Alerts can be defined at two levels: warning, and critical.
They are used to provide Sourcegraph health notifications for site administrators.
This step is optional, but highly recommended.
If you opt not to include an alert, you must explicitly set `NoAlert: true` and [provide relevant documentation for this observable](#add-documentation).

To get started, refer to [understanding alerts](../../admin/observability/alerting.md#understanding-alerts) for what your alert should indicate.
Then make a guess about what a good or bad value for your query is—it's OK if this isn't perfect, just do your best.
You can then use the [ObservableAlertDefinition](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/docs/monitoring/monitoring#ObservableAlertDefinition) to add an alert to your Observable, for example:

```diff
{
    Name:        "some_metric_behaviour",
    Description: "some behaviour of a metric over 5m",
    Query:       `histogram_quantile(0.99, sum by (le)(rate(search_request_duration{status="success}[5m])))`,
    Panel:       monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
+   Warning:     monitoring.Alert().GreaterOrEqual(20),
}
```

Options like only alerting after a certain duration (`.For(time.Duration)`) are also available—refer to the [monitoring library reference](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/docs/monitoring/monitoring#ObservableAlertDefinition).

### Add documentation

It's best if you also add some Markdown documentation with your best guess of what someone _might consider doing_ if they observe the alert firing (again, just your best guess is good enough here):

```diff
{
    Name:        "some_metric_behaviour",
    Description: "some behaviour of a metric over 5m",
    Query:       `histogram_quantile(0.99, sum by (le)(rate(search_request_duration{status="success}[5m])))`,
    Warning:     monitoring.Alert().GreaterOrEqual(20),
    Panel:       monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
+   PossibleSolutions: `
+       - Look at 'SERVICE' logs for details on the slow search queries.
+   `,
}
```

```diff
{
    Name:        "some_metric_behaviour",
    Description: "some behaviour of a metric over 5m",
    Query:       `histogram_quantile(0.99, sum by (le)(rate(search_request_duration{status="success}[5m])))`,
    NoAlert:     true,
    Panel:       monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
+   Interpretation: `
+       This value might be high under X, Y, and Z conditions.
+   `,
}
```

> NOTE: In both `PossibleSolutions` and `Interpretation`, you can write plain Markdown with some slight modifications, such as single quotes are used instead of backticks for code formatting, and indention will automatically be removed for you.

### Validate your observable

Run the monitoring generator from the root Sourcegraph directory:

```sh
RELOAD=false sg run monitoring-generator
```

This will validate your Observable configuration and let you know of any changes you need to make if required.
If the generator runs successfully, you should now [run the monitoring stack locally](../how-to/monitoring_local_dev.md) to validate the output and results of your observable by hand.

Once everything looks good, open a pull request with your observable to the main Sourcegraph codebase!

## Centralized observability

You can opt-in to [Sourcegraph Cloud centralized observability's](https://handbook.sourcegraph.com/departments/cloud/technical-docs/observability/) multi-instance overviews dashboard by setting `MultiInstance: true` on your Observable:

```diff
{
    Name:        "some_metric_behaviour",
    Description: "some behaviour of a metric over 5m",
    Query:       `histogram_quantile(0.99, sum by (le)(rate(search_request_duration{status="success}[5m])))`,
    Panel:       monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),

+   MultiInstance: true,
}
```

Multi-instance panels are best used on panels with only 1 or very few time series, since each Cloud instance gets its own, separate time series for the Observable's `Query` - for hundreds of instances, panels with multiple time series can become unreadable or very slow to load.
