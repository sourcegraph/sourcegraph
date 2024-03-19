# Telemetry Gateway
<a name="top"></a>

<!--
    DO NOT EDIT: This file is auto-generated with 'bazel run //doc/dev/background-information/telemetry:write_telemetrygateway_doc',
    and the template is in 'internal/telemetrygateway/v1/protoc-gen-doc.tmpl'.
-->

This page contains generated documentation for telemetry event data that gets exported to Sourcegraph from individual Sourcegraph instances.

> WARNING: This page primarily pertains to the new telemetry system introduced in Sourcegraph 5.2.1 - refer to [DEPRECATED: Telemetry](deprecated.md) for the legacy system which may still be in use if a callsite has not been migrated yet.

## Table of Contents

- [telemetrygateway/v1/telemetrygateway.proto](#telemetrygateway_v1_telemetrygateway-proto)
    - [Event](#telemetrygateway-v1-Event)
    - [EventBillingMetadata](#telemetrygateway-v1-EventBillingMetadata)
    - [EventFeatureFlags](#telemetrygateway-v1-EventFeatureFlags)
    - [EventFeatureFlags.FlagsEntry](#telemetrygateway-v1-EventFeatureFlags-FlagsEntry)
    - [EventInteraction](#telemetrygateway-v1-EventInteraction)
    - [EventInteraction.Geolocation](#telemetrygateway-v1-EventInteraction-Geolocation)
    - [EventMarketingTracking](#telemetrygateway-v1-EventMarketingTracking)
    - [EventParameters](#telemetrygateway-v1-EventParameters)
    - [EventParameters.LegacyMetadataEntry](#telemetrygateway-v1-EventParameters-LegacyMetadataEntry)
    - [EventParameters.MetadataEntry](#telemetrygateway-v1-EventParameters-MetadataEntry)
    - [EventSource](#telemetrygateway-v1-EventSource)
    - [EventSource.Client](#telemetrygateway-v1-EventSource-Client)
    - [EventSource.Server](#telemetrygateway-v1-EventSource-Server)
    - [EventUser](#telemetrygateway-v1-EventUser)
    - [Identifier](#telemetrygateway-v1-Identifier)
    - [Identifier.LicensedInstanceIdentifier](#telemetrygateway-v1-Identifier-LicensedInstanceIdentifier)
    - [Identifier.ManagedServiceIdentifier](#telemetrygateway-v1-Identifier-ManagedServiceIdentifier)
    - [Identifier.UnlicensedInstanceIdentifier](#telemetrygateway-v1-Identifier-UnlicensedInstanceIdentifier)
    - [RecordEventRequest](#telemetrygateway-v1-RecordEventRequest)
    - [RecordEventResponse](#telemetrygateway-v1-RecordEventResponse)
    - [RecordEventsRequest](#telemetrygateway-v1-RecordEventsRequest)
    - [RecordEventsRequest.EventsPayload](#telemetrygateway-v1-RecordEventsRequest-EventsPayload)
    - [RecordEventsRequestMetadata](#telemetrygateway-v1-RecordEventsRequestMetadata)
    - [RecordEventsResponse](#telemetrygateway-v1-RecordEventsResponse)
  
    - [TelemeteryGatewayService](#telemetrygateway-v1-TelemeteryGatewayService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="telemetrygateway_v1_telemetrygateway-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## telemetrygateway/v1/telemetrygateway.proto



<a name="telemetrygateway-v1-Event"></a>

### Event



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | <p>Generated ID of the event, currently expected to be UUID v4.</p> |
| timestamp | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | <p>Timestamp of when the original event was recorded.</p> |
| feature | [string](#string) |  | <p>Feature associated with the event in camelCase, e.g. 'myFeature'.</p> |
| action | [string](#string) |  | <p>Action associated with the event in camelCase, e.g. 'pageView'.</p> |
| source | [EventSource](#telemetrygateway-v1-EventSource) |  | <p>Source of the event.</p> |
| parameters | [EventParameters](#telemetrygateway-v1-EventParameters) |  | <p>Parameters of the event.</p> |
| user | [EventUser](#telemetrygateway-v1-EventUser) | optional | <p>Optional user associated with the event.</p><p>This field should be hydrated by the Sourcegraph server, and not provided</p><p>by clients.</p> |
| feature_flags | [EventFeatureFlags](#telemetrygateway-v1-EventFeatureFlags) | optional | <p>Optional feature flags configured in the context of the event.</p> |
| marketing_tracking | [EventMarketingTracking](#telemetrygateway-v1-EventMarketingTracking) | optional | <p>Optional marketing campaign tracking parameters.</p><p>🚨 SECURITY: This metadata is NEVER exported from single-tenant Sourcegraph</p><p>instances, and is only exported for events tracked in the public</p><p>Sourcegraph.com instance and managed services.</p> |
| interaction | [EventInteraction](#telemetrygateway-v1-EventInteraction) | optional | <p>Optional metadata identifying the interaction that generated the event.</p> |






<a name="telemetrygateway-v1-EventBillingMetadata"></a>

### EventBillingMetadata



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| product | [string](#string) |  | <p>Billing product ID associated with the event.</p> |
| category | [string](#string) |  | <p>Billing category ID the event falls into.</p> |






<a name="telemetrygateway-v1-EventFeatureFlags"></a>

### EventFeatureFlags



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| flags | [EventFeatureFlags.FlagsEntry](#telemetrygateway-v1-EventFeatureFlags-FlagsEntry) | repeated | <p>Evaluated feature flags. In Soucegraph we currently only support boolean</p><p>feature flags, but in the API we allow arbitrary string values for future</p><p>extensibility.</p><p>This field should be hydrated by the Sourcegraph server, and not provided</p><p>by clients.</p> |






<a name="telemetrygateway-v1-EventFeatureFlags-FlagsEntry"></a>

### EventFeatureFlags.FlagsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  | <p></p> |
| value | [string](#string) |  | <p></p> |






<a name="telemetrygateway-v1-EventInteraction"></a>

### EventInteraction



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| trace_id | [string](#string) | optional | <p>OpenTelemetry trace ID representing the interaction associated with the event.</p> |
| interaction_id | [string](#string) | optional | <p>Custom interaction ID representing the interaction associated with the event.</p> |
| geolocation | [EventInteraction.Geolocation](#telemetrygateway-v1-EventInteraction-Geolocation) | optional | <p>Geolocation associated with the interaction, typically inferred from the</p><p>originating client's IP address (which we do not collect).</p> |






<a name="telemetrygateway-v1-EventInteraction-Geolocation"></a>

### EventInteraction.Geolocation



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| country_code | [string](#string) |  | <p>Inferred ISO 3166-1 alpha-2 or alpha-3 country code</p> |






<a name="telemetrygateway-v1-EventMarketingTracking"></a>

### EventMarketingTracking
Marketing campaign tracking metadata.

🚨 SECURITY: This metadata is NEVER exported from single-tenant Sourcegraph
instances, and is only exported for events tracked in the public
Sourcegraph.com instance and managed services.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| url | [string](#string) | optional | <p>URL the event occurred on.</p> |
| first_source_url | [string](#string) | optional | <p>Initial URL the user landed on.</p> |
| cohort_id | [string](#string) | optional | <p>Cohort ID to identify the user as part of a specific A/B test.</p> |
| referrer | [string](#string) | optional | <p>Referrer URL that refers the user to Sourcegraph.</p> |
| last_source_url | [string](#string) | optional | <p>Last source URL visited by the user.</p> |
| device_session_id | [string](#string) | optional | <p>Device session ID to identify the user's session.</p> |
| session_referrer | [string](#string) | optional | <p>Session referrer URL for the user.</p> |
| session_first_url | [string](#string) | optional | <p>First URL the user visited in their current session.</p> |






<a name="telemetrygateway-v1-EventParameters"></a>

### EventParameters



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [int32](#int32) |  | <p>Version of the event parameters, used for indicating the "shape" of this</p><p>event's metadata, beginning at 0. Useful for denoting if the shape of</p><p>metadata has changed in any way.</p> |
| legacy_metadata | [EventParameters.LegacyMetadataEntry](#telemetrygateway-v1-EventParameters-LegacyMetadataEntry) | repeated | <p>DEPRECATED, legacy metadata format that only accepted int64 - use the new</p><p>'metadata' field instead, which accepts float values. Values sent through</p><p>this proto field will be merged into the new metadata attributes.</p><p>We don't use a [deprecated = true] tag because we use this field to handle</p><p>accepting exporters sending metadata in this format.</p> |
| metadata | [EventParameters.MetadataEntry](#telemetrygateway-v1-EventParameters-MetadataEntry) | repeated | <p>Strictly typed metadata, restricted to integer values to avoid accidentally</p><p>exporting sensitive or private data.</p> |
| private_metadata | [google.protobuf.Struct](#google-protobuf-Struct) | optional | <p>Additional potentially sensitive metadata - i.e. not restricted to integer</p><p>values.</p><p>🚨 SECURITY: This metadata is NOT exported from instances by default, as it</p><p>can contain arbitrarily-shaped data that may accidentally contain sensitive</p><p>or private contents.</p><p>This metadata is only exported on an allowlist basis based on terms of</p><p>use agreements and combinations of event feature and action, alongside</p><p>careful audit of callsites.</p> |
| billing_metadata | [EventBillingMetadata](#telemetrygateway-v1-EventBillingMetadata) | optional | <p>Optional billing-related metadata.</p> |






<a name="telemetrygateway-v1-EventParameters-LegacyMetadataEntry"></a>

### EventParameters.LegacyMetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  | <p></p> |
| value | [int64](#int64) |  | <p></p> |






<a name="telemetrygateway-v1-EventParameters-MetadataEntry"></a>

### EventParameters.MetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  | <p></p> |
| value | [double](#double) |  | <p></p> |






<a name="telemetrygateway-v1-EventSource"></a>

### EventSource



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| server | [EventSource.Server](#telemetrygateway-v1-EventSource-Server) |  | <p>Information about the server that is publishing the event, based on</p><p>RecordEventsRequestMetadata.Identifier.</p> |
| client | [EventSource.Client](#telemetrygateway-v1-EventSource-Client) | optional | <p>Information about the client that generated the event.</p> |






<a name="telemetrygateway-v1-EventSource-Client"></a>

### EventSource.Client



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| name | [string](#string) |  | <p>Source client of the event.</p> |
| version | [string](#string) | optional | <p>Version of the client.</p> |






<a name="telemetrygateway-v1-EventSource-Server"></a>

### EventSource.Server



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [string](#string) |  | <p>Version of the server emitting the event, corresponding to</p><p>RecordEventsRequestMetadata.Identifier. For example, if the Identifier</p><p>indicates the publisher is a Sourcegraph instance, the version represents</p><p>the version of the Sourcegraph server.</p> |






<a name="telemetrygateway-v1-EventUser"></a>

### EventUser



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| user_id | [int64](#int64) | optional | <p>Sourcegraph instance database user ID of the user. User IDs are specific to</p><p>a Sourcegraph instance, and are not universal across Sourcegraph instances.</p><p>We use an int64 as an ID because in Sourcegraph, database user IDs are</p><p>always integers.</p> |
| anonymous_user_id | [string](#string) | optional | <p>Randomized unique identifier representing the user (typically stored in</p><p>localstorage in web clients, or similar mechanisms elsewhere). This is</p><p>often used for unauthenticated users, but can persist to authenticated</p><p>users as well.</p> |
| sams_external_id | [string](#string) | optional | <p>Sourcegraph Accounts Management System (SAMS) account associated with the</p><p>user, represented by a SAMS external user ID in a UUID format. This is only</p><p>valid for services leveraging SAMS as an identity provider - in other words,</p><p>traditional Sourcegraph instances will not provide this.</p><p>Learn more about SAMS: https://handbook.sourcegraph.com/departments/engineering/teams/core-services/sams</p> |






<a name="telemetrygateway-v1-Identifier"></a>

### Identifier



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| licensed_instance | [Identifier.LicensedInstanceIdentifier](#telemetrygateway-v1-Identifier-LicensedInstanceIdentifier) |  | <p>A licensed Sourcegraph instance.</p> |
| unlicensed_instance | [Identifier.UnlicensedInstanceIdentifier](#telemetrygateway-v1-Identifier-UnlicensedInstanceIdentifier) |  | <p>An unlicensed Sourcegraph instance.</p> |
| managed_service | [Identifier.ManagedServiceIdentifier](#telemetrygateway-v1-Identifier-ManagedServiceIdentifier) |  | <p>A service operated and managed by the Sourcegraph team, for example</p><p>a service deployed by MSP: https://handbook.sourcegraph.com/departments/engineering/teams/core-services/managed-services/platform/</p><p>Valid SAMS client credentials are required to publish events under a</p><p>managed service identifier. The required scope is</p><p>'telemetry_gateway::events::publish'. See go/sams-client-credentials and</p><p>go/sams-token-scopes for more information.</p> |






<a name="telemetrygateway-v1-Identifier-LicensedInstanceIdentifier"></a>

### Identifier.LicensedInstanceIdentifier



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| license_key | [string](#string) |  | <p>License key configured in the Sourcegraph instance emitting the event.</p> |
| instance_id | [string](#string) |  | <p>Self-reported Sourcegraph instance identifier.</p> |
| external_url | [string](#string) |  | <p>Instance external URL defined in the instance site configuration.</p> |






<a name="telemetrygateway-v1-Identifier-ManagedServiceIdentifier"></a>

### Identifier.ManagedServiceIdentifier



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| service_id | [string](#string) |  | <p>Self-reported service identifier, for example 'my-service'.</p> |
| service_environment | [string](#string) | optional | <p>Self-reported service environment, for example 'prod' or 'dev'.</p> |






<a name="telemetrygateway-v1-Identifier-UnlicensedInstanceIdentifier"></a>

### Identifier.UnlicensedInstanceIdentifier



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| instance_id | [string](#string) |  | <p>Self-reported Sourcegraph instance identifier.</p> |
| external_url | [string](#string) |  | <p>Instance external URL defined in the instance site configuration.</p> |






<a name="telemetrygateway-v1-RecordEventRequest"></a>

### RecordEventRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [RecordEventsRequestMetadata](#telemetrygateway-v1-RecordEventsRequestMetadata) |  | <p>Metadata about the events being recorded.</p> |
| event | [Event](#telemetrygateway-v1-Event) |  | <p>Event to record.</p> |






<a name="telemetrygateway-v1-RecordEventResponse"></a>

### RecordEventResponse







<a name="telemetrygateway-v1-RecordEventsRequest"></a>

### RecordEventsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| metadata | [RecordEventsRequestMetadata](#telemetrygateway-v1-RecordEventsRequestMetadata) |  | <p>Metadata about the events being recorded.</p> |
| events | [RecordEventsRequest.EventsPayload](#telemetrygateway-v1-RecordEventsRequest-EventsPayload) |  | <p>Batch of events to record in a single request. Clients should aim to</p><p>batch large event backlogs into a series of smaller requests in the</p><p>RecordEvents stream, being mindful of common limits in individual message</p><p>sizes: https://protobuf.dev/programming-guides/api/#bound-req-res-sizes</p> |






<a name="telemetrygateway-v1-RecordEventsRequest-EventsPayload"></a>

### RecordEventsRequest.EventsPayload



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| events | [Event](#telemetrygateway-v1-Event) | repeated | <p></p> |






<a name="telemetrygateway-v1-RecordEventsRequestMetadata"></a>

### RecordEventsRequestMetadata



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| request_id | [string](#string) |  | <p>Client-provided request identifier for diagnostics purposes.</p> |
| identifier | [Identifier](#telemetrygateway-v1-Identifier) |  | <p>Telemetry publisher self-identification - for example, a Sourcegraph</p><p>instance of some other kind of service.</p> |






<a name="telemetrygateway-v1-RecordEventsResponse"></a>

### RecordEventsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| succeeded_events | [string](#string) | repeated | <p>IDs of all events that were successfully recorded in the request.</p><p>Note that if succeeded_events is a subset of events that were submitted,</p><p>then some events failed to record and should be retried.</p> |





 <!-- end messages -->

 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="telemetrygateway-v1-TelemeteryGatewayService"></a>

### TelemeteryGatewayService


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| RecordEvents | [RecordEventsRequest](#telemetrygateway-v1-RecordEventsRequest) stream | [RecordEventsResponse](#telemetrygateway-v1-RecordEventsResponse) stream | <p>RecordEvents streams telemetry events in batches to the Telemetry Gateway</p><p>service. Events should only be considered delivered if recording is</p><p>acknowledged in RecordEventsResponse.</p><p>This is the preferred mechanism for exporting large volumes of events in</p><p>bulk.</p><p>🚨 SECURITY: Callers exporting for single-tenant Sourcegraph should check</p><p>the attributes of the Event type to ensure that only the appropriate fields</p><p>are exported, as some fields should only be exported on an allowlist basis.</p> |
| RecordEvent | [RecordEventRequest](#telemetrygateway-v1-RecordEventRequest) | [RecordEventResponse](#telemetrygateway-v1-RecordEventResponse) | <p>RecordEvent records a single telemetry event to the Telemetry Gateway service.</p><p>If the RPC succeeds, then the event was successfully published.</p><p>This RPC currently ONLY accepts events published by ManagedServiceIdentifier,</p><p>as this mechanism is intended for low-volume managed services. Higher-volume</p><p>use cases should implement a batching mechanism and use the RecordEvents</p><p>RPC instead.</p><p>🚨 SECURITY: Callers exporting for single-tenant Sourcegraph should check</p><p>the attributes of the Event type to ensure that only the appropriate fields</p><p>are exported, as some fields should only be exported on an allowlist basis.</p> |

 <!-- end services -->



## Scalar Value Types

| .proto Type | Notes |
| ----------- | ----- |
| <a name="double" /> double |  |
| <a name="float" /> float |  |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. |
| <a name="uint32" /> uint32 | Uses variable-length encoding. |
| <a name="uint64" /> uint64 | Uses variable-length encoding. |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. |
| <a name="sfixed32" /> sfixed32 | Always four bytes. |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. |
| <a name="bool" /> bool |  |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. |

