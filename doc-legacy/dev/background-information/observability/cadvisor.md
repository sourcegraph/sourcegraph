# Sourcegraph cAdvisor

We ship a custom [cAdvisor](https://github.com/google/cadvisor) image as part of the standard Sourcegraph Kubernetes and docker-compose distribution.
cAdvisor exports container monitoring metrics scraped by [Prometheus](./prometheus.md) and visualized in [Grafana](./grafana.md).

The image is defined in [`docker-images/cadvisor`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/docker-images/cadvisor).

## Monitoring

Monitoring on cAdvisor metrics is defined in the [monitoring generator](./monitoring-generator.md).
cAdvisor observables are generally defined as [shared observables](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/monitoring/definitions/shared).

When adding monitoring on cAdvisor metrics, please ensure that the [metric can be identified](#identifying-containers) (if not, it is likely the [metric is not supported](#available-metrics)).

## Identifying containers

How relevant containers are identified from exported cAdvisor metrics is documented in [`CadvisorNameMatcher`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+type:symbol+CadvisorNameMatcher&patternType=literal), which generates the label matcher for [monitoring observables](#monitoring).

Because cAdvisor run on a *machine* and exports *container* metrics, standard strategies for identifying what container a metric belongs to (such as Prometheus scrape target labels) cannot be used, because all the metrics look like they belong to cAdvisor.
Making things complicated is how containers are identified on various environments (namely Kubernetes and docker-compose) varies, sometimes due to characteristics of the environments and sometimes due to naming inconsistencies within Sourcegraph.
Variations in how cAdvisor generates the `name` label it provides also makes things difficult (in some environments, it cannot generate one at all!).
This means that cAdvisor can pick up non-Sourcegraph metrics, which can be problematic—see [known issues](#known-issues) for more details and current workarounds.

## Available metrics

Exported metrics are documented in the [cAdvisor Prometheus metrics list](https://github.com/google/cadvisor/blob/master/docs/storage/prometheus.md#prometheus-container-metrics).
In the list, the column `-disable_metrics parameter` indicates the "group" the metric belongs in.

Container runtime and deployment environment compatability for various metrics seem to be grouped by these groups—before using a metric, ensure that the metric is supported in all relevant environments (for example, both Docker and `containerd` container runtimes).
Support is generally poorly documented, but a search through the [cAdvisor repository issues](https://github.com/google/cadvisor/issues) might provide some hints.

## Known issues

- cAdvisor can pick up non-Sourcegraph metrics (can cause issues with [our built-in observability](../../../admin/observability/index.md) and, in extreme cases, cause cAdvisor and Prometheus performance issues if the number of metrics is very large) due to how we currently [identitify containers](#identifying-containers): [sourcegraph#17365](https://github.com/sourcegraph/sourcegraph/issues/17365) ([Kubernetes workaround](../../../admin/deploy/kubernetes/configure.md#filtering-cadvisor-metrics))
- Metrics issues
  - `disk` metrics are not available in `containerd`: [cadvisor#2785](https://github.com/google/cadvisor/issues/2785)
  - `diskIO` metrics do not seem to be available in Kubernetes: [sourcegraph#12163](https://github.com/sourcegraph/sourcegraph/issues/12163)
- When using a Kustomize non-privileged overlay in a deployment, cAdvisor is disabled by default and hence cannot scrape container metrics for visualization in Grafana. cAdvisor requires elevated privileges to collect this data and hence will not work with this overlay.
