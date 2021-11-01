package definitions

import (
	"time"

	"github.com/sourcegraph/sourcegraph/monitoring/definitions/shared"
	"github.com/sourcegraph/sourcegraph/monitoring/monitoring"

	"github.com/grafana-tools/sdk"
)

func Frontend() *monitoring.Container {
	// frontend is sometimes called sourcegraph-frontend in various contexts
	const containerName = "(frontend|sourcegraph-frontend)"

	var sentinelSamplingIntervals []string
	for _, d := range []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		10 * time.Minute,
		30 * time.Minute,
		1 * time.Hour,
		90 * time.Minute,
		3 * time.Hour,
	} {
		sentinelSamplingIntervals = append(sentinelSamplingIntervals, d.Round(time.Second).String())
	}

	defaultSamplingInterval := (90 * time.Minute).Round(time.Second)

	orgMetricSpec := []struct{ name, route, description string }{
		{"org_members", "OrganizationMembers", "API requests to list organisation members"},
		{"create_org", "CreateOrganization", "API requests to create an organisation"},
		{"remove_org_member", "RemoveUserFromOrganization", "API requests to remove organisation member"},
		{"invite_org_member", "InviteUserToOrganization", "API requests to invite a new organisation member"},
		{"org_invite_respond", "RespondToOrganizationInvitation", "API requests to respond to an org invitation"},
		{"org_repositories", "OrgRepositories", "API requests to list repositories owned by an org"},
	}

	return &monitoring.Container{
		Name:        "frontend",
		Title:       "Frontend",
		Description: "Serves all end-user browser and API requests.",
		Variables: []monitoring.ContainerVariable{{
			Name:  "sentinel_sampling_duration",
			Label: "Sentinel query sampling duration",
			Options: monitoring.ContainerVariableOptions{
				Type:          monitoring.OptionTypeInterval,
				Options:       sentinelSamplingIntervals,
				DefaultOption: defaultSamplingInterval.String(),
			},
		}},
		Groups: []monitoring.Group{
			{
				Title: "Search at a glance",
				Rows: []monitoring.Row{
					{
						{
							Name:        "99th_percentile_search_request_duration",
							Description: "99th percentile successful search request duration over 5m",
							Query:       `histogram_quantile(0.99, sum by (le)(rate(src_search_streaming_latency_seconds_bucket{source="browser"}[5m])))`,

							Warning: monitoring.Alert().GreaterOrEqual(20),
							Panel:   monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- **Get details on the exact queries that are slow** by configuring '"observability.logSlowSearches": 20,' in the site configuration and looking for 'frontend' warning logs prefixed with 'slow search request' for additional details.
								- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
								- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the 'indexed-search.Deployment.yaml' if regularly hitting max CPU utilization.
								- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing 'cpus:' of the zoekt-webserver container in 'docker-compose.yml' if regularly hitting max CPU utilization.
							`,
						},
						{
							Name:        "90th_percentile_search_request_duration",
							Description: "90th percentile successful search request duration over 5m",
							Query:       `histogram_quantile(0.90, sum by (le)(rate(src_search_streaming_latency_seconds_bucket{source="browser"}[5m])))`,

							Warning: monitoring.Alert().GreaterOrEqual(15),
							Panel:   monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- **Get details on the exact queries that are slow** by configuring '"observability.logSlowSearches": 15,' in the site configuration and looking for 'frontend' warning logs prefixed with 'slow search request' for additional details.
								- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
								- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the 'indexed-search.Deployment.yaml' if regularly hitting max CPU utilization.
								- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing 'cpus:' of the zoekt-webserver container in 'docker-compose.yml' if regularly hitting max CPU utilization.
							`,
						},
					},
					{
						{
							Name:        "hard_timeout_search_responses",
							Description: "hard timeout search responses every 5m",
							Query:       `(sum(increase(src_graphql_search_response{status="timeout",source="browser",request_name!="CodeIntelSearch"}[5m])) + sum(increase(src_graphql_search_response{status="alert",alert_type="timed_out",source="browser",request_name!="CodeIntelSearch"}[5m]))) / sum(increase(src_graphql_search_response{source="browser",request_name!="CodeIntelSearch"}[5m])) * 100`,

							Warning:           monitoring.Alert().GreaterOrEqual(2).For(15 * time.Minute),
							Critical:          monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("hard timeout").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "hard_error_search_responses",
							Description: "hard error search responses every 5m",
							Query:       `sum by (status)(increase(src_graphql_search_response{status=~"error",source="browser",request_name!="CodeIntelSearch"}[5m])) / ignoring(status) group_left sum(increase(src_graphql_search_response{source="browser",request_name!="CodeIntelSearch"}[5m])) * 100`,

							Warning:           monitoring.Alert().GreaterOrEqual(2).For(15 * time.Minute),
							Critical:          monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("{{status}}").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "partial_timeout_search_responses",
							Description: "partial timeout search responses every 5m",
							Query:       `sum by (status)(increase(src_graphql_search_response{status="partial_timeout",source="browser",request_name!="CodeIntelSearch"}[5m])) / ignoring(status) group_left sum(increase(src_graphql_search_response{source="browser",request_name!="CodeIntelSearch"}[5m])) * 100`,

							Warning:           monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("{{status}}").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "search_alert_user_suggestions",
							Description: "search alert user suggestions shown every 5m",
							Query:       `sum by (alert_type)(increase(src_graphql_search_response{status="alert",alert_type!~"timed_out|no_results__suggest_quotes",source="browser",request_name!="CodeIntelSearch"}[5m])) / ignoring(alert_type) group_left sum(increase(src_graphql_search_response{source="browser",request_name!="CodeIntelSearch"}[5m])) * 100`,

							Warning: monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:   monitoring.Panel().LegendFormat("{{alert_type}}").Unit(monitoring.Percentage),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- This indicates your user's are making syntax errors or similar user errors.
							`,
						},
					},
					{
						{
							Name:        "page_load_latency",
							Description: "90th percentile page load latency over all routes over 10m",
							Query:       `histogram_quantile(0.9, sum by(le) (rate(src_http_request_duration_seconds_bucket{route!="raw",route!="blob",route!~"graphql.*"}[10m])))`,

							Critical: monitoring.Alert().GreaterOrEqual(2),
							Panel:    monitoring.Panel().LegendFormat("latency").Unit(monitoring.Seconds),
							Owner:    monitoring.ObservableOwnerCloudSaaS,
							PossibleSolutions: `
								- Confirm that the Sourcegraph frontend has enough CPU/memory using the provisioning panels.
								- Explore the data returned by the query in the dashboard panel and filter by different labels to identify any patterns
								- Trace a request to see what the slowest part is: https://docs.sourcegraph.com/admin/observability/tracing
							`,
							Interpretation: `
								Investigate potential sources of latency by selecting Explore and modifying the 'sum by(le)' section to include additional labels: for example, 'sum by(le, job)' or 'sum by (le, instance)'.
							`,
						},
						{
							Name:        "blob_load_latency",
							Description: "90th percentile blob load latency over 10m",
							Query:       `histogram_quantile(0.9, sum by(le) (rate(src_http_request_duration_seconds_bucket{route="blob"}[10m])))`,
							Critical:    monitoring.Alert().GreaterOrEqual(5),
							Panel:       monitoring.Panel().LegendFormat("latency").Unit(monitoring.Seconds),
							Owner:       monitoring.ObservableOwnerRepoManagement,
							PossibleSolutions: `
								- Confirm that the Sourcegraph frontend has enough CPU/memory using the provisioning panels.
								- Trace a request to see what the slowest part is: https://docs.sourcegraph.com/admin/observability/tracing
								- Check that gitserver containers have enough CPU/memory and are not getting throttled.
							`,
						},
					},
				},
			},
			{
				Title:  "Search-based code intelligence at a glance",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "99th_percentile_search_codeintel_request_duration",
							Description: "99th percentile code-intel successful search request duration over 5m",
							Owner:       monitoring.ObservableOwnerSearch,
							Query:       `histogram_quantile(0.99, sum by (le)(rate(src_graphql_field_seconds_bucket{type="Search",field="results",error="false",source="browser",request_name="CodeIntelSearch"}[5m])))`,

							Warning: monitoring.Alert().GreaterOrEqual(20),
							Panel:   monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
							PossibleSolutions: `
								- **Get details on the exact queries that are slow** by configuring '"observability.logSlowSearches": 20,' in the site configuration and looking for 'frontend' warning logs prefixed with 'slow search request' for additional details.
								- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
								- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the 'indexed-search.Deployment.yaml' if regularly hitting max CPU utilization.
								- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing 'cpus:' of the zoekt-webserver container in 'docker-compose.yml' if regularly hitting max CPU utilization.
								- This alert may indicate that your instance is struggling to process symbols queries on a monorepo, [learn more here](../how-to/monorepo-issues.md).
							`,
						},
						{
							Name:        "90th_percentile_search_codeintel_request_duration",
							Description: "90th percentile code-intel successful search request duration over 5m",
							Query:       `histogram_quantile(0.90, sum by (le)(rate(src_graphql_field_seconds_bucket{type="Search",field="results",error="false",source="browser",request_name="CodeIntelSearch"}[5m])))`,

							Warning: monitoring.Alert().GreaterOrEqual(15),
							Panel:   monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- **Get details on the exact queries that are slow** by configuring '"observability.logSlowSearches": 15,' in the site configuration and looking for 'frontend' warning logs prefixed with 'slow search request' for additional details.
								- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
								- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the 'indexed-search.Deployment.yaml' if regularly hitting max CPU utilization.
								- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing 'cpus:' of the zoekt-webserver container in 'docker-compose.yml' if regularly hitting max CPU utilization.
								- This alert may indicate that your instance is struggling to process symbols queries on a monorepo, [learn more here](../how-to/monorepo-issues.md).
							`,
						},
					},
					{
						{
							Name:        "hard_timeout_search_codeintel_responses",
							Description: "hard timeout search code-intel responses every 5m",
							Query:       `(sum(increase(src_graphql_search_response{status="timeout",source="browser",request_name="CodeIntelSearch"}[5m])) + sum(increase(src_graphql_search_response{status="alert",alert_type="timed_out",source="browser",request_name="CodeIntelSearch"}[5m]))) / sum(increase(src_graphql_search_response{source="browser",request_name="CodeIntelSearch"}[5m])) * 100`,

							Warning:           monitoring.Alert().GreaterOrEqual(2).For(15 * time.Minute),
							Critical:          monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("hard timeout").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "hard_error_search_codeintel_responses",
							Description: "hard error search code-intel responses every 5m",
							Query:       `sum by (status)(increase(src_graphql_search_response{status=~"error",source="browser",request_name="CodeIntelSearch"}[5m])) / ignoring(status) group_left sum(increase(src_graphql_search_response{source="browser",request_name="CodeIntelSearch"}[5m])) * 100`,

							Warning:           monitoring.Alert().GreaterOrEqual(2).For(15 * time.Minute),
							Critical:          monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("hard error").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "partial_timeout_search_codeintel_responses",
							Description: "partial timeout search code-intel responses every 5m",
							Query:       `sum by (status)(increase(src_graphql_search_response{status="partial_timeout",source="browser",request_name="CodeIntelSearch"}[5m])) / ignoring(status) group_left sum(increase(src_graphql_search_response{status="partial_timeout",source="browser",request_name="CodeIntelSearch"}[5m])) * 100`,

							Warning:           monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("partial timeout").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "search_codeintel_alert_user_suggestions",
							Description: "search code-intel alert user suggestions shown every 5m",
							Query:       `sum by (alert_type)(increase(src_graphql_search_response{status="alert",alert_type!~"timed_out",source="browser",request_name="CodeIntelSearch"}[5m])) / ignoring(alert_type) group_left sum(increase(src_graphql_search_response{source="browser",request_name="CodeIntelSearch"}[5m])) * 100`,

							Warning: monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:   monitoring.Panel().LegendFormat("{{alert_type}}").Unit(monitoring.Percentage),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- This indicates a bug in Sourcegraph, please [open an issue](https://github.com/sourcegraph/sourcegraph/issues/new/choose).
							`,
						},
					},
				},
			},
			{
				Title:  "Search GraphQL API usage at a glance",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "99th_percentile_search_api_request_duration",
							Description: "99th percentile successful search API request duration over 5m",
							Query:       `histogram_quantile(0.99, sum by (le)(rate(src_graphql_field_seconds_bucket{type="Search",field="results",error="false",source="other"}[5m])))`,

							Warning: monitoring.Alert().GreaterOrEqual(50),
							Panel:   monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- **Get details on the exact queries that are slow** by configuring '"observability.logSlowSearches": 20,' in the site configuration and looking for 'frontend' warning logs prefixed with 'slow search request' for additional details.
								- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
								- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the 'indexed-search.Deployment.yaml' if regularly hitting max CPU utilization.
								- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing 'cpus:' of the zoekt-webserver container in 'docker-compose.yml' if regularly hitting max CPU utilization.
							`,
						},
						{
							Name:        "90th_percentile_search_api_request_duration",
							Description: "90th percentile successful search API request duration over 5m",
							Query:       `histogram_quantile(0.90, sum by (le)(rate(src_graphql_field_seconds_bucket{type="Search",field="results",error="false",source="other"}[5m])))`,

							Warning: monitoring.Alert().GreaterOrEqual(40),
							Panel:   monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- **Get details on the exact queries that are slow** by configuring '"observability.logSlowSearches": 15,' in the site configuration and looking for 'frontend' warning logs prefixed with 'slow search request' for additional details.
								- **Check that most repositories are indexed** by visiting https://sourcegraph.example.com/site-admin/repositories?filter=needs-index (it should show few or no results.)
								- **Kubernetes:** Check CPU usage of zoekt-webserver in the indexed-search pod, consider increasing CPU limits in the 'indexed-search.Deployment.yaml' if regularly hitting max CPU utilization.
								- **Docker Compose:** Check CPU usage on the Zoekt Web Server dashboard, consider increasing 'cpus:' of the zoekt-webserver container in 'docker-compose.yml' if regularly hitting max CPU utilization.
							`,
						},
					},
					{
						{
							Name:        "hard_error_search_api_responses",
							Description: "hard error search API responses every 5m",
							Query:       `sum by (status)(increase(src_graphql_search_response{status=~"error",source="other"}[5m])) / ignoring(status) group_left sum(increase(src_graphql_search_response{source="other"}[5m]))`,

							Warning:           monitoring.Alert().GreaterOrEqual(2).For(15 * time.Minute),
							Critical:          monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("{{status}}").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "partial_timeout_search_api_responses",
							Description: "partial timeout search API responses every 5m",
							Query:       `sum(increase(src_graphql_search_response{status="partial_timeout",source="other"}[5m])) / sum(increase(src_graphql_search_response{source="other"}[5m]))`,

							Warning:           monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("partial timeout").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerSearch,
							PossibleSolutions: "none",
						},
						{
							Name:        "search_api_alert_user_suggestions",
							Description: "search API alert user suggestions shown every 5m",
							Query:       `sum by (alert_type)(increase(src_graphql_search_response{status="alert",alert_type!~"timed_out|no_results__suggest_quotes",source="other"}[5m])) / ignoring(alert_type) group_left sum(increase(src_graphql_search_response{status="alert",source="other"}[5m]))`,

							Warning: monitoring.Alert().GreaterOrEqual(5),
							Panel:   monitoring.Panel().LegendFormat("{{alert_type}}").Unit(monitoring.Percentage),
							Owner:   monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- This indicates your user's search API requests have syntax errors or a similar user error. Check the responses the API sends back for an explanation.
							`,
						},
					},
				},
			},

			shared.CodeIntelligence.NewResolversGroup(containerName),
			shared.CodeIntelligence.NewAutoIndexEnqueuerGroup(containerName),
			shared.CodeIntelligence.NewDBStoreGroup(containerName),
			shared.CodeIntelligence.NewIndexDBWorkerStoreGroup(containerName),
			shared.CodeIntelligence.NewLSIFStoreGroup(containerName),
			shared.CodeIntelligence.NewGitserverClientGroup(containerName),
			shared.CodeIntelligence.NewRepoUpdaterClientGroup(containerName),
			shared.CodeIntelligence.NewUploadStoreGroup(containerName),
			shared.CodeIntelligence.NewDependencyServiceGroup(containerName),
			shared.CodeIntelligence.NewLockfilesGroup(containerName),

			shared.Batches.NewDBStoreGroup(containerName),
			shared.Batches.NewServiceGroup(containerName),

			// src_oobmigration_total
			// src_oobmigration_duration_seconds_bucket
			// src_oobmigration_errors_total
			shared.Observation.NewGroup(containerName, monitoring.ObservableOwnerCodeIntel, shared.ObservationGroupOptions{
				GroupConstructorOptions: shared.GroupConstructorOptions{
					Namespace:       "Out-of-band migrations",
					DescriptionRoot: "up migration invocation (one batch processed)",
					Hidden:          true,

					ObservableConstructorOptions: shared.ObservableConstructorOptions{
						MetricNameRoot:        "oobmigration",
						MetricDescriptionRoot: "migration handler",
						Filters:               []string{`op="up"`},
					},
				},

				SharedObservationGroupOptions: shared.SharedObservationGroupOptions{
					Total:     shared.NoAlertsOption("none"),
					Duration:  shared.NoAlertsOption("none"),
					Errors:    shared.NoAlertsOption("none"),
					ErrorRate: shared.NoAlertsOption("none"),
				},
			}),

			// src_oobmigration_total
			// src_oobmigration_duration_seconds_bucket
			// src_oobmigration_errors_total
			shared.Observation.NewGroup(containerName, monitoring.ObservableOwnerCodeIntel, shared.ObservationGroupOptions{
				GroupConstructorOptions: shared.GroupConstructorOptions{
					Namespace:       "Out-of-band migrations",
					DescriptionRoot: "down migration invocation (one batch processed)",
					Hidden:          true,

					ObservableConstructorOptions: shared.ObservableConstructorOptions{
						MetricNameRoot:        "oobmigration",
						MetricDescriptionRoot: "migration handler",
						Filters:               []string{`op="down"`},
					},
				},

				SharedObservationGroupOptions: shared.SharedObservationGroupOptions{
					Total:     shared.NoAlertsOption("none"),
					Duration:  shared.NoAlertsOption("none"),
					Errors:    shared.NoAlertsOption("none"),
					ErrorRate: shared.NoAlertsOption("none"),
				},
			}),

			{
				Title:  "Internal service requests",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "internal_indexed_search_error_responses",
							Description: "internal indexed search error responses every 5m",
							Query:       `sum by(code) (increase(src_zoekt_request_duration_seconds_count{code!~"2.."}[5m])) / ignoring(code) group_left sum(increase(src_zoekt_request_duration_seconds_count[5m])) * 100`,
							Warning:     monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:       monitoring.Panel().LegendFormat("{{code}}").Unit(monitoring.Percentage),
							Owner:       monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- Check the Zoekt Web Server dashboard for indications it might be unhealthy.
							`,
						},
						{
							Name:        "internal_unindexed_search_error_responses",
							Description: "internal unindexed search error responses every 5m",
							Query:       `sum by(code) (increase(searcher_service_request_total{code!~"2.."}[5m])) / ignoring(code) group_left sum(increase(searcher_service_request_total[5m])) * 100`,
							Warning:     monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:       monitoring.Panel().LegendFormat("{{code}}").Unit(monitoring.Percentage),
							Owner:       monitoring.ObservableOwnerSearch,
							PossibleSolutions: `
								- Check the Searcher dashboard for indications it might be unhealthy.
							`,
						},
						{
							Name:        "internalapi_error_responses",
							Description: "internal API error responses every 5m by route",
							Query:       `sum by(category) (increase(src_frontend_internal_request_duration_seconds_count{code!~"2.."}[5m])) / ignoring(code) group_left sum(increase(src_frontend_internal_request_duration_seconds_count[5m])) * 100`,
							Warning:     monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:       monitoring.Panel().LegendFormat("{{category}}").Unit(monitoring.Percentage),
							Owner:       monitoring.ObservableOwnerCloudSaaS,
							PossibleSolutions: `
								- May not be a substantial issue, check the 'frontend' logs for potential causes.
							`,
						},
					},
					{
						{
							Name:              "99th_percentile_gitserver_duration",
							Description:       "99th percentile successful gitserver query duration over 5m",
							Query:             `histogram_quantile(0.99, sum by (le,category)(rate(src_gitserver_request_duration_seconds_bucket{job=~"(sourcegraph-)?frontend"}[5m])))`,
							Warning:           monitoring.Alert().GreaterOrEqual(20),
							Panel:             monitoring.Panel().LegendFormat("{{category}}").Unit(monitoring.Seconds),
							Owner:             monitoring.ObservableOwnerRepoManagement,
							PossibleSolutions: "none",
						},
						{
							Name:              "gitserver_error_responses",
							Description:       "gitserver error responses every 5m",
							Query:             `sum by (category)(increase(src_gitserver_request_duration_seconds_count{job=~"(sourcegraph-)?frontend",code!~"2.."}[5m])) / ignoring(code) group_left sum by (category)(increase(src_gitserver_request_duration_seconds_count{job=~"(sourcegraph-)?frontend"}[5m])) * 100`,
							Warning:           monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Panel:             monitoring.Panel().LegendFormat("{{category}}").Unit(monitoring.Percentage),
							Owner:             monitoring.ObservableOwnerRepoManagement,
							PossibleSolutions: "none",
						},
					},
					{
						{
							Name:              "observability_test_alert_warning",
							Description:       "warning test alert metric",
							Query:             `max by(owner) (observability_test_metric_warning)`,
							Warning:           monitoring.Alert().GreaterOrEqual(1),
							Panel:             monitoring.Panel().Max(1),
							Owner:             monitoring.ObservableOwnerDevOps,
							PossibleSolutions: "This alert is triggered via the `triggerObservabilityTestAlert` GraphQL endpoint, and will automatically resolve itself.",
						},
						{
							Name:              "observability_test_alert_critical",
							Description:       "critical test alert metric",
							Query:             `max by(owner) (observability_test_metric_critical)`,
							Critical:          monitoring.Alert().GreaterOrEqual(1),
							Panel:             monitoring.Panel().Max(1),
							Owner:             monitoring.ObservableOwnerDevOps,
							PossibleSolutions: "This alert is triggered via the `triggerObservabilityTestAlert` GraphQL endpoint, and will automatically resolve itself.",
						},
					},
				},
			},
			{
				Title:  "Authentication API requests",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:           "sign_in_rate",
							Description:    "rate of API requests to sign-in",
							Query:          `sum(irate(src_http_request_duration_seconds_count{route="sign-in",method="post"}[5m]))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Rate (QPS) of requests to sign-in`,
						},
						{
							Name:           "sign_in_latency_p99",
							Description:    "99 percentile of sign-in latency",
							Query:          `histogram_quantile(0.99, sum(rate(src_http_request_duration_seconds_bucket{route="sign-in",method="post"}[5m])) by (le))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Milliseconds),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `99% percentile of sign-in latency`,
						},
						{
							Name:           "sign_in_error_rate",
							Description:    "percentage of sign-in requests by http code",
							Query:          `sum by (code)(irate(src_http_request_duration_seconds_count{route="sign-in",method="post"}[5m]))/ ignoring (code) group_left sum(irate(src_http_request_duration_seconds_count{route="sign-in",method="post"}[5m]))*100`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Percentage),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Percentage of sign-in requests grouped by http code`,
						},
					},
					{
						{
							Name:        "sign_up_rate",
							Description: "rate of API requests to sign-up",
							Query:       `sum(irate(src_http_request_duration_seconds_count{route="sign-up",method="post"}[5m]))`,

							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Rate (QPS) of requests to sign-up`,
						},
						{
							Name:        "sign_up_latency_p99",
							Description: "99 percentile of sign-up latency",

							Query:          `histogram_quantile(0.99, sum(rate(src_http_request_duration_seconds_bucket{route="sign-up",method="post"}[5m])) by (le))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Milliseconds),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `99% percentile of sign-up latency`,
						},
						{
							Name:           "sign_up_code_percentage",
							Description:    "percentage of sign-up requests by http code",
							Query:          `sum by (code)(irate(src_http_request_duration_seconds_count{route="sign-up",method="post"}[5m]))/ ignoring (code) group_left sum(irate(src_http_request_duration_seconds_count{route="sign-out"}[5m]))*100`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Percentage),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Percentage of sign-up requests grouped by http code`,
						},
					},
					{
						{
							Name:           "sign_out_rate",
							Description:    "rate of API requests to sign-out",
							Query:          `sum(irate(src_http_request_duration_seconds_count{route="sign-out"}[5m]))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Rate (QPS) of requests to sign-out`,
						},
						{
							Name:           "sign_out_latency_p99",
							Description:    "99 percentile of sign-out latency",
							Query:          `histogram_quantile(0.99, sum(rate(src_http_request_duration_seconds_bucket{route="sign-out"}[5m])) by (le))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Milliseconds),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `99% percentile of sign-out latency`,
						},
						{
							Name:           "sign_out_error_rate",
							Description:    "percentage of sign-out requests that return non-303 http code",
							Query:          ` sum by (code)(irate(src_http_request_duration_seconds_count{route="sign-out"}[5m]))/ ignoring (code) group_left sum(irate(src_http_request_duration_seconds_count{route="sign-out"}[5m]))*100`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Percentage),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Percentage of sign-out requests grouped by http code`,
						},
					},
				}},
			{
				Title:  "Organisation GraphQL API requests",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:           "org_members_rate",
							Description:    "rate of API requests to list organisation members",
							Query:          `sum(irate(src_graphql_request_duration_seconds_count{route="OrganizationMembers"}[5m]))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.RequestsPerSecond),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Rate (QPS) of requests to list organisation members`,
						},
						{
							Name:           "org_members_latency_p99",
							Description:    "99 percentile latency of API requests to list organisation members",
							Query:          `histogram_quantile(0.99, sum(rate(src_graphql_request_duration_seconds_bucket{route="OrganizationMembers"}[5m])) by (le))`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Milliseconds),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `99 percentile of org-members latency`,
						},
						{
							Name:           "org_members_error_rate",
							Description:    "percentage of API requests to list organisation members that return an error",
							Query:          `sum (irate(src_graphql_request_duration_seconds_count{route="OrganizationMembers",success="false"}[5m]))/sum(irate(src_graphql_request_duration_seconds_count{route="OrganizationMembers"}[5m]))*100`,
							NoAlert:        true,
							Panel:          monitoring.Panel().Unit(monitoring.Percentage),
							Owner:          monitoring.ObservableOwnerRepoManagement,
							Interpretation: `Percentage of org-members API requests that return an error`,
						},
					},
				}},
			{
				Title:  "Cloud KMS and cache",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "cloudkms_cryptographic_requests",
							Description: "cryptographic requests to Cloud KMS every 1m",
							Query:       `sum(increase(src_cloudkms_cryptographic_total[1m]))`,
							Warning:     monitoring.Alert().GreaterOrEqual(15000).For(5 * time.Minute),
							Critical:    monitoring.Alert().GreaterOrEqual(30000).For(5 * time.Minute),
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerRepoManagement,
							PossibleSolutions: `
								- Revert recent commits that cause extensive listing from "external_services" and/or "user_external_accounts" tables.
							`,
						},
						{
							Name:        "encryption_cache_hit_ratio",
							Description: "average encryption cache hit ratio per workload",
							Query:       `min by (kubernetes_name) (src_encryption_cache_hit_total/(src_encryption_cache_hit_total+src_encryption_cache_miss_total))`,
							NoAlert:     true,
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerRepoManagement,
							Interpretation: `
								- Encryption cache hit ratio (hits/(hits+misses)) - minimum across all instances of a workload.
							`,
						},
						{
							Name:        "encryption_cache_evictions",
							Description: "rate of encryption cache evictions - sum across all instances of a given workload",
							Query:       `sum by (kubernetes_name) (irate(src_encryption_cache_eviction_total[5m]))`,
							NoAlert:     true,
							Panel:       monitoring.Panel().Unit(monitoring.Number),
							Owner:       monitoring.ObservableOwnerRepoManagement,
							Interpretation: `
								- Rate of encryption cache evictions (caused by cache exceeding its maximum size) - sum across all instances of a workload
							`,
						},
					},
				},
			},

			// Resource monitoring
			shared.NewDatabaseConnectionsMonitoringGroup("frontend"),
			shared.NewContainerMonitoringGroup(containerName, monitoring.ObservableOwnerDevOps, nil),
			shared.NewProvisioningIndicatorsGroup(containerName, monitoring.ObservableOwnerDevOps, nil),
			shared.NewGolangMonitoringGroup(containerName, monitoring.ObservableOwnerDevOps, nil),
			shared.NewKubernetesMonitoringGroup(containerName, monitoring.ObservableOwnerDevOps, nil),
			{
				Title:  "Ranking",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "mean_position_of_clicked_search_result_6h",
							Description: "mean position of clicked search result over 6h",
							Query:       "sum by (type) (rate(src_search_ranking_result_clicked_sum[6h]))/sum by (type) (rate(src_search_ranking_result_clicked_count[6h]))",
							NoAlert:     true,
							Panel: monitoring.Panel().With(
								monitoring.PanelOptions.LegendOnRight(),
								func(o monitoring.Observable, p *sdk.Panel) {
									p.GraphPanel.Legend.Current = true
									p.GraphPanel.Targets = []sdk.Target{{
										Expr:         o.Query,
										LegendFormat: "{{type}}",
									}, {
										Expr:         "sum by (app) (rate(src_search_ranking_result_clicked_sum[6h]))/sum by (app) (rate(src_search_ranking_result_clicked_count[6h]))",
										LegendFormat: "all",
									}}
									p.GraphPanel.Tooltip.Shared = true
								}),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "The top-most result on the search results has position 0. Low values are considered better. This metric only tracks top-level items and not individual line matches.",
						},
						{
							Name:        "distribution_of_clicked_search_result_type_over_6h_in_percent",
							Description: "distribution of clicked search result type over 6h in %",
							Query:       "round(sum(increase(src_search_ranking_result_clicked_sum{type=\"commit\"}[6h])) / sum (increase(src_search_ranking_result_clicked_sum[6h]))*100)",
							NoAlert:     true,
							Panel: monitoring.Panel().With(
								monitoring.PanelOptions.LegendOnRight(),
								func(o monitoring.Observable, p *sdk.Panel) {
									p.GraphPanel.Legend.Current = true
									p.GraphPanel.Targets = []sdk.Target{{
										Expr:         o.Query,
										LegendFormat: "commit",
									}, {
										Expr:         "round(sum(increase(src_search_ranking_result_clicked_sum{type=\"fileMatch\"}[6h])) / sum (increase(src_search_ranking_result_clicked_sum[6h]))*100)",
										LegendFormat: "fileMatch",
									}, {
										Expr:         "round(sum(increase(src_search_ranking_result_clicked_sum{type=\"repo\"}[6h])) / sum (increase(src_search_ranking_result_clicked_sum[6h]))*100)",
										LegendFormat: "repo",
									}}
									p.GraphPanel.Tooltip.Shared = true
								}),
							Owner:          monitoring.ObservableOwnerSearchCore,
							Interpretation: "The distribution of clicked search results by result type. At every point in time, the values should sum to 100.",
						},
					},
				},
			},

			{
				Title:  "Sentinel queries (only on sourcegraph.com)",
				Hidden: true,
				Rows: []monitoring.Row{
					{
						{
							Name:        "mean_successful_sentinel_duration_over_1h30m",
							Description: "mean successful sentinel search duration over 1h30m",
							// WARNING: if you change this, ensure that it will not trigger alerts on a customer instance
							// since these panels relate to metrics that don't exist on a customer instance.
							Query:          "sum(rate(src_search_response_latency_seconds_sum{source=~`searchblitz.*`, status=`success`}[1h30m])) / sum(rate(src_search_response_latency_seconds_count{source=~`searchblitz.*`, status=`success`}[1h30m]))",
							Warning:        monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Critical:       monitoring.Alert().GreaterOrEqual(8).For(30 * time.Minute),
							Panel:          monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds).With(monitoring.PanelOptions.NoLegend()),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `Mean search duration for all successful sentinel queries`,
							PossibleSolutions: `
								- Look at the breakdown by query to determine if a specific query type is being affected
								- Check for high CPU usage on zoekt-webserver
								- Check Honeycomb for unusual activity
							`,
						},
						{
							Name:        "mean_sentinel_stream_latency_over_1h30m",
							Description: "mean successful sentinel stream latency over 1h30m",
							// WARNING: if you change this, ensure that it will not trigger alerts on a customer instance
							// since these panels relate to metrics that don't exist on a customer instance.
							Query:    `sum(rate(src_search_streaming_latency_seconds_sum{source=~"searchblitz.*"}[1h30m])) / sum(rate(src_search_streaming_latency_seconds_count{source=~"searchblitz.*"}[1h30m]))`,
							Warning:  monitoring.Alert().GreaterOrEqual(2).For(15 * time.Minute),
							Critical: monitoring.Alert().GreaterOrEqual(3).For(30 * time.Minute),
							Panel: monitoring.Panel().LegendFormat("latency").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.NoLegend(),
								monitoring.PanelOptions.ColorOverride("latency", "#8AB8FF"),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `Mean time to first result for all successful streaming sentinel queries`,
							PossibleSolutions: `
								- Look at the breakdown by query to determine if a specific query type is being affected
								- Check for high CPU usage on zoekt-webserver
								- Check Honeycomb for unusual activity
							`,
						},
					},
					{
						{
							Name:        "90th_percentile_successful_sentinel_duration_over_1h30m",
							Description: "90th percentile successful sentinel search duration over 1h30m",
							// WARNING: if you change this, ensure that it will not trigger alerts on a customer instance
							// since these panels relate to metrics that don't exist on a customer instance.
							Query:          `histogram_quantile(0.90, sum by (le)(label_replace(rate(src_search_response_latency_seconds_bucket{source=~"searchblitz.*", status="success"}[1h30m]), "source", "$1", "source", "searchblitz_(.*)")))`,
							Warning:        monitoring.Alert().GreaterOrEqual(5).For(15 * time.Minute),
							Critical:       monitoring.Alert().GreaterOrEqual(10).For(30 * time.Minute),
							Panel:          monitoring.Panel().LegendFormat("duration").Unit(monitoring.Seconds).With(monitoring.PanelOptions.NoLegend()),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `90th percentile search duration for all successful sentinel queries`,
							PossibleSolutions: `
								- Look at the breakdown by query to determine if a specific query type is being affected
								- Check for high CPU usage on zoekt-webserver
								- Check Honeycomb for unusual activity
							`,
						},
						{
							Name:        "90th_percentile_sentinel_stream_latency_over_1h30m",
							Description: "90th percentile successful sentinel stream latency over 1h30m",
							// WARNING: if you change this, ensure that it will not trigger alerts on a customer instance
							// since these panels relate to metrics that don't exist on a customer instance.
							Query:    `histogram_quantile(0.90, sum by (le)(label_replace(rate(src_search_streaming_latency_seconds_bucket{source=~"searchblitz.*"}[1h30m]), "source", "$1", "source", "searchblitz_(.*)")))`,
							Warning:  monitoring.Alert().GreaterOrEqual(4).For(15 * time.Minute),
							Critical: monitoring.Alert().GreaterOrEqual(6).For(30 * time.Minute),
							Panel: monitoring.Panel().LegendFormat("latency").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.NoLegend(),
								monitoring.PanelOptions.ColorOverride("latency", "#8AB8FF"),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `90th percentile time to first result for all successful streaming sentinel queries`,
							PossibleSolutions: `
								- Look at the breakdown by query to determine if a specific query type is being affected
								- Check for high CPU usage on zoekt-webserver
								- Check Honeycomb for unusual activity
							`,
						},
					},
					{
						{
							Name:        "mean_successful_sentinel_duration_by_query",
							Description: "mean successful sentinel search duration by query",
							Query:       `sum(rate(src_search_response_latency_seconds_sum{source=~"searchblitz.*", status="success"}[$sentinel_sampling_duration])) by (source) / sum(rate(src_search_response_latency_seconds_count{source=~"searchblitz.*", status="success"}[$sentinel_sampling_duration])) by (source)`,
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{query}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `Mean search duration for successful sentinel queries, broken down by query. Useful for debugging whether a slowdown is limited to a specific type of query.`,
						},
						{
							Name:        "mean_sentinel_stream_latency_by_query",
							Description: "mean successful sentinel stream latency by query",
							Query:       `sum(rate(src_search_streaming_latency_seconds_sum{source=~"searchblitz.*"}[$sentinel_sampling_duration])) by (source) / sum(rate(src_search_streaming_latency_seconds_count{source=~"searchblitz.*"}[$sentinel_sampling_duration])) by (source)`,
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{query}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `Mean time to first result for successful streaming sentinel queries, broken down by query. Useful for debugging whether a slowdown is limited to a specific type of query.`,
						},
					},
					{
						{
							Name:        "90th_percentile_successful_sentinel_duration_by_query",
							Description: "90th percentile successful sentinel search duration by query",
							Query:       `histogram_quantile(0.90, sum(rate(src_search_response_latency_seconds_bucket{source=~"searchblitz.*", status="success"}[$sentinel_sampling_duration])) by (le, source))`,
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{query}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `90th percentile search duration for successful sentinel queries, broken down by query. Useful for debugging whether a slowdown is limited to a specific type of query.`,
						},
						{
							Name:        "90th_percentile_successful_stream_latency_by_query",
							Description: "90th percentile successful sentinel stream latency by query",
							Query:       `histogram_quantile(0.90, sum(rate(src_search_streaming_latency_seconds_bucket{source=~"searchblitz.*"}[$sentinel_sampling_duration])) by (le, source))`,
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{query}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `90th percentile time to first result for successful streaming sentinel queries, broken down by query. Useful for debugging whether a slowdown is limited to a specific type of query.`,
						},
					},
					{
						{
							Name:        "90th_percentile_unsuccessful_duration_by_query",
							Description: "90th percentile unsuccessful sentinel search duration by query",
							Query:       "histogram_quantile(0.90, sum(rate(src_search_response_latency_seconds_bucket{source=~`searchblitz.*`, status!=`success`}[$sentinel_sampling_duration])) by (le, source))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{source}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `90th percentile search duration of _unsuccessful_ sentinel queries (by error or timeout), broken down by query. Useful for debugging how the performance of failed requests affect UX.`,
						},
					},
					{
						{
							Name:        "75th_percentile_successful_sentinel_duration_by_query",
							Description: "75th percentile successful sentinel search duration by query",
							Query:       `histogram_quantile(0.75, sum(rate(src_search_response_latency_seconds_bucket{source=~"searchblitz.*", status="success"}[$sentinel_sampling_duration])) by (le, source))`,
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{query}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `75th percentile search duration of successful sentinel queries, broken down by query. Useful for debugging whether a slowdown is limited to a specific type of query.`,
						},
						{
							Name:        "75th_percentile_successful_stream_latency_by_query",
							Description: "75th percentile successful sentinel stream latency by query",
							Query:       `histogram_quantile(0.75, sum(rate(src_search_streaming_latency_seconds_bucket{source=~"searchblitz.*"}[$sentinel_sampling_duration])) by (le, source))`,
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{query}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `75th percentile time to first result for successful streaming sentinel queries, broken down by query. Useful for debugging whether a slowdown is limited to a specific type of query.`,
						},
					},
					{
						{
							Name:        "75th_percentile_unsuccessful_duration_by_query",
							Description: "75th percentile unsuccessful sentinel search duration by query",
							Query:       "histogram_quantile(0.75, sum(rate(src_search_response_latency_seconds_bucket{source=~`searchblitz.*`, status!=`success`}[$sentinel_sampling_duration])) by (le, source))",
							NoAlert:     true,
							Panel: monitoring.Panel().LegendFormat("{{source}}").Unit(monitoring.Seconds).With(
								monitoring.PanelOptions.LegendOnRight(),
								monitoring.PanelOptions.HoverShowAll(),
								monitoring.PanelOptions.HoverSort("descending"),
								monitoring.PanelOptions.Fill(0),
							),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `75th percentile search duration of _unsuccessful_ sentinel queries (by error or timeout), broken down by query. Useful for debugging how the performance of failed requests affect UX.`,
						},
					},
					{
						{
							Name:           "unsuccessful_status_rate",
							Description:    "unsuccessful status rate",
							Query:          `sum(rate(src_graphql_search_response{source=~"searchblitz.*", status!="success"}[$sentinel_sampling_duration])) by (status)`,
							NoAlert:        true,
							Panel:          monitoring.Panel().LegendFormat("{{status}}"),
							Owner:          monitoring.ObservableOwnerSearch,
							Interpretation: `The rate of unsuccessful sentinel queries, broken down by failure type.`,
						},
					},
				},
			},
		},
	}
}

