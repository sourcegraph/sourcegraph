# Alerts reference

<!-- DO NOT EDIT: generated via: bazel run //doc/admin/observability:write_monitoring_docs -->

This document contains a complete reference of all alerts in Sourcegraph's monitoring, and next steps for when you find alerts that are firing.
If your alert isn't mentioned here, or if the next steps don't help, [contact us](mailto:support@sourcegraph.com) for assistance.

To learn more about Sourcegraph's alerting and how to set up alerts, see [our alerting guide](https://docs.sourcegraph.com/admin/observability/alerting).

## frontend: 99th_percentile_search_request_duration

<p class="subtitle">99th percentile successful search request duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 20s+ 99th percentile successful search request duration over 5m

**Next steps**

- **Get details on the exact queries that are slow** by configuring `"observability.logSlowSearches": 20,` in the site configuration and looking for `frontend` warning logs prefixed with `slow search request` for additional details.
- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the `indexed-search.Deployment.yaml` if regularly hitting max CPU utilization.
- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml` if regularly hitting max CPU utilization.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-99th-percentile-search-request-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_99th_percentile_search_request_duration"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.99, sum by (le) (rate(src_search_streaming_latency_seconds_bucket{source="browser"}[5m])))) >= 20)`

</details>

<br />

## frontend: 90th_percentile_search_request_duration

<p class="subtitle">90th percentile successful search request duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 15s+ 90th percentile successful search request duration over 5m

**Next steps**

- **Get details on the exact queries that are slow** by configuring `"observability.logSlowSearches": 15,` in the site configuration and looking for `frontend` warning logs prefixed with `slow search request` for additional details.
- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the `indexed-search.Deployment.yaml` if regularly hitting max CPU utilization.
- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml` if regularly hitting max CPU utilization.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-90th-percentile-search-request-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_90th_percentile_search_request_duration"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.9, sum by (le) (rate(src_search_streaming_latency_seconds_bucket{source="browser"}[5m])))) >= 15)`

</details>

<br />

## frontend: hard_timeout_search_responses

<p class="subtitle">hard timeout search responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2%+ hard timeout search responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-hard-timeout-search-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_hard_timeout_search_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max(((sum(increase(src_graphql_search_response{request_name!="CodeIntelSearch",source="browser",status="timeout"}[5m])) + sum(increase(src_graphql_search_response{alert_type="timed_out",request_name!="CodeIntelSearch",source="browser",status="alert"}[5m]))) / sum(increase(src_graphql_search_response{request_name!="CodeIntelSearch",source="browser"}[5m])) * 100) >= 2)`

</details>

<br />

## frontend: hard_error_search_responses

<p class="subtitle">hard error search responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2%+ hard error search responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-hard-error-search-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_hard_error_search_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (status) (increase(src_graphql_search_response{request_name!="CodeIntelSearch",source="browser",status=~"error"}[5m])) / ignoring (status) group_left () sum(increase(src_graphql_search_response{request_name!="CodeIntelSearch",source="browser"}[5m])) * 100) >= 2)`

</details>

<br />

## frontend: partial_timeout_search_responses

<p class="subtitle">partial timeout search responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ partial timeout search responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-partial-timeout-search-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_partial_timeout_search_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (status) (increase(src_graphql_search_response{request_name!="CodeIntelSearch",source="browser",status="partial_timeout"}[5m])) / ignoring (status) group_left () sum(increase(src_graphql_search_response{request_name!="CodeIntelSearch",source="browser"}[5m])) * 100) >= 5)`

</details>

<br />

## frontend: search_alert_user_suggestions

<p class="subtitle">search alert user suggestions shown every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ search alert user suggestions shown every 5m for 15m0s

**Next steps**

- This indicates your user`s are making syntax errors or similar user errors.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-search-alert-user-suggestions).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_search_alert_user_suggestions"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (alert_type) (increase(src_graphql_search_response{alert_type!~"timed_out|no_results__suggest_quotes",request_name!="CodeIntelSearch",source="browser",status="alert"}[5m])) / ignoring (alert_type) group_left () sum(increase(src_graphql_search_response{request_name!="CodeIntelSearch",source="browser"}[5m])) * 100) >= 5)`

</details>

<br />

## frontend: page_load_latency

<p class="subtitle">90th percentile page load latency over all routes over 10m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2s+ 90th percentile page load latency over all routes over 10m

**Next steps**

- Confirm that the Sourcegraph frontend has enough CPU/memory using the provisioning panels.
- Investigate potential sources of latency by selecting Explore and modifying the `sum by(le)` section to include additional labels: for example, `sum by(le, job)` or `sum by (le, instance)`.
- Trace a request to see what the slowest part is: https://sourcegraph.com/docs/admin/observability/tracing
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-page-load-latency).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_page_load_latency"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.9, sum by (le) (rate(src_http_request_duration_seconds_bucket{route!="blob",route!="raw",route!~"graphql.*"}[10m])))) >= 2)`

</details>

<br />

## frontend: 99th_percentile_search_codeintel_request_duration

<p class="subtitle">99th percentile code-intel successful search request duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 20s+ 99th percentile code-intel successful search request duration over 5m

**Next steps**

- **Get details on the exact queries that are slow** by configuring `"observability.logSlowSearches": 20,` in the site configuration and looking for `frontend` warning logs prefixed with `slow search request` for additional details.
- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the `indexed-search.Deployment.yaml` if regularly hitting max CPU utilization.
- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml` if regularly hitting max CPU utilization.
- This alert may indicate that your instance is struggling to process symbols queries on a monorepo, [learn more here](../how-to/monorepo-issues.md).
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-99th-percentile-search-codeintel-request-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_99th_percentile_search_codeintel_request_duration"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.99, sum by (le) (rate(src_graphql_field_seconds_bucket{error="false",field="results",request_name="CodeIntelSearch",source="browser",type="Search"}[5m])))) >= 20)`

</details>

<br />

## frontend: 90th_percentile_search_codeintel_request_duration

<p class="subtitle">90th percentile code-intel successful search request duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 15s+ 90th percentile code-intel successful search request duration over 5m

**Next steps**

- **Get details on the exact queries that are slow** by configuring `"observability.logSlowSearches": 15,` in the site configuration and looking for `frontend` warning logs prefixed with `slow search request` for additional details.
- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the `indexed-search.Deployment.yaml` if regularly hitting max CPU utilization.
- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml` if regularly hitting max CPU utilization.
- This alert may indicate that your instance is struggling to process symbols queries on a monorepo, [learn more here](../how-to/monorepo-issues.md).
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-90th-percentile-search-codeintel-request-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_90th_percentile_search_codeintel_request_duration"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.9, sum by (le) (rate(src_graphql_field_seconds_bucket{error="false",field="results",request_name="CodeIntelSearch",source="browser",type="Search"}[5m])))) >= 15)`

</details>

<br />

## frontend: hard_timeout_search_codeintel_responses

<p class="subtitle">hard timeout search code-intel responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2%+ hard timeout search code-intel responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-hard-timeout-search-codeintel-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_hard_timeout_search_codeintel_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max(((sum(increase(src_graphql_search_response{request_name="CodeIntelSearch",source="browser",status="timeout"}[5m])) + sum(increase(src_graphql_search_response{alert_type="timed_out",request_name="CodeIntelSearch",source="browser",status="alert"}[5m]))) / sum(increase(src_graphql_search_response{request_name="CodeIntelSearch",source="browser"}[5m])) * 100) >= 2)`

</details>

<br />

## frontend: hard_error_search_codeintel_responses

<p class="subtitle">hard error search code-intel responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2%+ hard error search code-intel responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-hard-error-search-codeintel-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_hard_error_search_codeintel_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (status) (increase(src_graphql_search_response{request_name="CodeIntelSearch",source="browser",status=~"error"}[5m])) / ignoring (status) group_left () sum(increase(src_graphql_search_response{request_name="CodeIntelSearch",source="browser"}[5m])) * 100) >= 2)`

</details>

<br />

## frontend: partial_timeout_search_codeintel_responses

<p class="subtitle">partial timeout search code-intel responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ partial timeout search code-intel responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-partial-timeout-search-codeintel-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_partial_timeout_search_codeintel_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (status) (increase(src_graphql_search_response{request_name="CodeIntelSearch",source="browser",status="partial_timeout"}[5m])) / ignoring (status) group_left () sum(increase(src_graphql_search_response{request_name="CodeIntelSearch",source="browser",status="partial_timeout"}[5m])) * 100) >= 5)`

</details>

<br />

## frontend: search_codeintel_alert_user_suggestions

<p class="subtitle">search code-intel alert user suggestions shown every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ search code-intel alert user suggestions shown every 5m for 15m0s

**Next steps**

- This indicates a bug in Sourcegraph, please [open an issue](https://github.com/sourcegraph/sourcegraph/issues/new/choose).
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-search-codeintel-alert-user-suggestions).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_search_codeintel_alert_user_suggestions"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (alert_type) (increase(src_graphql_search_response{alert_type!~"timed_out",request_name="CodeIntelSearch",source="browser",status="alert"}[5m])) / ignoring (alert_type) group_left () sum(increase(src_graphql_search_response{request_name="CodeIntelSearch",source="browser"}[5m])) * 100) >= 5)`

</details>

<br />

## frontend: 99th_percentile_search_api_request_duration

<p class="subtitle">99th percentile successful search API request duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 50s+ 99th percentile successful search API request duration over 5m

**Next steps**

- **Get details on the exact queries that are slow** by configuring `"observability.logSlowSearches": 20,` in the site configuration and looking for `frontend` warning logs prefixed with `slow search request` for additional details.
- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the `indexed-search.Deployment.yaml` if regularly hitting max CPU utilization.
- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml` if regularly hitting max CPU utilization.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-99th-percentile-search-api-request-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_99th_percentile_search_api_request_duration"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.99, sum by (le) (rate(src_graphql_field_seconds_bucket{error="false",field="results",source="other",type="Search"}[5m])))) >= 50)`

</details>

<br />

## frontend: 90th_percentile_search_api_request_duration

<p class="subtitle">90th percentile successful search API request duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 40s+ 90th percentile successful search API request duration over 5m

**Next steps**

- **Get details on the exact queries that are slow** by configuring `"observability.logSlowSearches": 15,` in the site configuration and looking for `frontend` warning logs prefixed with `slow search request` for additional details.
- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the `indexed-search.Deployment.yaml` if regularly hitting max CPU utilization.
- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml` if regularly hitting max CPU utilization.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-90th-percentile-search-api-request-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_90th_percentile_search_api_request_duration"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.9, sum by (le) (rate(src_graphql_field_seconds_bucket{error="false",field="results",source="other",type="Search"}[5m])))) >= 40)`

</details>

<br />

## frontend: hard_error_search_api_responses

<p class="subtitle">hard error search API responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2%+ hard error search API responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-hard-error-search-api-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_hard_error_search_api_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (status) (increase(src_graphql_search_response{source="other",status=~"error"}[5m])) / ignoring (status) group_left () sum(increase(src_graphql_search_response{source="other"}[5m]))) >= 2)`

</details>

<br />

## frontend: partial_timeout_search_api_responses

<p class="subtitle">partial timeout search API responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ partial timeout search API responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-partial-timeout-search-api-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_partial_timeout_search_api_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(increase(src_graphql_search_response{source="other",status="partial_timeout"}[5m])) / sum(increase(src_graphql_search_response{source="other"}[5m]))) >= 5)`

</details>

<br />

## frontend: search_api_alert_user_suggestions

<p class="subtitle">search API alert user suggestions shown every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ search API alert user suggestions shown every 5m

**Next steps**

- This indicates your user`s search API requests have syntax errors or a similar user error. Check the responses the API sends back for an explanation.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-search-api-alert-user-suggestions).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_search_api_alert_user_suggestions"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (alert_type) (increase(src_graphql_search_response{alert_type!~"timed_out|no_results__suggest_quotes",source="other",status="alert"}[5m])) / ignoring (alert_type) group_left () sum(increase(src_graphql_search_response{source="other",status="alert"}[5m]))) >= 5)`

</details>

<br />

## frontend: frontend_site_configuration_duration_since_last_successful_update_by_instance

<p class="subtitle">maximum duration since last successful site configuration update (all "frontend" instances)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> frontend: 300s+ maximum duration since last successful site configuration update (all "frontend" instances)

**Next steps**

- This indicates that one or more "frontend" instances have not successfully updated the site configuration in over 5 minutes. This could be due to networking issues between services or problems with the site configuration service itself.
- Check for relevant errors in the "frontend" logs, as well as frontend`s logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-frontend-site-configuration-duration-since-last-successful-update-by-instance).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_frontend_frontend_site_configuration_duration_since_last_successful_update_by_instance"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(max_over_time(src_conf_client_time_since_last_successful_update_seconds{job=~"(sourcegraph-)?frontend"}[1m]))) >= 300)`

