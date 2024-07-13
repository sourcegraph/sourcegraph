package repos

import (
	"context"
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

var MockStatusMessages func(context.Context) ([]StatusMessage, error)

// FetchStatusMessages fetches repo related status messages.
func FetchStatusMessages(ctx context.Context, db database.DB, gitserverClient gitserver.Client) ([]StatusMessage, error) {
	if MockStatusMessages != nil {
		return MockStatusMessages(ctx)
	}
	var messages []StatusMessage

	if conf.Get().DisableAutoGitUpdates {
		messages = append(messages, StatusMessage{
			GitUpdatesDisabled: &GitUpdatesDisabled{
				Message: "Repositories will not be cloned or updated.",
			},
		})
	}

	// We first fetch affiliated sync errors since this will also find all the
	// external services the user cares about.
	externalServiceSyncErrors, err := db.ExternalServices().GetLatestSyncErrors(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetching sync errors")
	}

	stats, err := db.RepoStatistics().GetRepoStatistics(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "loading repo statistics")
	}

	// Return early since we don't have any affiliated external services
	if len(externalServiceSyncErrors) == 0 {
		// Explicit no repository message
		if stats.Total == 0 {
			messages = append(messages, StatusMessage{
				NoRepositoriesDetected: &NoRepositoriesDetected{
					Message: "No repositories have been added to Sourcegraph.",
				},
			})
		}
		return messages, nil
	}

	for _, syncError := range externalServiceSyncErrors {
		if syncError.Message != "" {
			messages = append(messages, StatusMessage{
				ExternalServiceSyncError: &ExternalServiceSyncError{
					Message:           syncError.Message,
					ExternalServiceId: syncError.ServiceID,
				},
			})
		}
	}

	if stats.FailedFetch > 0 {
		messages = append(messages, StatusMessage{
			SyncError: &SyncError{
				Message: fmt.Sprintf("%d %s failed last attempt to sync content from code host", stats.FailedFetch, pluralize(stats.FailedFetch, "repository", "repositories")),
			},
		})
	}

	if uncloned := stats.NotCloned + stats.Cloning; uncloned > 0 {
		var sentences []string
		if stats.NotCloned > 0 {
			sentences = append(sentences, fmt.Sprintf("%d %s enqueued for cloning.", stats.NotCloned, pluralize(stats.NotCloned, "repository", "repositories")))
		}
		if stats.Cloning > 0 {
			sentences = append(sentences, fmt.Sprintf("%d %s currently cloning...", stats.Cloning, pluralize(stats.Cloning, "repository", "repositories")))
		}
		messages = append(messages, StatusMessage{
			Cloning: &CloningProgress{
				Message: strings.Join(sentences, " "),
			},
		})
	}

	zoektRepoStats, err := db.ZoektRepos().GetStatistics(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "loading repo statistics")
	}
	if zoektRepoStats.NotIndexed > 0 {
		messages = append(messages, StatusMessage{
			Indexing: &IndexingProgress{
				NotIndexed: zoektRepoStats.NotIndexed,
				Indexed:    zoektRepoStats.Indexed,
			},
		})
	}

	diskUsageThreshold := conf.Get().SiteConfig().GitserverDiskUsageWarningThreshold
	if diskUsageThreshold == nil {
		// This is the default threshold if not configured
		diskUsageThreshold = pointers.Ptr(90)
	}

	si, err := gitserverClient.SystemsInfo(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "fetching gitserver disk info")
	}

	for _, s := range si {
		if s.PercentUsed >= float32(*diskUsageThreshold) {
			messages = append(messages, StatusMessage{
				GitserverDiskThresholdReached: &GitserverDiskThresholdReached{
					Message: fmt.Sprintf("The disk usage on gitserver %q is over %d%% (%.2f%% used).", s.Address, *diskUsageThreshold, s.PercentUsed),
				},
			})
		}
	}

	return messages, nil
}

func pluralize(count int, singularNoun, pluralNoun string) string {
	if count == 1 {
		return singularNoun
	}
	return pluralNoun
}

type GitUpdatesDisabled struct {
	Message string
}
type NoRepositoriesDetected struct {
	Message string
}

type CloningProgress struct {
	Message string
}

type ExternalServiceSyncError struct {
	Message           string
	ExternalServiceId int64
}

type SyncError struct {
	Message string
}

type IndexingProgress struct {
	NotIndexed int
	Indexed    int
}

type GitserverDiskThresholdReached struct {
	Message string
}

type StatusMessage struct {
	GitUpdatesDisabled            *GitUpdatesDisabled            `json:"git_updates_disabled"`
	NoRepositoriesDetected        *NoRepositoriesDetected        `json:"no_repositories_detected"`
	Cloning                       *CloningProgress               `json:"cloning"`
	ExternalServiceSyncError      *ExternalServiceSyncError      `json:"external_service_sync_error"`
	SyncError                     *SyncError                     `json:"sync_error"`
	Indexing                      *IndexingProgress              `json:"indexing"`
	GitserverDiskThresholdReached *GitserverDiskThresholdReached `json:"gitserver_disk_threshold_reached"`
}
