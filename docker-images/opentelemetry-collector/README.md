# Sourcegraph OpenTelemetry collector

This distribution of the [OpenTelemetry collector](https://opentelemetry.io/docs/collector/) ships with:

- [selected integrations](#integrations) (receivers, exporters, and extensions) for the OpenTelemetry collector
- [basic collector configuration](#configurations) that can be used out-of-the-box in `/etc/otel-collector/configs` with the `--config` flag for some common Sourcegraph deployment configurations.

This custom build undergoes Sourcegraph's [image vulnerability scanning](https://sourcegraph.com/docs/dev/background-information/ci#image-vulnerability-scanning) to audit the bundled dependencies.

To get started:

```sh
./build.sh
docker run sourcegraph/opentelemetry-collector:dev --config /etc/otel-collector/configs/jaeger.yaml
```

## Integrations

This image ships with selected integrations from the [opentelemetry-collector](https://github.com/open-telemetry/opentelemetry-collector) and [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) repositories using the [OpenTelemetry Collector builder](https://go.opentelemetry.io/collector/cmd/builder).

See [`builder.template.yaml`](builder.template.yaml) to see what integrations are currently bundled.

## Configurations

Bundled configurations are in `/etc/otel-collector/configs` and can be used with the `--config` flag for some common Sourcegraph deployment configurations:

- [`configs/jaeger.yaml`](configs/jaeger.yaml) - useful for sending traces to Sourcegraph's bundled Jaeger instance.
- [`configs/honeycomb.yaml`](configs/honeycomb.yaml)
- [`configs/logging.yaml`](configs/logging.yaml)

You can also mount your own configuration to provide to the `--config` flag.

Each configuration requires environment variables to configure certain values - refer to the configuration file comments for more details.
To learn more about configuration in general, see the official [collector configuration docs](https://opentelemetry.io/docs/collector/configuration).

In the out-of-the-box configurations, debug pages ("zPages") are available at port 55679 by default - see [Exposed zPages routes](https://github.com/open-telemetry/opentelemetry-collector/blob/main/extension/zpagesextension/README.md#exposed-zpages-routes).
Hello World