</details>

<br />

## frontend: internal_indexed_search_error_responses

<p class="subtitle">internal indexed search error responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ internal indexed search error responses every 5m for 15m0s

**Next steps**

- Check the Zoekt Web Server dashboard for indications it might be unhealthy.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-internal-indexed-search-error-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_internal_indexed_search_error_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (code) (increase(src_zoekt_request_duration_seconds_count{code!~"2.."}[5m])) / ignoring (code) group_left () sum(increase(src_zoekt_request_duration_seconds_count[5m])) * 100) >= 5)`

</details>

<br />

## frontend: internal_unindexed_search_error_responses

<p class="subtitle">internal unindexed search error responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ internal unindexed search error responses every 5m for 15m0s

**Next steps**

- Check the Searcher dashboard for indications it might be unhealthy.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-internal-unindexed-search-error-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_internal_unindexed_search_error_responses"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (code) (increase(searcher_service_request_total{code!~"2.."}[5m])) / ignoring (code) group_left () sum(increase(searcher_service_request_total[5m])) * 100) >= 5)`

</details>

<br />

## frontend: 99th_percentile_gitserver_duration

<p class="subtitle">99th percentile successful gitserver query duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 20s+ 99th percentile successful gitserver query duration over 5m

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-99th-percentile-gitserver-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_99th_percentile_gitserver_duration"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.99, sum by (le, category) (rate(src_gitserver_request_duration_seconds_bucket{job=~"(sourcegraph-)?frontend"}[5m])))) >= 20)`

</details>

<br />

## frontend: gitserver_error_responses

<p class="subtitle">gitserver error responses every 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5%+ gitserver error responses every 5m for 15m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-gitserver-error-responses).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_gitserver_error_responses"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (category) (increase(src_gitserver_request_duration_seconds_count{code!~"2..",job=~"(sourcegraph-)?frontend"}[5m])) / ignoring (code) group_left () sum by (category) (increase(src_gitserver_request_duration_seconds_count{job=~"(sourcegraph-)?frontend"}[5m])) * 100) >= 5)`

</details>

<br />

## frontend: observability_test_alert_warning

<p class="subtitle">warning test alert metric</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 1+ warning test alert metric

**Next steps**

- This alert is triggered via the `triggerObservabilityTestAlert` GraphQL endpoint, and will automatically resolve itself.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-observability-test-alert-warning).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_observability_test_alert_warning"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (owner) (observability_test_metric_warning)) >= 1)`

</details>

<br />

## frontend: observability_test_alert_critical

<p class="subtitle">critical test alert metric</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> frontend: 1+ critical test alert metric

**Next steps**

- This alert is triggered via the `triggerObservabilityTestAlert` GraphQL endpoint, and will automatically resolve itself.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-observability-test-alert-critical).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_frontend_observability_test_alert_critical"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max by (owner) (observability_test_metric_critical)) >= 1)`

</details>

<br />

## frontend: cloudkms_cryptographic_requests

<p class="subtitle">cryptographic requests to Cloud KMS every 1m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 15000+ cryptographic requests to Cloud KMS every 1m for 5m0s
- <span class="badge badge-critical">critical</span> frontend: 30000+ cryptographic requests to Cloud KMS every 1m for 5m0s

**Next steps**

- Revert recent commits that cause extensive listing from "external_services" and/or "user_external_accounts" tables.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-cloudkms-cryptographic-requests).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_cloudkms_cryptographic_requests",
  "critical_frontend_cloudkms_cryptographic_requests"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(increase(src_cloudkms_cryptographic_total[1m]))) >= 15000)`

Generated query for critical alert: `max((sum(increase(src_cloudkms_cryptographic_total[1m]))) >= 30000)`

</details>

<br />

## frontend: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> frontend: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_mean_blocked_seconds_per_conn_request",
  "critical_frontend_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="frontend"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="frontend"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="frontend"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="frontend"}[5m]))) >= 0.5)`

</details>

<br />

## frontend: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the (frontend|sourcegraph-frontend) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^(frontend|sourcegraph-frontend).*"}) >= 99)`

</details>

<br />

## frontend: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of (frontend|sourcegraph-frontend) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^(frontend|sourcegraph-frontend).*"}) >= 99)`

</details>

<br />

## frontend: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the (frontend|sourcegraph-frontend) service.
- **Docker Compose:** Consider increasing `cpus:` of the (frontend|sourcegraph-frontend) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^(frontend|sourcegraph-frontend).*"}[1d])) >= 80)`

</details>

<br />

## frontend: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the (frontend|sourcegraph-frontend) service.
- **Docker Compose:** Consider increasing `memory:` of the (frontend|sourcegraph-frontend) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^(frontend|sourcegraph-frontend).*"}[1d])) >= 80)`

</details>

<br />

## frontend: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the (frontend|sourcegraph-frontend) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^(frontend|sourcegraph-frontend).*"}[5m])) >= 90)`

</details>

<br />

## frontend: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of (frontend|sourcegraph-frontend) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^(frontend|sourcegraph-frontend).*"}[5m])) >= 90)`

</details>

<br />

## frontend: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of (frontend|sourcegraph-frontend) container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#frontend-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^(frontend|sourcegraph-frontend).*"})) >= 1)`

</details>

<br />

## frontend: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#frontend-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*(frontend|sourcegraph-frontend)"})) >= 10000)`

</details>

<br />

## frontend: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*(frontend|sourcegraph-frontend)"})) >= 2)`

</details>

<br />

## frontend: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> frontend: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod (frontend|sourcegraph-frontend)` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p (frontend|sourcegraph-frontend)`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_frontend_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*(frontend|sourcegraph-frontend)"}) / count by (app) (up{app=~".*(frontend|sourcegraph-frontend)"}) * 100) <= 90)`

</details>

<br />

## frontend: email_delivery_failures

<p class="subtitle">email delivery failure rate over 30 minutes</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 0%+ email delivery failure rate over 30 minutes
- <span class="badge badge-critical">critical</span> frontend: 10%+ email delivery failure rate over 30 minutes

**Next steps**

- Check your SMTP configuration in site configuration.
- Check `sourcegraph-frontend` logs for more detailed error messages.
- Check your SMTP provider for more detailed error messages.
- Use `sum(increase(src_email_send{success="false"}[30m]))` to check the raw count of delivery failures.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#frontend-email-delivery-failures).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_email_delivery_failures",
  "critical_frontend_email_delivery_failures"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(increase(src_email_send{success="false"}[30m])) / sum(increase(src_email_send[30m])) * 100) > 0)`

Generated query for critical alert: `max((sum(increase(src_email_send{success="false"}[30m])) / sum(increase(src_email_send[30m])) * 100) >= 10)`

</details>

<br />

## frontend: mean_successful_sentinel_duration_over_2h

<p class="subtitle">mean successful sentinel search duration over 2h</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5s+ mean successful sentinel search duration over 2h for 15m0s
- <span class="badge badge-critical">critical</span> frontend: 8s+ mean successful sentinel search duration over 2h for 30m0s

**Next steps**

- Look at the breakdown by query to determine if a specific query type is being affected
- Check for high CPU usage on zoekt-webserver
- Check Honeycomb for unusual activity
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#frontend-mean-successful-sentinel-duration-over-2h).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_mean_successful_sentinel_duration_over_2h",
  "critical_frontend_mean_successful_sentinel_duration_over_2h"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(rate(src_search_response_latency_seconds_sum{source=~"searchblitz.*",status="success"}[2h])) / sum(rate(src_search_response_latency_seconds_count{source=~"searchblitz.*",status="success"}[2h]))) >= 5)`

Generated query for critical alert: `max((sum(rate(src_search_response_latency_seconds_sum{source=~"searchblitz.*",status="success"}[2h])) / sum(rate(src_search_response_latency_seconds_count{source=~"searchblitz.*",status="success"}[2h]))) >= 8)`

</details>

<br />

## frontend: mean_sentinel_stream_latency_over_2h

<p class="subtitle">mean successful sentinel stream latency over 2h</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 2s+ mean successful sentinel stream latency over 2h for 15m0s
- <span class="badge badge-critical">critical</span> frontend: 3s+ mean successful sentinel stream latency over 2h for 30m0s

**Next steps**

- Look at the breakdown by query to determine if a specific query type is being affected
- Check for high CPU usage on zoekt-webserver
- Check Honeycomb for unusual activity
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#frontend-mean-sentinel-stream-latency-over-2h).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_mean_sentinel_stream_latency_over_2h",
  "critical_frontend_mean_sentinel_stream_latency_over_2h"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(rate(src_search_streaming_latency_seconds_sum{source=~"searchblitz.*"}[2h])) / sum(rate(src_search_streaming_latency_seconds_count{source=~"searchblitz.*"}[2h]))) >= 2)`

Generated query for critical alert: `max((sum(rate(src_search_streaming_latency_seconds_sum{source=~"searchblitz.*"}[2h])) / sum(rate(src_search_streaming_latency_seconds_count{source=~"searchblitz.*"}[2h]))) >= 3)`

</details>

<br />

## frontend: 90th_percentile_successful_sentinel_duration_over_2h

<p class="subtitle">90th percentile successful sentinel search duration over 2h</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 5s+ 90th percentile successful sentinel search duration over 2h for 15m0s
- <span class="badge badge-critical">critical</span> frontend: 10s+ 90th percentile successful sentinel search duration over 2h for 3h30m0s

**Next steps**

- Look at the breakdown by query to determine if a specific query type is being affected
- Check for high CPU usage on zoekt-webserver
- Check Honeycomb for unusual activity
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#frontend-90th-percentile-successful-sentinel-duration-over-2h).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_90th_percentile_successful_sentinel_duration_over_2h",
  "critical_frontend_90th_percentile_successful_sentinel_duration_over_2h"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.9, sum by (le) (label_replace(rate(src_search_response_latency_seconds_bucket{source=~"searchblitz.*",status="success"}[2h]), "source", "$1", "source", "searchblitz_(.*)")))) >= 5)`

Generated query for critical alert: `max((histogram_quantile(0.9, sum by (le) (label_replace(rate(src_search_response_latency_seconds_bucket{source=~"searchblitz.*",status="success"}[2h]), "source", "$1", "source", "searchblitz_(.*)")))) >= 10)`

</details>

<br />

## frontend: 90th_percentile_sentinel_stream_latency_over_2h

<p class="subtitle">90th percentile successful sentinel stream latency over 2h</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> frontend: 4s+ 90th percentile successful sentinel stream latency over 2h for 15m0s
- <span class="badge badge-critical">critical</span> frontend: 6s+ 90th percentile successful sentinel stream latency over 2h for 3h30m0s

**Next steps**

- Look at the breakdown by query to determine if a specific query type is being affected
- Check for high CPU usage on zoekt-webserver
- Check Honeycomb for unusual activity
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#frontend-90th-percentile-sentinel-stream-latency-over-2h).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_frontend_90th_percentile_sentinel_stream_latency_over_2h",
  "critical_frontend_90th_percentile_sentinel_stream_latency_over_2h"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.9, sum by (le) (label_replace(rate(src_search_streaming_latency_seconds_bucket{source=~"searchblitz.*"}[2h]), "source", "$1", "source", "searchblitz_(.*)")))) >= 4)`

Generated query for critical alert: `max((histogram_quantile(0.9, sum by (le) (label_replace(rate(src_search_streaming_latency_seconds_bucket{source=~"searchblitz.*"}[2h]), "source", "$1", "source", "searchblitz_(.*)")))) >= 6)`

</details>

<br />

## gitserver: cpu_throttling_time

<p class="subtitle">container CPU throttling time %</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 75%+ container CPU throttling time % for 2m0s
- <span class="badge badge-critical">critical</span> gitserver: 90%+ container CPU throttling time % for 5m0s

**Next steps**

- 	- Consider increasing the CPU limit for the container.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#gitserver-cpu-throttling-time).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_cpu_throttling_time",
  "critical_gitserver_cpu_throttling_time"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (container_label_io_kubernetes_pod_name) ((rate(container_cpu_cfs_throttled_periods_total{container_label_io_kubernetes_container_name="gitserver"}[5m]) / rate(container_cpu_cfs_periods_total{container_label_io_kubernetes_container_name="gitserver"}[5m])) * 100)) >= 75)`

Generated query for critical alert: `max((sum by (container_label_io_kubernetes_pod_name) ((rate(container_cpu_cfs_throttled_periods_total{container_label_io_kubernetes_container_name="gitserver"}[5m]) / rate(container_cpu_cfs_periods_total{container_label_io_kubernetes_container_name="gitserver"}[5m])) * 100)) >= 90)`

</details>

