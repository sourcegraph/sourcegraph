# How to add observability

This guide documents how to add a combination of logging, tracing, and monitoring via the all-in-one [`internal/observation` package](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/internal/observation).
If you're not ready to migrate completely to `internal/observation`, you can consult one of the guides for direct usage of specific observability components:

- [How to add logging](add_logging.md)
- [How to add monitoring](add_monitoring.md)
- [Set up local monitoring development](monitoring_local_dev.md)
- [Set up local OpenTelemetry development](opentelemetry_local_dev.md)

> NOTE: For how to *use* Sourcegraph's observability and an overview of our observability features, refer to the [observability for site administrators documentation](../../admin/observability/index.md).

## Core concepts

The high-level ideas behind the [`internal/observation` package](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/internal/observation) are:

- Each service creates an observation `Context` that carries a root logger, tracer, and a metrics registerer as its context.
- An observation `Context` can create an observation `Operation` which represents a section of code that can be invoked many times.
  - An observation `Operation` is configured with state that applies to all invocation of the code.
- An observation `Operation` can wrap a an invocation of a section of code by calling its `With` method. This prepares a trace and some state to be reconciled after the invocation has completed.
  - The `With` method returns a function that, when deferred, will emit metrics, additional logs, and finalize the trace span.

## Usage

```go
observationContext := observation.NewContext(log.Scoped("my-scope", "a simple description"))

metrics := metrics.NewREDMetrics(
    observationContext.Registerer,
    "thing",
    metrics.WithLabels("op"),
)

operation := observationContext.Operation(observation.Op{
    Name:         "Thing.SomeOperation",
    MetricLabelValues: []string{"some_operation"},
    Metrics:      metrics,
})

// You can log some logs directly using operation—these logs will be structured
// with context about your operation.
operation.Info("something happened!", log.String("additional", "context"))

function SomeOperation(ctx context.Context) (err error) {
    // logs and metrics may be available before or after the operation, so they
    // can be supplied either at the start of the operation, or after in the
    // defer of endObservation.

    ctx, trace, endObservation := operation.With(ctx, &err, observation.Args{ /* logs and metrics */ })
    defer func() { endObservation(1, observation.Args{ /* additional logs and metrics */ }) }()

    // ...

    // You can log some logs directly from the returned trace—these logs will be
    // structured with the trace ID, trace fields, and observation context.
    trace.Info("I did the thing!", log.Int("things", 3))

    // ...
}
```

Log fields and metric labels can be supplied at construction of an `Operation`, at invocation of an operation (the `With` function), or after the invocation completes but before the observation has terminated (the `endObservation` function). Log fields and metric labels are concatenated together in the order they are attached to an operation.
