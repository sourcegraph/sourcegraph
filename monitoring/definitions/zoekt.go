package definitions

import (
	"fmt"
	"time"

	"github.com/grafana-tools/sdk"

	"github.com/sourcegraph/sourcegraph/monitoring/definitions/shared"
	"github.com/sourcegraph/sourcegraph/monitoring/monitoring"
)

func Zoekt() *monitoring.Dashboard {
	const (
		indexServerContainerName = "zoekt-indexserver"
		webserverContainerName   = "zoekt-webserver"
		bundledContainerName     = "indexed-search"
		grpcServiceName          = "zoekt.webserver.v1.WebserverService"

		indexServerJob = "indexed-search-indexer"
		webserverJob   = "indexed-search"
	)

	grpcMethodVariable := shared.GRPCMethodVariable("zoekt_webserver", grpcServiceName)

	return &monitoring.Dashboard{
		Name:                     "zoekt",
		Title:                    "Zoekt",
		Description:              "Indexes repositories, populates the search index, and responds to indexed search queries.",
		NoSourcegraphDebugServer: true,
		Variables: []monitoring.ContainerVariable{
			{
				Label: "Instance",
				Name:  "instance",
				OptionsLabelValues: monitoring.ContainerVariableOptionsLabelValues{
					Query:         "index_num_indexed",
					LabelName:     "instance",
					ExampleOption: "zoekt-indexserver-0:6072",
				},
				Multi: true,
			},
			{
				Label: "Webserver Instance",
				Name:  "webserver_instance",
				OptionsLabelValues: monitoring.ContainerVariableOptionsLabelValues{
					Query:         "zoekt_webserver_watchdog_errors",
					LabelName:     "instance",
					ExampleOption: "zoekt-webserver-0:6072",
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
							Name:        "total_repos_aggregate",
							Description: "total number of repos (aggregate)",
							Query:       `sum by (__name__) ({__name__=~"index_num_assigned|index_num_indexed|index_queue_cap"})`,
							NoAlert:     true,
							Panel: monitoring.Panel().
								With(
									monitoring.PanelOptions.LegendOnRight(),
									monitoring.PanelOptions.HoverShowAll(),
								).
								MinAuto().
								LegendFormat("{{__name__}}"),
							Owner: monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Sudden changes can be caused by indexing configuration changes.

								Additionally, a discrepancy between "index_num_assigned" and "index_queue_cap" could indicate a bug.

								Legend:
								- index_num_assigned: # of repos assigned to Zoekt
								- index_num_indexed: # of repos Zoekt has indexed
								- index_queue_cap: # of repos Zoekt is aware of, including those that it has finished indexing
							`,
						},
						{
							Name:        "total_repos_per_instance",
							Description: "total number of repos (per instance)",
							Query:       `sum by (__name__, instance) ({__name__=~"index_num_assigned|index_num_indexed|index_queue_cap",instance=~"${instance:regex}"})`,
							NoAlert:     true,
							Panel: monitoring.Panel().
								With(
									monitoring.PanelOptions.LegendOnRight(),
									monitoring.PanelOptions.HoverShowAll(),
								).
								MinAuto().
								LegendFormat("{{instance}} {{__name__}}"),
							Owner: monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Sudden changes can be caused by indexing configuration changes.

								Additionally, a discrepancy between "index_num_assigned" and "index_queue_cap" could indicate a bug.

								Legend:
								- index_num_assigned: # of repos assigned to Zoekt
								- index_num_indexed: # of repos Zoekt has indexed
								- index_queue_cap: # of repos Zoekt is aware of, including those that it has finished processing
							`,
						},
					},
					{
						{
							Name:        "repos_stopped_tracking_total_aggregate",
							Description: "the number of repositories we stopped tracking over 5m (aggregate)",
							Query:       `sum(increase(index_num_stopped_tracking_total[5m]))`,
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("dropped").
								Unit(monitoring.Number).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "Repositories we stop tracking are soft-deleted during the next cleanup job.",
						},
						{
							Name:        "repos_stopped_tracking_total_per_instance",
							Description: "the number of repositories we stopped tracking over 5m (per instance)",
							Query:       "sum by (instance) (increase(index_num_stopped_tracking_total{instance=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{instance}}").
								Unit(monitoring.Number).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "Repositories we stop tracking are soft-deleted during the next cleanup job.",
						},
					},
					{
						{
							Name:        "average_resolve_revision_duration",
							Description: "average resolve revision duration over 5m",
							Query:       `sum(rate(resolve_revision_seconds_sum[5m])) / sum(rate(resolve_revision_seconds_count[5m]))`,
							Warning:     monitoring.Alert().GreaterOrEqual(15),
							Panel:       monitoring.Panel().LegendFormat("{{duration}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							NextSteps:   "none",
						},
						{
							Name:        "get_index_options_error_increase",
							Description: "the number of repositories we failed to get indexing options over 5m",
							Query:       `sum(increase(get_index_options_error_total[5m]))`,
							// This value can spike, so only if we have a
							// sustained error rate do we alert. On
							// Sourcegraph.com gitserver rollouts take a while
							// and this alert will fire during that time. So
							// we tuned Critical to atleast be as long as a
							// gitserver rollout. 2022-02-09 ~25m rollout.
							Warning:  monitoring.Alert().GreaterOrEqual(100).For(5 * time.Minute),
							Critical: monitoring.Alert().GreaterOrEqual(100).For(35 * time.Minute),
							Panel:    monitoring.Panel().Min(0),
							Owner:    monitoring.ObservableOwnerSearchCore,
							NextSteps: `
								- View error rates on gitserver and frontend to identify root cause.
								- Rollback frontend/gitserver deployment if due to a bad code change.
								- View error logs for 'getIndexOptions' via net/trace debug interface. For example click on a 'indexed-search-indexer-' on https://sourcegraph.com/-/debug/. Then click on Traces. Replace sourcegraph.com with your instance address.
							`,
							Interpretation: `
								When considering indexing a repository we ask for the index configuration
								from frontend per repository. The most likely reason this would fail is
								failing to resolve branch names to git SHAs.

								This value can spike up during deployments/etc. Only if you encounter
								sustained periods of errors is there an underlying issue. When sustained
								this indicates repositories will not get updated indexes.
							`,
						},
					},
				},
			},
			{
				Title: "Search requests",
				Rows: []monitoring.Row{
					{
						{
							Name:        "indexed_search_request_duration_p99_aggregate",
							Description: "99th percentile indexed search duration over 1m (aggregate)",
							Query:       `histogram_quantile(0.99, sum by (le, name)(rate(zoekt_search_duration_seconds_bucket[1m])))`, // TODO: split this into separate success/failure metrics
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the 99th percentile of search request durations over the last minute (aggregated across all instances).

								Large duration spikes can be an indicator of saturation and / or a performance regression.
							`,
						},
						{
							Name:        "indexed_search_request_duration_p90_aggregate",
							Description: "90th percentile indexed search duration over 1m (aggregate)",
							Query:       `histogram_quantile(0.90, sum by (le, name)(rate(zoekt_search_duration_seconds_bucket[1m])))`, // TODO: split this into separate success/failure metrics
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the 90th percentile of search request durations over the last minute (aggregated across all instances).

								Large duration spikes can be an indicator of saturation and / or a performance regression.
							`,
						},
						{
							Name:        "indexed_search_request_duration_p75_aggregate",
							Description: "75th percentile indexed search duration over 1m (aggregate)",
							Query:       `histogram_quantile(0.75, sum by (le, name)(rate(zoekt_search_duration_seconds_bucket[1m])))`, // TODO: split this into separate success/failure metrics
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the 75th percentile of search request durations over the last minute (aggregated across all instances).

								Large duration spikes can be an indicator of saturation and / or a performance regression.
							`,
						},
					},
					{
						{
							Name:        "indexed_search_request_duration_p99_by_instance",
							Description: "99th percentile indexed search duration over 1m (per instance)",
							Query:       "histogram_quantile(0.99, sum by (le, instance)(rate(zoekt_search_duration_seconds_bucket{instance=~`${instance:regex}`}[1m])))", // TODO: split this into separate success/failure metrics
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the 99th percentile of search request durations over the last minute (broken out per instance).

								Large duration spikes can be an indicator of saturation and / or a performance regression.
							`,
						},
						{
							Name:        "indexed_search_request_duration_p90_by_instance",
							Description: "90th percentile indexed search duration over 1m (per instance)",
							Query:       "histogram_quantile(0.90, sum by (le, instance)(rate(zoekt_search_duration_seconds_bucket{instance=~`${instance:regex}`}[1m])))", // TODO: split this into separate success/failure metrics
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the 90th percentile of search request durations over the last minute (broken out per instance).

								Large duration spikes can be an indicator of saturation and / or a performance regression.
							`,
						},
						{
							Name:        "indexed_search_request_duration_p75_by_instance",
							Description: "75th percentile indexed search duration over 1m (per instance)",
							Query:       "histogram_quantile(0.75, sum by (le, instance)(rate(zoekt_search_duration_seconds_bucket{instance=~`${instance:regex}`}[1m])))", // TODO: split this into separate success/failure metrics
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the 75th percentile of search request durations over the last minute (broken out per instance).

								Large duration spikes can be an indicator of saturation and / or a performance regression.
							`,
						},
					},
					{
						{
							Name:        "indexed_search_num_concurrent_requests_aggregate",
							Description: "amount of in-flight indexed search requests (aggregate)",
							Query:       `sum by (name) (zoekt_search_running)`,
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the current number of indexed search requests that are in-flight, aggregated across all instances.

								In-flight search requests include both running and queued requests.

								The number of in-flight requests can serve as a proxy for the general load that webserver instances are under.
							`,
						},
						{
							Name:        "indexed_search_num_concurrent_requests_by_instance",
							Description: "amount of in-flight indexed search requests (per instance)",
							Query:       "sum by (instance, name) (zoekt_search_running{instance=~`${instance:regex}`})",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the current number of indexed search requests that are-flight, broken out per instance.

								In-flight search requests include both running and queued requests.

								The number of in-flight requests can serve as a proxy for the general load that webserver instances are under.
							`,
						},
					},
					{
						{
							Name:        "indexed_search_concurrent_request_growth_rate_1m_aggregate",
							Description: "rate of growth of in-flight indexed search requests over 1m (aggregate)",
							Query:       `sum by (name) (deriv(zoekt_search_running[1m]))`,
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Number),

							Owner: monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the rate of growth of in-flight requests, aggregated across all instances.

								In-flight search requests include both running and queued requests.

								This metric gives a notion of how quickly the indexed-search backend is working through its request load
								(taking into account the request arrival rate and processing time). A sustained high rate of growth
								can indicate that the indexed-search backend is saturated.
							`,
						},
						{
							Name:        "indexed_search_concurrent_request_growth_rate_1m_per_instance",
							Description: "rate of growth of in-flight indexed search requests over 1m (per instance)",
							Query:       "sum by (instance) (deriv(zoekt_search_running[1m]))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.RequestsPerSecond),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								This dashboard shows the rate of growth of in-flight requests, broken out per instance.

								In-flight search requests include both running and queued requests.

								This metric gives a notion of how quickly the indexed-search backend is working through its request load
								(taking into account the request arrival rate and processing time). A sustained high rate of growth
								can indicate that the indexed-search backend is saturated.
							`,
						},
					},
					{
						{
							Name:        "indexed_search_request_errors",
							Description: "indexed search request errors every 5m by code",
							Query:       `sum by (code)(increase(src_zoekt_request_duration_seconds_count{code!~"2.."}[5m])) / ignoring(code) group_left sum(increase(src_zoekt_request_duration_seconds_count[5m])) * 100`,
							Warning:     monitoring.Alert().GreaterOrEqual(5).For(5 * time.Minute),
							Panel:       monitoring.Panel().LegendFormat("{{code}}").Unit(monitoring.Percentage),
							Owner:       monitoring.ObservableOwnerSearchCore,
							NextSteps:   "none",
						},
					},
					{
						{
							Name:        "zoekt_shards_sched",
							Description: "current number of zoekt scheduler processes in a state",
							Query:       "sum by (type, state) (zoekt_shards_sched)",
							NoAlert:     true,
							Panel: monitoring.Panel().With(
								monitoring.PanelOptions.LegendOnRight(),
								func(o monitoring.Observable, p *sdk.Panel) {
									p.GraphPanel.Targets = []sdk.Target{{
										Expr:         o.Query,
										LegendFormat: "{{type}} {{state}}",
									}}
									p.GraphPanel.Legend.Current = true
									p.GraphPanel.Tooltip.Shared = true
								}).MinAuto(),
							Owner: monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Each ongoing search request starts its life as an interactive query. If it
								takes too long it becomes a batch query. Between state transitions it can be queued.

								If you have a high number of batch queries it is a sign there is a large load
								of slow queries. Alternatively your systems are underprovisioned and normal
								search queries are taking too long.

								For a full explanation of the states see https://github.com/sourcegraph/zoekt/blob/930cd1c28917e64c87f0ce354a0fd040877cbba1/shards/sched.go#L311-L340
							`,
						},
						{
							Name:        "zoekt_shards_sched_total",
							Description: "rate of zoekt scheduler process state transitions in the last 5m",
							Query:       "sum by (type, state) (rate(zoekt_shards_sched[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().With(
								monitoring.PanelOptions.LegendOnRight(),
								func(o monitoring.Observable, p *sdk.Panel) {
									p.GraphPanel.Targets = []sdk.Target{{
										Expr:         o.Query,
										LegendFormat: "{{type}} {{state}}",
									}}
									p.GraphPanel.Legend.Current = true
									p.GraphPanel.Tooltip.Shared = true
								}).MinAuto(),
							Owner: monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Each ongoing search request starts its life as an interactive query. If it
								takes too long it becomes a batch query. Between state transitions it can be queued.

								If you have a high number of batch queries it is a sign there is a large load
								of slow queries. Alternatively your systems are underprovisioned and normal
								search queries are taking too long.

								For a full explanation of the states see https://github.com/sourcegraph/zoekt/blob/930cd1c28917e64c87f0ce354a0fd040877cbba1/shards/sched.go#L311-L340
							`,
						},
					},
				},
			},
			{
				Title: "Git fetch durations",
				Rows: []monitoring.Row{
					{
						{
							Name:        "90th_percentile_successful_git_fetch_durations_5m",
							Description: "90th percentile successful git fetch durations over 5m",
							Query:       `histogram_quantile(0.90, sum by (le, name)(rate(index_fetch_seconds_bucket{success="true"}[5m])))`,
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Long git fetch times can be a leading indicator of saturation.
							`,
						},
						{
							Name:        "90th_percentile_failed_git_fetch_durations_5m",
							Description: "90th percentile failed git fetch durations over 5m",
							Query:       `histogram_quantile(0.90, sum by (le, name)(rate(index_fetch_seconds_bucket{success="false"}[5m])))`,
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Long git fetch times can be a leading indicator of saturation.
							`,
						},
					},
				},
			},
			{
				Title: "Indexing results",
				Rows: []monitoring.Row{
					{
						{
							Name:        "repo_index_state_aggregate",
							Description: "index results state count over 5m (aggregate)",
							Query:       "sum by (state) (increase(index_repo_seconds_count[5m]))",
							NoAlert:     true,
							Owner:       monitoring.ObservableOwnerSearchCore,
							Panel: monitoring.Panel().LegendFormat("{{state}}").With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								func(o monitoring.Observable, p *sdk.Panel) {
									p.GraphPanel.Yaxes[0].LogBase = 2 // log to show the huge number of "noop" or "empty"
								},
							),
							Interpretation: `
							This dashboard shows the outcomes of recently completed indexing jobs across all index-server instances.

							A persistent failing state indicates some repositories cannot be indexed, perhaps due to size and timeouts.

							Legend:
							- fail -> the indexing jobs failed
							- success -> the indexing job succeeded and the index was updated
							- success_meta -> the indexing job succeeded, but only metadata was updated
							- noop -> the indexing job succeed, but we didn't need to update anything
							- empty -> the indexing job succeeded, but the index was empty (i.e. the repository is empty)
						`,
						},
						{
							Name:        "repo_index_state_per_instance",
							Description: "index results state count over 5m (per instance)",
							Query:       "sum by (instance, state) (increase(index_repo_seconds_count{instance=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Owner:       monitoring.ObservableOwnerSearchCore,
							Panel: monitoring.Panel().LegendFormat("{{instance}} {{state}}").With(
								monitoring.PanelOptions.LegendOnRight(),
								func(o monitoring.Observable, p *sdk.Panel) {
									p.GraphPanel.Yaxes[0].LogBase = 2  // log to show the huge number of "noop" or "empty"
									p.GraphPanel.Tooltip.Shared = true // show multiple lines simultaneously
								}),
							Interpretation: `
							This dashboard shows the outcomes of recently completed indexing jobs, split out across each index-server instance.

							(You can use the "instance" filter at the top of the page to select a particular instance.)

							A persistent failing state indicates some repositories cannot be indexed, perhaps due to size and timeouts.

							Legend:
							- fail -> the indexing jobs failed
							- success -> the indexing job succeeded and the index was updated
							- success_meta -> the indexing job succeeded, but only metadata was updated
							- noop -> the indexing job succeed, but we didn't need to update anything
							- empty -> the indexing job succeeded, but the index was empty (i.e. the repository is empty)
						`,
						},
					},
					{
						{
							Name:           "repo_index_success_speed_heatmap",
							Description:    "successful indexing durations",
							Query:          `sum by (le, state) (increase(index_repo_seconds_bucket{state="success"}[$__rate_interval]))`,
							NoAlert:        true,
							Panel:          monitoring.PanelHeatmap().With(zoektHeatMapPanelOptions),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "Latency increases can indicate bottlenecks in the indexserver.",
						},
						{
							Name:           "repo_index_fail_speed_heatmap",
							Description:    "failed indexing durations",
							Query:          `sum by (le, state) (increase(index_repo_seconds_bucket{state="fail"}[$__rate_interval]))`,
							NoAlert:        true,
							Panel:          monitoring.PanelHeatmap().With(zoektHeatMapPanelOptions),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "Failures happening after a long time indicates timeouts.",
						},
					},
					{
						{
							Name:        "repo_index_success_speed_p99",
							Description: "99th percentile successful indexing durations over 5m (aggregate)",
							Query:       "histogram_quantile(0.99, sum by (le, name)(rate(index_repo_seconds_bucket{state=\"success\"}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p99 duration of successful indexing jobs aggregated across all Zoekt instances.

							Latency increases can indicate bottlenecks in the indexserver.
						`,
						},
						{
							Name:        "repo_index_success_speed_p90",
							Description: "90th percentile successful indexing durations over 5m (aggregate)",
							Query:       "histogram_quantile(0.90, sum by (le, name)(rate(index_repo_seconds_bucket{state=\"success\"}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p90 duration of successful indexing jobs aggregated across all Zoekt instances.

							Latency increases can indicate bottlenecks in the indexserver.
						`,
						},
						{
							Name:        "repo_index_success_speed_p75",
							Description: "75th percentile successful indexing durations over 5m (aggregate)",
							Query:       "histogram_quantile(0.75, sum by (le, name)(rate(index_repo_seconds_bucket{state=\"success\"}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p75 duration of successful indexing jobs aggregated across all Zoekt instances.

							Latency increases can indicate bottlenecks in the indexserver.
						`,
						},
					},
					{
						{
							Name:        "repo_index_success_speed_p99_per_instance",
							Description: "99th percentile successful indexing durations over 5m (per instance)",
							Query:       "histogram_quantile(0.99, sum by (le, instance)(rate(index_repo_seconds_bucket{state=\"success\",instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p99 duration of successful indexing jobs broken out per Zoekt instance.

							Latency increases can indicate bottlenecks in the indexserver.
						`,
						},
						{
							Name:        "repo_index_success_speed_p90_per_instance",
							Description: "90th percentile successful indexing durations over 5m (per instance)",
							Query:       "histogram_quantile(0.90, sum by (le, instance)(rate(index_repo_seconds_bucket{state=\"success\",instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p90 duration of successful indexing jobs broken out per Zoekt instance.

							Latency increases can indicate bottlenecks in the indexserver.
						`,
						},
						{
							Name:        "repo_index_success_speed_p75_per_instance",
							Description: "75th percentile successful indexing durations over 5m (per instance)",
							Query:       "histogram_quantile(0.75, sum by (le, instance)(rate(index_repo_seconds_bucket{state=\"success\",instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p75 duration of successful indexing jobs broken out per Zoekt instance.

							Latency increases can indicate bottlenecks in the indexserver.
						`,
						},
					},
					{
						{
							Name:        "repo_index_failed_speed_p99",
							Description: "99th percentile failed indexing durations over 5m (aggregate)",
							Query:       "histogram_quantile(0.99, sum by (le, name)(rate(index_repo_seconds_bucket{state=\"fail\"}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p99 duration of failed indexing jobs aggregated across all Zoekt instances.

							Failures happening after a long time indicates timeouts.
						`,
						},
						{
							Name:        "repo_index_failed_speed_p90",
							Description: "90th percentile failed indexing durations over 5m (aggregate)",
							Query:       "histogram_quantile(0.90, sum by (le, name)(rate(index_repo_seconds_bucket{state=\"fail\"}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p90 duration of failed indexing jobs aggregated across all Zoekt instances.

							Failures happening after a long time indicates timeouts.
						`,
						},
						{
							Name:        "repo_index_failed_speed_p75",
							Description: "75th percentile failed indexing durations over 5m (aggregate)",
							Query:       "histogram_quantile(0.75, sum by (le, name)(rate(index_repo_seconds_bucket{state=\"fail\"}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p75 duration of failed indexing jobs aggregated across all Zoekt instances.

							Failures happening after a long time indicates timeouts.
						`,
						},
					},
					{
						{
							Name:        "repo_index_failed_speed_p99_per_instance",
							Description: "99th percentile failed indexing durations over 5m (per instance)",
							Query:       "histogram_quantile(0.99, sum by (le, instance)(rate(index_repo_seconds_bucket{state=\"fail\",instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p99 duration of failed indexing jobs broken out per Zoekt instance.

							Failures happening after a long time indicates timeouts.
						`,
						},
						{
							Name:        "repo_index_failed_speed_p90_per_instance",
							Description: "90th percentile failed indexing durations over 5m (per instance)",
							Query:       "histogram_quantile(0.90, sum by (le, instance)(rate(index_repo_seconds_bucket{state=\"fail\",instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p90 duration of failed indexing jobs broken out per Zoekt instance.

							Failures happening after a long time indicates timeouts.
						`,
						},
						{
							Name:        "repo_index_failed_speed_p75_per_instance",
							Description: "75th percentile failed indexing durations over 5m (per instance)",
							Query:       "histogram_quantile(0.75, sum by (le, instance)(rate(index_repo_seconds_bucket{state=\"fail\",instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p75 duration of failed indexing jobs broken out per Zoekt instance.

							Failures happening after a long time indicates timeouts.
						`,
						},
					},
				},
			},
			{
				Title: "Indexing queue statistics",
				Rows: []monitoring.Row{
					{
						{
							Name:           "indexed_num_scheduled_jobs_aggregate",
							Description:    "# scheduled index jobs (aggregate)",
							Query:          "sum(index_queue_len)", // total queue size amongst all index-server replicas
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("jobs"),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "A queue that is constantly growing could be a leading indicator of a bottleneck or under-provisioning",
						},
						{
							Name:           "indexed_num_scheduled_jobs_per_instance",
							Description:    "# scheduled index jobs (per instance)",
							Query:          "index_queue_len{instance=~`${instance:regex}`}",
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{instance}} jobs"),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "A queue that is constantly growing could be a leading indicator of a bottleneck or under-provisioning",
						},
					},
					{
						{
							Name:        "indexed_queueing_delay_heatmap",
							Description: "job queuing delay heatmap",
							Query:       "sum by (le) (increase(index_queue_age_seconds_bucket[$__rate_interval]))",
							NoAlert:     true,
							Panel:       monitoring.PanelHeatmap().With(zoektHeatMapPanelOptions),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							The queueing delay represents the amount of time an indexing job spent in the queue before it was processed.

							Large queueing delays can be an indicator of:
								- resource saturation
								- each Zoekt replica has too many jobs for it to be able to process all of them promptly. In this scenario, consider adding additional Zoekt replicas to distribute the work better .
						`,
						},
					},
					{
						{
							Name:        "indexed_queueing_delay_p99_9_aggregate",
							Description: "99.9th percentile job queuing delay over 5m (aggregate)",
							Query:       "histogram_quantile(0.999, sum by (le, name)(rate(index_queue_age_seconds_bucket[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p99.9 job queueing delay aggregated across all Zoekt instances.

							The queueing delay represents the amount of time an indexing job spent in the queue before it was processed.

							Large queueing delays can be an indicator of:
								- resource saturation
								- each Zoekt replica has too many jobs for it to be able to process all of them promptly. In this scenario, consider adding additional Zoekt replicas to distribute the work better.

							The 99.9 percentile dashboard is useful for capturing the long tail of queueing delays (on the order of 24+ hours, etc.).
						`,
						},
						{
							Name:        "indexed_queueing_delay_p90_aggregate",
							Description: "90th percentile job queueing delay over 5m (aggregate)",
							Query:       "histogram_quantile(0.90, sum by (le, name)(rate(index_queue_age_seconds_bucket[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p90 job queueing delay aggregated across all Zoekt instances.

							The queueing delay represents the amount of time an indexing job spent in the queue before it was processed.

							Large queueing delays can be an indicator of:
								- resource saturation
								- each Zoekt replica has too many jobs for it to be able to process all of them promptly. In this scenario, consider adding additional Zoekt replicas to distribute the work better.
						`,
						},
						{
							Name:        "indexed_queueing_delay_p75_aggregate",
							Description: "75th percentile job queueing delay over 5m (aggregate)",
							Query:       "histogram_quantile(0.75, sum by (le, name)(rate(index_queue_age_seconds_bucket[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{name}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p75 job queueing delay aggregated across all Zoekt instances.

							The queueing delay represents the amount of time an indexing job spent in the queue before it was processed.

							Large queueing delays can be an indicator of:
								- resource saturation
								- each Zoekt replica has too many jobs for it to be able to process all of them promptly. In this scenario, consider adding additional Zoekt replicas to distribute the work better.
						`,
						},
					},
					{
						{
							Name:        "indexed_queueing_delay_p99_9_per_instance",
							Description: "99.9th percentile job queuing delay over 5m (per instance)",
							Query:       "histogram_quantile(0.999, sum by (le, instance)(rate(index_queue_age_seconds_bucket{instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p99.9 job queueing delay, broken out per Zoekt instance.

							The queueing delay represents the amount of time an indexing job spent in the queue before it was processed.

							Large queueing delays can be an indicator of:
								- resource saturation
								- each Zoekt replica has too many jobs for it to be able to process all of them promptly. In this scenario, consider adding additional Zoekt replicas to distribute the work better.

							The 99.9 percentile dashboard is useful for capturing the long tail of queueing delays (on the order of 24+ hours, etc.).
						`,
						},
						{
							Name:        "indexed_queueing_delay_p90_per_instance",
							Description: "90th percentile job queueing delay over 5m (per instance)",
							Query:       "histogram_quantile(0.90, sum by (le, instance)(rate(index_queue_age_seconds_bucket{instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p90 job queueing delay, broken out per Zoekt instance.

							The queueing delay represents the amount of time an indexing job spent in the queue before it was processed.

							Large queueing delays can be an indicator of:
								- resource saturation
								- each Zoekt replica has too many jobs for it to be able to process all of them promptly. In this scenario, consider adding additional Zoekt replicas to distribute the work better.
						`,
						},
						{
							Name:        "indexed_queueing_delay_p75_per_instance",
							Description: "75th percentile job queueing delay over 5m (per instance)",
							Query:       "histogram_quantile(0.75, sum by (le, instance)(rate(index_queue_age_seconds_bucket{instance=~`${instance:regex}`}[5m])))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
							This dashboard shows the p75 job queueing delay, broken out per Zoekt instance.

							The queueing delay represents the amount of time an indexing job spent in the queue before it was processed.

							Large queueing delays can be an indicator of:
								- resource saturation
								- each Zoekt replica has too many jobs for it to be able to process all of them promptly. In this scenario, consider adding additional Zoekt replicas to distribute the work better.
						`,
						},
					},
				},
			},
			{
				Title: "Virtual Memory Statistics",
				Rows: []monitoring.Row{
					{
						{
							Name:        "memory_map_areas_percentage_used",
							Description: "process memory map areas percentage used (per instance)",
							Query:       fmt.Sprintf("(proc_metrics_memory_map_current_count{%s} / proc_metrics_memory_map_max_limit{%s}) * 100", "instance=~`${instance:regex}`", "instance=~`${instance:regex}`"),
							Panel: monitoring.Panel().LegendFormat("{{instance}}").
								Unit(monitoring.Percentage).
								With(monitoring.PanelOptions.LegendOnRight()),
							Warning:  monitoring.Alert().GreaterOrEqual(60),
							Critical: monitoring.Alert().GreaterOrEqual(80),
							Owner:    monitoring.ObservableOwnerSearchCore,

							Interpretation: `
								Processes have a limited about of memory map areas that they can use. In Zoekt, memory map areas
								are mainly used for loading shards into memory for queries (via mmap). However, memory map areas
								are also used for loading shared libraries, etc.

								_See https://en.wikipedia.org/wiki/Memory-mapped_file and the related articles for more information about memory maps._

								Once the memory map limit is reached, the Linux kernel will prevent the process from creating any
								additional memory map areas. This could cause the process to crash.
							`,
							NextSteps: `
								If you are running out of memory map areas, you could resolve this by:

								    - Enabling shard merging for Zoekt: Set SRC_ENABLE_SHARD_MERGING="1" for zoekt-indexserver. Use this option
								if your corpus of repositories has a high percentage of small, rarely updated repositories. See
								[documentation](https://sourcegraph.com/docs/code-search/features#shard-merging).
								    - Creating additional Zoekt replicas: This spreads all the shards out amongst more replicas, which
								means that each _individual_ replica will have fewer shards. This, in turn, decreases the
								amount of memory map areas that a _single_ replica can create (in order to load the shards into memory).
								    - Increasing the virtual memory subsystem's "max_map_count" parameter which defines the upper limit of memory areas
								a process can use. The default value of max_map_count is usually 65536. We recommend to set this value to 2x the number
								of repos to be indexed per Zoekt instance. This means, if you want to index 240k repositories with 3 Zoekt instances,
								set max_map_count to (240000 / 3) * 2 = 160000. The exact instructions for tuning this parameter can differ depending
								on your environment. See https://kernel.org/doc/Documentation/sysctl/vm.txt for more information.
							`,
						},
					},
				},
			},
			{
				Title:  "Compound shards",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "compound_shards_aggregate",
							Description: "# of compound shards (aggregate)",
							Query:       "sum(index_number_compound_shards) by (app)",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("aggregate").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								The total number of compound shards aggregated over all instances.

								This number should be consistent if the number of indexed repositories doesn't change.
							`,
						},
						{
							Name:        "compound_shards_per_instance",
							Description: "# of compound shards (per instance)",
							Query:       "sum(index_number_compound_shards{instance=~`${instance:regex}`}) by (instance)",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								The total number of compound shards per instance.

								This number should be consistent if the number of indexed repositories doesn't change.
							`,
						},
					},
					{
						{
							Name:        "average_shard_merging_duration_success",
							Description: "average successful shard merging duration over 1 hour",
							Query:       "sum(rate(index_shard_merging_duration_seconds_sum{error=\"false\"}[1h])) / sum(rate(index_shard_merging_duration_seconds_count{error=\"false\"}[1h]))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("average").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Average duration of a successful merge over the last hour.

								The duration depends on the target compound shard size. The larger the compound shard the longer a merge will take.
								Since the target compound shard size is set on start of zoekt-indexserver, the average duration should be consistent.
							`,
						},
						{
							Name:        "average_shard_merging_duration_error",
							Description: "average failed shard merging duration over 1 hour",
							Query:       "sum(rate(index_shard_merging_duration_seconds_sum{error=\"true\"}[1h])) / sum(rate(index_shard_merging_duration_seconds_count{error=\"true\"}[1h]))",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Average duration of a failed merge over the last hour.

								This curve should be flat. Any deviation should be investigated.
							`,
						},
					},
					{
						{
							Name:        "shard_merging_errors_aggregate",
							Description: "number of errors during shard merging (aggregate)",
							Query:       "sum(index_shard_merging_duration_seconds_count{error=\"true\"}) by (app)",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("aggregate").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Number of errors during shard merging aggregated over all instances.
							`,
						},
						{
							Name:        "shard_merging_errors_per_instance",
							Description: "number of errors during shard merging (per instance)",
							Query:       "sum(index_shard_merging_duration_seconds_count{instance=~`${instance:regex}`, error=\"true\"}) by (instance)",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Number of errors during shard merging per instance.
							`,
						},
					},
					{
						{
							Name:        "shard_merging_merge_running_per_instance",
							Description: "if shard merging is running (per instance)",
							Query:       "max by (instance) (index_shard_merging_running{instance=~`${instance:regex}`})",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Set to 1 if shard merging is running.
							`,
						},
						{
							Name:        "shard_merging_vacuum_running_per_instance",
							Description: "if vacuum is running (per instance)",
							Query:       "max by (instance) (index_vacuum_running{instance=~`${instance:regex}`})",
							NoAlert:     true,
							Panel:       monitoring.Panel().LegendFormat("{{instance}}").Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerSearchCore,
							Interpretation: `
								Set to 1 if vacuum is running.
							`,
						},
					},
				},
			},
			{
				Title:  "Network I/O pod metrics (only available on Kubernetes)",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "network_sent_bytes_aggregate",
							Description: "transmission rate over 5m (aggregate)",
							Query:       fmt.Sprintf("sum(rate(container_network_transmit_bytes_total{%s}[5m]))", shared.CadvisorPodNameMatcher(bundledContainerName)),
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat(bundledContainerName).
								Unit(monitoring.BytesPerSecond).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "The rate of bytes sent over the network across all Zoekt pods",
						},
						{
							Name:        "network_received_packets_per_instance",
							Description: "transmission rate over 5m (per instance)",
							Query:       "sum by (container_label_io_kubernetes_pod_name) (rate(container_network_transmit_bytes_total{container_label_io_kubernetes_pod_name=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}}").
								Unit(monitoring.BytesPerSecond).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "The amount of bytes sent over the network by individual Zoekt pods",
						},
					},
					{
						{
							Name:        "network_received_bytes_aggregate",
							Description: "receive rate over 5m (aggregate)",
							Query:       fmt.Sprintf("sum(rate(container_network_receive_bytes_total{%s}[5m]))", shared.CadvisorPodNameMatcher(bundledContainerName)),
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat(bundledContainerName).
								Unit(monitoring.BytesPerSecond).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "The amount of bytes received from the network across Zoekt pods",
						},
						{
							Name:        "network_received_bytes_per_instance",
							Description: "receive rate over 5m (per instance)",
							Query:       "sum by (container_label_io_kubernetes_pod_name) (rate(container_network_receive_bytes_total{container_label_io_kubernetes_pod_name=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}}").
								Unit(monitoring.BytesPerSecond).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "The amount of bytes received from the network by individual Zoekt pods",
						},
					},
					{
						{
							Name:        "network_transmitted_packets_dropped_by_instance",
							Description: "transmit packet drop rate over 5m (by instance)",
							Query:       "sum by (container_label_io_kubernetes_pod_name) (rate(container_network_transmit_packets_dropped_total{container_label_io_kubernetes_pod_name=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}}").
								Unit(monitoring.PacketsPerSecond).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "An increase in dropped packets could be a leading indicator of network saturation.",
						},
						{
							Name:        "network_transmitted_packets_errors_per_instance",
							Description: "errors encountered while transmitting over 5m (per instance)",
							Query:       "sum by (container_label_io_kubernetes_pod_name) (rate(container_network_transmit_errors_total{container_label_io_kubernetes_pod_name=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}} errors").
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "An increase in transmission errors could indicate a networking issue",
						},
						{
							Name:        "network_received_packets_dropped_by_instance",
							Description: "receive packet drop rate over 5m (by instance)",
							Query:       "sum by (container_label_io_kubernetes_pod_name) (rate(container_network_receive_packets_dropped_total{container_label_io_kubernetes_pod_name=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}}").
								Unit(monitoring.PacketsPerSecond).
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "An increase in dropped packets could be a leading indicator of network saturation.",
						},
						{
							Name:        "network_transmitted_packets_errors_by_instance",
							Description: "errors encountered while receiving over 5m (per instance)",
							Query:       "sum by (container_label_io_kubernetes_pod_name) (rate(container_network_receive_errors_total{container_label_io_kubernetes_pod_name=~`${instance:regex}`}[5m]))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{container_label_io_kubernetes_pod_name}} errors").
								With(monitoring.PanelOptions.LegendOnRight()),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "An increase in errors while receiving could indicate a networking issue.",
						},
					},
				},
			},

			shared.NewGRPCServerMetricsGroup(
				shared.GRPCServerMetricsOptions{
					HumanServiceName:   "zoekt-webserver",
					RawGRPCServiceName: grpcServiceName,

					MethodFilterRegex:    fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
					InstanceFilterRegex:  `${webserver_instance:regex}`,
					MessageSizeNamespace: "",
				}, monitoring.ObservableOwnerSearchCore),

			shared.NewGRPCInternalErrorMetricsGroup(
				shared.GRPCInternalErrorMetricsOptions{
					HumanServiceName:   "zoekt-webserver",
					RawGRPCServiceName: grpcServiceName,
					Namespace:          "src",

					MethodFilterRegex: fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
				}, monitoring.ObservableOwnerSearchCore),

			shared.NewGRPCRetryMetricsGroup(
				shared.GRPCRetryMetricsOptions{
					HumanServiceName:   "zoekt-webserver",
					RawGRPCServiceName: grpcServiceName,
					Namespace:          "src",

					MethodFilterRegex: fmt.Sprintf("${%s:regex}", grpcMethodVariable.Name),
				}, monitoring.ObservableOwnerSearchCore),

			shared.NewDiskMetricsGroup(
				shared.DiskMetricsGroupOptions{
					DiskTitle: "data",

					MetricMountNameLabel: "indexDir",
					MetricNamespace:      "zoekt_indexserver",

					ServiceName:         "zoekt",
					InstanceFilterRegex: `${instance:regex}`,
				},
				monitoring.ObservableOwnerSearchCore,
			),

			// Note:
			// zoekt_indexserver and zoekt_webserver are deployed together as part of the indexed-search service

			shared.NewContainerMonitoringGroup(indexServerContainerName, monitoring.ObservableOwnerSearchCore, &shared.ContainerMonitoringGroupOptions{
				CustomTitle: fmt.Sprintf("[%s] %s", indexServerContainerName, shared.TitleContainerMonitoring),
			}),
			shared.NewContainerMonitoringGroup(webserverContainerName, monitoring.ObservableOwnerSearchCore, &shared.ContainerMonitoringGroupOptions{
				CustomTitle: fmt.Sprintf("[%s] %s", webserverContainerName, shared.TitleContainerMonitoring),
			}),

			shared.NewProvisioningIndicatorsGroup(indexServerContainerName, monitoring.ObservableOwnerSearchCore, &shared.ContainerProvisioningIndicatorsGroupOptions{
				CustomTitle: fmt.Sprintf("[%s] %s", indexServerContainerName, shared.TitleResourceUtilization),
			}),
			shared.NewProvisioningIndicatorsGroup(webserverContainerName, monitoring.ObservableOwnerSearchCore, &shared.ContainerProvisioningIndicatorsGroupOptions{
				CustomTitle: fmt.Sprintf("[%s] %s", webserverContainerName, shared.TitleResourceUtilization),
			}),

			shared.NewGolangMonitoringGroup(indexServerJob, monitoring.ObservableOwnerSearchCore, &shared.GolangMonitoringOptions{ContainerNameInTitle: true}),
			shared.NewGolangMonitoringGroup(webserverJob, monitoring.ObservableOwnerSearchCore, &shared.GolangMonitoringOptions{ContainerNameInTitle: true}),

			// Note:
			// We show pod availability here for both the webserver and indexserver as they are bundled together.
			shared.NewKubernetesMonitoringGroup(bundledContainerName, monitoring.ObservableOwnerSearchCore, nil),
		},
	}
}

func zoektHeatMapPanelOptions(_ monitoring.Observable, p *sdk.Panel) {
	p.DataFormat = "tsbuckets"

	targets := p.GetTargets()
	if targets != nil {
		for _, t := range *targets {
			t.Format = "heatmap"
			t.LegendFormat = "{{le}}"

			p.SetTarget(&t)
		}
	}

	p.HeatmapPanel.YAxis.Format = string(monitoring.Seconds)
	p.HeatmapPanel.YBucketBound = "upper"

	p.HideZeroBuckets = true
	p.Color.Mode = "spectrum"
	p.Color.ColorScheme = "interpolateSpectral"
}