<br />

## gitserver: disk_space_remaining

<p class="subtitle">disk space remaining</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: less than 15% disk space remaining
- <span class="badge badge-critical">critical</span> gitserver: less than 10% disk space remaining for 10m0s

**Next steps**

- On a warning alert, you may want to provision more disk space: Disk pressure may result in decreased performance, users having to wait for repositories to clone, etc.
- On a critical alert, you need to provision more disk space. Running out of disk space will result in decreased performance, or complete service outage.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#gitserver-disk-space-remaining).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_disk_space_remaining",
  "critical_gitserver_disk_space_remaining"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min(((src_gitserver_disk_space_available / src_gitserver_disk_space_total) * 100) < 15)`

Generated query for critical alert: `min(((src_gitserver_disk_space_available / src_gitserver_disk_space_total) * 100) < 10)`

</details>

<br />

## gitserver: running_git_commands

<p class="subtitle">git commands running on each gitserver instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 50+ git commands running on each gitserver instance for 2m0s
- <span class="badge badge-critical">critical</span> gitserver: 100+ git commands running on each gitserver instance for 5m0s

**Next steps**

- **Check if the problem may be an intermittent and temporary peak** using the "Container monitoring" section at the bottom of the Git Server dashboard.
- **Single container deployments:** Consider upgrading to a [Docker Compose deployment](../deploy/docker-compose/migrate.md) which offers better scalability and resource isolation.
- **Kubernetes and Docker Compose:** Check that you are running a similar number of git server replicas and that their CPU/memory limits are allocated according to what is shown in the [Sourcegraph resource estimator](../deploy/resource_estimator.md).
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#gitserver-running-git-commands).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_running_git_commands",
  "critical_gitserver_running_git_commands"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (instance, cmd) (src_gitserver_exec_running)) >= 50)`

Generated query for critical alert: `max((sum by (instance, cmd) (src_gitserver_exec_running)) >= 100)`

</details>

<br />

## gitserver: echo_command_duration_test

<p class="subtitle">echo test command duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 0.02s+ echo test command duration for 30s
- <span class="badge badge-critical">critical</span> gitserver: 1s+ echo test command duration

**Next steps**

- **Single container deployments:** Upgrade to a [Docker Compose deployment](../deploy/docker-compose/migrate.md) which offers better scalability and resource isolation.
- **Kubernetes and Docker Compose:** Check that you are running a similar number of git server replicas and that their CPU/memory limits are allocated according to what is shown in the [Sourcegraph resource estimator](../deploy/resource_estimator.md).
- If your persistent volume is slow, you may want to provision more IOPS, usually by increasing the volume size.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#gitserver-echo-command-duration-test).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_echo_command_duration_test",
  "critical_gitserver_echo_command_duration_test"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_gitserver_echo_duration_seconds)) >= 0.02)`

Generated query for critical alert: `max((max(src_gitserver_echo_duration_seconds)) >= 1)`

</details>

<br />

## gitserver: repo_corrupted

<p class="subtitle">number of times a repo corruption has been identified</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> gitserver: 0+ number of times a repo corruption has been identified

**Next steps**

- Check the corruption logs for details. gitserver_repos.corruption_logs contains more information.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#gitserver-repo-corrupted).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_gitserver_repo_corrupted"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((sum(rate(src_gitserver_repo_corrupted[5m]))) > 0)`

</details>

<br />

## gitserver: repository_clone_queue_size

<p class="subtitle">repository clone queue size</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 25+ repository clone queue size

**Next steps**

- **If you just added several repositories**, the warning may be expected.
- **Check which repositories need cloning**, by visiting e.g. https://sourcegraph.example.com/site-admin/repositories?filter=not-cloned
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-repository-clone-queue-size).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_repository_clone_queue_size"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(src_gitserver_clone_queue)) >= 25)`

</details>

<br />

## gitserver: gitserver_site_configuration_duration_since_last_successful_update_by_instance

<p class="subtitle">maximum duration since last successful site configuration update (all "gitserver" instances)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> gitserver: 300s+ maximum duration since last successful site configuration update (all "gitserver" instances)

**Next steps**

- This indicates that one or more "gitserver" instances have not successfully updated the site configuration in over 5 minutes. This could be due to networking issues between services or problems with the site configuration service itself.
- Check for relevant errors in the "gitserver" logs, as well as frontend`s logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-gitserver-site-configuration-duration-since-last-successful-update-by-instance).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_gitserver_gitserver_site_configuration_duration_since_last_successful_update_by_instance"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(max_over_time(src_conf_client_time_since_last_successful_update_seconds{job=~".*gitserver"}[1m]))) >= 300)`

</details>

<br />

## gitserver: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> gitserver: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_mean_blocked_seconds_per_conn_request",
  "critical_gitserver_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="gitserver"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="gitserver"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="gitserver"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="gitserver"}[5m]))) >= 0.5)`

</details>

<br />

## gitserver: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the gitserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^gitserver.*"}) >= 99)`

</details>

<br />

## gitserver: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of gitserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^gitserver.*"}) >= 99)`

</details>

<br />

## gitserver: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the gitserver service.
- **Docker Compose:** Consider increasing `cpus:` of the gitserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^gitserver.*"}[1d])) >= 80)`

</details>

<br />

## gitserver: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the gitserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^gitserver.*"}[5m])) >= 90)`

</details>

<br />

## gitserver: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of gitserver container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#gitserver-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^gitserver.*"})) >= 1)`

</details>

<br />

## gitserver: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#gitserver-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*gitserver"})) >= 10000)`

</details>

<br />

## gitserver: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> gitserver: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_gitserver_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*gitserver"})) >= 2)`

</details>

<br />

## gitserver: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> gitserver: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod gitserver` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p gitserver`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#gitserver-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_gitserver_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*gitserver"}) / count by (app) (up{app=~".*gitserver"}) * 100) <= 90)`

</details>

<br />

## postgres: connections

<p class="subtitle">active connections</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: less than 5 active connections for 5m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-connections).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_connections"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((sum by (job) (pg_stat_activity_count{datname!~"template.*|postgres|cloudsqladmin"}) or sum by (job) (pg_stat_activity_count{datname!~"template.*|cloudsqladmin",job="codeinsights-db"})) <= 5)`

</details>

<br />

## postgres: usage_connections_percentage

<p class="subtitle">connection in use</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 80%+ connection in use for 5m0s
- <span class="badge badge-critical">critical</span> postgres: 100%+ connection in use for 5m0s

**Next steps**

- Consider increasing [max_connections](https://www.postgresql.org/docs/current/runtime-config-connection.html#GUC-MAX-CONNECTIONS) of the database instance, [learn more](https://sourcegraph.com/docs/admin/config/postgres-conf)
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-usage-connections-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_usage_connections_percentage",
  "critical_postgres_usage_connections_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (job) (pg_stat_activity_count) / (sum by (job) (pg_settings_max_connections) - sum by (job) (pg_settings_superuser_reserved_connections)) * 100) >= 80)`

Generated query for critical alert: `max((sum by (job) (pg_stat_activity_count) / (sum by (job) (pg_settings_max_connections) - sum by (job) (pg_settings_superuser_reserved_connections)) * 100) >= 100)`

</details>

<br />

## postgres: transaction_durations

<p class="subtitle">maximum transaction durations</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 0.3s+ maximum transaction durations for 5m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-transaction-durations).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_transaction_durations"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (job) (pg_stat_activity_max_tx_duration{datname!~"template.*|postgres|cloudsqladmin",job!="codeintel-db"}) or sum by (job) (pg_stat_activity_max_tx_duration{datname!~"template.*|cloudsqladmin",job="codeinsights-db"})) >= 0.3)`

</details>

<br />

## postgres: postgres_up

<p class="subtitle">database availability</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> postgres: less than 0 database availability for 5m0s

**Next steps**

- **Kubernetes:**
	- Determine if the pod was OOM killed using `kubectl describe pod (pgsql|codeintel-db|codeinsights)` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
	- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p (pgsql|codeintel-db|codeinsights)`.
	- Check if there is any OOMKILL event using the provisioning panels
	- Check kernel logs using `dmesg` for OOMKILL events on worker nodes
- **Docker Compose:**
	- Determine if the pod was OOM killed using `docker inspect -f '{{json .State}}' (pgsql|codeintel-db|codeinsights)` (look for `"OOMKilled":true`) and, if so, consider increasing the memory limit of the (pgsql|codeintel-db|codeinsights) container in `docker-compose.yml`.
	- Check the logs before the container restarted to see if there are `panic:` messages or similar using `docker logs (pgsql|codeintel-db|codeinsights)` (note this will include logs from the previous and currently running container).
	- Check if there is any OOMKILL event using the provisioning panels
	- Check kernel logs using `dmesg` for OOMKILL events
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#postgres-postgres-up).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_postgres_postgres_up"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((pg_up) <= 0)`

</details>

<br />

## postgres: invalid_indexes

<p class="subtitle">invalid indexes (unusable by the query planner)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> postgres: 1+ invalid indexes (unusable by the query planner)

**Next steps**

- Drop and re-create the invalid trigger - please contact Sourcegraph to supply the trigger definition.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#postgres-invalid-indexes).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_postgres_invalid_indexes"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `sum((max by (relname) (pg_invalid_index_count)) >= 1)`

</details>

<br />

## postgres: pg_exporter_err

<p class="subtitle">errors scraping postgres exporter</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 1+ errors scraping postgres exporter for 5m0s

**Next steps**

- Ensure the Postgres exporter can access the Postgres database. Also, check the Postgres exporter logs for errors.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#postgres-pg-exporter-err).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_pg_exporter_err"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((pg_exporter_last_scrape_error) >= 1)`

</details>

<br />

## postgres: migration_in_progress

<p class="subtitle">active schema migration</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> postgres: 1+ active schema migration for 5m0s

**Next steps**

- The database migration has been in progress for 5 or more minutes - please contact Sourcegraph if this persists.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#postgres-migration-in-progress).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_postgres_migration_in_progress"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((pg_sg_migration_status) >= 1)`

</details>

<br />

## postgres: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the (pgsql|codeintel-db|codeinsights) service.
- **Docker Compose:** Consider increasing `cpus:` of the (pgsql|codeintel-db|codeinsights) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^(pgsql|codeintel-db|codeinsights).*"}[1d])) >= 80)`

</details>

<br />

## postgres: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the (pgsql|codeintel-db|codeinsights) service.
- **Docker Compose:** Consider increasing `memory:` of the (pgsql|codeintel-db|codeinsights) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^(pgsql|codeintel-db|codeinsights).*"}[1d])) >= 80)`

</details>

<br />

## postgres: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the (pgsql|codeintel-db|codeinsights) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^(pgsql|codeintel-db|codeinsights).*"}[5m])) >= 90)`

</details>

<br />

## postgres: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of (pgsql|codeintel-db|codeinsights) container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^(pgsql|codeintel-db|codeinsights).*"}[5m])) >= 90)`

</details>

<br />

## postgres: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> postgres: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of (pgsql|codeintel-db|codeinsights) container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#postgres-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_postgres_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^(pgsql|codeintel-db|codeinsights).*"})) >= 1)`

</details>

<br />

## postgres: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> postgres: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod (pgsql|codeintel-db|codeinsights)` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p (pgsql|codeintel-db|codeinsights)`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#postgres-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_postgres_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*(pgsql|codeintel-db|codeinsights)"}) / count by (app) (up{app=~".*(pgsql|codeintel-db|codeinsights)"}) * 100) <= 90)`

</details>

<br />

## precise-code-intel-worker: codeintel_upload_queued_max_age

<p class="subtitle">unprocessed upload record queue longest time in queue</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 18000s+ unprocessed upload record queue longest time in queue

**Next steps**

- An alert here could be indicative of a few things: an upload surfacing a pathological performance characteristic,
precise-code-intel-worker being underprovisioned for the required upload processing throughput, or a higher replica
count being required for the volume of uploads.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-codeintel-upload-queued-max-age).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_codeintel_upload_queued_max_age"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_codeintel_upload_queued_duration_seconds_total{job=~"^precise-code-intel-worker.*"})) >= 18000)`

</details>

<br />

## precise-code-intel-worker: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> precise-code-intel-worker: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_mean_blocked_seconds_per_conn_request",
  "critical_precise-code-intel-worker_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="precise-code-intel-worker"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="precise-code-intel-worker"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="precise-code-intel-worker"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="precise-code-intel-worker"}[5m]))) >= 0.5)`

</details>

<br />

## precise-code-intel-worker: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the precise-code-intel-worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^precise-code-intel-worker.*"}) >= 99)`

</details>

<br />

## precise-code-intel-worker: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of precise-code-intel-worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^precise-code-intel-worker.*"}) >= 99)`

</details>

<br />

