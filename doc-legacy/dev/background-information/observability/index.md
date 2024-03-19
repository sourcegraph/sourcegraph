# Developing observability

This documentation is for generalized, usecase-agnostic development of Sourcegraph's observability.
Sourcegraph employees should also refer to the [handbook's observability section](https://handbook.sourcegraph.com/engineering/observability) for Sourcegraph-specific documentation.

> NOTE: For how to *use* Sourcegraph's observability and an overview of our observability features, refer to the [observability for site administrators documentation](../../../admin/observability/index.md).

## Overview

Observability at Sourcegraph includes:

| | Description | Examples |
|:--|------------|--------|
| **Monitoring** | how you know _when_ something is wrong | Dashboards & metrics, alerting, health checks |
| **Debugging** | how you debug _what_ is wrong | Tracing, logging |

## Concepts

- [Sourcegraph monitoring pillars](https://handbook.sourcegraph.com/engineering/observability/monitoring_pillars)
- [Sourcegraph monitoring architecture](https://handbook.sourcegraph.com/engineering/observability/monitoring_architecture)

## Guides

- [How to add observability](../../how-to/add_observability.md)
- [How to add logging](../../how-to/add_logging.md)
- [How to find monitoring](../../how-to/find_monitoring.md)
- [How to add monitoring](../../how-to/add_monitoring.md)
- [Set up local monitoring development](../../how-to/monitoring_local_dev.md)
- [Set up local OpenTelemetry development](../../how-to/opentelemetry_local_dev.md)

## Components

- [Monitoring generator](./monitoring-generator.md)
- [Sourcegraph Grafana](./grafana.md)
- [Sourcegraph Prometheus](./prometheus.md)
- [Sourcegraph cAdvisor](./cadvisor.md)
- [Observability for site administrators](../../../admin/observability/index.md)
