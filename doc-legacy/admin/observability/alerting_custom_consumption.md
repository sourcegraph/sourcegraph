# Custom consumption of Sourcegraph alerts

If Sourcegraph's builtin [alerting](alerting.md) (which can notify you via email, Slack, PagerDuty, webhook, and more) is not sufficient for you, or if you just prefer to consume the alerts programatically for some reason, then this page is for you.

For more information about Sourcegraph alerts, see [high level alerting metrics](metrics.md#high-level-alerting-metrics).

## Prometheus queries

Below are examples of how to query alerts that are being monitored or are currently firing in Sourcegraph. If you do wish to query warning alerts, too, then simply replace `critical` with `warning` in any of the below query examples.

### "How many critical alerts were firing in the last minute?"

Prometheus query:

```prometheus
sum(max by (service_name,name,description)(max_over_time(alert_count{level="critical",name!=""}[1m])))
```

Example `curl` query:

```sh
curl 'http://$PROMETHEUS_URL/api/v1/query?query=sum%28max%20by%20%28service_name%2Cname%2Cdescription%29%28max_over_time%28alert_count%7Blevel%3D%22critical%22%2Cname%21%3D%22%22%7D%5B1m%5D%29%29%29
```

Example response:

```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {},
        "value": [
          1585250319.243,
          "0"
        ]
      }
    ]
  }
}
```

This only ever returns a single result, representing the maximum number of critical alerts firing across all Sourcegraph services in the last minute (relative to the time the query executed / the returned unix timestamp `1585250319.243`). The above shows that `"0"` alerts were firing, and if the number was non-zero, it would represent the max number of alerts firing across all services in the last minute.

### "How many critical alerts were firing in the last minute, per service?"

Prometheus query:

```prometheus
sum by (service_name)(max by (service_name,name,description)(max_over_time(alert_count{level="critical",name!=""}[1m])))
```

Example `curl` query:

```sh
curl 'http://$PROMETHEUS_URL/api/v1/query?query=sum%20by%20%28service_name%29%28max%20by%20%28service_name%2Cname%2Cdescription%29%28max_over_time%28alert_count%7Blevel%3D%22critical%22%2Cname%21%3D%22%22%7D%5B1m%5D%29%29%29'
```

Example response:

```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "service_name": "frontend"
        },
        "value": [
          1585250083.874,
          "0"
        ]
      },
      {
        "metric": {
          "service_name": "zoekt-indexserver"
        },
        "value": [
          1585250083.874,
          "0"
        ]
      },
      ...
    ]
  }
}
```

This returns a `result` for each service of Sourcegraph where at least one critical alert is defined and being monitored. For example, the first result (`frontend`) indicates that in the last minute (relative to the time the query executed / the returned unix timestamp `1585250083.874`) that `"0"` alerts for the `frontend` service were firing. If the number was non-zero, it would represent the max number of alerts firing on that service in the last minute.

### "How many critical alerts were firing in the last minute, per defined alert?"

Prometheus query:

```prometheus
max by (service_name,name,description)(max_over_time(alert_count{level="critical",name!=""}[1m]))
```

Example `curl` query:

```sh
curl 'http://$PROMETHEUS_URL/api/v1/query?query=max%20by%20%28service_name%2Cname%2Cdescription%29%28max_over_time%28alert_count%7Blevel%3D%22critical%22%2Cname%21%3D%22%22%7D%5B1m%5D%29%29'
```

Example response:

```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "description": "gitserver: 50+ concurrent command executions (abnormally high load)",
          "name": "high_concurrent_execs",
          "service_name": "gitserver"
        },
        "value": [
          1585249844.475,
          "0"
        ]
      },
      {
        "metric": {
          "description": "gitserver: 100+ concurrent command executions (abnormally high load)",
          "name": "high_concurrent_execs",
          "service_name": "gitserver"
        },
        "value": [
          1585249844.475,
          "0"
        ]
      },
      ...
    ]
  }
}
```

This returns a `result` for each defined critical alert that Sourcegraph is monitoring. For example, the first result (`high_concurrent_execs`) indicates that in the last minute (relative to the time the query executed / the returned unix timestamp `1585249844.475`) that `"0"` alerts were firing. Any value >= 1 here, would indicate that alert has fired in the last minute.
