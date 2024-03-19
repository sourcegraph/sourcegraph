# Troubleshooting guide

This is a guide for investigating errors and performance issues, with the goal of resolving the
issue or generating a high-quality issue report.

**How to use this guide:**

1. Scan through [Specific scenarios](#specific-scenarios) to see if any of these applies to you. If
   any do, follow the instructions in the subsection.
1. Scan through [General scenarios](#general-scenarios) to find which scenario(s) applies to you. If any do,
   follow the instructions to update your instance or collect information for the issue report.
1. If you cannot resolve the issue on your own, file an issue on the [Sourcegraph issue
   tracker](https://github.com/sourcegraph/sourcegraph/issues) with the collected
   information. Enterprise customers may alternatively file a support ticket or email
   support@sourcegraph.com.

> NOTE: This guide assumes you have site admin privileges on the Sourcegraph instance. If you are
> non-admin user, report your error the site admin and have them walk through this guide.

## Specific scenarios

#### Scenario: search *and* code pages take a long time to load

If this is the case, this could indicate high gitserver load. To confirm, take the following steps:

1. [Open Grafana](metrics.md#grafana).
1. **If using Sourcegraph 3.14**: Simply check if either of these alerts are firing:
    - `gitserver: 50+ concurrent command executions (abnormally high load)`
    - `gitserver: echo command execution duration exceeding 1s`
1. **If using an older version of Sourcegraph:**
    - Go to the Sourcegraph Internal > Gitserver rev2 dashboard.
    - Examine the "Echo Duration Seconds" dashboard (tracks the `src_gitserver_echo_duration_seconds`
      metric) and "Commands running concurrently" dashboard (tracks the `src_gitserver_exec_running`
      metric). If either of these is high (> 1s echo duration or 100s simultaneous execs), then this
      indicates gitserver is under heavy load and likely the bottleneck.
1. Confirm your gitserver is not under-provisioned, by e.g. comparing its allocated resources with what the [resource estimator](../deploy/resource_estimator.md) shows.

Solution: set `USE_ENHANCED_LANGUAGE_DETECTION=false` in the Sourcegraph runtime
environment.

#### Scenario: no cloning, syncing, updating or deleting is happening

Observed state: Sourcegraph instance does not react to any updates to code hosts and no cloning is happening.
The cause of this state could be repo-updater queries that are too large for the limits of the running Postgres DB.
One symptom is seeing a line like the one below in the repo-updater logs: 

```text
t=2020-05-28T18:41:02+0000 lvl=eror msg=Syncer error="syncer.sync.store.upsert-repos: delete: driver: bad connection
```

or seeing the same error in the "Code host status panel" (Clicking the cloud icon).

The fix is to increase the memory on Postgres DB which will increase certain Postgres-internal limits and will allow
the queries from repo-updater to go through.

Another cause could be that the `repo-updater` is in a crash loop for some reason. If there are large numbers of repos
to be updated it could be from `Out of memory` errors. A fix here is to increase the memory for `repo-updater` instead.

## General scenarios

This section contains a list of scenarios, each of which contains instructions that include
[actions](#actions) that are appropriate to the scenario. Note that more than one scenario may apply
to a given issue.

#### Scenario: the issue is NOT performance-related and there is a consistent reproduction.

Record the following information in the issue report:

1. Reproduction steps
1. A screenshot of the error page or error message
1. The [output of the browser developer console](#check-browser-console)
1. [Log output](#examine-logs) while reproducing the issue
1. [Sourcegraph configuration](#copy-configuration)
1. When was the most recent update or deployment change applied?

#### Scenario: the issue is NOT performance-related, but it is hard to reproduce.

Without a consistent reproduction, the issue will be harder to diagnose, so we recommend trying to
find a repro if possible. If that isn't possible, file an issue with the following information:

1. Steps the user took before encountering the issue, including as much detail as possible.
  1. Bad example: "User encountered a 502 error when trying to search for something."
  1. Good example: "User encountered a 502 error on the search results page when trying to conduct a
     global search for the following query. On refresh, the search worked with no error. The desired
     result appears in our main repository, which is rather large (takes about a minute to fully
     clone). The issue doesn't reproduce consistently, but we saw two other reports like this around
     2pm PT yesterday, during peak usage hours."
1. [Examine the error rates](#check-error-rates) for any anomalies.
1. If you know the approximate time the issue occurred or if there is a spike in error rate around a
   certain time, [copy the logs](#examine-logs) around that time.
1. Note any pattern in the issue reports. E.g., did users encountering the issue all visit the same
   repository or belong to the same organization. Do site admins encounter the issue or only
   non-admin users?
1. [Sourcegraph configuration](#copy-configuration)
1. When was the most recent update or deployment change applied?

#### Scenario: the issue is performance-related and there is a consistent reproduction

1. Include the reproduction steps in the error report, along with relevant context (e.g., repository
   size).
   1. Bad example: "User did a search and it timed out."
   1. Good example: "User issued the following search query in the following repository. The
      repository is one of our larger repositories (takes about 1 minute to fully clone and the size
      of the `.git` directory is 5GB). The results page took about 60 seconds to load and when it
      finally did, the results was an error message that said 'timeout'".
1. Open the [browser developer network panel](#check-browser-network-panel) and identify slow
   requests.
1. [Use Jaeger](#collect-a-jaeger-trace) to drill down into slow requests and understand which
   components of the request are slow. Remember that many Sourcegraph API requests identify the
   Jaeger trace ID in the `x-trace` HTTP response header, which makes it easy to look up the trace
   corresponding to a particular request.
   1. If Jaeger is unavailable or unreliable, you can collect trace data from [the Go net/trace
   endpoint](#examine-go-net-trace).
1. Copy the [Sourcegraph configuration](#copy-configuration) to the error report.

#### Scenario: the issue is performance-related and there is NOT a consistent reproduction

Without a consistent reproduction, the issue will be harder to diagnose, so we recommend trying to
find a repro if possible. If that isn't possible, try the following:

1. Examine [resource usage](#check-resource-usage), [usage stats](#check-end-user-stats), and [error
   rates](#check-error-rates) over time in Grafana and Prometheus.
  1. Are there spikes in latencies or error rate over time?
  1. Are there spikes in usage or traffic over time that correlate with when the issue is reported.
  1. Are there spikes in memory usage, CPU, or disk usage over time?
1. If you know the approximate time the issue occurred or if there is a suspicious spike in metrics
   around a certain time, [check the logs](#examine-logs) around that time.
1. If the issue is ongoing or if you know the time during which the issue occurred, [search
   Jaeger](#collect-a-jaeger-trace) for long-running request traces in the appropriate time window.
  1. If Jaeger is unavailable, you can alternatively use the Go net/trace endpoint. (You will have
     to scan the traces for each service to look for slow traces.)
1. If tracing points to a specific service as the source of high latency, [examine the
   logs](#examine-logs) and [net/trace info](#examine-go-net-trace) for that service.

#### Scenario: multiple actions are slow or Sourcegraph as a whole feels sluggish

If Sourcegraph feels sluggish overall, the likely culprit is resource allocation.

> NOTE: some of these recommendations involve increasing the replica or shard count of individual
> services. This is only possible when Sourcegraph is deployed into a Kubernetes cluster.

1. [Examine memory, CPU, and disk usage metrics](#check-resource-usage).
1. If the metrics indicate high resource consumption, adjust the resource allocation higher.
1. If metrics are unavailable or inaccessible, here is a rough correspondence between end-user
   slowness and the services that are usually the culprit:
   1. Global search (i.e., no repository scope is specified) results page takes a long time to load.
     1. Increase indexed-search memory limit or CPU limit. The number of indexed-search
        shards can also be increased if using Sourcegraph on Kubernetes.
   1. Search results show up quickly, but code snippets take awhile to populate. File contents take
      awhile to load.
     1. Increase gitserver memory usage. Gitserver memory may be the bottleneck, especially if there
        are many repositories or repositories are large.
     1. Increase number of gitserver shards. This can help if memory is the bottleneck. It can also
        help if there are too many repositories per shard. Gitserver "shells out" to `git` for every
        repository data request, so a high volume of user traffic that generates many simultaneous
        requests for many repositories can lead to a spike in Linux process exec latency.
     1. Increase memory and CPU limit of syntect-server. This helps if syntax highlighting is the
        bottleneck.
   1. Multiple UI pages take awhile to load.
     1. Increase frontend CPU and memory limit.
   1. Searches show intermittent HTTP 502 errors or timeouts, possibly concurrent with frontend
      container restarts.
     1. Increase frontend memory and CPU. This may indicate the frontend is running out of memory
        when loading search results. This can be a problem when dealing with large monorepos.
1. If it is unclear which service is underallocated, [examine Jaeger](#collect-a-jaeger-trace) to
   identify long-running traces and see which services take up the most time.
   1. Alternatively, you can use the [Go net/trace endpoint](#examine-go-net-trace) to pull trace
      data.

#### Scenario: Prometheus scraping metrics outside Sourcegraph Kubernetes namespace 

If you are seeing cAdvisor metrics from a namespace outside of the one Sourcegraph is currently deployed into. 

1. Uncomment our namespaced Prometheus [cAdvisor configuration](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/deploy-sourcegraph%24+cAdvisor-specific+customization+&patternType=literal)
2. Apply this configuration and restart Prometheus

_Note:_ This is unneeded if you are using the 'namespaced' overlay

#### Scenario: search timeouts

If your users are experiencing search timeouts or search performance issues, please perform the following steps:

1. Try appending variations of `index:only`, `timeout:60s` and `count:999999` to the search query to see if it is still slow.
1. [Access Grafana directly](metrics.md#accessing-grafana-directly).
1. Select the **+** icon on the left-hand side, then choose **Import**.
1. Paste [this JSON](https://gist.githubusercontent.com/slimsag/3fcc134f5ce09728188b94b463131527/raw/f8b545f4ce14b0c30a93f05cd1ee469594957a2c/sourcegraph-debug-search-timeouts.json) into the input and click **Load**.
1. Once the dashboard appears, include screenshots of **the entire** dashboard in the issue report.
1. Include the logs of `zoekt-webserver` container in the `indexed-search` pods. If you are using single Docker container enable [debug logs](index.md#Logs) first.

#### Scenario: zoekt-webserver is in a `CrashloopBackOff` and `err cannot allocate memory`

Sourcegraph uses a mmap to store its indices. The default operating system limits on mmap counts may to be too low, which may result in out of memory exceptions.
If you are seeing this error on large scale deployments with a lot repos to be indexed, please use the following steps to verify the source of the errors:

1. Ensure pods are not actually running out of allocated memory and being OOMKilled by running (if they are, then you should give them more memory and the below will not help you):

        
        kubectl top pod indexed-search-<pod_number>
        
        kubectl describe nodes 


1. On the host operating system execute `sudo sysctl -n vm.max_map_count`.

        $ sysctl -n vm.max_map_count
        65530
1. Calculate the number of repos in your deployment divided by the number of `indexed-search` replicas. For example:
    
    * 250,000 repositories / 2 indexed-search repliacas = 125,000 repos to index per replica. 
1. If the `vm.max_map_count` is lower than the result of the above calculation. Adjust the `vm.max_map_count` by executing:

        sudo sysctl -w vm.max_map_count=262144
1. Verify the change.

        $ sudo sysctl -n vm.max_map_count
        262144
1. Ensure the change will persist a system reboot by updating the `vm.max_map_count` setting in `/etc/sysctl.conf`.

#### Scenario: zoekt-webserver is restarting due to watchdog

zoekt-webserver has a built in watchdog which ensures it can respond to search requests. If the watchdog fails, it will panic causing the process to exit with a non-zero exit code. This is like a Kubernetes health check, but works across all our deployment environments.

By default the watchdog runs every 30s. If the watchdog fails 3 consecutive times (with a 30s sleep in-between) it will trigger the panic. This is usually indicative a server which is consistently overloaded. It is recommended to increase the CPU quota assigned to it or horizontally scale to more replicas.

From Sourcegraph 3.22 you can configure the watchdog via environment variables:

- `ZOEKT_WATCHDOG_TICK` :: Duration of how often it runs. (default 30s)
- `ZOEKT_WATCHDOG_ERRORS` :: Consecutive error count before exit. (default 3)

If either is 0 the watchdog is disabled.

You can further diagnose an overloaded zoekt-webserver via watchdog logs or metrics. See log messages mentioning watchdog, or view the following metrics in grafana:

- `zoekt_webserver_watchdog_errors` :: The current error count for zoekt
  watchdog.
- `zoekt_webserver_watchdog_total` :: The total number of requests done by
  zoekt watchdog.
- `zoekt_webserver_watchdog_errors_total` :: The total number of errors from
  zoekt watchdog.


## Actions

This section contains various actions that can be taken to collect information or update Sourcegraph
in order to resolve an error or performance issue. You should typically not read this section
directly, but start with the [General scenarios](#general-scenarios) section to determine which actions are
appropriate.

### Check browser console

Open the browser JavaScript console (right-click in the browser > Inspect to open developer tools,
then click the `Console` tab).

### Check browser network panel

Open the browser developer network page (right-click in the browser > Inspect to open developer tools,
then click the `Network` tab).

If you are new to the network page, check out this [great introduction to the Chrome developer tools
Network panel](https://developers.google.com/web/tools/chrome-devtools/network).

* Check the waterfall diagram at the top and the Waterfall column in the list of network requests to
  quickly identify high-latency requests.
* Clicking on a request will open up a panel that provides additional details about the request.
  * If a GraphQL request is taking a long time, you should obtain its Jaeger trace ID by inspecting
    the Headers tab of this panel and finding the `X-Trace` or `x-trace` response header value. Once
    you've obtained this trace ID, [look it up in Jaeger](#collect-a-jaeger-trace).
* You can check `Preserve log` to preserve the list of requests across page loads and reloads.

### Check resource usage

[Access Prometheus](metrics.md#accessing-prometheus) and examine the following metrics:

**Memory:** `process_resident_memory_bytes` is a gauge that tracks memory usage per backend process.

* Example: `process_resident_memory_bytes{app="indexed-search"}` shows memory usage for each
  indexed-search instance.

**CPU:** `process_cpu_seconds_total` is a counter that tracks cumulative CPU seconds used .

* Example: `rate(process_cpu_seconds_total{app="sourcegraph-frontend"}[1m])` shows average CPU usage
  for each sourcegraph-frontend instance over the last minute.

**Disk:** `gitserver_disk_free_percent` is a gauge that tracks free disk space on gitserver.

### Check end-user stats

Go to `/site-admin/usage-statistics` to view daily, weekly, and monthly user statistics.

To drill down (e.g., into sub-daily traffic, visits per page type, latencies, etc.), [access
Grafana](metrics.md#grafana) and visit the Sourcegraph Internal > HTTP dashboard page, which
includes the following panels:

* QPS by Status Code
* QPS by URL Route
* P90 Request Duration (request latency at the 90th percentile)

Grafana contains ready-made dashboards derived from Prometheus metrics. Any chart in Grafana
mentioned here can be viewed in Prometheus by clicking the dropdown menu next to the Grafana panel
title > Edit > copying the expression in the Metrics field.

### Check error rates

[Access Grafana](metrics.md#grafana) and view the following charts:

* Folder: Sourcegraph Internal > Dashboard: HTTP > Chart: QPS by Status Code
  * This shows request rates by HTTP status code for end-user requests.
* Folder: General
  * This contains dashboards for each core service in Sourcegraph. Examine each for high-level
    metrics important to the health of each service.

### Collect a Jaeger trace

If you are looking for the trace associated with a specific request,

* [Find the trace ID in the HTTP response in the browser developer tools "Network" tab](#check-browser-network-panel).
* [Access Jaeger](tracing.md#accessing-jaeger) and look up the trace ID.

If you do not have a specific request or cannot find the trace ID,

* [Access Jaeger](tracing.md#accessing-jaeger).
* Search for a matching span by setting the appropriate fields in the sidebar.

2 ways: start with a span ID, or manually locate your span by searching the Jaeger GUI

### Examine logs

If you are using the single Docker container or Docker Compose deployment option, logs are printed
to `stdout` and `stderr`. You should be able to access these using your infrastructure provider's
standard log viewing mechanism.

If you are using Kubernetes,

* Retrieve logs with `kubectl logs $POD_ID`.
* Tail logs with `kubectl logs -f $POD_ID`.
* If a pod container died, you can access the previous container logs with `kubectl logs -p
  $POD_ID`. This can be useful for diagnosing why a container crashed.
* You can tail logs for all pods associated with a given deployment: `kubectl logs -f
  deployment/sourcegraph-frontend --container=frontend --since=10m`


### Examine Go net/trace

Each core service has an endpoint which displays traces using Go's
[net/trace](https://pkg.go.dev/golang.org/x/net/trace) package.

To access this data,

1. First ensure you are logged in as a site admin.
1. Go to the URL path `/-/debug`. This page should show a list of links with the names of each core
   service (e.g., `frontend`, `gitserver`, etc.)
1. Click on the service you'd like to examine.
1. Click "Requests`. This brings you to a page where you can view traces for that service.
  * You can filter to traces by duration or error state.
  * You can show histograms of durations by minute, hour, or in total (since the process started)

On older versions of Sourcegraph on Kubernetes, the `/-/debug` URL path may be inaccessible. If this
is the case, you'll need to forward port 6060 on the main container of a given pod to access its
traces. For example, to access to traces of the first gitserver shard,

1. `kubectl port-forward gitserver-0 6060`
1. Go to `http://localhost:6060` in your browser, and click on "Requests".

### Copy configuration

Go the the URL path `/site-admin/report-bug` to obtain an all-in-one text box of all Sourcegraph
configuration (which includes site configuration, code host configuration, and global
settings). This lets you easily copy all configuration to an issue report (NOTE: remember to redact
any secrets).

### Collect instance stats

The following statistics are useful background context when reporting a performance issue:

* Number of repositories (can be found on the `/site-admin/repositories` page, search for "repositories total")
* Size distribution of repositories (e.g., are there one or more large "monorepos" that contain most of the code?)
* Number of users and daily usage stats from `/site-admin/usage-statistics`