## precise-code-intel-worker: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the precise-code-intel-worker service.
- **Docker Compose:** Consider increasing `cpus:` of the precise-code-intel-worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^precise-code-intel-worker.*"}[1d])) >= 80)`

</details>

<br />

## precise-code-intel-worker: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the precise-code-intel-worker service.
- **Docker Compose:** Consider increasing `memory:` of the precise-code-intel-worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^precise-code-intel-worker.*"}[1d])) >= 80)`

</details>

<br />

## precise-code-intel-worker: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the precise-code-intel-worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^precise-code-intel-worker.*"}[5m])) >= 90)`

</details>

<br />

## precise-code-intel-worker: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of precise-code-intel-worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^precise-code-intel-worker.*"}[5m])) >= 90)`

</details>

<br />

## precise-code-intel-worker: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of precise-code-intel-worker container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#precise-code-intel-worker-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^precise-code-intel-worker.*"})) >= 1)`

</details>

<br />

## precise-code-intel-worker: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#precise-code-intel-worker-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*precise-code-intel-worker"})) >= 10000)`

</details>

<br />

## precise-code-intel-worker: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> precise-code-intel-worker: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_precise-code-intel-worker_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*precise-code-intel-worker"})) >= 2)`

</details>

<br />

## precise-code-intel-worker: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> precise-code-intel-worker: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod precise-code-intel-worker` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p precise-code-intel-worker`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#precise-code-intel-worker-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_precise-code-intel-worker_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*precise-code-intel-worker"}) / count by (app) (up{app=~".*precise-code-intel-worker"}) * 100) <= 90)`

</details>

<br />

## redis: redis-store_up

<p class="subtitle">redis-store availability</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> redis: less than 1 redis-store availability for 10s

**Next steps**

- Ensure redis-store is running
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#redis-redis-store-up).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_redis_redis-store_up"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((redis_up{app="redis-store"}) < 1)`

</details>

<br />

## redis: redis-cache_up

<p class="subtitle">redis-cache availability</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> redis: less than 1 redis-cache availability for 10s

**Next steps**

- Ensure redis-cache is running
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#redis-redis-cache-up).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_redis_redis-cache_up"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((redis_up{app="redis-cache"}) < 1)`

</details>

<br />

## redis: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the redis-cache service.
- **Docker Compose:** Consider increasing `cpus:` of the redis-cache container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^redis-cache.*"}[1d])) >= 80)`

</details>

<br />

## redis: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the redis-cache service.
- **Docker Compose:** Consider increasing `memory:` of the redis-cache container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^redis-cache.*"}[1d])) >= 80)`

</details>

<br />

## redis: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the redis-cache container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^redis-cache.*"}[5m])) >= 90)`

</details>

<br />

## redis: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of redis-cache container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^redis-cache.*"}[5m])) >= 90)`

</details>

<br />

## redis: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of redis-cache container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#redis-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^redis-cache.*"})) >= 1)`

</details>

<br />

## redis: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the redis-store service.
- **Docker Compose:** Consider increasing `cpus:` of the redis-store container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^redis-store.*"}[1d])) >= 80)`

</details>

<br />

## redis: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the redis-store service.
- **Docker Compose:** Consider increasing `memory:` of the redis-store container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^redis-store.*"}[1d])) >= 80)`

</details>

<br />

## redis: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the redis-store container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^redis-store.*"}[5m])) >= 90)`

</details>

<br />

## redis: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of redis-store container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^redis-store.*"}[5m])) >= 90)`

</details>

<br />

## redis: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> redis: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of redis-store container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#redis-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_redis_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^redis-store.*"})) >= 1)`

</details>

<br />

## redis: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> redis: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod redis-cache` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p redis-cache`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_redis_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*redis-cache"}) / count by (app) (up{app=~".*redis-cache"}) * 100) <= 90)`

</details>

<br />

## redis: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> redis: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod redis-store` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p redis-store`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#redis-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_redis_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*redis-store"}) / count by (app) (up{app=~".*redis-store"}) * 100) <= 90)`

</details>

<br />

## worker: worker_job_codeintel-upload-janitor_count

<p class="subtitle">number of worker instances running the codeintel-upload-janitor job</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: less than 1 number of worker instances running the codeintel-upload-janitor job for 1m0s
- <span class="badge badge-critical">critical</span> worker: less than 1 number of worker instances running the codeintel-upload-janitor job for 5m0s

**Next steps**

- Ensure your instance defines a worker container such that:
	- `WORKER_JOB_ALLOWLIST` contains "codeintel-upload-janitor" (or "all"), and
	- `WORKER_JOB_BLOCKLIST` does not contain "codeintel-upload-janitor"
- Ensure that such a container is not failing to start or stay active
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-worker-job-codeintel-upload-janitor-count).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_worker_job_codeintel-upload-janitor_count",
  "critical_worker_worker_job_codeintel-upload-janitor_count"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `(min((sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-upload-janitor"})) < 1)) or (absent(sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-upload-janitor"})) == 1)`

Generated query for critical alert: `(min((sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-upload-janitor"})) < 1)) or (absent(sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-upload-janitor"})) == 1)`

</details>

<br />

## worker: worker_job_codeintel-commitgraph-updater_count

<p class="subtitle">number of worker instances running the codeintel-commitgraph-updater job</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: less than 1 number of worker instances running the codeintel-commitgraph-updater job for 1m0s
- <span class="badge badge-critical">critical</span> worker: less than 1 number of worker instances running the codeintel-commitgraph-updater job for 5m0s

**Next steps**

- Ensure your instance defines a worker container such that:
	- `WORKER_JOB_ALLOWLIST` contains "codeintel-commitgraph-updater" (or "all"), and
	- `WORKER_JOB_BLOCKLIST` does not contain "codeintel-commitgraph-updater"
- Ensure that such a container is not failing to start or stay active
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-worker-job-codeintel-commitgraph-updater-count).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_worker_job_codeintel-commitgraph-updater_count",
  "critical_worker_worker_job_codeintel-commitgraph-updater_count"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `(min((sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-commitgraph-updater"})) < 1)) or (absent(sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-commitgraph-updater"})) == 1)`

Generated query for critical alert: `(min((sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-commitgraph-updater"})) < 1)) or (absent(sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-commitgraph-updater"})) == 1)`

</details>

<br />

## worker: worker_job_codeintel-autoindexing-scheduler_count

<p class="subtitle">number of worker instances running the codeintel-autoindexing-scheduler job</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: less than 1 number of worker instances running the codeintel-autoindexing-scheduler job for 1m0s
- <span class="badge badge-critical">critical</span> worker: less than 1 number of worker instances running the codeintel-autoindexing-scheduler job for 5m0s

**Next steps**

- Ensure your instance defines a worker container such that:
	- `WORKER_JOB_ALLOWLIST` contains "codeintel-autoindexing-scheduler" (or "all"), and
	- `WORKER_JOB_BLOCKLIST` does not contain "codeintel-autoindexing-scheduler"
- Ensure that such a container is not failing to start or stay active
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-worker-job-codeintel-autoindexing-scheduler-count).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_worker_job_codeintel-autoindexing-scheduler_count",
  "critical_worker_worker_job_codeintel-autoindexing-scheduler_count"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `(min((sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-autoindexing-scheduler"})) < 1)) or (absent(sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-autoindexing-scheduler"})) == 1)`

Generated query for critical alert: `(min((sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-autoindexing-scheduler"})) < 1)) or (absent(sum(src_worker_jobs{job=~"^worker.*",job_name="codeintel-autoindexing-scheduler"})) == 1)`

</details>

<br />

## worker: codeintel_commit_graph_queued_max_age

<p class="subtitle">repository queue longest time in queue</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 3600s+ repository queue longest time in queue

**Next steps**

- An alert here is generally indicative of either underprovisioned worker instance(s) and/or
an underprovisioned main postgres instance.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-codeintel-commit-graph-queued-max-age).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_codeintel_commit_graph_queued_max_age"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_codeintel_commit_graph_queued_duration_seconds_total{job=~"^worker.*"})) >= 3600)`

</details>

<br />

## worker: perms_syncer_outdated_perms

<p class="subtitle">number of entities with outdated permissions</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 100+ number of entities with outdated permissions for 5m0s

**Next steps**

- **Enabled permissions for the first time:** Wait for few minutes and see if the number goes down.
- **Otherwise:** Increase the API rate limit to [GitHub](https://sourcegraph.com/docs/admin/external_service/github#github-com-rate-limits), [GitLab](https://sourcegraph.com/docs/admin/external_service/gitlab#internal-rate-limits) or [Bitbucket Server](https://sourcegraph.com/docs/admin/external_service/bitbucket_server#internal-rate-limits).
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-perms-syncer-outdated-perms).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_perms_syncer_outdated_perms"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (type) (src_repo_perms_syncer_outdated_perms)) >= 100)`

</details>

<br />

## worker: perms_syncer_sync_duration

<p class="subtitle">95th permissions sync duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 30s+ 95th permissions sync duration for 5m0s

**Next steps**

- Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-perms-syncer-sync-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_perms_syncer_sync_duration"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.95, max by (le, type) (rate(src_repo_perms_syncer_sync_duration_seconds_bucket[1m])))) >= 30)`

</details>

<br />

## worker: perms_syncer_sync_errors

<p class="subtitle">permissions sync error rate</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> worker: 1+ permissions sync error rate for 1m0s

**Next steps**

- Check the network connectivity the Sourcegraph and the code host.
- Check if API rate limit quota is exhausted on the code host.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-perms-syncer-sync-errors).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_worker_perms_syncer_sync_errors"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max by (type) (ceil(rate(src_repo_perms_syncer_sync_errors_total[1m])))) >= 1)`

</details>

<br />

## worker: insights_queue_unutilized_size

<p class="subtitle">insights queue size that is not utilized (not processing)</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 0+ insights queue size that is not utilized (not processing) for 30m0s

**Next steps**

- Verify code insights worker job has successfully started. Restart worker service and monitoring startup logs, looking for worker panics.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#worker-insights-queue-unutilized-size).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_insights_queue_unutilized_size"
]
```

<sub>*Managed by the [Sourcegraph Code Search team](https://handbook.sourcegraph.com/departments/engineering/teams/code-search).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_query_runner_worker_total{job=~"^worker.*"}) > 0 and on (job) sum by (op) (increase(src_workerutil_dbworker_store_insights_query_runner_jobs_store_total{job=~"^worker.*",op="Dequeue"}[5m])) < 1) > 0)`

</details>

<br />

## worker: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> worker: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_mean_blocked_seconds_per_conn_request",
  "critical_worker_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="worker"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="worker"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="worker"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="worker"}[5m]))) >= 0.5)`

</details>

<br />

## worker: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^worker.*"}) >= 99)`

</details>

<br />

## worker: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^worker.*"}) >= 99)`

</details>

<br />

## worker: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the worker service.
- **Docker Compose:** Consider increasing `cpus:` of the worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^worker.*"}[1d])) >= 80)`

</details>

<br />

## worker: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the worker service.
- **Docker Compose:** Consider increasing `memory:` of the worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^worker.*"}[1d])) >= 80)`

</details>

<br />

## worker: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^worker.*"}[5m])) >= 90)`

</details>

<br />

## worker: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of worker container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^worker.*"}[5m])) >= 90)`

</details>

<br />

## worker: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of worker container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#worker-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^worker.*"})) >= 1)`

</details>

<br />

## worker: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#worker-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*worker"})) >= 10000)`

</details>

<br />

## worker: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> worker: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_worker_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*worker"})) >= 2)`

</details>

<br />

## worker: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> worker: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod worker` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p worker`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_worker_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*worker"}) / count by (app) (up{app=~".*worker"}) * 100) <= 90)`

</details>

<br />

## worker: worker_site_configuration_duration_since_last_successful_update_by_instance

<p class="subtitle">maximum duration since last successful site configuration update (all "worker" instances)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> worker: 300s+ maximum duration since last successful site configuration update (all "worker" instances)

**Next steps**

- This indicates that one or more "worker" instances have not successfully updated the site configuration in over 5 minutes. This could be due to networking issues between services or problems with the site configuration service itself.
- Check for relevant errors in the "worker" logs, as well as frontend`s logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#worker-worker-site-configuration-duration-since-last-successful-update-by-instance).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_worker_worker_site_configuration_duration_since_last_successful_update_by_instance"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(max_over_time(src_conf_client_time_since_last_successful_update_seconds{job=~"^worker.*"}[1m]))) >= 300)`

</details>

<br />

## repo-updater: src_repoupdater_max_sync_backoff

<p class="subtitle">time since oldest sync</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: 32400s+ time since oldest sync for 10m0s

**Next steps**

