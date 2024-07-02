package syntactic_indexing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/policies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/reposcheduler"
	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	testutils "github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/testkit"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/jobstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestSyntacticIndexingScheduler(t *testing.T) {
	observationCtx := observation.TestContextTB(t)

	// Bootstrap scheduler
	frontendRawDB := dbtest.NewDB(t)
	codeintelRawDB := dbtest.NewCodeintelDB(t)
	db := database.NewDB(observationCtx.Logger, frontendRawDB)
	config := &SchedulerConfig{
		PolicyBatchSize:     100,
		RepositoryBatchSize: 2500,
	}
	gitserverClient := gitserver.NewMockClient()
	scheduler, jobStore, policiesSvc := bootstrapScheduler(t, observationCtx, frontendRawDB, codeintelRawDB, gitserverClient, config)

	ctx := context.Background()

	// Setup repositories
	tacosRepoId, tacosRepoName, tacosCommit := api.RepoID(1), "github.com/tangy/tacos", testutils.MakeCommit(1)
	empanadasRepoId, empanadasRepoName, empanadasCommit := api.RepoID(2), "github.com/salty/empanadas", testutils.MakeCommit(2)
	mangosRepoId, mangosRepoName, _ := api.RepoID(3), "gitlab.com/juicy/mangos", testutils.MakeCommit(3)
	testutils.InsertRepo(t, db, tacosRepoId, tacosRepoName)
	testutils.InsertRepo(t, db, empanadasRepoId, empanadasRepoName)
	testutils.InsertRepo(t, db, mangosRepoId, mangosRepoName)

	setupRepoPolicies(t, ctx, db, policiesSvc)

	gitserverClient.ResolveRevisionFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, rev string, options gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		isTacos := repo == api.RepoName(tacosRepoName) && rev == string(tacosCommit)
		isEmpanadas := repo == api.RepoName(empanadasRepoName) && rev == string(empanadasCommit)

		if isTacos || isEmpanadas {
			return api.CommitID(rev), nil
		} else {
			return api.CommitID("what"), errors.New(fmt.Sprintf("Unexpected repo (`%s`) and revision (`%s`) requested from gitserver: ", repo, rev))
		}
	})

	gitserverClient.ListRefsFunc.SetDefaultHook(func(ctx context.Context, repoName api.RepoName, opts gitserver.ListRefsOpts) ([]gitdomain.Ref, error) {
		ref := gitdomain.Ref{
			Name:   "refs/head/main",
			Type:   gitdomain.RefTypeBranch,
			IsHead: true,
		}
		switch string(repoName) {
		case empanadasRepoName:
			ref.CommitID = api.CommitID(empanadasCommit)
		case tacosRepoName:
			ref.CommitID = api.CommitID(tacosCommit)
		default:
			return nil, errors.New(fmt.Sprintf("Unexpected repo (`%s`) requested from gitserver's ListRef", repoName))
		}
		return []gitdomain.Ref{ref}, nil
	})

	err := scheduler.Schedule(observationCtx, ctx, time.Now())
	require.NoError(t, err)
	require.Equal(t, 2, unwrap(jobStore.DBWorkerStore().QueuedCount(ctx, false))(t))

	job1, recordReturned, err := jobStore.DBWorkerStore().Dequeue(ctx, "worker-1", []*sqlf.Query{})
	require.NoError(t, err)
	require.True(t, recordReturned)

	job2, recordReturned, err := jobStore.DBWorkerStore().Dequeue(ctx, "worker-1", []*sqlf.Query{})
	require.NoError(t, err)
	require.True(t, recordReturned)

	// There are only two records because in our policies setup we have
	// explicitly disabled syntactic indexing for the last remaining repository
	job3, recordReturned, err := jobStore.DBWorkerStore().Dequeue(ctx, "worker-1", []*sqlf.Query{})
	require.Nil(t, job3)
	require.False(t, recordReturned)
	require.NoError(t, err)

	// Ensure the test is resilient to order changes
	tacosJob := &jobstore.SyntacticIndexingJob{}
	empanadasJob := &jobstore.SyntacticIndexingJob{}

	if job1.RepositoryName == tacosRepoName {
		tacosJob = job1
		empanadasJob = job2
	} else {
		require.Equal(t, empanadasRepoName, job1.RepositoryName)
		tacosJob = job2
		empanadasJob = job1
	}

	require.Equal(t, tacosRepoName, tacosJob.RepositoryName)
	require.Equal(t, tacosCommit, tacosJob.Commit)

	require.Equal(t, empanadasRepoName, empanadasJob.RepositoryName)
	require.Equal(t, empanadasCommit, empanadasJob.Commit)

}

