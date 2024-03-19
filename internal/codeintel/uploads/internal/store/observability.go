package store

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	// Not used yet.
	list *observation.Operation

	// Commits
	getStaleSourcedCommits              *observation.Operation
	deleteSourcedCommits                *observation.Operation
	updateSourcedCommits                *observation.Operation
	getCommitsVisibleToUpload           *observation.Operation
	getOldestCommitDate                 *observation.Operation
	getCommitGraphMetadata              *observation.Operation
	hasCommit                           *observation.Operation
	repositoryIDsWithErrors             *observation.Operation
	numRepositoriesWithCodeIntelligence *observation.Operation
	getRecentIndexesSummary             *observation.Operation

	// Repositories
	getRepositoriesForIndexScan             *observation.Operation
	getRepositoriesMaxStaleAge              *observation.Operation
	getRecentUploadsSummary                 *observation.Operation
	getLastUploadRetentionScanForRepository *observation.Operation
	setRepositoryAsDirty                    *observation.Operation
	setRepositoryAsDirtyWithTx              *observation.Operation
	getDirtyRepositories                    *observation.Operation
	repoName                                *observation.Operation
	setRepositoriesForRetentionScan         *observation.Operation
	hasRepository                           *observation.Operation

	// Uploads
	getIndexers                          *observation.Operation
	getUploads                           *observation.Operation
	getUploadByID                        *observation.Operation
	getUploadsByIDs                      *observation.Operation
	getVisibleUploadsMatchingMonikers    *observation.Operation
	updateUploadsVisibleToCommits        *observation.Operation
	writeVisibleUploads                  *observation.Operation
	persistNearestUploads                *observation.Operation
	persistNearestUploadsLinks           *observation.Operation
	persistUploadsVisibleAtTip           *observation.Operation
	updateUploadRetention                *observation.Operation
	updateCommittedAt                    *observation.Operation
	sourcedCommitsWithoutCommittedAt     *observation.Operation
	deleteUploadsWithoutRepository       *observation.Operation
	deleteUploadsStuckUploading          *observation.Operation
	softDeleteExpiredUploadsViaTraversal *observation.Operation
	softDeleteExpiredUploads             *observation.Operation
	hardDeleteUploadsByIDs               *observation.Operation
	deleteUploadByID                     *observation.Operation
	insertUpload                         *observation.Operation
	addUploadPart                        *observation.Operation
	markQueued                           *observation.Operation
	markFailed                           *observation.Operation
	deleteUploads                        *observation.Operation

	// Dumps
	findClosestCompletedUploads                   *observation.Operation
	findClosestCompletedUploadsFromGraphFragment  *observation.Operation
	getCompletedUploadsWithDefinitionsForMonikers *observation.Operation
	getCompletedUploadsByIDs                      *observation.Operation
	deleteOverlappingCompletedUploads             *observation.Operation

	// Packages
	updatePackages *observation.Operation

	// References
	updatePackageReferences *observation.Operation
	referencesForUpload     *observation.Operation

	// Audit logs
	deleteOldAuditLogs *observation.Operation

	// Dependencies
	insertDependencySyncingJob *observation.Operation

	reindexUploads                 *observation.Operation
	reindexUploadByID              *observation.Operation
	deleteIndexesWithoutRepository *observation.Operation

	getIndexes                 *observation.Operation
	getIndexByID               *observation.Operation
	getIndexesByIDs            *observation.Operation
	deleteIndexByID            *observation.Operation
	deleteIndexes              *observation.Operation
	reindexIndexByID           *observation.Operation
	reindexIndexes             *observation.Operation
	processStaleSourcedCommits *observation.Operation
	expireFailedRecords        *observation.Operation
}