- An alert here indicates that no code host connections have synced in at least 9h0m0s. This indicates that there could be a configuration issue
with your code hosts connections or networking issues affecting communication with your code hosts.
- Check the code host status indicator (cloud icon in top right of Sourcegraph homepage) for errors.
- Make sure external services do not have invalid tokens by navigating to them in the web UI and clicking save. If there are no errors, they are valid.
- Check the repo-updater logs for errors about syncing.
- Confirm that outbound network connections are allowed where repo-updater is deployed.
- Check back in an hour to see if the issue has resolved itself.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-src-repoupdater-max-sync-backoff).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_src_repoupdater_max_sync_backoff"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(src_repoupdater_max_sync_backoff)) >= 32400)`

</details>

<br />

## repo-updater: src_repoupdater_syncer_sync_errors_total

<p class="subtitle">site level external service sync error rate</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 0.5+ site level external service sync error rate for 10m0s
- <span class="badge badge-critical">critical</span> repo-updater: 1+ site level external service sync error rate for 10m0s

**Next steps**

- An alert here indicates errors syncing site level repo metadata with code hosts. This indicates that there could be a configuration issue
with your code hosts connections or networking issues affecting communication with your code hosts.
- Check the code host status indicator (cloud icon in top right of Sourcegraph homepage) for errors.
- Make sure external services do not have invalid tokens by navigating to them in the web UI and clicking save. If there are no errors, they are valid.
- Check the repo-updater logs for errors about syncing.
- Confirm that outbound network connections are allowed where repo-updater is deployed.
- Check back in an hour to see if the issue has resolved itself.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-src-repoupdater-syncer-sync-errors-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_src_repoupdater_syncer_sync_errors_total",
  "critical_repo-updater_src_repoupdater_syncer_sync_errors_total"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (family) (rate(src_repoupdater_syncer_sync_errors_total{owner!="user",reason!="internal_rate_limit",reason!="invalid_npm_path"}[5m]))) > 0.5)`

Generated query for critical alert: `max((max by (family) (rate(src_repoupdater_syncer_sync_errors_total{owner!="user",reason!="internal_rate_limit",reason!="invalid_npm_path"}[5m]))) > 1)`

</details>

<br />

## repo-updater: syncer_sync_start

<p class="subtitle">repo metadata sync was started</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 0 repo metadata sync was started for 9h0m0s

**Next steps**

- Check repo-updater logs for errors.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-syncer-sync-start).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_syncer_sync_start"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max by (family) (rate(src_repoupdater_syncer_start_sync{family="Syncer.SyncExternalService"}[9h]))) <= 0)`

</details>

<br />

## repo-updater: syncer_sync_duration

<p class="subtitle">95th repositories sync duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 30s+ 95th repositories sync duration for 5m0s

**Next steps**

- Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-syncer-sync-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_syncer_sync_duration"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.95, max by (le, family, success) (rate(src_repoupdater_syncer_sync_duration_seconds_bucket[1m])))) >= 30)`

</details>

<br />

## repo-updater: source_duration

<p class="subtitle">95th repositories source duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 30s+ 95th repositories source duration for 5m0s

**Next steps**

- Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-source-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_source_duration"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((histogram_quantile(0.95, max by (le) (rate(src_repoupdater_source_duration_seconds_bucket[1m])))) >= 30)`

</details>

<br />

## repo-updater: syncer_synced_repos

<p class="subtitle">repositories synced</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 0 repositories synced for 9h0m0s

**Next steps**

- Check network connectivity to code hosts
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-syncer-synced-repos).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_syncer_synced_repos"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(rate(src_repoupdater_syncer_synced_repos_total[1m]))) <= 0)`

</details>

<br />

## repo-updater: sourced_repos

<p class="subtitle">repositories sourced</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 0 repositories sourced for 9h0m0s

**Next steps**

- Check network connectivity to code hosts
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-sourced-repos).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_sourced_repos"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max(rate(src_repoupdater_source_repos_total[1m]))) <= 0)`

</details>

<br />

## repo-updater: purge_failed

<p class="subtitle">repositories purge failed</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 0+ repositories purge failed for 5m0s

**Next steps**

- Check repo-updater`s connectivity with gitserver and gitserver logs
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-purge-failed).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_purge_failed"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(rate(src_repoupdater_purge_failed[1m]))) > 0)`

</details>

<br />

## repo-updater: sched_auto_fetch

<p class="subtitle">repositories scheduled due to hitting a deadline</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 0 repositories scheduled due to hitting a deadline for 9h0m0s

**Next steps**

- Check repo-updater logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-sched-auto-fetch).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_sched_auto_fetch"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max(rate(src_repoupdater_sched_auto_fetch[1m]))) <= 0)`

</details>

<br />

## repo-updater: sched_known_repos

<p class="subtitle">repositories managed by the scheduler</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 0 repositories managed by the scheduler for 10m0s

**Next steps**

- Check repo-updater logs. This is expected to fire if there are no user added code hosts
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-sched-known-repos).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_sched_known_repos"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max(src_repoupdater_sched_known_repos)) <= 0)`

</details>

<br />

## repo-updater: sched_update_queue_length

<p class="subtitle">rate of growth of update queue length over 5 minutes</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: 0+ rate of growth of update queue length over 5 minutes for 2h0m0s

**Next steps**

- Check repo-updater logs for indications that the queue is not being processed. The queue length should trend downwards over time as items are sent to GitServer
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-sched-update-queue-length).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_sched_update_queue_length"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(deriv(src_repoupdater_sched_update_queue_length[5m]))) > 0)`

</details>

<br />

## repo-updater: sched_loops

<p class="subtitle">scheduler loops</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 0 scheduler loops for 9h0m0s

**Next steps**

- Check repo-updater logs for errors. This is expected to fire if there are no user added code hosts
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-sched-loops).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_sched_loops"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max(rate(src_repoupdater_sched_loops[1m]))) <= 0)`

</details>

<br />

## repo-updater: src_repoupdater_stale_repos

<p class="subtitle">repos that haven't been fetched in more than 8 hours</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 1+ repos that haven't been fetched in more than 8 hours for 25m0s

**Next steps**

- 							Check repo-updater logs for errors.
							Check for rows in gitserver_repos where LastError is not an empty string.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-src-repoupdater-stale-repos).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_src_repoupdater_stale_repos"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_repoupdater_stale_repos)) >= 1)`

</details>

<br />

## repo-updater: sched_error

<p class="subtitle">repositories schedule error rate</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: 1+ repositories schedule error rate for 25m0s

**Next steps**

- Check repo-updater logs for errors
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-sched-error).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_sched_error"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(rate(src_repoupdater_sched_error[1m]))) >= 1)`

</details>

<br />

## repo-updater: src_repoupdater_external_services_total

<p class="subtitle">the total number of external services</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: 20000+ the total number of external services for 1h0m0s

**Next steps**

- Check for spikes in external services, could be abuse
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-src-repoupdater-external-services-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_src_repoupdater_external_services_total"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(src_repoupdater_external_services_total)) >= 20000)`

</details>

<br />

## repo-updater: repoupdater_queued_sync_jobs_total

<p class="subtitle">the total number of queued sync jobs</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 100+ the total number of queued sync jobs for 1h0m0s

**Next steps**

- **Check if jobs are failing to sync:** "SELECT * FROM external_service_sync_jobs WHERE state = `errored`";
- **Increase the number of workers** using the `repoConcurrentExternalServiceSyncers` site config.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-repoupdater-queued-sync-jobs-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_repoupdater_queued_sync_jobs_total"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_repoupdater_queued_sync_jobs_total)) >= 100)`

</details>

<br />

## repo-updater: repoupdater_completed_sync_jobs_total

<p class="subtitle">the total number of completed sync jobs</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 100000+ the total number of completed sync jobs for 1h0m0s

**Next steps**

- Check repo-updater logs. Jobs older than 1 day should have been removed.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-repoupdater-completed-sync-jobs-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_repoupdater_completed_sync_jobs_total"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_repoupdater_completed_sync_jobs_total)) >= 100000)`

</details>

<br />

## repo-updater: repoupdater_errored_sync_jobs_percentage

<p class="subtitle">the percentage of external services that have failed their most recent sync</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 10%+ the percentage of external services that have failed their most recent sync for 1h0m0s

**Next steps**

- Check repo-updater logs. Check code host connectivity
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-repoupdater-errored-sync-jobs-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_repoupdater_errored_sync_jobs_percentage"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_repoupdater_errored_sync_jobs_percentage)) > 10)`

</details>

<br />

## repo-updater: github_graphql_rate_limit_remaining

<p class="subtitle">remaining calls to GitHub graphql API before hitting the rate limit</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 250 remaining calls to GitHub graphql API before hitting the rate limit

**Next steps**

- Consider creating a new token for the indicated resource (the `name` label for series below the threshold in the dashboard) under a dedicated machine user to reduce rate limit pressure.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-github-graphql-rate-limit-remaining).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_github_graphql_rate_limit_remaining"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max by (name) (src_github_rate_limit_remaining_v2{resource="graphql"})) <= 250)`

</details>

<br />

## repo-updater: github_rest_rate_limit_remaining

<p class="subtitle">remaining calls to GitHub rest API before hitting the rate limit</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 250 remaining calls to GitHub rest API before hitting the rate limit

**Next steps**

- Consider creating a new token for the indicated resource (the `name` label for series below the threshold in the dashboard) under a dedicated machine user to reduce rate limit pressure.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-github-rest-rate-limit-remaining).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_github_rest_rate_limit_remaining"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max by (name) (src_github_rate_limit_remaining_v2{resource="rest"})) <= 250)`

</details>

<br />

## repo-updater: github_search_rate_limit_remaining

<p class="subtitle">remaining calls to GitHub search API before hitting the rate limit</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: less than 5 remaining calls to GitHub search API before hitting the rate limit

**Next steps**

- Consider creating a new token for the indicated resource (the `name` label for series below the threshold in the dashboard) under a dedicated machine user to reduce rate limit pressure.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-github-search-rate-limit-remaining).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_github_search_rate_limit_remaining"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((max by (name) (src_github_rate_limit_remaining_v2{resource="search"})) <= 5)`

</details>

<br />

## repo-updater: gitlab_rest_rate_limit_remaining

<p class="subtitle">remaining calls to GitLab rest API before hitting the rate limit</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: less than 30 remaining calls to GitLab rest API before hitting the rate limit

**Next steps**

- Try restarting the pod to get a different public IP.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-gitlab-rest-rate-limit-remaining).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_gitlab_rest_rate_limit_remaining"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((max by (name) (src_gitlab_rate_limit_remaining{resource="rest"})) <= 30)`

</details>

<br />

## repo-updater: repo_updater_site_configuration_duration_since_last_successful_update_by_instance

<p class="subtitle">maximum duration since last successful site configuration update (all "repo_updater" instances)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: 300s+ maximum duration since last successful site configuration update (all "repo_updater" instances)

**Next steps**

- This indicates that one or more "repo_updater" instances have not successfully updated the site configuration in over 5 minutes. This could be due to networking issues between services or problems with the site configuration service itself.
- Check for relevant errors in the "repo_updater" logs, as well as frontend`s logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-repo-updater-site-configuration-duration-since-last-successful-update-by-instance).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_repo_updater_site_configuration_duration_since_last_successful_update_by_instance"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(max_over_time(src_conf_client_time_since_last_successful_update_seconds{job=~".*repo-updater"}[1m]))) >= 300)`

</details>

<br />

## repo-updater: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> repo-updater: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_mean_blocked_seconds_per_conn_request",
  "critical_repo-updater_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="repo-updater"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="repo-updater"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="repo-updater"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="repo-updater"}[5m]))) >= 0.5)`

</details>

<br />

## repo-updater: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the repo-updater container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^repo-updater.*"}) >= 99)`

</details>

<br />

## repo-updater: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: 90%+ container memory usage by instance for 10m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of repo-updater container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^repo-updater.*"}) >= 90)`

</details>

<br />

## repo-updater: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the repo-updater service.
- **Docker Compose:** Consider increasing `cpus:` of the repo-updater container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^repo-updater.*"}[1d])) >= 80)`

</details>

<br />

## repo-updater: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the repo-updater service.
- **Docker Compose:** Consider increasing `memory:` of the repo-updater container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^repo-updater.*"}[1d])) >= 80)`

</details>

<br />

## repo-updater: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the repo-updater container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^repo-updater.*"}[5m])) >= 90)`

</details>

<br />

## repo-updater: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of repo-updater container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^repo-updater.*"}[5m])) >= 90)`

</details>

<br />

## repo-updater: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of repo-updater container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#repo-updater-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^repo-updater.*"})) >= 1)`

</details>

<br />

## repo-updater: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#repo-updater-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*repo-updater"})) >= 10000)`

</details>

<br />

## repo-updater: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> repo-updater: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_repo-updater_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*repo-updater"})) >= 2)`

