# Telemetry export architecture

> WARNING: This is a guide intended for development reference.
>
> To learn more about telemetry export, refer to the [Sourcegraph adminstrator documentation on telemetry](../../../admin/telemetry/index.md).

This page outlines the architecture and components involved in Sourcegraph's new telemetry export system.

In the [lifecycle of an event](./index.md#event-lifecycle), events are first [stored](#storing-events) then [exported](#exporting-events) to [Telemetry Gateway](#telemetry-gateway).

See [testing events](./index.md#testing-events) for a summary of how to observe your events during development.

> WARNING: This page primarily pertains to the new telemetry system introduced in Sourcegraph 5.2.1 - refer to [DEPRECATED: Telemetry](deprecated.md) for the legacy system which may still be in use if a callsite has not been migrated yet.

## Storing events

Once [recorded](./index.md#recording-events), telemetry events are stored in two places:

1. The structured `event_logs` table, for use in [admin analytics](../../../admin/analytics.md), translated from the [Telemetry Gateway format](./index.md#exported-event-schema) on a best-effort basis.
2. The unstructured `telemetry_events_export_queue` table, which stores raw event payloads in Protobuf wire format for [export](#exporting-events).
   1. This table only retains events until they are marked as exported. Once exported, they are pruned after the duration specified by `TELEMETRY_GATEWAY_EXPORTER_EXPORTED_EVENTS_RETENTION`.

The "tee" store, including the translation from Telemetry Gateway event schema to the `event_logs` table, is implemented in [`internal/telemetry/teestore`](https://github.com/sourcegraph/sourcegraph/blob/main/internal/telemetry/teestore).

Note that before events are stored into `telemetry_events_export_queue`, [sensitive attributes are stripped](./index.md#sensitive-attributes) - this means that the contents of `telemetry_events_export_queue` are exactly what gets exported from an instance.

## Exporting events

The [`telemetrygatewayexporter`](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/worker/internal/telemetrygatewayexporter/telemetrygatewayexporter.go) running in the worker service spawns a set of background jobs that handle:

1. Reporting metrics on the `telemetry_events_export_queue`
2. Cleaning up already-exported entries in the `telemetry_events_export_queue`
3. Exporting batches of not-yet-exported entries in the `telemetry_events_export_queue` to the Telemetry Gateway service

When exporting events, we explicitly only mark an event as successfully exported when the Telemetry Gateway returns a response with a particular event's generated ID. This ensures we always export events at least once.

## Telemetry Gateway

The Telemetry Gateway is a managed Sourcegraph service that ingests event exports from all Sourcegraph instances, and handles manipulating the events and publishing raw payloads to a Pub/Sub topic.
It exposes a gRPC API defined in [`telemetrygateway/v1`](https://github.com/sourcegraph/sourcegraph/tree/main/internal/telemetrygateway/v1) - see [exported events schema](./index.md#exported-event-schema).

From the gRPC API, the Telemetry Gateway constructs raw JSON events to publish to a designated Pub/Sub topic that eventually makes its way into BigQuery.

Also see [How to set up Telemetry Gateway locally](../../how-to/telemetry_gateway.md).

For details about live Telemetry Gateway deployments, refer to [the handbook Telemetry Gateway page](https://handbook.sourcegraph.com/departments/engineering/teams/core-services/managed-services/telemetry-gateway/).
