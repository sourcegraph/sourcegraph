package definitions

import (
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/monitoring/definitions/shared"
	"github.com/sourcegraph/sourcegraph/monitoring/monitoring"
)

func Worker() *monitoring.Container {
	const containerName = "worker"

	var workerJobs = []struct {
		Name  string
		Owner monitoring.ObservableOwner
	}{
		{Name: "codeintel-janitor", Owner: monitoring.ObservableOwnerCodeIntel},
		{Name: "codeintel-commitgraph", Owner: monitoring.ObservableOwnerCodeIntel},
		{Name: "codeintel-auto-indexing", Owner: monitoring.ObservableOwnerCodeIntel},
	}

	var activeJobObservables []monitoring.Observable
	for _, job := range workerJobs {
		activeJobObservables = append(activeJobObservables, monitoring.Observable{
			Name:          fmt.Sprintf("worker_job_%s_count", job.Name),
			Description:   fmt.Sprintf("number of worker instances running the %s job", job.Name),
			Query:         fmt.Sprintf(`sum (src_worker_jobs{job="worker", job_name="%s"})`, job.Name),
			Panel:         monitoring.Panel().LegendFormat(fmt.Sprintf("instances running %s", job.Name)),
			DataMustExist: true,
			Warning:       monitoring.Alert().Less(1, nil).For(1 * time.Minute),
			Critical:      monitoring.Alert().Less(1, nil).For(5 * time.Minute),
			Owner:         job.Owner,
			PossibleSolutions: fmt.Sprintf(`
				- Ensure your instance defines a worker container such that:
					- `+"`"+`WORKER_JOB_ALLOWLIST`+"`"+` contains "%[1]s" (or "all"), and
					- `+"`"+`WORKER_JOB_BLOCKLIST`+"`"+` does not contain "%[1]s"
				- Ensure that such a container is not failing to start or stay active
			`, job.Name),
		})
	}

	panelsPerRow := 4
	if rem := len(activeJobObservables) % panelsPerRow; rem == 1 || rem == 2 {
		// If we'd leave one or two panels on the only/last row, then reduce
		// the number of panels in previous rows so that we have less of a width
		// difference at the end
		panelsPerRow = 3
	}

	var activeJobRows []monitoring.Row
	for _, observable := range activeJobObservables {
		if n := len(activeJobRows); n == 0 || len(activeJobRows[n-1]) >= panelsPerRow {
			activeJobRows = append(activeJobRows, nil)
		}

		n := len(activeJobRows)
		activeJobRows[n-1] = append(activeJobRows[n-1], observable)
	}

	activeJobsGroup := monitoring.Group{
		Title: "Active jobs",
		Rows: append(
			[]monitoring.Row{
				{
					{
						Name:        "worker_job_count",
						Description: "number of worker instances running each job",
						Query:       `sum by (job_name) (src_worker_jobs{job="worker"})`,
						Panel:       monitoring.Panel().LegendFormat("instances running {{job_name}}"),
						NoAlert:     true,
						Interpretation: `
							The number of worker instances running each job type.
							It is necessary for each job type to be managed by at least one worker instance.
						`,
					},
				},
			},
			activeJobRows...,
		),
	}

	return &monitoring.Container{
		Name:        "worker",
		Title:       "Worker",
		Description: "Manages background processes.",
		Groups: []monitoring.Group{
			// src_worker_jobs
			activeJobsGroup,

			shared.CodeIntelligence.NewCommitGraphQueueGroup(containerName),
			shared.CodeIntelligence.NewCommitGraphProcessorGroup(containerName),
			shared.CodeIntelligence.NewDependencyIndexQueueGroup(containerName),
			shared.CodeIntelligence.NewDependencyIndexProcessorGroup(containerName),
			shared.CodeIntelligence.NewJanitorGroup(containerName),
			shared.CodeIntelligence.NewIndexSchedulerGroup(containerName),
			shared.CodeIntelligence.NewAutoIndexEnqueuerGroup(containerName),
			shared.CodeIntelligence.NewDBStoreGroup(containerName),
			shared.CodeIntelligence.NewLSIFStoreGroup(containerName),
			shared.CodeIntelligence.NewDependencyIndexDBWorkerStoreGroup(containerName),
			shared.CodeIntelligence.NewGitserverClientGroup(containerName),

			// src_codeintel_background_upload_resets_total
			// src_codeintel_background_upload_reset_failures_total
			// src_codeintel_background_upload_reset_errors_total
			shared.WorkerutilResetter.NewGroup(containerName, monitoring.ObservableOwnerCodeIntel, shared.ResetterGroupOptions{
				GroupConstructorOptions: shared.GroupConstructorOptions{
					Namespace:       "codeintel",
					DescriptionRoot: "lsif_upload record resetter",
					Hidden:          true,

					ObservableConstructorOptions: shared.ObservableConstructorOptions{
						MetricNameRoot:        "codeintel_background_upload",
						MetricDescriptionRoot: "lsif_upload",
					},
				},

				RecordResets:        shared.NoAlertsOption("none"),
				RecordResetFailures: shared.NoAlertsOption("none"),
				Errors:              shared.NoAlertsOption("none"),
			}),

			// src_codeintel_background_index_resets_total
			// src_codeintel_background_index_reset_failures_total
			// src_codeintel_background_index_reset_errors_total
			shared.WorkerutilResetter.NewGroup(containerName, monitoring.ObservableOwnerCodeIntel, shared.ResetterGroupOptions{
				GroupConstructorOptions: shared.GroupConstructorOptions{
					Namespace:       "codeintel",
					DescriptionRoot: "lsif_index record resetter",
					Hidden:          true,

					ObservableConstructorOptions: shared.ObservableConstructorOptions{
						MetricNameRoot:        "codeintel_background_index",
						MetricDescriptionRoot: "lsif_index",
					},
				},

				RecordResets:        shared.NoAlertsOption("none"),
				RecordResetFailures: shared.NoAlertsOption("none"),
				Errors:              shared.NoAlertsOption("none"),
			}),

			// src_codeintel_background_dependency_index_resets_total
			// src_codeintel_background_dependency_index_reset_failures_total
			// src_codeintel_background_dependency_index_reset_errors_total
			shared.WorkerutilResetter.NewGroup(containerName, monitoring.ObservableOwnerCodeIntel, shared.ResetterGroupOptions{
				GroupConstructorOptions: shared.GroupConstructorOptions{
					Namespace:       "codeintel",
					DescriptionRoot: "lsif_dependency_index record resetter",
					Hidden:          true,

					ObservableConstructorOptions: shared.ObservableConstructorOptions{
						MetricNameRoot:        "codeintel_background_dependency_index",
						MetricDescriptionRoot: "lsif_dependency_index",
					},
				},

				RecordResets:        shared.NoAlertsOption("none"),
				RecordResetFailures: shared.NoAlertsOption("none"),
				Errors:              shared.NoAlertsOption("none"),
			}),

			// Resource monitoring
			shared.NewFrontendInternalAPIErrorResponseMonitoringGroup(containerName, monitoring.ObservableOwnerCodeIntel, nil),
			shared.NewDatabaseConnectionsMonitoringGroup(containerName),
			shared.NewContainerMonitoringGroup(containerName, monitoring.ObservableOwnerCodeIntel, nil),
			shared.NewProvisioningIndicatorsGroup(containerName, monitoring.ObservableOwnerCodeIntel, nil),
			shared.NewGolangMonitoringGroup(containerName, monitoring.ObservableOwnerCodeIntel, nil),
			shared.NewKubernetesMonitoringGroup(containerName, monitoring.ObservableOwnerCodeIntel, nil),
		},
	}
}