</details>

<br />

## repo-updater: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> repo-updater: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod repo-updater` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p repo-updater`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#repo-updater-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_repo-updater_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Source team](https://handbook.sourcegraph.com/departments/engineering/teams/source).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*repo-updater"}) / count by (app) (up{app=~".*repo-updater"}) * 100) <= 90)`

</details>

<br />

## searcher: replica_traffic

<p class="subtitle">requests per second per replica over 10m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 5+ requests per second per replica over 10m

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#searcher-replica-traffic).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_replica_traffic"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (instance) (rate(searcher_service_request_total[10m]))) >= 5)`

</details>

<br />

## searcher: unindexed_search_request_errors

<p class="subtitle">unindexed search request errors every 5m by code</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 5%+ unindexed search request errors every 5m by code for 5m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-unindexed-search-request-errors).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_unindexed_search_request_errors"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (code) (increase(searcher_service_request_total{code!="200",code!="canceled"}[5m])) / ignoring (code) group_left () sum(increase(searcher_service_request_total[5m])) * 100) >= 5)`

</details>

<br />

## searcher: searcher_site_configuration_duration_since_last_successful_update_by_instance

<p class="subtitle">maximum duration since last successful site configuration update (all "searcher" instances)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> searcher: 300s+ maximum duration since last successful site configuration update (all "searcher" instances)

**Next steps**

- This indicates that one or more "searcher" instances have not successfully updated the site configuration in over 5 minutes. This could be due to networking issues between services or problems with the site configuration service itself.
- Check for relevant errors in the "searcher" logs, as well as frontend`s logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-searcher-site-configuration-duration-since-last-successful-update-by-instance).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_searcher_searcher_site_configuration_duration_since_last_successful_update_by_instance"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(max_over_time(src_conf_client_time_since_last_successful_update_seconds{job=~".*searcher"}[1m]))) >= 300)`

</details>

<br />

## searcher: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> searcher: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_mean_blocked_seconds_per_conn_request",
  "critical_searcher_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="searcher"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="searcher"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="searcher"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="searcher"}[5m]))) >= 0.5)`

</details>

<br />

## searcher: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the searcher container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^searcher.*"}) >= 99)`

</details>

<br />

## searcher: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of searcher container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^searcher.*"}) >= 99)`

</details>

<br />

## searcher: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the searcher service.
- **Docker Compose:** Consider increasing `cpus:` of the searcher container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^searcher.*"}[1d])) >= 80)`

</details>

<br />

## searcher: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the searcher service.
- **Docker Compose:** Consider increasing `memory:` of the searcher container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^searcher.*"}[1d])) >= 80)`

</details>

<br />

## searcher: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the searcher container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^searcher.*"}[5m])) >= 90)`

</details>

<br />

## searcher: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of searcher container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^searcher.*"}[5m])) >= 90)`

</details>

<br />

## searcher: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of searcher container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#searcher-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^searcher.*"})) >= 1)`

</details>

<br />

## searcher: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#searcher-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*searcher"})) >= 10000)`

</details>

<br />

## searcher: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> searcher: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_searcher_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*searcher"})) >= 2)`

</details>

<br />

## searcher: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> searcher: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod searcher` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p searcher`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#searcher-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_searcher_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*searcher"}) / count by (app) (up{app=~".*searcher"}) * 100) <= 90)`

</details>

<br />

## symbols: symbols_site_configuration_duration_since_last_successful_update_by_instance

<p class="subtitle">maximum duration since last successful site configuration update (all "symbols" instances)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> symbols: 300s+ maximum duration since last successful site configuration update (all "symbols" instances)

**Next steps**

- This indicates that one or more "symbols" instances have not successfully updated the site configuration in over 5 minutes. This could be due to networking issues between services or problems with the site configuration service itself.
- Check for relevant errors in the "symbols" logs, as well as frontend`s logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-symbols-site-configuration-duration-since-last-successful-update-by-instance).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_symbols_symbols_site_configuration_duration_since_last_successful_update_by_instance"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(max_over_time(src_conf_client_time_since_last_successful_update_seconds{job=~".*symbols"}[1m]))) >= 300)`

</details>

<br />

## symbols: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> symbols: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_mean_blocked_seconds_per_conn_request",
  "critical_symbols_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="symbols"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="symbols"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="symbols"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="symbols"}[5m]))) >= 0.5)`

</details>

<br />

## symbols: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the symbols container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^symbols.*"}) >= 99)`

</details>

<br />

## symbols: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of symbols container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^symbols.*"}) >= 99)`

</details>

<br />

## symbols: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the symbols service.
- **Docker Compose:** Consider increasing `cpus:` of the symbols container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^symbols.*"}[1d])) >= 80)`

</details>

<br />

## symbols: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the symbols service.
- **Docker Compose:** Consider increasing `memory:` of the symbols container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^symbols.*"}[1d])) >= 80)`

</details>

<br />

## symbols: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the symbols container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^symbols.*"}[5m])) >= 90)`

</details>

<br />

## symbols: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of symbols container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^symbols.*"}[5m])) >= 90)`

</details>

<br />

## symbols: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of symbols container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#symbols-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^symbols.*"})) >= 1)`

</details>

<br />

## symbols: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#symbols-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*symbols"})) >= 10000)`

</details>

<br />

## symbols: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> symbols: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_symbols_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*symbols"})) >= 2)`

</details>

<br />

## symbols: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> symbols: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod symbols` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p symbols`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#symbols-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_symbols_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*symbols"}) / count by (app) (up{app=~".*symbols"}) * 100) <= 90)`

</details>

<br />

## syntect-server: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> syntect-server: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the syntect-server container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#syntect-server-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_syntect-server_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^syntect-server.*"}) >= 99)`

</details>

<br />

## syntect-server: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> syntect-server: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of syntect-server container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#syntect-server-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_syntect-server_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^syntect-server.*"}) >= 99)`

</details>

<br />

## syntect-server: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> syntect-server: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the syntect-server service.
- **Docker Compose:** Consider increasing `cpus:` of the syntect-server container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#syntect-server-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_syntect-server_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^syntect-server.*"}[1d])) >= 80)`

</details>

<br />

## syntect-server: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> syntect-server: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the syntect-server service.
- **Docker Compose:** Consider increasing `memory:` of the syntect-server container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#syntect-server-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_syntect-server_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^syntect-server.*"}[1d])) >= 80)`

</details>

<br />

## syntect-server: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> syntect-server: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the syntect-server container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#syntect-server-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_syntect-server_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^syntect-server.*"}[5m])) >= 90)`

</details>

<br />

## syntect-server: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> syntect-server: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of syntect-server container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#syntect-server-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_syntect-server_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^syntect-server.*"}[5m])) >= 90)`

</details>

<br />

## syntect-server: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> syntect-server: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of syntect-server container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#syntect-server-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_syntect-server_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^syntect-server.*"})) >= 1)`

</details>

<br />

## syntect-server: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> syntect-server: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod syntect-server` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p syntect-server`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#syntect-server-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_syntect-server_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*syntect-server"}) / count by (app) (up{app=~".*syntect-server"}) * 100) <= 90)`

</details>

<br />

## zoekt: average_resolve_revision_duration

<p class="subtitle">average resolve revision duration over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 15s+ average resolve revision duration over 5m

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-average-resolve-revision-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_average_resolve_revision_duration"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(rate(resolve_revision_seconds_sum[5m])) / sum(rate(resolve_revision_seconds_count[5m]))) >= 15)`

</details>

<br />

## zoekt: get_index_options_error_increase

<p class="subtitle">the number of repositories we failed to get indexing options over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 100+ the number of repositories we failed to get indexing options over 5m for 5m0s
- <span class="badge badge-critical">critical</span> zoekt: 100+ the number of repositories we failed to get indexing options over 5m for 35m0s

**Next steps**

- View error rates on gitserver and frontend to identify root cause.
- Rollback frontend/gitserver deployment if due to a bad code change.
- View error logs for `getIndexOptions` via net/trace debug interface. For example click on a `indexed-search-indexer-` on https://sourcegraph.com/-/debug/. Then click on Traces. Replace sourcegraph.com with your instance address.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#zoekt-get-index-options-error-increase).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_get_index_options_error_increase",
  "critical_zoekt_get_index_options_error_increase"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(increase(get_index_options_error_total[5m]))) >= 100)`

Generated query for critical alert: `max((sum(increase(get_index_options_error_total[5m]))) >= 100)`

</details>

<br />

## zoekt: indexed_search_request_errors

<p class="subtitle">indexed search request errors every 5m by code</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 5%+ indexed search request errors every 5m by code for 5m0s

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-indexed-search-request-errors).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_indexed_search_request_errors"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (code) (increase(src_zoekt_request_duration_seconds_count{code!~"2.."}[5m])) / ignoring (code) group_left () sum(increase(src_zoekt_request_duration_seconds_count[5m])) * 100) >= 5)`

</details>

<br />

## zoekt: memory_map_areas_percentage_used

<p class="subtitle">process memory map areas percentage used (per instance)</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 60%+ process memory map areas percentage used (per instance)
- <span class="badge badge-critical">critical</span> zoekt: 80%+ process memory map areas percentage used (per instance)

**Next steps**

- If you are running out of memory map areas, you could resolve this by:

    - Enabling shard merging for Zoekt: Set SRC_ENABLE_SHARD_MERGING="1" for zoekt-indexserver. Use this option
