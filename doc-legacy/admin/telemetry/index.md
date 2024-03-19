# Telemetry

Understanding how individuals and organizations use Sourcegraph is key to providing the highest level of support to Sourcegraph's customers.
To enable this, Sourcegraph collects several types of usage data from Sourcegraph instances by having Sourcegraph isntances emit telemetry:

- [Aggregated pings](../pings.md)
- [Telemetry events](#telemetry-events) (Sourcegraph 5.2.1 and later)

If you have any questions about telemetry collection, please reach out to your Sourcegraph account representative.

## Telemetry Events

<span class="badge badge-note">Sourcegraph 5.2.1+</span>

> WARNING: This section refers to a new system for collecting telemetry introduced in Sourcegraph 5.2.1, and only applies to telemetry that has been migrated to use this new system.
> Certain Sourcegraph components - namely [Cody editor extensions](../../cody/index.md) - will continue to use a legacy mechanism for reporting telemetry directly to Sourcegraph.com until the migration is complete.

Sourcegraph instances 5.2.1 and later collect telemetry events to understand usage patterns and help improve the product. Telemetry events can be generated when certain user actions occur, like opening files or performing searches. This data helps us provide the highest level of support to Sourcegraph's customers.

Sensitive data/PII exfiltration, intentional or not, is a significant concern to Sourcegraph that we take very seriously.
Some of th measures we take to ensure privacy and data security are:

1. Telemetry events are, by default, only allowed to export numeric metadata - for example, string values that may contain sensitive contents are generally redacted. String values we do retain are generally categorized information.
2. User identifiers are numeric and anonymized, as identifiers are specific per-instance.
3. Data will be encrypted while in motion from each Sourcegraph instance to Sourcegraph.

You can also explore our [telemetry development reference](../../dev/background-information/telemetry/index.md) to learn more about new system with which we record telemetry events, and refer to our [telemetry events data schema](../../dev/background-information/telemetry/protocol.md) for specific attributes that are and are not exported by default.