func unwrap[T any](v T, err error) func(*testing.T) T {
	return func(t *testing.T) T {
		require.NoError(t, err)
		return v
	}
}

func bootstrapScheduler(t *testing.T, observationCtx *observation.Context,
	frontendRawDB *sql.DB, codeintelDB *sql.DB, gitserverClient gitserver.Client,
	config *SchedulerConfig) (SyntacticJobScheduler, jobstore.SyntacticIndexingJobStore, *policies.Service) {
	frontendDB := database.NewDB(observationCtx.Logger, frontendRawDB)
	codeIntelDB := codeintelshared.NewCodeIntelDB(observationCtx.Logger, codeintelDB)
	uploadsSvc := uploads.NewService(observationCtx, frontendDB, codeIntelDB, gitserverClient.Scoped("uploads"))
	policiesSvc := policies.NewService(observationCtx, frontendDB, uploadsSvc, gitserverClient.Scoped("policies"))

	schedulerConfig.Load()
	matcher := policies.NewMatcher(
		gitserverClient,
		policies.IndexingExtractor,
		true,
		true,
	)
	repoSchedulingStore := reposcheduler.NewSyntacticStore(observationCtx, frontendDB)
	repoSchedulingSvc := reposcheduler.NewService(repoSchedulingStore)
	jobStore := unwrap(jobstore.NewStoreWithDB(observationCtx, frontendRawDB))(t)

	repoStore := frontendDB.Repos()
	enqueuer := NewIndexEnqueuer(observationCtx, jobStore, repoSchedulingStore, repoStore)
	scheduler := unwrap(NewSyntacticJobScheduler(repoSchedulingSvc, *matcher, *policiesSvc, repoStore, enqueuer, *config))(t)
	return scheduler, jobStore, policiesSvc
}

func setupRepoPolicies(t *testing.T, ctx context.Context, db database.DB, policies *policies.Service) {

	if _, err := db.ExecContext(context.Background(), `TRUNCATE lsif_configuration_policies`); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	query := `
		INSERT INTO lsif_configuration_policies (
			id,
			repository_id,
			name,
			type,
			pattern,
			repository_patterns,
			retention_enabled,
			retention_duration_hours,
			retain_intermediate_commits,
			syntactic_indexing_enabled,
			indexing_enabled,
			index_commit_max_age_hours,
			index_intermediate_commits
		) VALUES
			--                        							              ↙ retention_enabled
			--                        							              |    ↙ retention_duration_hours
			--                        							              |    |    ↙ retain_intermediate_commits
			--                        							              |    |    |     ↙ syntactic_indexing_enabled
			--                        							              |    |    |     |      ↙ indexing_enabled
			--                        							              |    |    |     |      |      ↙ index_commit_max_age_hours
			--                        							              |    |    |     |      |      |     ↙ index_intermediate_commits
			(1000, 2,    'policy  1 abc', 'GIT_TREE', '', null,              false, 0, false, true,  false,  0, false),
			-- Policy below specifically disables syntactic indexing for repo with ID=3
			(1003, 3,    'policy  3 bcd', 'GIT_TREE', '', null,              false, 0, false, false, false, 0,  false),
			-- Policy below enables syntactic indexing for all repositories starting with 'github.com'
			(1100, NULL, 'policy 10 def', 'GIT_TREE', '', '{github.com/*}',  false, 0, false, true,  false, 0,  false)
	`
	unwrap(db.ExecContext(ctx, query))(t)

	// Policy 1100 is the only one that contains repository patterns.
	// For it to be matched against our repository, we need to update
	// an extra bit of database state - a lookup table identifying
	// policies and repositories that were matched by them
	//
	// The other two policies (1000 and 1003) have explicit repository_id set
	// and don't need any extra database state to be returned by policy matcher.
	for _, policyID := range []int{1100} {
		policy, _, err := policies.GetConfigurationPolicyByID(ctx, policyID)
		require.NoError(t, err)
		require.NoError(t, policies.UpdateReposMatchingPolicyPatterns(ctx, policy))
	}
}
