# Logs

This document describes the log output from Sourcegraph services and how to configure it.

Note: For request logs, see [Outbound request log](outbound-request-log.md).

## Log levels

A Sourcegraph service's log level is configured via the environment variable `SRC_LOG_LEVEL`. The valid values (from most to least verbose) are:

* `debug`: Debug. Output all logs. Default in cluster deployments. A legacy `dbug` value is also supported.
* `info`: Informational.
* `warn`: Warning. Default in Docker deployments.
* `eror`: Error.
* `crit`: Critical.

Learn more about how to apply these environment variables in [docker-compose](../deploy/docker-compose/index.md#set-environment-variables) and [server](../deploy/docker-single-container/index.md#environment-variables) deployments.

## Log format

A Sourcegraph service's log output format is configured via the environment variable `SRC_LOG_FORMAT`. The valid values are:

* `condensed`: Optimized for human readability.
* `json`: Machine-readable JSON format.
  * For certain services and log entries, Sourcegraph exports a [OpenTelemetry-compliant log data model](#opentelemetry).
* `json_gcp`: Machine-readable JSON format, tailored to [GCP cloud-logging expected structure.](https://cloud.google.com/logging/docs/structured-logging#special-payload-fields) 
* `logfmt`: The [logfmt](https://github.com/kr/logfmt) format.
  * Note that `logfmt` is no longer supported with [Sourcegraph's new internal logging standards](../../dev/how-to/add_logging.md) - if you need structured logs, we recommend using `json` instead. If set to `logfmt`, log output from new loggers will be in `condensed` format.

### OpenTelemetry

When [configured to export JSON logs](#log-format), Sourcegraph services that have migrated to the [new internal logging standard](../../dev/how-to/add_logging.md) that will export a JSON log format compliant with [OpenTelemetry's log data model](https://opentelemetry.io/docs/reference/specification/logs/data-model/):

```json
{
  "Timestamp": 1651000257893614000,
  "InstrumentationScope": "string",
  "SeverityText": "string (DEBUG, INFO, ...)",
  "Body": "string",
  "Attributes": { "key": "value" },
  "Resource": {
    "service.name": "string",
    "service.version": "string",
    "service.instance.id": "string",
  },
  "TraceId": "string (optional)",
  "SpanId": "string (optional)",
}
```

We also include the following non-OpenTelemetry fields:

```json
{
  "Caller": "string",
  "Function": "string",
}
```

## Log level overrides

A Sourcegraph service's log level can be configured for a specific `InstrumentationScope` and it's children. For example you can keep your log level at error, but turn on debug logs for a specific component. This is only used to increase verbosity. IE it can't be used to mute a scope.

This is configured by the environment variable `SRC_LOG_SCOPE_LEVEL`. It has the format `SCOPE_0=LEVEL_0,SCOPE_1=LEVEL_1,...`. For example to turn on debug logs for `service.UpdateScheduler` and `repoPurgeWorker` you would set the following on the `repo-updater` service:

```
SRC_LOG_SCOPE_LEVEL=service.UpdateScheduler=debug,repoPurgeWorker=debug
```

Note that this will also affect child scopes. So in the example you will also receive debug logs from `service.UpdateScheduler.RunUpdateLoop`.

## Log sampling

Sourcegraph services that have migrated to the [new internal logging standard](../../dev/how-to/add_logging.md) have log sampling enabled by default.
The first 100 identical log entries per second will always be output, but thereafter only every 100th identical message will be output.

This behaviour can be configured for each service using the following environment variables:

* `SRC_LOG_SAMPLING_INITIAL`: the number of entries with identical messages to always output per second
* `SRC_LOG_SAMPLING_THEREAFTER`: the number of entries with identical messages to discard before emitting another one per second, after `SRC_LOG_SAMPLING_INITIAL`.

Setting `SRC_LOG_SAMPLING_INITIAL` to `0` or `-1` will disable log sampling entirely.
