package shared

import "github.com/sourcegraph/sourcegraph/monitoring/monitoring"

// GitServer exports available shared observable and group constructors related to gitserver and
// the client. Some of these panels are useful from multiple container contexts, so we maintain
// this struct as a place of authority over team alert definitions.
var GitServer gitServer

// gitServer provides `GitServer` implementations.
type gitServer struct{}

// src_gitserver_backend_total
// src_gitserver_backend_duration_seconds_bucket
// src_gitserver_backend_errors_total
func (gitServer) NewBackendGroup(containerName string, hidden bool) monitoring.Group {
	g := Observation.NewGroup(containerName, monitoring.ObservableOwnerSource, ObservationGroupOptions{
		GroupConstructorOptions: GroupConstructorOptions{
			Hidden:          hidden,
			Namespace:       "gitserver",
			DescriptionRoot: "Gitserver Backend",

			ObservableConstructorOptions: ObservableConstructorOptions{
				MetricNameRoot: "gitserver_backend",
				By:             []string{"op"},
			},
		},

		SharedObservationGroupOptions: SharedObservationGroupOptions{
			Total:     NoAlertsOption("none"),
			Duration:  NoAlertsOption("none"),
			Errors:    NoAlertsOption("none"),
			ErrorRate: NoAlertsOption("none"),
		},
		Aggregate: &SharedObservationGroupOptions{
			Total:     NoAlertsOption("none"),
			Duration:  NoAlertsOption("none"),
			Errors:    NoAlertsOption("none"),
			ErrorRate: NoAlertsOption("none"),
		},
	})

	g.Rows = append([]monitoring.Row{
		{
			{
				Name:           "concurrent_backend_operations",
				Description:    "number of concurrently running backend operations",
				Query:          "src_gitserver_backend_concurrent_operations",
				NoAlert:        true,
				Panel:          monitoring.Panel().LegendFormat("{{op}}").Min(0),
				Owner:          monitoring.ObservableOwnerSource,
				Interpretation: "The number of requests that are currently being handled by gitserver backend layer, at the point in time of scraping.",
			},
		},
	}, g.Rows...)

	return g
}

// src_gitserver_client_total
// src_gitserver_client_duration_seconds_bucket
// src_gitserver_client_errors_total
func (gitServer) NewClientGroup(containerName string) monitoring.Group {
	return Observation.NewGroup(containerName, monitoring.ObservableOwnerSource, ObservationGroupOptions{
		GroupConstructorOptions: GroupConstructorOptions{
			Namespace:       "gitserver",
			DescriptionRoot: "Gitserver Client",
			Hidden:          true,

			ObservableConstructorOptions: ObservableConstructorOptions{
				MetricNameRoot:        "gitserver_client",
				MetricDescriptionRoot: "graphql",
				By:                    []string{"op", "scope"},
			},
		},

		SharedObservationGroupOptions: SharedObservationGroupOptions{
			Total:     NoAlertsOption("none"),
			Duration:  NoAlertsOption("none"),
			Errors:    NoAlertsOption("none"),
			ErrorRate: NoAlertsOption("none"),
		},
		Aggregate: &SharedObservationGroupOptions{
			Total:     NoAlertsOption("none"),
			Duration:  NoAlertsOption("none"),
			Errors:    NoAlertsOption("none"),
			ErrorRate: NoAlertsOption("none"),
		},
	})
}