if your corpus of repositories has a high percentage of small, rarely updated repositories. See
[documentation](https://sourcegraph.com/docs/code-search/features#shard-merging).
    - Creating additional Zoekt replicas: This spreads all the shards out amongst more replicas, which
means that each _individual_ replica will have fewer shards. This, in turn, decreases the
amount of memory map areas that a _single_ replica can create (in order to load the shards into memory).
    - Increasing the virtual memory subsystem`s "max_map_count" parameter which defines the upper limit of memory areas
a process can use. The default value of max_map_count is usually 65536. We recommend to set this value to 2x the number
of repos to be indexed per Zoekt instance. This means, if you want to index 240k repositories with 3 Zoekt instances,
set max_map_count to (240000 / 3) * 2 = 160000. The exact instructions for tuning this parameter can differ depending
on your environment. See https://kernel.org/doc/Documentation/sysctl/vm.txt for more information.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#zoekt-memory-map-areas-percentage-used).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_memory_map_areas_percentage_used",
  "critical_zoekt_memory_map_areas_percentage_used"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max(((proc_metrics_memory_map_current_count / proc_metrics_memory_map_max_limit) * 100) >= 60)`

Generated query for critical alert: `max(((proc_metrics_memory_map_current_count / proc_metrics_memory_map_max_limit) * 100) >= 80)`

</details>

<br />

## zoekt: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the zoekt-indexserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^zoekt-indexserver.*"}) >= 99)`

</details>

<br />

## zoekt: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of zoekt-indexserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^zoekt-indexserver.*"}) >= 99)`

</details>

<br />

## zoekt: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^zoekt-webserver.*"}) >= 99)`

</details>

<br />

## zoekt: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of zoekt-webserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^zoekt-webserver.*"}) >= 99)`

</details>

<br />

## zoekt: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the zoekt-indexserver service.
- **Docker Compose:** Consider increasing `cpus:` of the zoekt-indexserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^zoekt-indexserver.*"}[1d])) >= 80)`

</details>

<br />

## zoekt: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the zoekt-indexserver service.
- **Docker Compose:** Consider increasing `memory:` of the zoekt-indexserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^zoekt-indexserver.*"}[1d])) >= 80)`

</details>

<br />

## zoekt: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the zoekt-indexserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^zoekt-indexserver.*"}[5m])) >= 90)`

</details>

<br />

## zoekt: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of zoekt-indexserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^zoekt-indexserver.*"}[5m])) >= 90)`

</details>

<br />

## zoekt: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of zoekt-indexserver container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#zoekt-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^zoekt-indexserver.*"})) >= 1)`

</details>

<br />

## zoekt: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the zoekt-webserver service.
- **Docker Compose:** Consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^zoekt-webserver.*"}[1d])) >= 80)`

</details>

<br />

## zoekt: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the zoekt-webserver service.
- **Docker Compose:** Consider increasing `memory:` of the zoekt-webserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^zoekt-webserver.*"}[1d])) >= 80)`

</details>

<br />

## zoekt: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the zoekt-webserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^zoekt-webserver.*"}[5m])) >= 90)`

</details>

<br />

## zoekt: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of zoekt-webserver container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^zoekt-webserver.*"}[5m])) >= 90)`

</details>

<br />

## zoekt: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> zoekt: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of zoekt-webserver container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#zoekt-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_zoekt_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^zoekt-webserver.*"})) >= 1)`

</details>

<br />

## zoekt: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> zoekt: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod indexed-search` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p indexed-search`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#zoekt-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_zoekt_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Search Platform team](https://handbook.sourcegraph.com/departments/engineering/teams/search/core).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*indexed-search"}) / count by (app) (up{app=~".*indexed-search"}) * 100) <= 90)`

</details>

<br />

## prometheus: prometheus_rule_eval_duration

<p class="subtitle">average prometheus rule group evaluation duration over 10m by rule group</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 30s+ average prometheus rule group evaluation duration over 10m by rule group

**Next steps**

- Check the Container monitoring (not available on server) panels and try increasing resources for Prometheus if necessary.
- If the rule group taking a long time to evaluate belongs to `/sg_prometheus_addons`, try reducing the complexity of any custom Prometheus rules provided.
- If the rule group taking a long time to evaluate belongs to `/sg_config_prometheus`, please [open an issue](https://github.com/sourcegraph/sourcegraph/issues/new?assignees=&labels=&template=bug_report.md&title=).
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#prometheus-prometheus-rule-eval-duration).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_prometheus_rule_eval_duration"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (rule_group) (avg_over_time(prometheus_rule_group_last_duration_seconds[10m]))) >= 30)`

</details>

<br />

## prometheus: prometheus_rule_eval_failures

<p class="subtitle">failed prometheus rule evaluations over 5m by rule group</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 0+ failed prometheus rule evaluations over 5m by rule group

**Next steps**

- Check Prometheus logs for messages related to rule group evaluation (generally with log field `component="rule manager"`).
- If the rule group failing to evaluate belongs to `/sg_prometheus_addons`, ensure any custom Prometheus configuration provided is valid.
- If the rule group taking a long time to evaluate belongs to `/sg_config_prometheus`, please [open an issue](https://github.com/sourcegraph/sourcegraph/issues/new?assignees=&labels=&template=bug_report.md&title=).
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#prometheus-prometheus-rule-eval-failures).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_prometheus_rule_eval_failures"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (rule_group) (rate(prometheus_rule_evaluation_failures_total[5m]))) > 0)`

</details>

<br />

## prometheus: alertmanager_notification_latency

<p class="subtitle">alertmanager notification latency over 1m by integration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 1s+ alertmanager notification latency over 1m by integration

**Next steps**

- Check the Container monitoring (not available on server) panels and try increasing resources for Prometheus if necessary.
- Ensure that your [`observability.alerts` configuration](https://sourcegraph.com/docs/admin/observability/alerting#setting-up-alerting) (in site configuration) is valid.
- Check if the relevant alert integration service is experiencing downtime or issues.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-alertmanager-notification-latency).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_alertmanager_notification_latency"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (integration) (rate(alertmanager_notification_latency_seconds_sum[1m]))) >= 1)`

</details>

<br />

## prometheus: alertmanager_notification_failures

<p class="subtitle">failed alertmanager notifications over 1m by integration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 0+ failed alertmanager notifications over 1m by integration

**Next steps**

- Ensure that your [`observability.alerts` configuration](https://sourcegraph.com/docs/admin/observability/alerting#setting-up-alerting) (in site configuration) is valid.
- Check if the relevant alert integration service is experiencing downtime or issues.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-alertmanager-notification-failures).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_alertmanager_notification_failures"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (integration) (rate(alertmanager_notifications_failed_total[1m]))) > 0)`

</details>

<br />

## prometheus: prometheus_config_status

<p class="subtitle">prometheus configuration reload status</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: less than 1 prometheus configuration reload status

**Next steps**

- Check Prometheus logs for messages related to configuration loading.
- Ensure any [custom configuration you have provided Prometheus](https://sourcegraph.com/docs/admin/observability/metrics#prometheus-configuration) is valid.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#prometheus-prometheus-config-status).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_prometheus_config_status"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((prometheus_config_last_reload_successful) < 1)`

</details>

<br />

## prometheus: alertmanager_config_status

<p class="subtitle">alertmanager configuration reload status</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: less than 1 alertmanager configuration reload status

**Next steps**

- Ensure that your [`observability.alerts` configuration](https://sourcegraph.com/docs/admin/observability/alerting#setting-up-alerting) (in site configuration) is valid.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#prometheus-alertmanager-config-status).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_alertmanager_config_status"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `min((alertmanager_config_last_reload_successful) < 1)`

</details>

<br />

## prometheus: prometheus_tsdb_op_failure

<p class="subtitle">prometheus tsdb failures by operation over 1m by operation</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 0+ prometheus tsdb failures by operation over 1m by operation

**Next steps**

- Check Prometheus logs for messages related to the failing operation.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-prometheus-tsdb-op-failure).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_prometheus_tsdb_op_failure"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((increase(label_replace({__name__=~"prometheus_tsdb_(.*)_failed_total"}, "operation", "$1", "__name__", "(.+)s_failed_total")[5m:1m])) > 0)`

</details>

<br />

## prometheus: prometheus_target_sample_exceeded

<p class="subtitle">prometheus scrapes that exceed the sample limit over 10m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 0+ prometheus scrapes that exceed the sample limit over 10m

**Next steps**

- Check Prometheus logs for messages related to target scrape failures.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-prometheus-target-sample-exceeded).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_prometheus_target_sample_exceeded"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((increase(prometheus_target_scrapes_exceeded_sample_limit_total[10m])) > 0)`

</details>

<br />

## prometheus: prometheus_target_sample_duplicate

<p class="subtitle">prometheus scrapes rejected due to duplicate timestamps over 10m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 0+ prometheus scrapes rejected due to duplicate timestamps over 10m

**Next steps**

- Check Prometheus logs for messages related to target scrape failures.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-prometheus-target-sample-duplicate).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_prometheus_target_sample_duplicate"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((increase(prometheus_target_scrapes_sample_duplicate_timestamp_total[10m])) > 0)`

</details>

<br />

## prometheus: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the prometheus container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^prometheus.*"}) >= 99)`

</details>

<br />

## prometheus: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of prometheus container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^prometheus.*"}) >= 99)`

</details>

<br />

## prometheus: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the prometheus service.
- **Docker Compose:** Consider increasing `cpus:` of the prometheus container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^prometheus.*"}[1d])) >= 80)`

</details>

<br />

## prometheus: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the prometheus service.
- **Docker Compose:** Consider increasing `memory:` of the prometheus container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^prometheus.*"}[1d])) >= 80)`

</details>

<br />

## prometheus: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the prometheus container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^prometheus.*"}[5m])) >= 90)`

</details>

<br />

## prometheus: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of prometheus container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^prometheus.*"}[5m])) >= 90)`

</details>

<br />

## prometheus: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> prometheus: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of prometheus container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#prometheus-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_prometheus_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^prometheus.*"})) >= 1)`

</details>

<br />

## prometheus: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> prometheus: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod prometheus` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p prometheus`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#prometheus-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_prometheus_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*prometheus"}) / count by (app) (up{app=~".*prometheus"}) * 100) <= 90)`

</details>

<br />

## executor: executor_handlers

<p class="subtitle">executor active handlers</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> executor: 0 active executor handlers and > 0 queue size for 5m0s

**Next steps**

- Check to see the state of any compute VMs, they may be taking longer than expected to boot.
- Make sure the executors appear under Site Admin > Executors.
- Check the Grafana dashboard section for APIClient, it should do frequent requests to Dequeue and Heartbeat and those must not fail.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#executor-executor-handlers).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_executor_executor_handlers"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Custom query for critical alert: `min(((sum(src_executor_processor_handlers{sg_job=~"^sourcegraph-executors.*"}) or vector(0)) == 0 and (sum by (queue) (src_executor_total{job=~"^sourcegraph-executors.*"})) > 0) <= 0)`

</details>

<br />

## executor: executor_processor_error_rate

<p class="subtitle">executor operation error rate over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> executor: 100%+ executor operation error rate over 5m for 1h0m0s

**Next steps**

- Determine the cause of failure from the auto-indexing job logs in the site-admin page.
- This alert fires if all executor jobs have been failing for the past hour. The alert will continue for up
to 5 hours until the error rate is no longer 100%, even if there are no running jobs in that time, as the
problem is not know to be resolved until jobs start succeeding again.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#executor-executor-processor-error-rate).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_executor_executor_processor_error_rate"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Custom query for warning alert: `max((last_over_time(sum(increase(src_executor_processor_errors_total{sg_job=~"^sourcegraph-executors.*"}[5m]))[5h:]) / (last_over_time(sum(increase(src_executor_processor_total{sg_job=~"^sourcegraph-executors.*"}[5m]))[5h:]) + last_over_time(sum(increase(src_executor_processor_errors_total{sg_job=~"^sourcegraph-executors.*"}[5m]))[5h:])) * 100) >= 100)`

</details>

<br />

## executor: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> executor: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#executor-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_executor_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (sg_instance) (go_goroutines{sg_job=~".*sourcegraph-executors"})) >= 10000)`

</details>

<br />

## executor: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> executor: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#executor-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_executor_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (sg_instance) (go_gc_duration_seconds{sg_job=~".*sourcegraph-executors"})) >= 2)`

</details>

<br />

## codeintel-uploads: codeintel_commit_graph_queued_max_age

<p class="subtitle">repository queue longest time in queue</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> codeintel-uploads: 3600s+ repository queue longest time in queue

**Next steps**

- An alert here is generally indicative of either underprovisioned worker instance(s) and/or
an underprovisioned main postgres instance.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#codeintel-uploads-codeintel-commit-graph-queued-max-age).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_codeintel-uploads_codeintel_commit_graph_queued_max_age"
]
```

<sub>*Managed by the [Sourcegraph Code intelligence team](https://handbook.sourcegraph.com/departments/engineering/teams/code-intelligence).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(src_codeintel_commit_graph_queued_duration_seconds_total)) >= 3600)`

</details>

<br />

## telemetry: telemetry_gateway_exporter_queue_growth

<p class="subtitle">rate of growth of export queue over 30m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> telemetry: 1+ rate of growth of export queue over 30m for 1h0m0s
- <span class="badge badge-critical">critical</span> telemetry: 1+ rate of growth of export queue over 30m for 36h0m0s

**Next steps**

- Check the "number of events exported per batch over 30m" dashboard panel to see if export throughput is at saturation.
- Increase `TELEMETRY_GATEWAY_EXPORTER_EXPORT_BATCH_SIZE` to export more events per batch.
- Reduce `TELEMETRY_GATEWAY_EXPORTER_EXPORT_INTERVAL` to schedule more export jobs.
- See worker logs in the `worker.telemetrygateway-exporter` log scope for more details to see if any export errors are occuring - if logs only indicate that exports failed, reach out to Sourcegraph with relevant log entries, as this may be an issue in Sourcegraph`s Telemetry Gateway service.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#telemetry-telemetry-gateway-exporter-queue-growth).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_telemetry_telemetry_gateway_exporter_queue_growth",
  "critical_telemetry_telemetry_gateway_exporter_queue_growth"
]
```

<sub>*Managed by the [Sourcegraph Data & Analytics team](https://handbook.sourcegraph.com/departments/engineering/teams/data-analytics).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max(deriv(src_telemetrygatewayexporter_queue_size[30m]))) > 1)`

Generated query for critical alert: `max((max(deriv(src_telemetrygatewayexporter_queue_size[30m]))) > 1)`

</details>

<br />

## telemetry: telemetrygatewayexporter_exporter_errors_total

<p class="subtitle">events exporter operation errors every 30m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> telemetry: 0+ events exporter operation errors every 30m

**Next steps**

- Failures indicate that exporting of telemetry events from Sourcegraph are failing. This may affect the performance of the database as the backlog grows.
- See worker logs in the `worker.telemetrygateway-exporter` log scope for more details. If logs only indicate that exports failed, reach out to Sourcegraph with relevant log entries, as this may be an issue in Sourcegraph`s Telemetry Gateway service.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#telemetry-telemetrygatewayexporter-exporter-errors-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_telemetry_telemetrygatewayexporter_exporter_errors_total"
]
```

<sub>*Managed by the [Sourcegraph Data & Analytics team](https://handbook.sourcegraph.com/departments/engineering/teams/data-analytics).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(increase(src_telemetrygatewayexporter_exporter_errors_total{job=~"^worker.*"}[30m]))) > 0)`

</details>

<br />

## telemetry: telemetrygatewayexporter_queue_cleanup_errors_total

<p class="subtitle">export queue cleanup operation errors every 30m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> telemetry: 0+ export queue cleanup operation errors every 30m

**Next steps**

- Failures indicate that pruning of already-exported telemetry events from the database is failing. This may affect the performance of the database as the export queue table grows.
- See worker logs in the `worker.telemetrygateway-exporter` log scope for more details.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#telemetry-telemetrygatewayexporter-queue-cleanup-errors-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_telemetry_telemetrygatewayexporter_queue_cleanup_errors_total"
]
```

<sub>*Managed by the [Sourcegraph Data & Analytics team](https://handbook.sourcegraph.com/departments/engineering/teams/data-analytics).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(increase(src_telemetrygatewayexporter_queue_cleanup_errors_total{job=~"^worker.*"}[30m]))) > 0)`

</details>

<br />

## telemetry: telemetrygatewayexporter_queue_metrics_reporter_errors_total

<p class="subtitle">export backlog metrics reporting operation errors every 30m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> telemetry: 0+ export backlog metrics reporting operation errors every 30m

**Next steps**

- Failures indicate that reporting of telemetry events metrics is failing. This may affect the reliability of telemetry events export metrics.
- See worker logs in the `worker.telemetrygateway-exporter` log scope for more details.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#telemetry-telemetrygatewayexporter-queue-metrics-reporter-errors-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_telemetry_telemetrygatewayexporter_queue_metrics_reporter_errors_total"
]
```

<sub>*Managed by the [Sourcegraph Data & Analytics team](https://handbook.sourcegraph.com/departments/engineering/teams/data-analytics).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum(increase(src_telemetrygatewayexporter_queue_metrics_reporter_errors_total{job=~"^worker.*"}[30m]))) > 0)`

</details>

<br />

## telemetry: telemetry_job_error_rate

<p class="subtitle">usage data exporter operation error rate over 5m</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> telemetry: 0%+ usage data exporter operation error rate over 5m for 30m0s

**Next steps**

- Involved cloud team to inspect logs of the managed instance to determine error sources.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#telemetry-telemetry-job-error-rate).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_telemetry_telemetry_job_error_rate"
]
```

<sub>*Managed by the [Sourcegraph Data & Analytics team](https://handbook.sourcegraph.com/departments/engineering/teams/data-analytics).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (op) (increase(src_telemetry_job_errors_total{job=~"^worker.*"}[5m])) / (sum by (op) (increase(src_telemetry_job_total{job=~"^worker.*"}[5m])) + sum by (op) (increase(src_telemetry_job_errors_total{job=~"^worker.*"}[5m]))) * 100) > 0)`

