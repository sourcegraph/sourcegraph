# Sourcegraph Prometheus

The `sourcegraph/prometheus` image provides an all-in-one image through `prom-wrapper` with:

- Vanilla Prometheus with embedded Sourcegraph configuration
- Bundled Alertmanager with a `siteConfigSubscriber` sidecar service to automatically apply relevant configuration changes to Alertmanager

To learn more, refer to the [Sourcegraph observability developer guide](https://sourcegraph.com/docs/dev/background-information/observability) and [monitoring architecture](https://handbook.sourcegraph.com/engineering/observability/monitoring_architecture#sourcegraph-prometheus).

## Image API

```shell script
docker run \
    -v ${PROMETHEUS_DISK}:/prometheus \
    -v ${CONFIG_DIR}:/sg_prometheus_add_ons \
    sourcegraph/prometheus
```

Image expects two volumes mounted:

- at `/prometheus` a data directory where logs, the tsdb and other prometheus data files will live
- at `/sg_prometheus_add_ons` a directory that contains additional config files of two types:
  - rule files which must have the suffix `_rules.yml` in their filename (ie `gitserver_rules.yml`)
  - target files which must have the suffix `_targets.yml` in their filename (ie `local_targets.yml`)
  - if this directory contains a file named `prometheus.yml` it will be used as the main prometheus config file

You can specify additional flags to pass to Prometheus by setting the environment variable `PROMETHEUS_ADDITIONAL_FLAGS`, and similarly for Alertmanager, you can set the environment variable `ALERTMANAGER_ADDITIONAL_FLAGS`. For example, this can be used to leverage [high-availability Alertmanager](https://github.com/prometheus/alertmanager#high-availability) alongside `ALERTMANAGER_ENABLE_CLUSTER=true`.

`prom-wrapper` also accepts a few configuration options through environment variables - see [`cmd/prom-wrapper/main.go`](./cmd/prom-wrapper/main.go) for more details.

Alertmanager components can be disabled entirely with `DISABLE_ALERTMANAGER=true`.
Hello World