func orgMetricRows(orgMetricSpec []struct {
	name        string
	route       string
	description string
}) []monitoring.Row {
	result := []monitoring.Row{}
	for _, m := range orgMetricSpec {
		result = append(result, monitoring.Row{

			{
				Name:           m.name + "_rate",
				Description:    "rate of " + m.description,
				Query:          `sum(irate(src_graphql_request_duration_seconds_count{route="` + m.route + `"}[5m]))`,
				NoAlert:        true,
				Panel:          monitoring.Panel().Unit(monitoring.RequestsPerSecond),
				Owner:          monitoring.ObservableOwnerCoreApplication,
				Interpretation: `Rate (QPS) of ` + m.description,
			},
			{
				Name:           m.name + "_latency_p99",
				Description:    "99 percentile latency of " + m.description,
				Query:          `histogram_quantile(0.99, sum(rate(src_graphql_request_duration_seconds_bucket{route="` + m.route + `"}[5m])) by (le))`,
				NoAlert:        true,
				Panel:          monitoring.Panel().Unit(monitoring.Milliseconds),
				Owner:          monitoring.ObservableOwnerCoreApplication,
				Interpretation: `99 percentile latency of` + m.description,
			},
			{
				Name:           m.name + "_error_rate",
				Description:    "percentage of " + m.description + " that return an error",
				Query:          `sum (irate(src_graphql_request_duration_seconds_count{route="` + m.route + `",success="false"}[5m]))/sum(irate(src_graphql_request_duration_seconds_count{route="` + m.route + `"}[5m]))*100`,
				NoAlert:        true,
				Panel:          monitoring.Panel().Unit(monitoring.Percentage),
				Owner:          monitoring.ObservableOwnerCoreApplication,
				Interpretation: `Percentage of ` + m.description + ` that return an error`,
			},
		})
	}
	return result
}
