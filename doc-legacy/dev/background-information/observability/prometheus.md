# Sourcegraph Prometheus

> WARNING: Looking for Prometheus documentation for Sourcegraph administrators?
> See the [metrics and dashboards documentation](../../../admin/observability/metrics.md#prometheus).

We ship a custom Prometheus image as part of a standard Sourcegraph distribution.
It currently [bundles Alertmanager](#alertmanager) as well as [integrations to the Sourcegraph web application](#prom-wrapper).
Learn more about it in our [monitoring architecture](https://handbook.sourcegraph.com/engineering/observability/monitoring_architecture#sourcegraph-prometheus).

Adding recording rules, alerts, etc. to this image is handled by the [monitoring generator](./monitoring-generator.md).

The image is defined in [`docker-images/prometheus`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/docker-images/prometheus).

## Metrics

See the [metrics and dashboards documentation](../../../admin/observability/metrics.md#grafana).

To learn more about developing metrics, see the [observability developer guides](./index.md#guides).

## Prom-wrapper

The entrypoint of the image is a sidecar program called the prom-wrapper.
It manages Prometheus and Alertmanager, and provides integration with the Sourcegraph frontend.
Learn more about it [here](https://handbook.sourcegraph.com/engineering/observability/monitoring_architecture#prom-wrapper).

The source code for this program is currently kept in [`docker-images/prometheus/cmd/prom-wrapper`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/docker-images/prometheus/cmd/prom-wrapper).
The prom-wrapper also exports an API which can be leveraged through the [`internal/src-prometheus` package](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/internal/src-prometheus).

To learn more about developing our observability stack, see the [local Sourcegraph monitoring development guide](../../how-to/monitoring_local_dev.md).

## Alertmanager

The [Sourcegraph Prometheus image ships with Alertmanager](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Edocker-images/prometheus/Dockerfile+FROM+prom/alertmanager&patternType=literal), which provides our [alerting capabilities](../../../admin/observability/alerting.md).

Note that [prom-wrapper](#prom-wrapper) uses a [fork of Alertmanager](https://github.com/sourcegraph/alertmanager) to better manipulate Alertmanager configuration—prom-wrapper needs to be able to write alertmanager configuration with secrets, etc, which the Alertmanager project is currently not planning on accepting changes for ([alertmanager#2316](https://github.com/prometheus/alertmanager/pull/2316)).
This *does not* affect the version of Alertmanager that we ship with, the fork exists purely for use as a library.

## Upgrading Prometheus or Alertmanager

When upgrading, it is better to upgrade both at once since the two projects share some common dependencies.
To perform an upgrade:

1. Make the appropriate version and sum changes to the [`sourcegraph/prometheus` Dockerfile](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+FROM+prom/:%5Bimg%7Eprometheus%7Calertmanager%5D::%5Bversion.%5D+OR+FROM+prom/alertmanager::%5Bversion.%5D+OR+LABEL+com.sourcegraph.:%5Bimg%7Eprometheus%7Calertmanager%5D.version%3D:%5Bversion.%5D&patternType=structural)
1. Ensure no image update steps are required by checking upstream Dockerfiles where required as noted in the [`sourcegraph/prometheus` Dockerfile](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/prometheus/Dockerfile) where appropriate
1. Upgrade the [Alertmanager and Prometheus Go client dependencies](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:go.mod+prometheus/alertmanager+OR+prometheus/client_golang&patternType=literal) where appropriate
   1. For the Alertmanager dependency, the fork needs to be upgraded to the appropriate version first: [`sourcegraph/alertmanager`](https://github.com/sourcegraph/alertmanager)
1. Ensure the image still builds: `./docker-images/prometheus/build.sh`
1. [Run the monitoring stack locally](../../how-to/monitoring_local_dev.md) and verify that:
   1. If upgrading Prometheus: all Prometheus rules are evaluated successfully (`localhost:9090/rules`)
   1. If upgrading Alertmanager: Alertmanager starts up correctly (`localhost:9090/alertmanager/#/status`), and [`observability.alerts` can be configured in site config](../../../admin/observability/alerting.md) (check this by adding an entry, e.g. Slack alerts) via the Sourcegraph web application, e.g:

      ```json
      "observability.alerts": [
        {
          "level": "critical",
          "notifier": {
            "type": "slack",
            "url": "https://sourcegraph.com",
          }
        }
      ]
      ```
