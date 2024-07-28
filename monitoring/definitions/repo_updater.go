package definitions

import (
	"fmt"
	"time"

	"github.com/grafana-tools/sdk"

	"github.com/sourcegraph/sourcegraph/monitoring/definitions/shared"
	"github.com/sourcegraph/sourcegraph/monitoring/monitoring"
)

func RepoUpdater() *monitoring.Dashboard {
	const (
		containerName   = "repo-updater"
		grpcServiceName = "repoupdater.v1.RepoUpdaterService"

		// This is set a bit longer than maxSyncInterval in internal/repos/syncer.go
		syncDurationThreshold = 9 * time.Hour
	)

	scrapeJobRegex := fmt.Sprintf(".*%s", containerName)

	containerMonitoringOptions := &shared.ContainerMonitoringGroupOptions{
		MemoryUsage: func(observable shared.Observable) shared.Observable {
			return observable.WithWarning(nil).WithCritical(monitoring.Alert().GreaterOrEqual(90).For(10 * time.Minute))
		},
	}

	grpcMethodVariable := shared.GRPCMethodVariable("repo_updater", grpcServiceName)

	return &monitoring.Dashboard{
		Name:        "repo-updater",
		Title:       "Repo Updater",
		Description: "Manages interaction with code hosts, instructs Gitserver to update repositories.",
		Variables: []monitoring.ContainerVariable{
			{

				Label: "Instance",
				Name:  "instance",
				OptionsLabelValues: monitoring.ContainerVariableOptionsLabelValues{
					Query:         "src_repoupdater_syncer_sync_last_time",
					LabelName:     "instance",
					ExampleOption: "repo-updater:3182",
				},
				Multi: true,
			},
			grpcMethodVariable,
		},
		Groups: []monitoring.Group{
			{
				Title: "Repositories",
				Rows: []monitoring.Row{
					{
						{
							Name:        "syncer_sync_last_time",
							Description: "time since last sync",
							Query:       `max(timestamp(vector(time()))) - max(src_repoupdater_syncer_sync_last_time)`,
							NoAlert:     true,
							Panel:       monitoring.Panel().Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSource,
							Interpretation: `
								A high value here indicates issues synchronizing repo metadata.
								If the value is persistently high, make sure all external services have valid tokens.
							`,
						},
						{
							Name:        "src_repoupdater_max_sync_backoff",
							Description: "time since oldest sync",
							Query:       `max(src_repoupdater_max_sync_backoff)`,
							Critical:    monitoring.Alert().GreaterOrEqual(syncDurationThreshold.Seconds()).For(10 * time.Minute),
							Panel:       monitoring.Panel().Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps: fmt.Sprintf(`
								An alert here indicates that no code host connections have synced in at least %v. This indicates that there could be a configuration issue
								with your code hosts connections or networking issues affecting communication with your code hosts.
								- Check the code host status indicator (cloud icon in top right of Sourcegraph homepage) for errors.
								- Make sure external services do not have invalid tokens by navigating to them in the web UI and clicking save. If there are no errors, they are valid.
								- Check the repo-updater logs for errors about syncing.
								- Confirm that outbound network connections are allowed where repo-updater is deployed.
								- Check back in an hour to see if the issue has resolved itself.
							`, syncDurationThreshold),
						},
						{
							Name:        "src_repoupdater_syncer_sync_errors_total",
							Description: "site level external service sync error rate",
							Query:       `max by (family) (rate(src_repoupdater_syncer_sync_errors_total{owner!="user",reason!="invalid_npm_path",reason!="internal_rate_limit"}[5m]))`,
							Warning:     monitoring.Alert().Greater(0.5).For(10 * time.Minute),
							Critical:    monitoring.Alert().Greater(1).For(10 * time.Minute),
							Panel:       monitoring.Panel().LegendFormat("{{family}}").Unit(monitoring.Number).With(monitoring.PanelOptions.ZeroIfNoData()),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps: `
								An alert here indicates errors syncing site level repo metadata with code hosts. This indicates that there could be a configuration issue
								with your code hosts connections or networking issues affecting communication with your code hosts.
								- Check the code host status indicator (cloud icon in top right of Sourcegraph homepage) for errors.
								- Make sure external services do not have invalid tokens by navigating to them in the web UI and clicking save. If there are no errors, they are valid.
								- Check the repo-updater logs for errors about syncing.
								- Confirm that outbound network connections are allowed where repo-updater is deployed.
								- Check back in an hour to see if the issue has resolved itself.
							`,
						},
					},
					{
						{
							Name:        "syncer_sync_start",
							Description: "repo metadata sync was started",
							Query:       fmt.Sprintf(`max by (family) (rate(src_repoupdater_syncer_start_sync{family="Syncer.SyncExternalService"}[%s]))`, syncDurationThreshold.String()),
							Warning:     monitoring.Alert().LessOrEqual(0).For(syncDurationThreshold),
							Panel:       monitoring.Panel().LegendFormat("Family: {{family}} Owner: {{owner}}").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater logs for errors.",
						},
						{
							Name:        "syncer_sync_duration",
							Description: "95th repositories sync duration",
							Query:       `histogram_quantile(0.95, max by (le, family, success) (rate(src_repoupdater_syncer_sync_duration_seconds_bucket[1m])))`,
							Warning:     monitoring.Alert().GreaterOrEqual(30).For(5 * time.Minute),
							Panel:       monitoring.Panel().LegendFormat("{{family}}-{{success}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host",
						},
						{
							Name:        "source_duration",
							Description: "95th repositories source duration",
							Query:       `histogram_quantile(0.95, max by (le) (rate(src_repoupdater_source_duration_seconds_bucket[1m])))`,
							Warning:     monitoring.Alert().GreaterOrEqual(30).For(5 * time.Minute),
							Panel:       monitoring.Panel().Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host",
						},
					},
					{
						{
							Name:        "syncer_synced_repos",
							Description: "repositories synced",
							Query:       `max(rate(src_repoupdater_syncer_synced_repos_total[1m]))`,
							Warning: monitoring.Alert().LessOrEqual(0).
								AggregateBy(monitoring.AggregatorMax).
								For(syncDurationThreshold),
							Panel:     monitoring.Panel().LegendFormat("{{state}}").Unit(monitoring.Number),
							Owner:     monitoring.ObservableOwnerSource,
							NextSteps: "Check network connectivity to code hosts",
						},
						{
							Name:        "sourced_repos",
							Description: "repositories sourced",
							Query:       `max(rate(src_repoupdater_source_repos_total[1m]))`,
							Warning:     monitoring.Alert().LessOrEqual(0).For(syncDurationThreshold),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check network connectivity to code hosts",
						},
					},
					{
						{
							Name:        "purge_failed",
							Description: "repositories purge failed",
							Query:       `max(rate(src_repoupdater_purge_failed[1m]))`,
							Warning:     monitoring.Alert().Greater(0).For(5 * time.Minute),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater's connectivity with gitserver and gitserver logs",
						},
					},
					{
						{
							Name:        "sched_auto_fetch",
							Description: "repositories scheduled due to hitting a deadline",
							Query:       `max(rate(src_repoupdater_sched_auto_fetch[1m]))`,
							Warning:     monitoring.Alert().LessOrEqual(0).For(syncDurationThreshold),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater logs.",
						},
						{
							Name:        "sched_manual_fetch",
							Description: "repositories scheduled due to user traffic",
							Query:       `max(rate(src_repoupdater_sched_manual_fetch[1m]))`,
							NoAlert:     true,
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							Interpretation: `
								Check repo-updater logs if this value is persistently high.
								This does not indicate anything if there are no user added code hosts.
							`,
						},
					},
					{
						{
							Name:        "sched_known_repos",
							Description: "repositories managed by the scheduler",
							Query:       `max(src_repoupdater_sched_known_repos)`,
							Warning:     monitoring.Alert().LessOrEqual(0).For(10 * time.Minute),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater logs. This is expected to fire if there are no user added code hosts",
						},
						{
							Name:        "sched_update_queue_length",
							Description: "rate of growth of update queue length over 5 minutes",
							Query:       `max(deriv(src_repoupdater_sched_update_queue_length[5m]))`,
							// Alert if the derivative is positive for longer than 30 minutes
							Critical:  monitoring.Alert().Greater(0).For(120 * time.Minute),
							Panel:     monitoring.Panel().Unit(monitoring.Number),
							Owner:     monitoring.ObservableOwnerSource,
							NextSteps: "Check repo-updater logs for indications that the queue is not being processed. The queue length should trend downwards over time as items are sent to GitServer",
						},
						{
							Name:        "sched_loops",
							Description: "scheduler loops",
							Query:       `max(rate(src_repoupdater_sched_loops[1m]))`,
							Warning:     monitoring.Alert().LessOrEqual(0).For(syncDurationThreshold),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater logs for errors. This is expected to fire if there are no user added code hosts",
						},
					},
					{
						{
							Name:        "src_repoupdater_stale_repos",
							Description: "repos that haven't been fetched in more than 8 hours",
							Query:       `max(src_repoupdater_stale_repos)`,
							Warning:     monitoring.Alert().GreaterOrEqual(1).For(25 * time.Minute),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps: `
								Check repo-updater logs for errors.
								Check for rows in gitserver_repos where LastError is not an empty string.
`,
						},
						{
							Name:        "sched_error",
							Description: "repositories schedule error rate",
							Query:       `max(rate(src_repoupdater_sched_error[1m]))`,
							Critical:    monitoring.Alert().GreaterOrEqual(1).For(25 * time.Minute),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater logs for errors",
						},
					},
					{
						{
							Name:        "repo_fetch_p75_latency",
							Description: "75th percentile fetch latency over 2m",
							Query:       "histogram_quantile(0.75, sum by (le, name)(rate(src_repoupdater_sched_latency_bucket[2m])))",
							Panel: monitoring.Panel().LegendFormat("Latency").
								Unit(monitoring.Seconds).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "The 75th percentile latency is the time between a repo changed on the code host and the repo being updated in gitserver. Lower is better.",
						},
						{
							Name:        "repo_fetch_histogram",
							Description: "repo fetch latency",
							Query:       `sum by (le) (rate(src_repoupdater_sched_latency_bucket[2m]))`,
							Panel: monitoring.PanelHeatmap().With(func(o monitoring.Observable, p *sdk.Panel) {
								p.HeatmapPanel.YAxis.Format = string(monitoring.Seconds)
								p.HeatmapPanel.DataFormat = "tsbuckets"
								p.HeatmapPanel.Targets[0].Format = "heatmap"
								p.HeatmapPanel.Targets[0].LegendFormat = "{{le}}"
							}),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "The latency is the time between a repo changed on the code host and the repo being updated in gitserver. Lower is better.",
						},
						{
							Name:           "repo_fetch_result",
							Description:    "rate of repo fetch success and failure",
							Query:          `sum by (result) (rate(src_repoupdater_sched_fetch_result[1m]))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{result}}").Unit(monitoring.Number),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "The rate of repo fetch success and failure. Lower is better.",
						},
						{
							Name:           "repo_fetch_hit_rate",
							Description:    "rate of repo fetches that resulted in updates",
							Query:          `(sum(rate(src_repoupdater_sched_fetch_result{result="changed"}[2m])) / sum(rate(src_repoupdater_sched_fetch_result{result=~"(changed|unchanged)"}[2m]))) * 100`,
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{result}}").Unit(monitoring.Percentage),
							Owner:          monitoring.ObservableOwnerSource,
							Interpretation: "The rate of repo fetch success and failure. Higher is better.",
						},
					},
				},
			},
			{
				Title:  "External services",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "src_repoupdater_external_services_total",
							Description: "the total number of external services",
							Query:       `max(src_repoupdater_external_services_total)`,
							Critical:    monitoring.Alert().GreaterOrEqual(20000).For(1 * time.Hour),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check for spikes in external services, could be abuse",
						},
					},
					{
						{
							Name:        "repoupdater_queued_sync_jobs_total",
							Description: "the total number of queued sync jobs",
							Query:       `max(src_repoupdater_queued_sync_jobs_total)`,
							Warning:     monitoring.Alert().GreaterOrEqual(100).For(1 * time.Hour),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps: `
								- **Check if jobs are failing to sync:** "SELECT * FROM external_service_sync_jobs WHERE state = 'errored'";
								- **Increase the number of workers** using the 'repoConcurrentExternalServiceSyncers' site config.
							`,
						},
						{
							Name:        "repoupdater_completed_sync_jobs_total",
							Description: "the total number of completed sync jobs",
							Query:       `max(src_repoupdater_completed_sync_jobs_total)`,
							Warning:     monitoring.Alert().GreaterOrEqual(100000).For(1 * time.Hour),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater logs. Jobs older than 1 day should have been removed.",
						},
						{
							Name:        "repoupdater_errored_sync_jobs_percentage",
							Description: "the percentage of external services that have failed their most recent sync",
							Query:       `max(src_repoupdater_errored_sync_jobs_percentage)`,
							Warning:     monitoring.Alert().Greater(10).For(1 * time.Hour),
							Panel:       monitoring.Panel().Unit(monitoring.Percentage),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps:   "Check repo-updater logs. Check code host connectivity",
						},
					},
					{
						{
							Name:        "github_graphql_rate_limit_remaining",
							Description: "remaining calls to GitHub graphql API before hitting the rate limit",
							Query:       `max by (name) (src_github_rate_limit_remaining_v2{resource="graphql"})`,
							// 5% of initial limit of 5000
							Warning: monitoring.Alert().LessOrEqual(250),
							Panel:   monitoring.Panel().LegendFormat("{{name}}"),
							Owner:   monitoring.ObservableOwnerSource,
							NextSteps: `
								- Consider creating a new token for the indicated resource (the 'name' label for series below the threshold in the dashboard) under a dedicated machine user to reduce rate limit pressure.
							`,
						},
						{
							Name:        "github_rest_rate_limit_remaining",
							Description: "remaining calls to GitHub rest API before hitting the rate limit",
							Query:       `max by (name) (src_github_rate_limit_remaining_v2{resource="rest"})`,
							// 5% of initial limit of 5000
							Warning: monitoring.Alert().LessOrEqual(250),
							Panel:   monitoring.Panel().LegendFormat("{{name}}"),
							Owner:   monitoring.ObservableOwnerSource,
							NextSteps: `
								- Consider creating a new token for the indicated resource (the 'name' label for series below the threshold in the dashboard) under a dedicated machine user to reduce rate limit pressure.
							`,
						},
						{
							Name:        "github_search_rate_limit_remaining",
							Description: "remaining calls to GitHub search API before hitting the rate limit",
							Query:       `max by (name) (src_github_rate_limit_remaining_v2{resource="search"})`,
							Warning:     monitoring.Alert().LessOrEqual(5),
							Panel:       monitoring.Panel().LegendFormat("{{name}}"),
							Owner:       monitoring.ObservableOwnerSource,
							NextSteps: `
								- Consider creating a new token for the indicated resource (the 'name' label for series below the threshold in the dashboard) under a dedicated machine user to reduce rate limit pressure.
							`,
						},
					},
					{
						{
							Name:           "github_graphql_rate_limit_wait_duration",
							Description:    "time spent waiting for the GitHub graphql API rate limiter",
							Query:          `max by(name) (rate(src_github_rate_limit_wait_duration_seconds{resource="graphql"}[5m]))`,
							Panel:          monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "Indicates how long we're waiting on the rate limit once it has been exceeded",
						},
						{
							Name:           "github_rest_rate_limit_wait_duration",
							Description:    "time spent waiting for the GitHub rest API rate limiter",
							Query:          `max by(name) (rate(src_github_rate_limit_wait_duration_seconds{resource="rest"}[5m]))`,
							Panel:          monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "Indicates how long we're waiting on the rate limit once it has been exceeded",
						},
						{
							Name:           "github_search_rate_limit_wait_duration",
							Description:    "time spent waiting for the GitHub search API rate limiter",
							Query:          `max by(name) (rate(src_github_rate_limit_wait_duration_seconds{resource="search"}[5m]))`,
							Panel:          monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "Indicates how long we're waiting on the rate limit once it has been exceeded",
						},
					},
					{
						{
							Name:        "gitlab_rest_rate_limit_remaining",
							Description: "remaining calls to GitLab rest API before hitting the rate limit",
							Query:       `max by (name) (src_gitlab_rate_limit_remaining{resource="rest"})`,
							// 5% of initial limit of 600
							Critical:  monitoring.Alert().LessOrEqual(30),
							Panel:     monitoring.Panel().LegendFormat("{{name}}"),
							Owner:     monitoring.ObservableOwnerSource,
							NextSteps: `Try restarting the pod to get a different public IP.`,
						},
						{
							Name:           "gitlab_rest_rate_limit_wait_duration",
							Description:    "time spent waiting for the GitLab rest API rate limiter",
							Query:          `max by (name) (rate(src_gitlab_rate_limit_wait_duration_seconds{resource="rest"}[5m]))`,
							Panel:          monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "Indicates how long we're waiting on the rate limit once it has been exceeded",
						},
					},
					{
						{
							Name:           "src_internal_rate_limit_wait_duration_bucket",
							Description:    "95th percentile time spent successfully waiting on our internal rate limiter",
							Query:          `histogram_quantile(0.95, sum(rate(src_internal_rate_limit_wait_duration_bucket{failed="false"}[5m])) by (le, urn))`,
							Panel:          monitoring.Panel().LegendFormat("{{urn}}").Unit(monitoring.Seconds),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "Indicates how long we're waiting on our internal rate limiter when communicating with a code host",
						},
						{
							Name:           "src_internal_rate_limit_wait_error_count",
							Description:    "rate of failures waiting on our internal rate limiter",
							Query:          `sum by (urn) (rate(src_internal_rate_limit_wait_duration_count{failed="true"}[5m]))`,
							Panel:          monitoring.Panel().LegendFormat("{{urn}}"),
							Owner:          monitoring.ObservableOwnerSource,
							NoAlert:        true,
							Interpretation: "The rate at which we fail our internal rate limiter.",
						},
					},
				},
			},

			shared.GitServer.NewClientGroup(containerName),
			shared.GitServer.NewRepoClientGroup(containerName),

			shared.Batches.NewDBStoreGroup(containerName),
			shared.Batches.NewServiceGroup(containerName),

			shared.CodeIntelligence.NewCoursierGroup(containerName),
			shared.CodeIntelligence.NewNpmGroup(containerName),

			shared.NewGRPCServerMetricsGroup(
				shared.GRPCServerMetricsOptions{
					HumanServiceName:   "repo_updater",
					RawGRPCServiceName: grpcServiceName,

					MethodFilterRegex:    fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
					InstanceFilterRegex:  `${instance:regex}`,
					MessageSizeNamespace: "src",
				}, monitoring.ObservableOwnerSource),

			shared.NewGRPCInternalErrorMetricsGroup(
				shared.GRPCInternalErrorMetricsOptions{
					HumanServiceName:   "repo_updater",
					RawGRPCServiceName: grpcServiceName,
					Namespace:          "src",

					MethodFilterRegex: fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
				}, monitoring.ObservableOwnerSource),

			shared.NewGRPCRetryMetricsGroup(
				shared.GRPCRetryMetricsOptions{
					HumanServiceName:   "repo_updater",
					RawGRPCServiceName: grpcServiceName,
					Namespace:          "src",

					MethodFilterRegex: fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
				}, monitoring.ObservableOwnerSource),

			shared.NewSiteConfigurationClientMetricsGroup(shared.SiteConfigurationMetricsOptions{
				HumanServiceName:    "repo_updater",
				InstanceFilterRegex: `${instance:regex}`,
				JobFilterRegex:      scrapeJobRegex,
			}, monitoring.ObservableOwnerInfraOrg),
			shared.HTTP.NewHandlersGroup(containerName),
			shared.NewDatabaseConnectionsMonitoringGroup(containerName, monitoring.ObservableOwnerSource),
			shared.NewContainerMonitoringGroup(containerName, monitoring.ObservableOwnerSource, containerMonitoringOptions),
			shared.NewProvisioningIndicatorsGroup(containerName, monitoring.ObservableOwnerSource, nil),
			shared.NewGolangMonitoringGroup(containerName, monitoring.ObservableOwnerSource, nil),
			shared.NewKubernetesMonitoringGroup(containerName, monitoring.ObservableOwnerSource, nil),
		},
	}
}
