# DEPRECATED: Adding, changing, and debugging user event data

> WARNING: **This process is deprecated.** To export Telemetry events from Sourcegraph instances, refer to the new [telemetry reference](./telemetry/index.md).

This document outlines the process for adding or changing the raw user event data collected from Sourcegraph instances. This is limited to certain managed instances (cloud) where the customer has signed a corresponding data collection agreement.

### User event data philosophy

[Raw user event data](https://docs.sourcegraph.com/dev/background-information/data-usage-pipeline) is collected from logs in the `event_logs` table in the instance primary database and sent to Sourcegraph centralized analytics. These [events](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:internal/database/event_logs.go+Event+type:symbol+select:symbol.struct&patternType=standard) are a product of events performed by users or the system and represent our customers’ most sensitive data. We must preserve and build trust through only careful additions and changes to events added to the egress pipeline.

All user event data must be:
1. Anonymous (with only one exception, the email address of the initial site installer)
2. Non-specific (e.g., no repo names, no usernames, no file names, no specific search queries, etc.)

### Adding events to the raw user event data pipeline

Ensure that any events added to the data pipeline are consistent with the user level data [FAQ](https://docs.google.com/document/d/1vXHoMBnvI_SlOjft4Q1Zhb5ZoScS1IjZ4V1LSKgVxv8/edit#).

### Changing the BigQuery view

Edit the query from the [cloud console](https://console.cloud.google.com/bigquery?project=telligentsourcegraph&ws=!1m5!1m4!4m3!1stelligentsourcegraph!2sdotcom_events!3sevents_usage) to add additional data that is passed through the event batches. Any data that does not exist will persist as null rather than fail–this will ensure backward compatibility.

```(
WITH data as (
SELECT JSON_EXTRACT_ARRAY(data, '$') as json
FROM `telligentsourcegraph.dotcom_events.events_usage_raw`
)
select
--flattened_data,
JSON_EXTRACT_SCALAR(flattened_data, '$.name') as name,
JSON_EXTRACT_SCALAR(flattened_data, '$.url') as url,
JSON_EXTRACT_SCALAR(flattened_data, '$.user_id') as user_id,
JSON_EXTRACT_SCALAR(flattened_data, '$.anonymous_user_id') as anonymous_user_id,
JSON_EXTRACT_SCALAR(flattened_data, '$.source') as source,
JSON_EXTRACT_SCALAR(flattened_data, '$.argument') as argument,
JSON_EXTRACT_SCALAR(flattened_data, '$.version') as version,
JSON_EXTRACT_SCALAR(flattened_data, '$.timestamp') as timestamp,
JSON_EXTRACT_SCALAR(flattened_data, '$.firstSourceURL') as firstSourceURL,
JSON_EXTRACT_SCALAR(flattened_data, '$.first_source_url') as first_source_url,
JSON_EXTRACT_SCALAR(flattened_data, '$.feature_flags') as feature_flags,
JSON_EXTRACT_SCALAR(flattened_data, '$.cohort_id') as cohort_id,
JSON_EXTRACT_SCALAR(flattened_data, '$.referrer') as referrer,
JSON_EXTRACT_SCALAR(flattened_data, '$.public_argument') as public_argument,
JSON_EXTRACT_SCALAR(flattened_data, '$.device_id') as device_id,
JSON_EXTRACT_SCALAR(flattened_data, '$.insert_id') as insert_id,
JSON_EXTRACT_SCALAR(flattened_data, '$.last_source_url') as last_source_url,
JSON_EXTRACT_SCALAR(flattened_data, '$.site_id') as site_id,
JSON_EXTRACT_SCALAR(flattened_data, '$.license_key') as license_key,
JSON_EXTRACT_SCALAR(flattened_data, '$.initial_admin_email') as initial_admin_email,
JSON_EXTRACT_SCALAR(flattened_data, '$.deploy_type') as deploy_type,
from data
cross join unnest(data.json) as flattened_data
)
```

### Debugging

Working backward from where the data is coming from, we should receive alerts for anomalies along each step of the process here.
1. At the [view](https://console.cloud.google.com/bigquery?project=telligentsourcegraph&ws=!1m5!1m4!4m3!1stelligentsourcegraph!2sdotcom_events!3sevents_usage), the details expose the raw SQL of flattening out the payload batches then extracting the desired field from the JSON structure.
2. In the [raw table](https://console.cloud.google.com/bigquery?project=telligentsourcegraph&ws=!1m5!1m4!4m3!1stelligentsourcegraph!2sdotcom_events!3sevents_usage_raw), the subscription unloads any new data into this table as an array of JSON objects. The payloads are capped at source to 10MB.
3. In the [subscription](https://console.cloud.google.com/cloudpubsub/subscription/detail/dotcom-events-usage?project=telligentsourcegraph), there are metrics available in the cloud console. There is not currently any automated snapshot generation.
4. In the [pub/sub topic](https://console.cloud.google.com/cloudpubsub/topic/detail/dotcom-events-usage?project=telligentsourcegraph), there are also metrics available in the cloud console. There is not currently any automated snapshot generation. There is currently a 7 day retention duration for messages in the queue.
5. In the instance, this would point to an issue with the instance sending data at all. Check if there is also an interruption in [ping](https://docs.sourcegraph.com/dev/background-information/adding_ping_data) or other telemetry data.