</details>

<br />

## telemetry: telemetry_job_utilized_throughput

<p class="subtitle">utilized percentage of maximum throughput</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> telemetry: 90%+ utilized percentage of maximum throughput for 30m0s

**Next steps**

- Throughput utilization is high. This could be a signal that this instance is producing too many events for the export job to keep up. Configure more throughput using the maxBatchSize option.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#telemetry-telemetry-job-utilized-throughput).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_telemetry_telemetry_job_utilized_throughput"
]
```

<sub>*Managed by the [Sourcegraph Data & Analytics team](https://handbook.sourcegraph.com/departments/engineering/teams/data-analytics).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((rate(src_telemetry_job_total{op="SendEvents"}[1h]) / on () group_right () src_telemetry_job_max_throughput * 100) > 90)`

</details>

<br />

## otel-collector: otel_span_refused

<p class="subtitle">spans refused per receiver</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> otel-collector: 1+ spans refused per receiver for 5m0s

**Next steps**

- Check logs of the collector and configuration of the receiver
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#otel-collector-otel-span-refused).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_otel-collector_otel_span_refused"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (receiver) (rate(otelcol_receiver_refused_spans[1m]))) > 1)`

</details>

<br />

## otel-collector: otel_span_export_failures

<p class="subtitle">span export failures by exporter</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> otel-collector: 1+ span export failures by exporter for 5m0s

**Next steps**

- Check the configuration of the exporter and if the service being exported is up
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#otel-collector-otel-span-export-failures).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_otel-collector_otel_span_export_failures"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (exporter) (rate(otelcol_exporter_send_failed_spans[1m]))) > 1)`

</details>

<br />

## otel-collector: otelcol_exporter_enqueue_failed_spans

<p class="subtitle">exporter enqueue failed spans</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> otel-collector: 0+ exporter enqueue failed spans for 5m0s

**Next steps**

- Check the configuration of the exporter and if the service being exported is up. This may be caused by a queue full of unsettled elements, so you may need to decrease your sending rate or horizontally scale collectors.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#otel-collector-otelcol-exporter-enqueue-failed-spans).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_otel-collector_otelcol_exporter_enqueue_failed_spans"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (exporter) (rate(otelcol_exporter_enqueue_failed_spans{job=~"^.*"}[1m]))) > 0)`

</details>

<br />

## otel-collector: otelcol_processor_dropped_spans

<p class="subtitle">spans dropped per processor per minute</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> otel-collector: 0+ spans dropped per processor per minute for 5m0s

**Next steps**

- Check the configuration of the processor
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#otel-collector-otelcol-processor-dropped-spans).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_otel-collector_otelcol_processor_dropped_spans"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (processor) (rate(otelcol_processor_dropped_spans[1m]))) > 0)`

</details>

<br />

## otel-collector: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> otel-collector: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the otel-collector container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#otel-collector-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_otel-collector_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^otel-collector.*"}) >= 99)`

</details>

<br />

## otel-collector: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> otel-collector: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of otel-collector container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#otel-collector-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_otel-collector_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^otel-collector.*"}) >= 99)`

</details>

<br />

## otel-collector: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> otel-collector: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod otel-collector` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p otel-collector`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#otel-collector-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_otel-collector_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*otel-collector"}) / count by (app) (up{app=~".*otel-collector"}) * 100) <= 90)`

</details>

<br />

## embeddings: embeddings_site_configuration_duration_since_last_successful_update_by_instance

<p class="subtitle">maximum duration since last successful site configuration update (all "embeddings" instances)</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> embeddings: 300s+ maximum duration since last successful site configuration update (all "embeddings" instances)

**Next steps**

- This indicates that one or more "embeddings" instances have not successfully updated the site configuration in over 5 minutes. This could be due to networking issues between services or problems with the site configuration service itself.
- Check for relevant errors in the "embeddings" logs, as well as frontend`s logs.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-embeddings-site-configuration-duration-since-last-successful-update-by-instance).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_embeddings_embeddings_site_configuration_duration_since_last_successful_update_by_instance"
]
```

<sub>*Managed by the [Sourcegraph Infrastructure Org team](https://handbook.sourcegraph.com/departments/engineering/infrastructure).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `max((max(max_over_time(src_conf_client_time_since_last_successful_update_seconds{job=~".*embeddings"}[1m]))) >= 300)`

</details>

<br />

## embeddings: mean_blocked_seconds_per_conn_request

<p class="subtitle">mean blocked seconds per conn request</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 0.1s+ mean blocked seconds per conn request for 10m0s
- <span class="badge badge-critical">critical</span> embeddings: 0.5s+ mean blocked seconds per conn request for 10m0s

**Next steps**

- Increase SRC_PGSQL_MAX_OPEN together with giving more memory to the database if needed
- Scale up Postgres memory/cpus - [see our scaling guide](https://sourcegraph.com/docs/admin/config/postgres-conf)
- If using GCP Cloud SQL, check for high lock waits or CPU usage in query insights
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-mean-blocked-seconds-per-conn-request).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_mean_blocked_seconds_per_conn_request",
  "critical_embeddings_mean_blocked_seconds_per_conn_request"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="embeddings"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="embeddings"}[5m]))) >= 0.1)`

Generated query for critical alert: `max((sum by (app_name, db_name) (increase(src_pgsql_conns_blocked_seconds{app_name="embeddings"}[5m])) / sum by (app_name, db_name) (increase(src_pgsql_conns_waited_for{app_name="embeddings"}[5m]))) >= 0.5)`

</details>

<br />

## embeddings: container_cpu_usage

<p class="subtitle">container cpu usage total (1m average) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 99%+ container cpu usage total (1m average) across all cores by instance

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the embeddings container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-container-cpu-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_container_cpu_usage"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_cpu_usage_percentage_total{name=~"^embeddings.*"}) >= 99)`

</details>

<br />

## embeddings: container_memory_usage

<p class="subtitle">container memory usage by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 99%+ container memory usage by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of embeddings container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-container-memory-usage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_container_memory_usage"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((cadvisor_container_memory_usage_percentage_total{name=~"^embeddings.*"}) >= 99)`

</details>

<br />

## embeddings: provisioning_container_cpu_usage_long_term

<p class="subtitle">container cpu usage total (90th percentile over 1d) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 80%+ container cpu usage total (90th percentile over 1d) across all cores by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the `Deployment.yaml` for the embeddings service.
- **Docker Compose:** Consider increasing `cpus:` of the embeddings container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-provisioning-container-cpu-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_provisioning_container_cpu_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((quantile_over_time(0.9, cadvisor_container_cpu_usage_percentage_total{name=~"^embeddings.*"}[1d])) >= 80)`

</details>

<br />

## embeddings: provisioning_container_memory_usage_long_term

<p class="subtitle">container memory usage (1d maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 80%+ container memory usage (1d maximum) by instance for 336h0m0s

**Next steps**

- **Kubernetes:** Consider increasing memory limits in the `Deployment.yaml` for the embeddings service.
- **Docker Compose:** Consider increasing `memory:` of the embeddings container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-provisioning-container-memory-usage-long-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_provisioning_container_memory_usage_long_term"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^embeddings.*"}[1d])) >= 80)`

</details>

<br />

## embeddings: provisioning_container_cpu_usage_short_term

<p class="subtitle">container cpu usage total (5m maximum) across all cores by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 90%+ container cpu usage total (5m maximum) across all cores by instance for 30m0s

**Next steps**

- **Kubernetes:** Consider increasing CPU limits in the the relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `cpus:` of the embeddings container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-provisioning-container-cpu-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_provisioning_container_cpu_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_cpu_usage_percentage_total{name=~"^embeddings.*"}[5m])) >= 90)`

</details>

<br />

## embeddings: provisioning_container_memory_usage_short_term

<p class="subtitle">container memory usage (5m maximum) by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 90%+ container memory usage (5m maximum) by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of embeddings container in `docker-compose.yml`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-provisioning-container-memory-usage-short-term).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_provisioning_container_memory_usage_short_term"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max_over_time(cadvisor_container_memory_usage_percentage_total{name=~"^embeddings.*"}[5m])) >= 90)`

</details>

<br />

## embeddings: container_oomkill_events_total

<p class="subtitle">container OOMKILL events total by instance</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 1+ container OOMKILL events total by instance

**Next steps**

- **Kubernetes:** Consider increasing memory limit in relevant `Deployment.yaml`.
- **Docker Compose:** Consider increasing `memory:` of embeddings container in `docker-compose.yml`.
- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#embeddings-container-oomkill-events-total).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_container_oomkill_events_total"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (name) (container_oom_events_total{name=~"^embeddings.*"})) >= 1)`

</details>

<br />

## embeddings: go_goroutines

<p class="subtitle">maximum active goroutines</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 10000+ maximum active goroutines for 10m0s

**Next steps**

- More help interpreting this metric is available in the [dashboards reference](./dashboards.md#embeddings-go-goroutines).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_go_goroutines"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_goroutines{job=~".*embeddings"})) >= 10000)`

</details>

<br />

## embeddings: go_gc_duration_seconds

<p class="subtitle">maximum go garbage collection duration</p>

**Descriptions**

- <span class="badge badge-warning">warning</span> embeddings: 2s+ maximum go garbage collection duration

**Next steps**

- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-go-gc-duration-seconds).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "warning_embeddings_go_gc_duration_seconds"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for warning alert: `max((max by (instance) (go_gc_duration_seconds{job=~".*embeddings"})) >= 2)`

</details>

<br />

## embeddings: pods_available_percentage

<p class="subtitle">percentage pods available</p>

**Descriptions**

- <span class="badge badge-critical">critical</span> embeddings: less than 90% percentage pods available for 10m0s

**Next steps**

- Determine if the pod was OOM killed using `kubectl describe pod embeddings` (look for `OOMKilled: true`) and, if so, consider increasing the memory limit in the relevant `Deployment.yaml`.
- Check the logs before the container restarted to see if there are `panic:` messages or similar using `kubectl logs -p embeddings`.
- Learn more about the related dashboard panel in the [dashboards reference](./dashboards.md#embeddings-pods-available-percentage).
- **Silence this alert:** If you are aware of this alert and want to silence notifications for it, add the following to your site configuration and set a reminder to re-evaluate the alert:

```json
"observability.silenceAlerts": [
  "critical_embeddings_pods_available_percentage"
]
```

<sub>*Managed by the [Sourcegraph Cody team](https://handbook.sourcegraph.com/departments/engineering/teams/cody).*</sub>

<details>
<summary>Technical details</summary>

Generated query for critical alert: `min((sum by (app) (up{app=~".*embeddings"}) / count by (app) (up{app=~".*embeddings"}) * 100) <= 90)`

</details>

<br />
