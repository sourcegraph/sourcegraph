package store

import (
	"context"

	"github.com/keegancsmith/sqlf"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// HasRepository determines if there is LSIF data for the given repository.
func (s *store) HasRepository(ctx context.Context, repositoryID api.RepoID) (_ bool, err error) {
	ctx, _, endObservation := s.operations.hasRepository.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", int(repositoryID)),
	}})
	defer endObservation(1, observation.Args{})

	_, found, err := basestore.ScanFirstInt(s.db.Query(ctx, sqlf.Sprintf(hasRepositoryQuery, int(repositoryID))))
	return found, err
}

const hasRepositoryQuery = `
SELECT 1 FROM lsif_uploads WHERE state NOT IN ('deleted', 'deleting') AND repository_id = %s LIMIT 1
`

// HasCommit determines if the given commit is known for the given repository.
func (s *store) HasCommit(ctx context.Context, repositoryID api.RepoID, commit api.CommitID) (_ bool, err error) {
	ctx, _, endObservation := s.operations.hasCommit.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", int(repositoryID)),
		attribute.String("commit", string(commit)),
	}})
	defer endObservation(1, observation.Args{})

	count, _, err := basestore.ScanFirstInt(s.db.Query(
		ctx,
		sqlf.Sprintf(
			hasCommitQuery,
			repositoryID, dbutil.CommitBytea(commit),
			repositoryID, dbutil.CommitBytea(commit),
		),
	))

	return count > 0, err
}

const hasCommitQuery = `
SELECT
	(SELECT COUNT(*) FROM lsif_nearest_uploads WHERE repository_id = %s AND commit_bytea = %s) +
	(SELECT COUNT(*) FROM lsif_nearest_uploads_links WHERE repository_id = %s AND commit_bytea = %s)
`

// InsertDependencySyncingJob inserts a new dependency syncing job and returns its identifier.
func (s *store) InsertDependencySyncingJob(ctx context.Context, uploadID int) (id int, err error) {
	ctx, _, endObservation := s.operations.insertDependencySyncingJob.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("id", id),
		}})
	}()

	id, _, err = basestore.ScanFirstInt(s.db.Query(ctx, sqlf.Sprintf(insertDependencySyncingJobQuery, uploadID)))
	return id, err
}

const insertDependencySyncingJobQuery = `
INSERT INTO lsif_dependency_syncing_jobs (upload_id) VALUES (%s)
RETURNING id
`
