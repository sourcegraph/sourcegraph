# Sourcegraph Grafana

> WARNING: Looking for Grafana documentation for Sourcegraph administrators?
> See the [metrics and dashboards documentation](../../../admin/observability/metrics.md#grafana).

We ship a custom Grafana image as part of a standard Sourcegraph distribution.
Learn more about it in our [monitoring architecture](https://handbook.sourcegraph.com/engineering/observability/monitoring_architecture#sourcegraph-grafana).

Adding dashboards, panels, etc. to this image is handled by the [monitoring generator](./monitoring-generator.md).

The image is defined in [`docker-images/grafana`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/docker-images/grafana).

## Dashboards

See the [metrics and dashboards documentation](../../../admin/observability/metrics.md#grafana).

To learn more about building dashboards, see the [observability developer guides](./index.md#guides).

## Upgrading Grafana

To upgrade Grafana:

1. Make the appropriate version changes to the [`sourcegraph/grafana` Dockerfile](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+FROM+grafana/grafana::%5Bversion.%5D+OR+LABEL+com.sourcegraph.grafana.version%3D:%5Bversion.%5D&patternType=structural)
1. Ensure that no migration steps are required: [Migrate from previous Grafana container versions](https://grafana.com/docs/grafana/latest/installation/docker/#migrate-from-previous-docker-containers-versions)
1. Ensure the image still builds: `./docker-images/grafana/build.sh`
1. [Run the monitoring stack locally](../../how-to/monitoring_local_dev.md) and verify that all generated Grafana dashboards still render correctly
