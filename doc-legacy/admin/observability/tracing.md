# Tracing

In site configuration, you can enable tracing globally by configuring a sampling mode in `observability.tracing`.
There are currently three modes:

* `"sampling": "selective"` (default) will cause a trace to be recorded only when `trace=1` is present as a URL parameter (though background jobs may still emit traces).
* `"sampling": "all"` will cause a trace to be recorded on every request.
* `"sampling": "none"` will disable all tracing.

`"selective"` is the recommended default, because collecting traces on all requests can be quite memory- and network-intensive.
If you have a large Sourcegraph instance (e.g,. more than 10k repositories), turn this on with caution.
Note that the policies above are implemented at an application level—to sample all traces, please configure your tracing backend directly.

We support the following tracing backend types:

* [`"type": "opentelemetry"`](#opentelemetry) (default)
* [`"type": "jaeger"`](#jaeger)

In addition, we also export some tracing [via net/trace](#nettrace).

## How to use traces

Tracing is a powerful debugging tool that can break down where time is spent over the lifecycle of a
request and help pinpoint the source of high latency or errors.
To get started with using traces, you must first [configure a tracing backend](#tracing-backends).

We generally follow the following algorithm to root-cause issues with traces:

1. Reproduce a slower user request (e.g., a search query that takes too long or times out) and acquire a trace:
   1. [Trace a search query](#trace-a-search-query)
   2. [Trace a GraphQL request](#trace-a-graphql-request)
2. Explore the breakdown of the request tree in the UI of your [tracing backend](#tracing-backends), such as Honeycomb or Jaeger. Look for:
   1. items near the leaves that take up a significant portion of the overall request time.
   2. spans that have errors attached to them
   3. [log entries](./logs.md) that correspond to spans in the trace (using the `TraceId` and `SpanId` fields)
3. Report this information to Sourcegraph (via [issue](https://github.com/sourcegraph/sourcegraph/issues/new) or [reaching out directly](https://sourcegraph.com/contact/request-info/)) by screenshotting the relevant trace or sharing the trace JSON.

### Trace a search query

To trace a search query, run a search on your Sourcegraph instance with the `?trace=1` query parameter.
A link to the [exported trace](#tracing-backends) should be show up in the search results:

![link to trace](https://user-images.githubusercontent.com/23356519/184953302-099bcb62-ccdb-4eed-be5d-801b7fe16d97.png)

Note that getting a trace URL requires `urlTemplate` to be configured.

### Trace a GraphQL request

To receive a traceID on a GraphQL request, include the header `X-Sourcegraph-Should-Trace: true` with the request.
The response headers of the response will now include an `x-trace-url` entry, which will have a URL the [exported trace](#tracing-backends).

Note that getting a trace URL requires `urlTemplate` to be configured.

## Tracing backends

Tracing backends can be configured for Sourcegraph to export traces to.
We support exporting traces via [OpenTelemetry](#opentelemetry) (recommended), or directly to [Jaeger](#jaeger).

### OpenTelemetry

To learn about exporting traces to various backends using OpenTelemetry, review our [OpenTelemetry documentation](./opentelemetry.md).
Once configured, you can set up a `urlTemplate` that points to your traces backend, which allows you to use the following variables:

* `{{ .TraceID }}` is the full trace ID
* `{{ .ExternalURL }}` is the external URL of your Sourcegraph instance

For example, if you [export your traces to Honeycomb](./opentelemetry.md#otlp-compatible-backends), your configuration might look like:

```json
{
  "observability.tracing": {
    "type": "opentelemetry",
    "urlTemplate": "https://ui.honeycomb.io/$ORG/environments/$DATASET/trace?trace_id={{ .TraceID }}"
  }
}
```

You can test the exporter by [tracing a search query](#trace-a-search-query).

### Jaeger

There are two ways to export traces to Jaeger:

1. **Recommended:** Configuring the [OpenTelemetry Collector](opentelemetry.md) (`"type": "opentelemetry"` in `observability.tracing`) to [send traces to a Jaeger instance](opentelemetry.md#jaeger).
2. Using the legacy `"type": "jaeger"` configuration in `observability.tracing` to send spans directly to Jaeger.

We strongly recommend using option 1 to use Jaeger, which is supported via opt-in mechanisms for each of our core deployment methods—to learn more, refer to the [Jaeger exporter documentation](opentelemetry.md#jaeger).

To use option 2 instead, which enables behaviour similar to how Sourcegraph exported traces before Sourcegraph 4.0, [Jaeger client environment variables](https://github.com/jaegertracing/jaeger-client-go#environment-variables) must be set on all services for traces to export to Jaeger correctly using `"observability.tracing": { "type": "jaeger" }`.

A mechanism within Sourcegraph is available to reverse-proxy a Jaeger instance by setting the `JAEGER_SERVER_URL` environment variable on the `frontend` service, which allows you to access Jaeger using `/-/debug/jaeger`.
The Jaeger instance will also need `QUERY_BASE_PATH='/-/debug/jaeger'` to be configured.
Once set up, you can use the following URL template for traces exported to Jaeger:

```json
{
  "observability.tracing": {
    // set "type" to "opentelemetry" for option 1, "jaeger" for option 2
    "urlTemplate": "{{ .ExternalURL }}/-/debug/jaeger/trace/{{ .TraceID }}"
  }
}
```

You can test the exporter by [tracing a search query](#trace-a-search-query).
