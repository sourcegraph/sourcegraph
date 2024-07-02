package testkit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/jobstore"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/stretchr/testify/require"
)

func InsertRepo(t testing.TB, db database.DB, id api.RepoID, name string) {
	if name == "" {
		name = fmt.Sprintf("n-%d", id)
	}

	deletedAt := sqlf.Sprintf("NULL")
	if strings.HasPrefix(name, "DELETED-") {
		deletedAt = sqlf.Sprintf("%s", "2024-02-08 15:06:50.973329+00")
	}
	insertRepoQuery := sqlf.Sprintf(
		`INSERT INTO repo (id, name, deleted_at, private) VALUES (%s, %s, %s, %s) ON CONFLICT (id) DO NOTHING`,
		id,
		name,
		deletedAt,
		false,
	)
	_, err := db.ExecContext(context.Background(), insertRepoQuery.Query(sqlf.PostgresBindVar), insertRepoQuery.Args()...)
	require.NoError(t, err, "unexpected error while upserting repository")

	status := "cloned"
	if strings.HasPrefix(name, "DELETED-") {
		status = "not_cloned"
	}
	updateGitserverRepoQuery := sqlf.Sprintf(
		`UPDATE gitserver_repos SET clone_status = %s WHERE repo_id = %s`,
		status,
		id,
	)

	_, err = db.ExecContext(context.Background(), updateGitserverRepoQuery.Query(sqlf.PostgresBindVar), updateGitserverRepoQuery.Args()...)
	require.NoError(t, err, "unexpected error while upserting gitserver repository")
}

func MakeCommit(i int) api.CommitID {
	return api.CommitID(fmt.Sprintf("%040d", i))
}

func InsertSyntacticIndexingRecords(t testing.TB, db database.DB, records ...jobstore.SyntacticIndexingJob) {
	for _, index := range records {
		if index.Commit == "" {
			index.Commit = MakeCommit(index.ID)
		}
		if index.State == "" {
			index.State = jobstore.Completed
		}
		if index.RepositoryID == 0 {
			index.RepositoryID = 50
		}
		// Ensure we have a repo for the inner join in select queries
		InsertRepo(t, db, index.RepositoryID, index.RepositoryName)

		query := sqlf.Sprintf(`
			INSERT INTO syntactic_scip_indexing_jobs (
				id,
				commit,
				queued_at,
				state,
				failure_message,
				started_at,
				finished_at,
				process_after,
				num_resets,
				num_failures,
				repository_id,
				should_reindex,
				enqueuer_user_id
			) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
		`,
			index.ID,
			index.Commit,
			index.QueuedAt,
			index.State,
			index.FailureMessage,
			index.StartedAt,
			index.FinishedAt,
			index.ProcessAfter,
			index.NumResets,
			index.NumFailures,
			index.RepositoryID,
			index.ShouldReindex,
			index.EnqueuerUserID,
		)

		_, err := db.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...)
		require.NoError(t, err, "unexpected error while inserting index")
	}
}