var m = new(metrics.SingletonREDMetrics)

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			"codeintel_uploads_store",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.uploads.store.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
		})
	}

	return &operations{
		// Not used yet.
		list: op("List"),

		// Commits
		getCommitsVisibleToUpload: op("CommitsVisibleToUploads"),
		getOldestCommitDate:       op("GetOldestCommitDate"),
		getStaleSourcedCommits:    op("GetStaleSourcedCommits"),
		getCommitGraphMetadata:    op("GetCommitGraphMetadata"),
		deleteSourcedCommits:      op("DeleteSourcedCommits"),
		updateSourcedCommits:      op("UpdateSourcedCommits"),
		hasCommit:                 op("HasCommit"),

		// Repositories
		getRepositoriesForIndexScan:             op("GetRepositoriesForIndexScan"),
		getRepositoriesMaxStaleAge:              op("GetRepositoriesMaxStaleAge"),
		getRecentUploadsSummary:                 op("GetRecentUploadsSummary"),
		getLastUploadRetentionScanForRepository: op("GetLastUploadRetentionScanForRepository"),
		getDirtyRepositories:                    op("GetDirtyRepositories"),
		setRepositoryAsDirty:                    op("SetRepositoryAsDirty"),
		setRepositoryAsDirtyWithTx:              op("SetRepositoryAsDirtyWithTx"),
		repoName:                                op("RepoName"),
		setRepositoriesForRetentionScan:         op("SetRepositoriesForRetentionScan"),
		hasRepository:                           op("HasRepository"),

		// Uploads
		getIndexers:                          op("GetIndexers"),
		getUploads:                           op("GetUploads"),
		getUploadByID:                        op("GetUploadByID"),
		getUploadsByIDs:                      op("GetUploadsByIDs"),
		getVisibleUploadsMatchingMonikers:    op("GetVisibleUploadsMatchingMonikers"),
		updateUploadsVisibleToCommits:        op("UpdateUploadsVisibleToCommits"),
		updateUploadRetention:                op("UpdateUploadRetention"),
		updateCommittedAt:                    op("UpdateCommittedAt"),
		sourcedCommitsWithoutCommittedAt:     op("SourcedCommitsWithoutCommittedAt"),
		deleteUploadsStuckUploading:          op("DeleteUploadsStuckUploading"),
		softDeleteExpiredUploadsViaTraversal: op("SoftDeleteExpiredUploadsViaTraversal"),
		deleteUploadsWithoutRepository:       op("DeleteUploadsWithoutRepository"),
		softDeleteExpiredUploads:             op("SoftDeleteExpiredUploads"),
		hardDeleteUploadsByIDs:               op("HardDeleteUploadsByIDs"),
		deleteUploadByID:                     op("DeleteUploadByID"),
		insertUpload:                         op("InsertUpload"),
		addUploadPart:                        op("AddUploadPart"),
		markQueued:                           op("MarkQueued"),
		markFailed:                           op("MarkFailed"),
		deleteUploads:                        op("DeleteUploads"),

		writeVisibleUploads:        op("writeVisibleUploads"),
		persistNearestUploads:      op("persistNearestUploads"),
		persistNearestUploadsLinks: op("persistNearestUploadsLinks"),
		persistUploadsVisibleAtTip: op("persistUploadsVisibleAtTip"),

		// Dumps
		findClosestCompletedUploads:                   op("FindClosestCompletedUploads"),
		findClosestCompletedUploadsFromGraphFragment:  op("FindClosestCompletedUploadsFromGraphFragment"),
		getCompletedUploadsWithDefinitionsForMonikers: op("GetUploadsWithDefinitionsForMonikers"),
		getCompletedUploadsByIDs:                      op("GetCompletedUploadsByIDs"),
		deleteOverlappingCompletedUploads:             op("DeleteOverlappingCompletedUploads"),

		// Packages
		updatePackages: op("UpdatePackages"),

		// References
		updatePackageReferences: op("UpdatePackageReferences"),
		referencesForUpload:     op("ReferencesForUpload"),

		// Audit logs
		deleteOldAuditLogs: op("DeleteOldAuditLogs"),

		// Dependencies
		insertDependencySyncingJob: op("InsertDependencySyncingJob"),

		reindexUploads:                 op("ReindexUploads"),
		reindexUploadByID:              op("ReindexUploadByID"),
		deleteIndexesWithoutRepository: op("DeleteIndexesWithoutRepository"),

		getIndexes:                          op("GetIndexes"),
		getIndexByID:                        op("GetIndexByID"),
		getIndexesByIDs:                     op("GetIndexesByIDs"),
		deleteIndexByID:                     op("DeleteIndexByID"),
		deleteIndexes:                       op("DeleteIndexes"),
		reindexIndexByID:                    op("ReindexIndexByID"),
		reindexIndexes:                      op("ReindexIndexes"),
		processStaleSourcedCommits:          op("ProcessStaleSourcedCommits"),
		expireFailedRecords:                 op("ExpireFailedRecords"),
		repositoryIDsWithErrors:             op("RepositoryIDsWithErrors"),
		numRepositoriesWithCodeIntelligence: op("NumRepositoriesWithCodeIntelligence"),
		getRecentIndexesSummary:             op("GetRecentIndexesSummary"),
	}
}
