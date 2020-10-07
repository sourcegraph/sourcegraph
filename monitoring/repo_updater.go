package main

import (
	"time"
)

func RepoUpdater() *Container {
	return &Container{
		Name:        "repo-updater",
		Title:       "Repo Updater",
		Description: "Manages interaction with code hosts, instructs Gitserver to update repositories.",
		Groups: []Group{
			{
				Title: "General",
				Rows: []Row{
					{
						sharedFrontendInternalAPIErrorResponses("repo-updater", ObservableOwnerCloud),
					},
				},
			},
			{
				Title: "Repositories",
				Rows: []Row{
					{
						Observable{
							Name:              "syncer_sync_last_time",
							Description:       "time gap since last sync",
							Query:             `max(timestamp(vector(time()))) - max(src_repoupdater_syncer_sync_last_time)`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: time.Hour.Seconds(), For: 5 * time.Minute},
							PanelOptions:      PanelOptions().LegendFormat("seconds").Unit(Seconds),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "Make sure there are external serviced added with valid tokens",
						},
					},
					{
						Observable{
							Name:              "syncer_sync_start",
							Description:       "sync was started",
							Query:             `src_repoupdater_syncer_start_sync`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 100, For: 5 * time.Minute},
							PanelOptions:      PanelOptions().LegendFormat("{{family}}-{{external_service_id}}").Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "None",
						},
						Observable{
							Name:              "syncer_sync_duration",
							Description:       "95th repositories sync duration",
							Query:             `histogram_quantile(0.95, rate(src_repoupdater_syncer_sync_duration_seconds_bucket[1m]))`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 30, For: 5 * time.Minute},
							PanelOptions:      PanelOptions().LegendFormat("seconds").Unit(Seconds),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host.",
						},
						Observable{
							Name:              "source_duration",
							Description:       "95th repositories source duration",
							Query:             `histogram_quantile(0.95, rate(src_repoupdater_source_duration_seconds_bucket[1m]))`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 30, For: 5 * time.Minute},
							PanelOptions:      PanelOptions().LegendFormat("seconds").Unit(Seconds),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host.",
						},
					},
					{
						Observable{
							Name:              "syncer_synced_repos",
							Description:       "repositories synced",
							Query:             `rate(src_repoupdater_syncer_synced_repos_total[1m])`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 1000, For: 5 * time.Minute}, // NOTE: There is really no point to have such warning
							PanelOptions:      PanelOptions().LegendFormat("{{state}}").Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
						Observable{
							Name:              "sourced_repos",
							Description:       "repositories sourced",
							Query:             `rate(src_repoupdater_source_repos_total[1m])`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 1000, For: 5 * time.Minute}, // NOTE: There is really no point to have such warning
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
					},
					{
						Observable{
							Name:              "purge_failed",
							Description:       "repositories purge failed",
							Query:             `rate(src_repoupdater_purge_failed[1m])`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 1, For: 5 * time.Minute},
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
						Observable{
							Name:              "purge_success",
							Description:       "repositories purge succeeded",
							Query:             `rate(src_repoupdater_purge_success[1m])`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 10, For: 5 * time.Minute},
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
					},
					{
						Observable{
							Name:              "sched_auto_fetch",
							Description:       "repositories scheduled due to hitting a deadline",
							Query:             `rate(src_repoupdater_sched_auto_fetch[1m])`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 1000, For: 5 * time.Minute}, // NOTE: There is really no point to have such warning
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
						Observable{
							Name:              "sched_manual_fetch",
							Description:       "repositories scheduled due to user traffic",
							Query:             `rate(src_repoupdater_sched_manual_fetch[1m])`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 1000, For: 5 * time.Minute}, // NOTE: There is really no point to have such warning
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
					},
					{
						Observable{
							Name:              "sched_known_repos",
							Description:       "repositories managed by the scheduler",
							Query:             `src_repoupdater_sched_known_repos`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 1000, For: 5 * time.Minute}, // NOTE: There is really no point to have such warning
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
						Observable{
							Name:              "sched_update_queue_length",
							Description:       "repositories queued for update",
							Query:             `src_repoupdater_sched_update_queue_length`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 1000, For: 5 * time.Minute}, // NOTE: There is really no point to have such warning
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
						Observable{
							Name:              "sched_loops",
							Description:       "scheduler loops",
							Query:             `rate(src_repoupdater_sched_loops[1m])`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 10, For: 5 * time.Minute}, // NOTE: There is really no point to have such warning
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
					},
					{
						Observable{
							Name:              "sched_error",
							Description:       "repositories schedule error rate",
							Query:             `rate(src_repoupdater_sched_error[1m])`,
							DataMayNotExist:   true,
							Critical:          Alert{GreaterOrEqual: 1, For: time.Minute},
							PanelOptions:      PanelOptions().Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "none",
						},
					},
				},
			},
			{
				Title:  "Permissions",
				Hidden: true,
				Rows: []Row{
					{
						Observable{
							Name:              "perms_syncer_perms",
							Description:       "time gap between least and most up to date permissions",
							Query:             `src_repoupdater_perms_syncer_perms_gap_seconds`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: (3 * 24 * time.Hour).Seconds(), For: 5 * time.Minute}, // 3 days
							PanelOptions:      PanelOptions().LegendFormat("{{type}}").Unit(Seconds),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "Increase the API rate limit to [GitHub](https://docs.sourcegraph.com/admin/external_service/github#github-com-rate-limits), [GitLab](https://docs.sourcegraph.com/admin/external_service/gitlab#internal-rate-limits) or [Bitbucket Server](https://docs.sourcegraph.com/admin/external_service/bitbucket_server#internal-rate-limits).",
						},
						Observable{
							Name:              "perms_syncer_stale_perms",
							Description:       "number of entities with stale permissions",
							Query:             `src_repoupdater_perms_syncer_stale_perms`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 100, For: 5 * time.Minute},
							PanelOptions:      PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "Increase the API rate limit to [GitHub](https://docs.sourcegraph.com/admin/external_service/github#github-com-rate-limits), [GitLab](https://docs.sourcegraph.com/admin/external_service/gitlab#internal-rate-limits) or [Bitbucket Server](https://docs.sourcegraph.com/admin/external_service/bitbucket_server#internal-rate-limits).",
						},
						Observable{
							Name:            "perms_syncer_no_perms",
							Description:     "number of entities with no permissions",
							Query:           `src_repoupdater_perms_syncer_no_perms`,
							DataMayNotExist: true,
							Warning:         Alert{GreaterOrEqual: 100, For: 5 * time.Minute},
							PanelOptions:    PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:           ObservableOwnerCloud,
							PossibleSolutions: `
								- **Enabled permissions for the first time:** Wait for few minutes and see if the number goes down.
								- **Otherwise:** Increase the API rate limit to [GitHub](https://docs.sourcegraph.com/admin/external_service/github#github-com-rate-limits), [GitLab](https://docs.sourcegraph.com/admin/external_service/gitlab#internal-rate-limits) or [Bitbucket Server](https://docs.sourcegraph.com/admin/external_service/bitbucket_server#internal-rate-limits).
							`,
						},
					},
					{
						Observable{
							Name:              "perms_syncer_sync_duration",
							Description:       "95th permissions sync duration",
							Query:             `histogram_quantile(0.95, rate(src_repoupdater_perms_syncer_sync_duration_seconds_bucket[1m]))`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 30, For: 5 * time.Minute},
							PanelOptions:      PanelOptions().LegendFormat("{{type}}").Unit(Seconds),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "Check the network latency is reasonable (<50ms) between the Sourcegraph and the code host.",
						},
						Observable{
							Name:            "perms_syncer_queue_size",
							Description:     "permissions sync queued items",
							Query:           `src_repoupdater_perms_syncer_queue_size`,
							DataMayNotExist: true,
							Warning:         Alert{GreaterOrEqual: 100, For: 5 * time.Minute},
							PanelOptions:    PanelOptions().Unit(Number),
							Owner:           ObservableOwnerCloud,
							PossibleSolutions: `
								- **Enabled permissions for the first time:** Wait for few minutes and see if the number goes down.
								- **Otherwise:** Increase the API rate limit to [GitHub](https://docs.sourcegraph.com/admin/external_service/github#github-com-rate-limits), [GitLab](https://docs.sourcegraph.com/admin/external_service/gitlab#internal-rate-limits) or [Bitbucket Server](https://docs.sourcegraph.com/admin/external_service/bitbucket_server#internal-rate-limits).
							`,
						},
					},
					{
						Observable{
							Name:              "authz_filter_duration",
							Description:       "95th authorization duration",
							Query:             `histogram_quantile(0.95, rate(src_frontend_authz_filter_duration_seconds_bucket{success="true"}[1m]))`,
							DataMayNotExist:   true,
							Critical:          Alert{GreaterOrEqual: 1, For: time.Minute},
							PanelOptions:      PanelOptions().LegendFormat("seconds").Unit(Seconds),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "Check if database is overloaded.",
						},
						Observable{
							Name:            "perms_syncer_sync_errors",
							Description:     "permissions sync error rate",
							Query:           `rate(src_repoupdater_perms_syncer_sync_errors_total[1m]) / rate(src_repoupdater_perms_syncer_sync_duration_seconds_count[1m])`,
							DataMayNotExist: true,
							Critical:        Alert{GreaterOrEqual: 1, For: time.Minute},
							PanelOptions:    PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:           ObservableOwnerCloud,
							PossibleSolutions: `
								- Check the network connectivity the Sourcegraph and the code host.
								- Check if API rate limit quota is exhausted on the code host.
							`,
						},
					},
				},
			},
			{
				Title:  "External services",
				Hidden: true,
				Rows: []Row{
					{
						Observable{
							Name:              "src_repoupdater_external_services_total",
							Description:       "the total number of external services",
							Query:             `src_repoupdater_external_services_total`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 20000, For: 1 * time.Hour},
							PanelOptions:      PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "None",
						},
						Observable{
							Name:              "src_repoupdater_user_external_services_total",
							Description:       "the total number of user added external services",
							Query:             `src_repoupdater_user_external_services_total`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 20000, For: 1 * time.Hour},
							PanelOptions:      PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "None",
						},
					},
					{
						Observable{
							Name:            "repoupdater_queued_sync_jobs_total",
							Description:     "the total number of queued sync jobs",
							Query:           `src_repoupdater_queued_sync_jobs_total`,
							DataMayNotExist: true,
							Warning:         Alert{GreaterOrEqual: 100, For: 1 * time.Hour},
							PanelOptions:    PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:           ObservableOwnerCloud,
							PossibleSolutions: `
								- **Check if jobs are failing to sync:** "SELECT * FROM external_service_sync_jobs WHERE state = 'errored'";
								- **Increase the number of workers** using the 'repoConcurrentExternalServiceSyncers' site config.
							`,
						},
						Observable{
							Name:              "repoupdater_completed_sync_jobs_total",
							Description:       "the total number of completed sync jobs",
							Query:             `src_repoupdater_completed_sync_jobs_total`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 100000, For: 1 * time.Hour},
							PanelOptions:      PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "None",
						},
						Observable{
							Name:              "repoupdater_errored_sync_jobs_total",
							Description:       "the total number of errored sync jobs",
							Query:             `src_repoupdater_errored_sync_jobs_total`,
							DataMayNotExist:   true,
							Warning:           Alert{GreaterOrEqual: 100, For: 1 * time.Hour},
							PanelOptions:      PanelOptions().LegendFormat("{{type}}").Unit(Number),
							Owner:             ObservableOwnerCloud,
							PossibleSolutions: "None",
						},
					},
				},
			},
			{
				Title:  "Container monitoring (not available on server)",
				Hidden: true,
				Rows: []Row{
					{
						sharedContainerCPUUsage("repo-updater", ObservableOwnerCloud),
						sharedContainerMemoryUsage("repo-updater", ObservableOwnerCloud),
					},
					{
						sharedContainerRestarts("repo-updater", ObservableOwnerCloud),
						sharedContainerFsInodes("repo-updater", ObservableOwnerCloud),
					},
				},
			},
			{
				Title:  "Provisioning indicators (not available on server)",
				Hidden: true,
				Rows: []Row{
					{
						sharedProvisioningCPUUsageLongTerm("repo-updater", ObservableOwnerCloud),
						sharedProvisioningMemoryUsageLongTerm("repo-updater", ObservableOwnerCloud),
					},
					{
						sharedProvisioningCPUUsageShortTerm("repo-updater", ObservableOwnerCloud),
						sharedProvisioningMemoryUsageShortTerm("repo-updater", ObservableOwnerCloud),
					},
				},
			},
			{
				Title:  "Golang runtime monitoring",
				Hidden: true,
				Rows: []Row{
					{
						sharedGoGoroutines("repo-updater", ObservableOwnerCloud),
						sharedGoGcDuration("repo-updater", ObservableOwnerCloud),
					},
				},
			},
			{
				Title:  "Kubernetes monitoring (ignore if using Docker Compose or server)",
				Hidden: true,
				Rows: []Row{
					{
						sharedKubernetesPodsAvailable("repo-updater", ObservableOwnerCloud),
					},
				},
			},
		},
	}
}
