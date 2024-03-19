package definitions

import (
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/monitoring/definitions/shared"
	"github.com/sourcegraph/sourcegraph/monitoring/monitoring"
)

func GitServer() *monitoring.Dashboard {
	const (
		containerName   = "gitserver"
		grpcServiceName = "gitserver.v1.GitserverService"
	)

	scrapeJobRegex := fmt.Sprintf(".*%s", containerName)

	gitserverHighMemoryNoAlertTransformer := func(observable shared.Observable) shared.Observable {
		return observable.WithNoAlerts(`Git Server is expected to use up all the memory it is provided.`)
	}

	provisioningIndicatorsOptions := &shared.ContainerProvisioningIndicatorsGroupOptions{
		LongTermMemoryUsage:  gitserverHighMemoryNoAlertTransformer,
		ShortTermMemoryUsage: gitserverHighMemoryNoAlertTransformer,
	}

	grpcMethodVariable := shared.GRPCMethodVariable("gitserver", grpcServiceName)

	return &monitoring.Dashboard{
		Name:        "gitserver",
		Title:       "Git Server",
		Description: "Stores, manages, and operates Git repositories.",
		Variables: []monitoring.ContainerVariable{
			{
				Label: "Shard",
				Name:  "shard",
				OptionsLabelValues: monitoring.ContainerVariableOptionsLabelValues{
					Query:         "src_gitserver_exec_running",
					LabelName:     "instance",
					ExampleOption: "gitserver-0:6060",
				},
				Multi: true,
			},
			grpcMethodVariable,
		},
		Groups: []monitoring.Group{
			{
				Title: "General",
				Rows: []monitoring.Row{
					{
						{
							Name:        "go_routines",
							Description: "go routines",
							Query:       "go_goroutines{app=\"gitserver\", instance=~`${shard:regex}`}",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{instance}}").
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner: monitoring.ObservableOwnerSource,
							Interpretation: `
						`,
						},
					},
					{
						{
							Name:        "cpu_throttling_time",
							Description: "container CPU throttling time %",
							Query:       "sum by (container_label_io_kubernetes_pod_name) ((rate(container_cpu_cfs_throttled_periods_total{container_label_io_kubernetes_container_name=\"gitserver\", container_label_io_kubernetes_pod_name=~`${shard:regex}`}[5m]) / rate(container_cpu_cfs_periods_total{container_label_io_kubernetes_container_name=\"gitserver\", container_label_io_kubernetes_pod_name=~`${shard:regex}`}[5m])) * 100)",
							Warning:     monitoring.Alert().GreaterOrEqual(75).For(2 * time.Minute),
							Critical:    monitoring.Alert().GreaterOrEqual(90).For(5 * time.Minute),
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}}").
								Unit(monitoring.Percentage).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner: monitoring.ObservableOwnerSource,
							Interpretation: `
								- A high value indicates that the container is spending too much time waiting for CPU cycles.
						`,
							NextSteps: `
								- Consider increasing the CPU limit for the container.
						`,
						},
						{
							Name:        "cpu_usage_seconds",
							Description: "cpu usage seconds",
							Query:       "sum by (container_label_io_kubernetes_pod_name) (rate(container_cpu_usage_seconds_total{container_label_io_kubernetes_container_name=\"gitserver\", container_label_io_kubernetes_pod_name=~`${shard:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}}").
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner: monitoring.ObservableOwnerSource,
							Interpretation: `
								- This value should not exceed 75% of the CPU limit over a longer period of time.
								- We cannot alert on this as we don't know the resource allocation.

								- If this value is high for a longer time, consider increasing the CPU limit for the container.
						`,
						},
					},
					{
						{
							Name:        "disk_space_remaining",
							Description: "disk space remaining",
							Query:       "(src_gitserver_disk_space_available{instance=~`${shard:regex}`} / src_gitserver_disk_space_total{instance=~`${shard:regex}`}) * 100",
							// Warning alert when we have disk space remaining that is
							// approaching the default SRC_REPOS_DESIRED_PERCENT_FREE
							Warning: monitoring.Alert().Less(15),
							// Critical alert when we have less space remaining than the
							// default SRC_REPOS_DESIRED_PERCENT_FREE some amount of time.
							// This means that gitserver should be evicting repos, but it's
							// either filling up faster than it can evict, or there is an
							// issue with the janitor job.
							Critical: monitoring.Alert().Less(10).For(10 * time.Minute),
							Panel: monitoring.Panel().LegendFormat("{{instance}}").
								Unit(monitoring.Percentage).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner: monitoring.ObservableOwnerSource,
							Interpretation: `
								Indicates disk space remaining for each gitserver instance, which is used to determine when to start evicting least-used repository clones from disk (default 10%, configured by 'SRC_REPOS_DESIRED_PERCENT_FREE').
							`,
							NextSteps: `
								- On a warning alert, you may want to provision more disk space: Disk pressure may result in decreased performance, users having to wait for repositories to clone, etc.
								- On a critical alert, you need to provision more disk space. Running out of disk space will result in decreased performance, or complete service outage.
							`,
						},
					},
					{
						{
							Name:        "running_git_commands",
							Description: "git commands running on each gitserver instance",
							Query:       "sum by (instance, cmd) (src_gitserver_exec_running{instance=~`${shard:regex}`})",
							Warning:     monitoring.Alert().GreaterOrEqual(50).For(2 * time.Minute),
							Critical:    monitoring.Alert().GreaterOrEqual(100).For(5 * time.Minute),
							Panel: monitoring.Panel().LegendFormat("{{instance}} {{cmd}}").
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner: monitoring.ObservableOwnerSource,
							Interpretation: `
								A high value signals load.
							`,
							NextSteps: `
								- **Check if the problem may be an intermittent and temporary peak** using the "Container monitoring" section at the bottom of the Git Server dashboard.
								- **Single container deployments:** Consider upgrading to a [Docker Compose deployment](../deploy/docker-compose/migrate.md) which offers better scalability and resource isolation.
								- **Kubernetes and Docker Compose:** Check that you are running a similar number of git server replicas and that their CPU/memory limits are allocated according to what is shown in the [Sourcegraph resource estimator](../deploy/resource_estimator.md).
							`,
						},
						{
							Name:           "git_commands_received",
							Description:    "rate of git commands received",
							Query:          "sum by (cmd) (rate(src_gitserver_exec_duration_seconds_count{instance=~`${shard:regex}`}[5m]))",
							NoAlert:        true,
							Interpretation: "per second rate per command",
							Panel: monitoring.Panel().LegendFormat("{{cmd}}").
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner: monitoring.ObservableOwnerSource,
						},
					},
					{
						{
							Name:        "echo_command_duration_test",
							Description: "echo test command duration",
							Query:       "max(src_gitserver_echo_duration_seconds)",
							Warning:     monitoring.Alert().GreaterOrEqual(0.020).For(30 * time.Second),
							Critical:    monitoring.Alert().GreaterOrEqual(1),
							Panel:       monitoring.Panel().LegendFormat("running commands").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSource,
							Interpretation: `
							A high value here likely indicates a problem, especially if consistently high.
							You can query for individual commands using 'sum by (cmd)(src_gitserver_exec_running)' in Grafana ('/-/debug/grafana') to see if a specific Git Server command might be spiking in frequency.
							On a healthy linux node, this number should be less than 5ms, ideally closer to 2ms.
							A high process spawning overhead will affect latency of gitserver APIs.

							Various factors can affect process spawning overhead, but the most common we've seen is IOPS contention on the underlying volume, or high CPU throttling.
							`,
							NextSteps: `
								- **Single container deployments:** Upgrade to a [Docker Compose deployment](../deploy/docker-compose/migrate.md) which offers better scalability and resource isolation.
								- **Kubernetes and Docker Compose:** Check that you are running a similar number of git server replicas and that their CPU/memory limits are allocated according to what is shown in the [Sourcegraph resource estimator](../deploy/resource_estimator.md).
								- If your persistent volume is slow, you may want to provision more IOPS, usually by increasing the volume size.
							`,
						},
						{
							Name:        "repo_corrupted",
							Description: "number of times a repo corruption has been identified",
							Query:       `sum(rate(src_gitserver_repo_corrupted[5m]))`,
							Critical:    monitoring.Alert().Greater(0),
							Panel:       monitoring.Panel().LegendFormat("corruption events").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							Interpretation: `
								A non-null value here indicates that a problem has been detected with the gitserver repository storage.
								Repository corruptions are never expected. This is a real issue. Gitserver should try to recover from them
								by recloning repositories, but this may take a while depending on repo size.
							`,
							NextSteps: `
								- Check the corruption logs for details. gitserver_repos.corruption_logs contains more information.
							`,
						},
					},
					{
						{
							Name:        "repository_clone_queue_size",
							Description: "repository clone queue size",
							Query:       "sum(src_gitserver_clone_queue)",
							Warning:     monitoring.Alert().GreaterOrEqual(25),
							Panel:       monitoring.Panel().LegendFormat("queue size"),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps: `
								- **If you just added several repositories**, the warning may be expected.
								- **Check which repositories need cloning**, by visiting e.g. https://sourcegraph.example.com/site-admin/repositories?filter=not-cloned
							`,
						},
						{
							Name:          "src_gitserver_repo_count",
							Description:   "number of repositories on gitserver",
							Query:         "src_gitserver_repo_count",
							NoAlert:       true,
							Panel:         monitoring.Panel().LegendFormat("repo count"),
							Owner:         monitoring.ObservableOwnerSource,
							MultiInstance: true,
							Interpretation: `
								This metric is only for informational purposes. It indicates the total number of repositories on gitserver.

								It does not indicate any problems with the instance.
							`,
						},
					},
				},
			},
			{
				Title:  "Gitservice for internal cloning",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:           "aggregate_gitservice_request_duration",
							Description:    "95th percentile gitservice request duration aggregate",
							Query:          "histogram_quantile(0.95, sum(rate(src_gitserver_gitservice_duration_seconds_bucket{type=`gitserver`, error=`false`}[5m])) by (le))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{le}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `A high value means any internal service trying to clone a repo from gitserver is slowed down.`,
						},
						{
							Name:           "gitservice_request_duration",
							Description:    "95th percentile gitservice request duration per shard",
							Query:          "histogram_quantile(0.95, sum(rate(src_gitserver_gitservice_duration_seconds_bucket{type=`gitserver`, error=`false`, instance=~`${shard:regex}`}[5m])) by (le, instance))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `A high value means any internal service trying to clone a repo from gitserver is slowed down.`,
						},
					},
					{
						{
							Name:           "aggregate_gitservice_error_request_duration",
							Description:    "95th percentile gitservice error request duration aggregate",
							Query:          "histogram_quantile(0.95, sum(rate(src_gitserver_gitservice_duration_seconds_bucket{type=`gitserver`, error=`true`}[5m])) by (le))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{le}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `95th percentile gitservice error request duration aggregate`,
						},
						{
							Name:           "gitservice_request_duration",
							Description:    "95th percentile gitservice error request duration per shard",
							Query:          "histogram_quantile(0.95, sum(rate(src_gitserver_gitservice_duration_seconds_bucket{type=`gitserver`, error=`true`, instance=~`${shard:regex}`}[5m])) by (le, instance))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `95th percentile gitservice error request duration per shard`,
						},
					},
					{
						{
							Name:           "aggregate_gitservice_request_rate",
							Description:    "aggregate gitservice request rate",
							Query:          "sum(rate(src_gitserver_gitservice_duration_seconds_count{type=`gitserver`, error=`false`}[5m]))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("gitservers").Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `Aggregate gitservice request rate`,
						},
						{
							Name:           "gitservice_request_rate",
							Description:    "gitservice request rate per shard",
							Query:          "sum(rate(src_gitserver_gitservice_duration_seconds_count{type=`gitserver`, error=`false`, instance=~`${shard:regex}`}[5m]))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `Per shard gitservice request rate`,
						},
					},
					{
						{
							Name:           "aggregate_gitservice_request_error_rate",
							Description:    "aggregate gitservice request error rate",
							Query:          "sum(rate(src_gitserver_gitservice_duration_seconds_count{type=`gitserver`, error=`true`}[5m]))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("gitservers").Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `Aggregate gitservice request error rate`,
						},
						{
							Name:           "gitservice_request_error_rate",
							Description:    "gitservice request error rate per shard",
							Query:          "sum(rate(src_gitserver_gitservice_duration_seconds_count{type=`gitserver`, error=`true`, instance=~`${shard:regex}`}[5m]))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `Per shard gitservice request error rate`,
						},
					},
					{
						{
							Name:           "aggregate_gitservice_requests_running",
							Description:    "aggregate gitservice requests running",
							Query:          "sum(src_gitserver_gitservice_running{type=`gitserver`})",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("gitservers").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `Aggregate gitservice requests running`,
						},
						{
							Name:           "gitservice_requests_running",
							Description:    "gitservice requests running per shard",
							Query:          "sum(src_gitserver_gitservice_running{type=`gitserver`, instance=~`${shard:regex}`}) by (instance)",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: `Per shard gitservice requests running`,
						},
					},
				},
			},
			{
				Title:  "Gitserver cleanup jobs",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:           "janitor_running",
							Description:    "janitor process is running",
							Query:          "max by (instance) (src_gitserver_janitor_running{instance=~`${shard:regex}`})",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("janitor process running").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "1, if the janitor process is currently running",
						},
					},
					{
						{
							Name:           "janitor_job_duration",
							Description:    "95th percentile job run duration",
							Query:          "histogram_quantile(0.95, sum(rate(src_gitserver_janitor_job_duration_seconds_bucket{instance=~`${shard:regex}`}[5m])) by (le, job_name))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{job_name}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "95th percentile job run duration",
						},
					},
					{
						{
							Name:           "janitor_job_failures",
							Description:    "failures over 5m (by job)",
							Query:          "sum by (job_name) (rate(src_gitserver_janitor_job_duration_seconds_count{instance=~`${shard:regex}`,success=\"false\"}[5m]))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{job_name}}").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "the rate of failures over 5m (by job)",
						},
					},
					{
						{
							Name:           "repos_removed",
							Description:    "repositories removed due to disk pressure",
							Query:          "sum by (instance) (rate(src_gitserver_repos_removed_disk_pressure{instance=~`${shard:regex}`}[5m]))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "Repositories removed due to disk pressure",
						},
					},
					{
						{
							Name:           "non_existent_repos_removed",
							Description:    "repositories removed because they are not defined in the DB",
							Query:          "sum by (instance) (increase(src_gitserver_non_existing_repos_removed[5m]))",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "Repositoriess removed because they are not defined in the DB",
						},
					},
					{
						{
							Name:           "sg_maintenance_reason",
							Description:    "successful sg maintenance jobs over 1h (by reason)",
							Query:          `sum by (reason) (rate(src_gitserver_maintenance_status{success="true"}[1h]))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{reason}}").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "the rate of successful sg maintenance jobs and the reason why they were triggered",
						},
					},
					{
						{
							Name:           "git_prune_skipped",
							Description:    "successful git prune jobs over 1h",
							Query:          `sum by (skipped) (rate(src_gitserver_prune_status{success="true"}[1h]))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("skipped={{skipped}}").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "the rate of successful git prune jobs over 1h and whether they were skipped",
						},
					},
				},
			},

			{
				Title:  "Search",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:           "search_latency",
							Description:    "mean time until first result is sent",
							Query:          "rate(src_gitserver_search_latency_seconds_sum[5m]) / rate(src_gitserver_search_latency_seconds_count[5m])",
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: "Mean latency (time to first result) of gitserver search requests",
						},
						{
							Name:           "search_duration",
							Description:    "mean search duration",
							Query:          "rate(src_gitserver_search_duration_seconds_sum[5m]) / rate(src_gitserver_search_duration_seconds_count[5m])",
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: "Mean duration of gitserver search requests",
						},
					},
					{
						{
							Name:           "search_rate",
							Description:    "rate of searches run by pod",
							Query:          "rate(src_gitserver_search_latency_seconds_count{instance=~`${shard:regex}`}[5m])",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: "The rate of searches executed on gitserver by pod",
						},
						{
							Name:           "running_searches",
							Description:    "number of searches currently running by pod",
							Query:          "sum by (instance) (src_gitserver_search_running{instance=~`${shard:regex}`})",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: "The number of searches currently executing on gitserver by pod",
						},
					},
				},
			},

			shared.GitServer.NewBackendGroup(containerName, true),
			shared.GitServer.NewClientGroup("*"),

			shared.NewDiskMetricsGroup(
				shared.DiskMetricsGroupOptions{
					DiskTitle: "repos",

					MetricMountNameLabel: "reposDir",
					MetricNamespace:      "gitserver",

					ServiceName:         "gitserver",
					InstanceFilterRegex: `${shard:regex}`,
				},
				monitoring.ObservableOwnerSource,
			),

			shared.NewGRPCServerMetricsGroup(
				shared.GRPCServerMetricsOptions{
					HumanServiceName:   "gitserver",
					RawGRPCServiceName: grpcServiceName,

					MethodFilterRegex:    fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
					InstanceFilterRegex:  `${shard:regex}`,
					MessageSizeNamespace: "src",
				}, monitoring.ObservableOwnerSource),

			shared.NewGRPCInternalErrorMetricsGroup(
				shared.GRPCInternalErrorMetricsOptions{
					HumanServiceName:   "gitserver",
					RawGRPCServiceName: grpcServiceName,
					Namespace:          "src",

					MethodFilterRegex: fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
				}, monitoring.ObservableOwnerSource),

			shared.NewGRPCRetryMetricsGroup(
				shared.GRPCRetryMetricsOptions{
					HumanServiceName:   "gitserver",
					RawGRPCServiceName: grpcServiceName,
					Namespace:          "src",

					MethodFilterRegex: fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
				}, monitoring.ObservableOwnerSource),

			shared.NewSiteConfigurationClientMetricsGroup(shared.SiteConfigurationMetricsOptions{
				HumanServiceName:    "gitserver",
				InstanceFilterRegex: `${shard:regex}`,
				JobFilterRegex:      scrapeJobRegex,
			}, monitoring.ObservableOwnerInfraOrg),

			shared.CodeIntelligence.NewCoursierGroup(containerName),
			shared.CodeIntelligence.NewNpmGroup(containerName),

			shared.HTTP.NewHandlersGroup(containerName),
			shared.NewDatabaseConnectionsMonitoringGroup(containerName, monitoring.ObservableOwnerSource),
			shared.NewContainerMonitoringGroup(containerName, monitoring.ObservableOwnerSource, nil),
			shared.NewProvisioningIndicatorsGroup(containerName, monitoring.ObservableOwnerSource, provisioningIndicatorsOptions),
			shared.NewGolangMonitoringGroup(containerName, monitoring.ObservableOwnerSource, nil),
			shared.NewKubernetesMonitoringGroup(containerName, monitoring.ObservableOwnerSource, nil),
		},
	}
}
