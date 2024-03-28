# Pings service

This directory contains source code for the [Pings service](https://handbook.sourcegraph.com/departments/engineering/teams/core-services/managed-services/pings/). This document describes details for local development of the service.

The following command should boot up a running service:

```shell
$ sg run pings
...
[          pings] INFO pings shared/main.go:47 service ready {"address": ":10086"}
```

The default configuration can be found in the [`sg.config.yaml`](https://sourcegraph.sourcegraph.com/search?q=context%3Aglobal+repo%3A%5Egithub%5C.com%2Fsourcegraph%2Fsourcegraph%24%40main+f%3Asg.config.yaml+content%3A%22pings%3A%22&patternType=standard&sm=1&groupBy=path) file.

To send a test request:

```shell
curl http://localhost:10086/updates?site=df0eed23-0e8c-4721-9849-147d20d59911&version=0.0.0
```

A "200 OK" status code is expected.

> [!NOTE]
> To test a more realistic request:
>
> 1. Grab the ping request from `/site-admin/pings` of any Sourcegraph instance and save to a file named `ping.json`
> 1. Do `curl -X POST -H "Content-Type: application/json" -d @ping.json http://localhost:10086/updates`

This would send an entry to the GCP Pub/Sub topic `server-update-checks-test`, and once processed, should be available in the BigQuery table [`sourcegraph_analytics.update_checks_test`](https://console.cloud.google.com/bigquery?project=telligentsourcegraph&ws=!1m5!1m4!4m3!1stelligentsourcegraph!2ssourcegraph_analytics!3supdate_checks_test). You need to make the [Entitle request](https://app.entitle.io/request?targetType=resource&duration=10800&justification=Test%20pings%20service&integrationId=52e29e01-d551-4186-88a3-65ff4f28b8c3&resourceId=53946931-0002-469c-9b5f-c5af70bd1ffe&roleId=ea1606fd-2302-487d-83eb-d1f140478416&grantMethodId=ea1606fd-2302-487d-83eb-d1f140478416) to access this page.

When there is an error processing the Pub/Sub message, the entry would be instead sent to another BigQuery table [`sourcegraph_analytics.update_checks_test_error_records`](https://console.cloud.google.com/bigquery?project=telligentsourcegraph&ws=!1m5!1m4!4m3!1stelligentsourcegraph!2ssourcegraph_analytics!3supdate_checks_test_error_records).
Hello World
