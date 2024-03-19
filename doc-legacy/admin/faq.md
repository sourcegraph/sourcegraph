# Administration FAQ

## How does Sourcegraph store repositories on disk?

Sourcegraph stores bare Git repositories (without a working tree), which is a complete mirror of the repository on your code host.

If you are keen for more details on what bare Git repositories are, [check out this discussion on StackOverflow](https://stackoverflow.com/q/5540883).

The directories should contain just a few files and directories, namely: HEAD, config, description, hooks, info, objects, packed-refs, refs

## Does Sourcegraph support svn?

For Subversion and other non-Git code hosts, the recommended way to make these accessible in
Sourcegraph is through [`src-expose`](external_service/other.md#experimental-src-expose).

Alternatively, you can use [`git-svn`](https://git-scm.com/docs/git-svn) or
[`svg2git`](https://github.com/svn-all-fast-export/svn2git) to convert Subversion repositories to
Git repositories. Unlike `src-expose`, this will preserve commit history, but is generally much
slower.

## How do I access Sourcegraph if my authentication provider experiences an outage?

If you are using an external authentication provider and the provider experiences an outage, users
will be unable to sign into Sourcegraph. A site administrator can configure an alternate sign-in
method by modifying the `auth.providers` field in site configuration. However, the site
administrator may themselves be unable to sign in. If this is the case, then a site administrator
can update the configuration if they have direct `docker exec` or `kubectl exec` access to the
Sourcegraph instance. Follow the [instructions to update the site config if the web UI is
inaccessible](config/site_config.md#editing-your-site-configuration-if-you-cannot-access-the-web-ui).

## How do I set up redirect URLs in Sourcegraph?

Sometimes URLs in Sourcegraph may change. For example, if a code host configuration is
updated to use a different `repositoryPathPattern`, this will change the repository URLs on
Sourcegraph. Users may wish to preserve links to the old URLs, and this requires adding redirects.

We recommend configuring redirects in a reverse proxy. If you are running Sourcegraph as a single
Docker image, you can deploy a reverse proxy such as [Caddy](https://caddyserver.com/) or
[NGINX](https://www.nginx.com) in front of it. Refer to the
[Caddy](https://github.com/caddyserver/caddy/wiki/v2:-Documentation#rewrite) or
[NGINX](https://www.nginx.com/blog/creating-nginx-rewrite-rules/) documentation for URL rewrites.

If you are running Sourcegraph as a Kubernetes cluster, you have two additional options:

1. If you are using [NGINX
   ingress](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/docs/configure.md#ingress-controller)
   (`kubectl get ingress | grep sourcegraph-frontend`), modify
   [`sourcegraph-frontend.Ingress.yaml`](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/base/frontend/sourcegraph-frontend.Ingress.yaml)
   by [adding a rewrite rule](https://kubernetes.github.io/ingress-nginx/examples/rewrite/).
   
## What external HTTP checks are configured?

We leave the choice of which external HTTP check monitor up to our users. What we provide out-of-the-box is extensive metrics monitoring through Prometheus and Grafana, with builtin alert thresholds and dashboards, and Kubernetes/Docker HTTP health checks which verify that the server is running. Sourcegraph's frontend has a default health check at
https://$SOURCEGRAPH_BASE_URL/healthz. Some users choose to set up their own external HTTP health checker which tests if the homepage loads, a repository page loads, and if a search GraphQL request returns successfully. 

## Monitoring

Please visit our [Observability Docs](https://docs.sourcegraph.com/admin/observability) for more in-depth information about observability in Sourcegraph.

### What should I look at when my instance is having performance issues?

Sourcegraph comes with built-in monitoring in the form of [Grafana](https://docs.sourcegraph.com/admin/observability/metrics#grafana), connected to [Prometheus](https://docs.sourcegraph.com/admin/observability/metrics#prometheus) for metrics and alerting.

Generally, Grafana should be the first stop you make when experiencing a system performance issue. From there you can look for system alerts or metrics that would provide you with more insights on what’s causing the performance issue. You can learn more about [accessing Grafana here](https://docs.sourcegraph.com/admin/observability/metrics#grafana).

### What are the key values/alerts to look for when looking at the Grafana Dashboard?

Please refer to the [Dashboards](https://docs.sourcegraph.com/admin/observability/metrics#dashboards) guide for more on how to use our Grafana dashboards.

Please refer to [Understanding alerts](https://docs.sourcegraph.com/admin/observability/alerting#understanding-alerts) for examples and suggested actions for alerts.

### How do I know when more resources are needed for a specified service?

All resource dashboards contain a section called `Provisioning indicators` that provide information about the current resource usage of containers. These can be used to determine if a scale-up is needed ([example panel](https://docs.sourcegraph.com/admin/observability/dashboards#frontend-provisioning-container-cpu-usage-long-term)).

More information on each available panel in the dashboards is available in the [Dashboards reference](https://docs.sourcegraph.com/admin/observability/dashboards).

### What does this `<ALERT-MESSAGE>` mean?

See [Alert solutions](https://docs.sourcegraph.com/admin/observability/alerts) to learn about each alert and their possible solutions.

### What’s the threshold for each resource?

All resources dashboards contain a section called `Container monitoring` that indicate thresholds at which alerts will fire for each resource ([example alert](https://docs.sourcegraph.com/admin/observability/alerts#frontend-container-cpu-usage)).

More information on each available panel in the dashboards is available in the [Dashboards reference](https://docs.sourcegraph.com/admin/observability/dashboards).

### How much resources should I add after receiving alerts about running out of resources?

You should make the decision based on the metrics from the relevant Grafana dashboard linked in each alert.
  
### What are some of the important alerts that I should be aware of?

We recommend paying closer attention to [critical alerts](https://docs.sourcegraph.com/admin/observability/alerting#understanding-alerts).

### How do I set up alerts?

Please refer to our guide on [setting up alerting](https://docs.sourcegraph.com/admin/observability/alerting#setting-up-alerting).

### How do I create a custom alert?

Creating a custom alert is not recommended and currently not supported by Sourcegraph. However, please provide feedback on the monitoring dashboards and alerts if you find anything could be improved via our issue tracker.

More advanced users can also refer to [our FAQ item about custom consumption of Sourcegraph metrics](#can-i-consume-sourcegraph-s-metrics-in-my-own-monitoring-system-datadog-new-relic-etc).

### Can I consume Sourcegraph's metrics in my own monitoring system (Datadog, New Relic, etc.)?

Sourcegraph provides [high-level alerting metrics](./observability/metrics.md#high-level-alerting-metrics) which you can integrate into your own monitoring system—see the [alerting custom consumption guide](./observability/alerting_custom_consumption.md) for more details.

While it is technically possible to consume all of Sourcegraph's metrics in an external system, our recommendation is to utilize the builtin monitoring tools and configure Sourcegraph to [send alerts to your own PagerDuty, Slack, email, etc.](./observability/alerting.md). Metrics and thresholds can change with each release, therefore manually defining the alerts required to monitor Sourcegraph's health is not recommended. Sourcegraph automatically updates the dashboards and alerts on each release to ensure the displayed information is up-to-date.

Other monitoring systems that support Prometheus scraping (for example, Datadog and New Relic) or [Prometheus federation](https://prometheus.io/docs/prometheus/latest/federation/) can be configured to federate Sourcegraph's [high-level alerting metrics](./observability/metrics.md#high-level-alerting-metrics). For information on how to configure those systems, please check your provider's documentation.

### I am getting "Error: Cluster information not available" in the Instrumentation page, what should I do?

This error is expected if your instance was not [deployed with Kubernetes](./deploy/kubernetes/index.md). The Instrumentation page is currently only available for Kubernetes instances.

## Troubleshooting

Please refer to our [dedicated troubleshooting page](troubleshooting.md).
