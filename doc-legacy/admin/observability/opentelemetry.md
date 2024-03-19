# OpenTelemetry

<span class="badge badge-note">Sourcegraph 4.0+</span>

> WARNING: Sourcegraph is actively working on implementing [OpenTelemetry](https://opentelemetry.io/) for all observability data. **The first—and currently only—[signal](https://opentelemetry.io/docs/concepts/signals/) to be fully integrated is [tracing](./tracing.md)**.

Sourcegraph exports OpenTelemetry data to a bundled [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) instance.
This service can be configured to ingest, process, and then export observability data to an observability backend of choice.
This approach offers a great deal of flexibility.

## Configuration

Sourcegraph's OpenTelemetry Collector is deployed with a [custom image, `sourcegraph/opentelemetry-collector`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/docker-images/opentelemetry-collector), and is configured with a configuration YAML file.
By default, `sourcegraph/opentelemetry-collector` is configured to not do anything with the data it receives, but [exporters to various backends](#exporters) can be configured for each signal we currently support—**currently, only [traces data](#tracing) is supported**.

Refer to the [documentation](https://opentelemetry.io/docs/collector/configuration/) for an in-depth explanation of the parts that compose a full collector pipeline.

For more details on configuring the OpenTelemetry collector for your deployment method, refer to the deployment-specific guidance:

- [Kubernetes with Kustomize](../deploy/kubernetes/configure.md#tracing)
- [Kubernetes with Helm](../deploy/kubernetes/helm.md#opentelemetry-collector)
- [Docker Compose](../deploy/docker-compose/operations.md#opentelemetry-collector)

## Tracing

Sourcegraph traces are exported in OpenTelemetry format to the bundled OpenTelemetry collector.
To learn more about Sourcegraph traces in general, refer to our [tracing documentation](tracing.md).

`sourcegraph/opentelemetry-collector` includes the following exporters that support traces:

- [OTLP-compatible backends](#otlp-compatible-backends) (includes services like Honeycomb and Grafana Tempo)
- [Jaeger](#jaeger)
- [Google Cloud](#google-cloud)

> NOTE: In case you require an additional exporter from the [`opentelemetry-collector-contrib` repository](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter), please [open an issue](https://github.com/sourcegraph/sourcegraph/issues).

Basic configuration for each tracing backend type is described below. Note that just adding a backend to the `exporters` block does not enable it—it must also be added to the `service` block.
Refer to the next snippet for a basic but complete example, which is the [default out-of-the-box configuration](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/opentelemetry-collector/configs/logging.yaml):

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

exporters:
  logging:
    loglevel: warn
    sampling_initial: 5
    sampling_thereafter: 200

service:
  pipelines:
    traces:
      receivers:
        - otlp
      exporters:
        - logging # The exporter name must be added here to enable it
```

### Sampling traces

To reduce the volume of traces being exported, the collector can be configured to apply sampling to the exported traces. Sourcegraph bundles the [probabilistic sampler](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor) and the [tail sampler](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/tailsamplingprocessor/README.md) as part of it's default collector container image.

If enabled, this sampling mechanism will be applied to all traces, regardless if a request was explictly marked as to be traced.

#### Probabilistic sampler

The probabilistic sampler hashes Trace IDs and determines whether a trace should be sampled based on this hash. Note that semantic convention of tags on a trace take precedence over Trace ID hashing when deciding whether a trace should be sampled or not.

Refer to the next snippet for an example on how to update the configuration to enable sampling using the probabilistic sampler.

```yaml
exporters:
  # ...

processors:
  probabilistic_sampler:
    hash_seed: 22 # An integer used to compute the hash algorithm. Note that all collectors for a given tier (e.g. behind the same load balancer) should have the same hash_seed.
    sampling_percentage: 10.0 # (default = 0): Percentage at which traces are sampled; >= 100 samples all traces

service:
  pipelines:
    # ...
    traces:
      #...
      processors: [probabilistic_sampler] # Plug the probabilistic sampler to the traces.
```

### Tail sampling

The tail sampler samples traces according to policies and the sampling decision of whether a trace should be sampled is determined at the _tail end_ of a pipeline. For more information on the supported policies and other configuration options of the sampler see [tail sampler configuration](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/tailsamplingprocessor/README.md).

The sampler waits for a certain amount of spans before making applying the configured policy. Due to it keeping a certain amount of spans in memory the sampler incurs as slight performance cost compared to the Probabilistic sampler. For a better comparison on probabilistic vs tail sampling processors see [here](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/tailsamplingprocessor/README.md#probabilistic-sampling-processor-compared-to-the-tail-sampling-processor-with-the-probabilistic-policy)

Refer to the next snippet for an example on how to update the configuration to enable tail sampling with a particular policy.

```yaml
receivers:
    # ...
exporters:
    # ...

processors:
  tail_sampling:
    # Wait time since the first span of a trace before making a sampling decision
    decision_wait: 30s # default value = 30s
    # Number of traces kept in memory
    num_traces: 50000 # default value = 50000
    # Expected number of new traces (helps in allocating data structures)
    expected_new_traces_per_sec: 10 # default value = 0
    # Recommended reading to understand how the policies are applied:
    # https://sourcegraph.com/github.com/open-telemetry/opentelemetry-collector-contrib@71dd19d2e59cd1f8aa9844461089d5c17efaa0ca/-/blob/processor/tailsamplingprocessor/processor.go?L214
    policies:
      [
          {
            # If a span contains `sampling_retain: true`, it will always be sampled (not dropped),
            # regardless of the probabilistic sampling.
            name: policy-retain,
            type: string_attribute,
            string_attribute: {key: sampling.retain, values: ['true']},
          },
          {
            # Only keep 10% of the traces.
            name: policy-probalistic,
            type: probabilistic,
            probabilistic: {sampling_percentage: 10}
          }
        ]

service:
  pipelines:
    traces:
      # ...
      processors: [tail_sampling]
```

### Filtering traces

As part of the default container image Sourcegraph bundles the [filter processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/filterprocessor/README.md). By configuring a pipeline to have a filter processor one is able to include or exclude (depending on configuration!) on whether a trace should be allowed through the pipeline and be exported.

Refer to the following snippet where a filter processor is configured to only allow traces with the service name "foobar" to continue through the pipeline. All other traces that do not have this service name will be dropped.

```yaml
exporters:
    # ...

receivers:
    # ...

processors:
   filter/foobar: # the format is <processor type>/<name> - note a name is NOT required
      spans:
          include:
             match_type: strict # regexp is also a supported type
             services:
             - "foobar"

service:
    pipelines:
        traces: # pipeline accepts all traces
        traces/foobar: # pipeline that only export foobar traces
            # ...
            processors: [filter/foobar]
```
## Exporters

Exporters send observability data from OpenTelemetry collector to desired backends.
Each exporter can support one, or several, OpenTelemetry signals.

This section outlines some common configurations for exporters—for more details, refer to the [official OpenTelemetry exporters documentation](https://opentelemetry.io/docs/collector/configuration/#exporters).

> NOTE: In case you require an additional exporter from the [`opentelemetry-collector-contrib` repository](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter), please [open an issue](https://github.com/sourcegraph/sourcegraph/issues).

### OTLP-compatible backends

Backends compatible with the [OpenTelemetry protocol (OTLP)](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md) include services like [Honeycomb](https://docs.honeycomb.io/getting-data-in/opentelemetry-overview/) and [Grafana Tempo](https://grafana.com/blog/2021/04/13/how-to-send-traces-to-grafana-clouds-tempo-service-with-opentelemetry-collector/).
OTLP-compatible backends typically accept the [OTLP gRPC protocol](#otlp-grpc-backends), but they can also implement the [OTLP HTTP protocol](#otlp-http-backends).

#### OTLP gRPC backends

Refer to the [`otlp` exporter documentation](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/otlpexporter/README.md) for all available options.

```yaml
exporters:
  otlp:
    endpoint: otelcol2:4317
    tls:
      cert_file: file.cert
      key_file: file.key
  otlp/2:
    endpoint: otelcol2:4317
    tls:
      insecure: true
```

#### OTLP HTTP backends

Refer to the [`otlphttp` exporter documentation](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter/README.md) for all available options.

```yaml
exporters:
  otlphttp:
    endpoint: https://example.com:4318/v1/traces
```

### Jaeger

Refer to the [`jaeger` exporter documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/jaegerexporter/README.md) for all options.

Most Sourcegraph deployment methods still ship with an opt-in Jaeger instance—to set this up, follow the relevant deployment guides, which will also set up the appropriate configuration for you:

- [Kubernetes with Kustomize](../deploy/kubernetes/configure.md#deploy-opentelemetry-collector-with-jaeger-as-tracing-backend)
- [Kubernetes with Helm](../deploy/kubernetes/helm.md#enable-the-bundled-jaeger-deployment)
- [Docker Compose](../deploy/docker-compose/operations.md#enable-the-bundled-jaeger-deployment)

If you wish to do additional configuration or connect to your own Jaeger instance, the deployed Collector image is bundled with a [basic configuration with Jaeger exporting](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/opentelemetry-collector/configs/jaeger.yaml).
If this configuration serves your needs, you do not have to provide a separate config—the Collector startup command can be set to `/bin/otelcol-sourcegraph --config=/etc/otel-collector/configs/jaeger.yaml`. Note that this requires the environment variable `$JAEGER_HOST` to be set on the Collector instance (i.e. the container in Kubernetes or Docker Compose):

```yaml
exporters:
  jaeger:
    # Default Jaeger gRPC server
    endpoint: "$JAEGER_HOST:14250"
    tls:
      insecure: true
```

### Google Cloud

Refer to the [`googlecloud` exporter documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/googlecloudexporter/README.md) for all available options.

If you run Sourcegraph on a GCP workload, all requests will be authenticated automatically. The documentation describes other authentication methods.

```yaml
exporters:
  googlecloud:
    # See docs
    project: project-name # or fetched from credentials
    retry_on_failure:
      enabled: false
```
