package syntactic_indexing

import (
	"context"
	"testing"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/jobstore"
	testutils "github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/testkit"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/stretchr/testify/require"
)

func TestSyntacticIndexingStoreDequeue(t *testing.T) {
	/*
		The purpose of this test is to verify that the DB schema
		we're using for the syntactic code intel work matches
		the requirements of dbworker interface,
		and that we can dequeue records through this interface.

		The schema is sensitive to column names and types, and to the fact
		that we are using a Postgres view to query repository name alongside
		indexing records,
		so it's important that we use the real Postgres in this test to prevent
		schema/implementation drift.
	*/
	observationContext := observation.TestContextTB(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(observationContext.Logger, sqlDB)

	jobStore, err := jobstore.NewStoreWithDB(observationContext, sqlDB)
	require.NoError(t, err, "unexpected error creating dbworker stores")
	store := jobStore.DBWorkerStore()

	ctx := context.Background()

	initCount, _ := store.QueuedCount(ctx, true)

	require.Equal(t, 0, initCount)

	Queued := jobstore.Queued

	commit1, commit2, commit3 := testutils.MakeCommit(1), testutils.MakeCommit(2), testutils.MakeCommit(3)

	testutils.InsertSyntacticIndexingRecords(t, db,
		// Even though this record is the oldest in the queue,
		// it is associated with a deleted repository.
		// The view that we use for dequeuing should not return this
		// record at all, and the first one should still be the record with ID=1
		jobstore.SyntacticIndexingJob{
			ID:             500,
			Commit:         commit3,
			RepositoryID:   4,
			RepositoryName: "DELETED-org/repo",
			State:          Queued,
			QueuedAt:       time.Now().Add(time.Second * -100),
		},
		jobstore.SyntacticIndexingJob{
			ID:             1,
			Commit:         commit1,
			RepositoryID:   1,
			RepositoryName: "tangy/tacos",
			State:          Queued,
			QueuedAt:       time.Now().Add(time.Second * -5),
		},
		jobstore.SyntacticIndexingJob{
			ID:             2,
			Commit:         commit2,
			RepositoryID:   2,
			RepositoryName: "salty/empanadas",
			State:          Queued,
			QueuedAt:       time.Now().Add(time.Second * -2),
		},
		jobstore.SyntacticIndexingJob{
			ID:             3,
			Commit:         commit3,
			RepositoryID:   3,
			RepositoryName: "juicy/mangoes",
			State:          jobstore.Processing,
			QueuedAt:       time.Now().Add(time.Second * -1),
		},
	)

	afterCount, _ := store.QueuedCount(ctx, true)

	require.Equal(t, 3, afterCount)

	record1, hasRecord, err := store.Dequeue(ctx, "worker1", nil)

	require.NoError(t, err)
	require.True(t, hasRecord)
	require.Equal(t, 1, record1.ID)
	require.Equal(t, "tangy/tacos", record1.RepositoryName)
	require.Equal(t, commit1, record1.Commit)

	record2, hasRecord, err := store.Dequeue(ctx, "worker2", nil)

	require.NoError(t, err)
	require.True(t, hasRecord)
	require.Equal(t, 2, record2.ID)
	require.Equal(t, "salty/empanadas", record2.RepositoryName)
	require.Equal(t, commit2, record2.Commit)

	_, hasRecord, err = store.Dequeue(ctx, "worker2", nil)
	require.NoError(t, err)
	require.False(t, hasRecord)

}

func TestSyntacticIndexingStoreEnqueue(t *testing.T) {
	/*
		The purpose of this test is to verify that methods InsertIndexingJobs and IsQueued
		correctly interact with each other, and that the records inserted using those methods
		are valid from the point of view of the DB worker interface
	*/
	observationContext := observation.TestContextTB(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(observationContext.Logger, sqlDB)
	ctx := context.Background()

	jobStore, err := jobstore.NewStoreWithDB(observationContext, sqlDB)
	require.NoError(t, err, "unexpected error creating dbworker stores")
	store := jobStore.DBWorkerStore()

	tacosRepoId, tacosRepoName, tacosCommit := api.RepoID(1), "tangy/tacos", testutils.MakeCommit(1)
	empanadasRepoId, empanadasRepoName, empanadasCommit := api.RepoID(2), "salty/empanadas", testutils.MakeCommit(2)
	mangosRepoId, mangosRepoName, mangosCommit := api.RepoID(2), "juicy/mangos", testutils.MakeCommit(2)

	testutils.InsertRepo(t, db, tacosRepoId, tacosRepoName)
	testutils.InsertRepo(t, db, empanadasRepoId, empanadasRepoName)
	testutils.InsertRepo(t, db, mangosRepoId, mangosRepoName)

	jobStore.InsertIndexingJobs(ctx, []jobstore.SyntacticIndexingJob{
		{
			ID:             1,
			Commit:         tacosCommit,
			RepositoryID:   tacosRepoId,
			RepositoryName: tacosRepoName,
			State:          jobstore.Queued,
			QueuedAt:       time.Now().Add(time.Second * -5),
		},
		{
			ID:             2,
			Commit:         empanadasCommit,
			RepositoryID:   empanadasRepoId,
			RepositoryName: empanadasRepoName,
			State:          jobstore.Queued,
			QueuedAt:       time.Now().Add(time.Second * -2),
		},
	})

	// Assertions below verify the interactions between InsertIndexes and IsQueued
	tacosIsQueued, err := jobStore.IsQueued(ctx, tacosRepoId, tacosCommit)
	require.NoError(t, err)
	require.True(t, tacosIsQueued)

	empanadasIsQueued, err := jobStore.IsQueued(ctx, empanadasRepoId, empanadasCommit)
	require.NoError(t, err)
	require.True(t, empanadasIsQueued)

	mangosIsQueued, err := jobStore.IsQueued(ctx, mangosRepoId, mangosCommit)
	require.NoError(t, err)
	require.True(t, mangosIsQueued)

	// Assertions below verify that records inserted by InsertIndexes are
	// still visible by DB Worker interface
	afterCount, _ := store.QueuedCount(ctx, true)

	require.Equal(t, 2, afterCount)

	record1, hasRecord, err := store.Dequeue(ctx, "worker1", nil)

	require.NoError(t, err)
	require.True(t, hasRecord)
	require.Equal(t, 1, record1.ID)
	require.Equal(t, tacosRepoName, record1.RepositoryName)
	require.Equal(t, tacosCommit, record1.Commit)

	record2, hasRecord, err := store.Dequeue(ctx, "worker2", nil)

	require.NoError(t, err)
	require.True(t, hasRecord)
	require.Equal(t, 2, record2.ID)
	require.Equal(t, empanadasRepoName, record2.RepositoryName)
	require.Equal(t, empanadasCommit, record2.Commit)

	_, hasRecord, err = store.Dequeue(ctx, "worker2", nil)
	require.False(t, hasRecord)
	require.NoError(t, err)

}
