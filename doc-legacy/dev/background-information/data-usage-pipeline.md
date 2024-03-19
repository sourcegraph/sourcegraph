# DEPRECATED: Event level data usage pipeline

> WARNING: **This process is deprecated.** To export Telemetry events from Sourcegraph instances, refer to the new [telemetry reference](./telemetry/index.md).

This document outlines information about the ability to export raw user event data from Sourcegraph. This is limited
to certain managed instances (cloud) where the customer has signed a corresponding data collection agreement.

### What is it?

This process is a background job that can be enabled that will periodically scrape the `event_logs` table in the primary database
and send it to Sourcegraph centralized analytics. [Events](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:internal/database/event_logs.go+Event+type:symbol+select:symbol.struct&patternType=standard) stored in `event_logs` are product events performed by users or the system. More information can be found in [RFC 719: Managed Instance Telemetry](https://docs.google.com/document/d/1N9aO0uTlvwXI7FzdPjIUCn_d1tRkJUfsc0urWigRf6s/edit).

The [job interval](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%cmd/worker/internal/telemetry/telemetry_job%5C.go+JobCooldownDuration&patternType=standard) determines how often the job is executed. The [batch size option](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%cmd/worker/internal/telemetry/telemetry_job%5C.go+getBatchSize+type:symbol&patternType=standard) determines how many records can be pulled in a single scrape. The batch size has a [default value](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%cmd/worker/internal/telemetry/telemetry_job%5C.go+MaxEventsCountDefault&patternType=standard) and can be overridden with a site setting:
``` json
  "exportUsageTelemetry": {
    "batchSize": 100,
  }
```

The scraping job maintains state using a bookmark stored in the primary postgres database in the table `event_logs_scrape_state`. [If the bookmark is not found, one will be inserted](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/worker/internal/telemetry/telemetry_job.go?L424-440) such that the bookmark is the most recent event at the time.

The scraping process has a crude at-least once semantics guarantee. If any scrape should fail, the bookmark state will not be updated causing future scrapes to retry the same set of events.

### Allow list

Only events that [exist in an allow list](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@735bc0f69ce417ecce55a9194dbf349c954043e3/-/blob/internal/database/event_logs.go?L321-324) will be scraped. Events are keyed in the allow list by the `event_logs.name` column. The allow list can be found in the primary
postgres database in the table `event_logs_export_allowlist`.

#### Adding to the allow list
Modifying the allow list is performed using database migrations. To simplify this process and ensure consistency, use the [sg](https://docs.sourcegraph.com/dev/background-information/sg) tool:
``` shell
sg telemetry allowlist add --migration [event]
```

``` shell
sg telemetry allowlist remove --migration [event]
```

If you want to modify many events you can pass a file of newline delimited event names using `xargs`:
``` shell
cat /location/of/my/events/file | xargs sg telemetry allowlist add --migration
```

Create a pull request and get a review from the Data Engineering team.


#### Determine if an event is in the allow list
Currently, there is not a single document that shows the entire allow list. There are two options:
1. Start Sourcegraph and migrate to the latest version, and query the database
```postgresql
select * from event_logs_export_allowlist;
```
2. [Look through migration files](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:migrations+lang:sql+MY_EVENT_NAME&patternType=standard) to see if the event you are looking for has been added and not deleted


### How to enable for a managed instance
1. Ensure the managed instance has the [appropriate IAM policy](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-managed/-/blob/modules/terraform-managed-instance-new/iam.tf?L19-31&utm_source=raycast-sourcegraph&utm_campaign=search) applied
2. Update the managed instance deployment manifest to include the following environment variables:
   1. `EXPORT_USAGE_DATA_ENABLED=true`
   2. [`EXPORT_USAGE_DATA_TOPIC_NAME`](https://sourcegraph.sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/deploy-sourcegraph-managed%24+EXPORT_USAGE_DATA_TOPIC_NAME&patternType=standard)
   3. [`EXPORT_USAGE_DATA_TOPIC_PROJECT`](https://sourcegraph.sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/deploy-sourcegraph-managed%24+EXPORT_USAGE_DATA_TOPIC_PROJECT&patternType=standard)
3. Deploy the updated deployment manifest and restart the `worker` service.

### Monitoring
Each Sourcegraph instance with this export job enabled will emit metrics that are prefixed with `src_telemetry_job`.
