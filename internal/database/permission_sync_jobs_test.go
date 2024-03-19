package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestPermissionSyncJobs_CreateAndList(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	user, err := db.Users().Create(ctx, NewUser{Username: "horse"})
	require.NoError(t, err)

	store := PermissionSyncJobsWith(logger, db)
	usersStore := UsersWith(logger, db)
	reposStore := ReposWith(logger, db)

	// Create users.
	user1, err := usersStore.Create(ctx, NewUser{Username: "test-user-1", DisplayName: "t0pc0d3r"})
	require.NoError(t, err)
	user2, err := usersStore.Create(ctx, NewUser{Username: "test-user-2"})
	require.NoError(t, err)

	// Create repos.
	repo1 := types.Repo{Name: "test-repo-1", ID: 101}
	require.NoError(t, reposStore.Create(ctx, &repo1))
	repo2 := types.Repo{Name: "test-repo-2", ID: 201}
	require.NoError(t, reposStore.Create(ctx, &repo2))
	repo3 := types.Repo{Name: "test-repo-3", ID: 303}
	require.NoError(t, reposStore.Create(ctx, &repo3))

	jobs, err := store.List(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)
	require.Len(t, jobs, 0, "jobs returned even though database is empty")

	opts := PermissionSyncJobOpts{Priority: HighPriorityPermissionsSync, InvalidateCaches: true, Reason: ReasonUserNoPermissions, NoPerms: true, TriggeredByUserID: user.ID}
	require.NoError(t, store.CreateRepoSyncJob(ctx, repo1.ID, opts))

	opts = PermissionSyncJobOpts{Priority: MediumPriorityPermissionsSync, InvalidateCaches: true, Reason: ReasonManualUserSync}
	require.NoError(t, store.CreateUserSyncJob(ctx, user1.ID, opts))

	opts = PermissionSyncJobOpts{Priority: LowPriorityPermissionsSync, InvalidateCaches: true, Reason: ReasonUserEmailVerified}
	require.NoError(t, store.CreateUserSyncJob(ctx, user2.ID, opts))

	// Adding 1 failed and 1 partially successful job for repoID = 2.
	require.NoError(t, store.CreateRepoSyncJob(ctx, repo2.ID, PermissionSyncJobOpts{Priority: LowPriorityPermissionsSync, Reason: ReasonGitHubRepoEvent}))
	codeHostStates := getSampleCodeHostStates()
	clock := timeutil.NewFakeClock(time.Now(), 0)
	finishedTime := clock.Now()
	finishSyncJobWithState(t, db, ctx, 4, finishedTime, PermissionsSyncJobStateFailed, codeHostStates[1:])
	// Adding a reason and a message.
	_, err = db.ExecContext(ctx, "UPDATE permission_sync_jobs SET cancellation_reason='i tried to cancel but it already failed', failure_message='imma failure' WHERE id=4")
	require.NoError(t, err)

	// Partial success (one of `codeHostStates` failed).
	require.NoError(t, store.CreateRepoSyncJob(ctx, repo2.ID, PermissionSyncJobOpts{Priority: LowPriorityPermissionsSync, Reason: ReasonGitHubRepoEvent}))
	finishSyncJobWithState(t, db, ctx, 5, finishedTime, PermissionsSyncJobStateCompleted, codeHostStates)

	// Creating a sync job for repoID = 3 and marking it as completed.
	require.NoError(t, store.CreateRepoSyncJob(ctx, repo3.ID, PermissionSyncJobOpts{Priority: LowPriorityPermissionsSync, Reason: ReasonGitHubRepoEvent}))
	// Success.
	finishSyncJobWithState(t, db, ctx, 6, finishedTime, PermissionsSyncJobStateCompleted, codeHostStates[:1])

	jobs, err = store.List(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)

	require.Len(t, jobs, 6, "wrong number of jobs returned")

	wantJobs := []*PermissionSyncJob{
		{
			ID:                jobs[0].ID,
			State:             PermissionsSyncJobStateQueued,
			RepositoryID:      int(repo1.ID),
			Priority:          HighPriorityPermissionsSync,
			InvalidateCaches:  true,
			Reason:            ReasonUserNoPermissions,
			NoPerms:           true,
			TriggeredByUserID: user.ID,
		},
		{
			ID:               jobs[1].ID,
			State:            PermissionsSyncJobStateQueued,
			UserID:           int(user1.ID),
			Priority:         MediumPriorityPermissionsSync,
			InvalidateCaches: true,
			Reason:           ReasonManualUserSync,
		},
		{
			ID:               jobs[2].ID,
			State:            PermissionsSyncJobStateQueued,
			UserID:           int(user2.ID),
			Priority:         LowPriorityPermissionsSync,
			InvalidateCaches: true,
			Reason:           ReasonUserEmailVerified,
		},
		{
			ID:                 jobs[3].ID,
			State:              PermissionsSyncJobStateFailed,
			RepositoryID:       int(repo2.ID),
			Priority:           LowPriorityPermissionsSync,
			Reason:             ReasonGitHubRepoEvent,
			FinishedAt:         finishedTime,
			CodeHostStates:     codeHostStates[1:],
			FailureMessage:     pointers.Ptr("imma failure"),
			CancellationReason: pointers.Ptr("i tried to cancel but it already failed"),
			IsPartialSuccess:   false,
		},
		{
			ID:               jobs[4].ID,
			State:            PermissionsSyncJobStateCompleted,
			RepositoryID:     int(repo2.ID),
			Priority:         LowPriorityPermissionsSync,
			Reason:           ReasonGitHubRepoEvent,
			FinishedAt:       finishedTime,
			CodeHostStates:   codeHostStates,
			IsPartialSuccess: true,
		},
		{
			ID:               jobs[5].ID,
			State:            PermissionsSyncJobStateCompleted,
			RepositoryID:     int(repo3.ID),
			Priority:         LowPriorityPermissionsSync,
			Reason:           ReasonGitHubRepoEvent,
			FinishedAt:       finishedTime,
			CodeHostStates:   codeHostStates[:1],
			IsPartialSuccess: false,
		},
	}
	if diff := cmp.Diff(jobs, wantJobs, cmpopts.IgnoreFields(PermissionSyncJob{}, "QueuedAt")); diff != "" {
		t.Fatalf("jobs[0] has wrong attributes: %s", diff)
	}
	for i, j := range jobs {
		require.NotZerof(t, j.QueuedAt, "job %d has no QueuedAt set", i)
	}

	listTests := []struct {
		name     string
		opts     ListPermissionSyncJobOpts
		wantJobs []*PermissionSyncJob
	}{
		{
			name:     "ID",
			opts:     ListPermissionSyncJobOpts{ID: jobs[0].ID},
			wantJobs: jobs[:1],
		},
		{
			name:     "RepoID",
			opts:     ListPermissionSyncJobOpts{RepoID: jobs[0].RepositoryID},
			wantJobs: jobs[:1],
		},
		{
			name:     "UserID",
			opts:     ListPermissionSyncJobOpts{UserID: jobs[1].UserID},
			wantJobs: jobs[1:2],
		},
		{
			name:     "UserID",
			opts:     ListPermissionSyncJobOpts{UserID: jobs[2].UserID},
			wantJobs: jobs[2:3],
		},
		{
			name:     "State=queued",
			opts:     ListPermissionSyncJobOpts{State: PermissionsSyncJobStateQueued},
			wantJobs: jobs[:3],
		},
		{
			name:     "State=completed (partially successful shouldn't be included)",
			opts:     ListPermissionSyncJobOpts{State: PermissionsSyncJobStateCompleted},
			wantJobs: jobs[5:6],
		},
		{
			name:     "State=failed",
			opts:     ListPermissionSyncJobOpts{State: PermissionsSyncJobStateFailed},
			wantJobs: jobs[3:4],
		},
		{
			name:     "Partial success",
			opts:     ListPermissionSyncJobOpts{PartialSuccess: true},
			wantJobs: jobs[4:5],
		},
		{
			name:     "Partial success overrides provided state",
			opts:     ListPermissionSyncJobOpts{State: PermissionsSyncJobStateFailed, PartialSuccess: true},
			wantJobs: jobs[4:5],
		},
		{
			name:     "Reason filtering",
			opts:     ListPermissionSyncJobOpts{Reason: ReasonManualUserSync},
			wantJobs: jobs[1:2],
		},
		{
			name:     "ReasonGroup filtering",
			opts:     ListPermissionSyncJobOpts{ReasonGroup: PermissionsSyncJobReasonGroupWebhook},
			wantJobs: jobs[3:],
		},
		{
			name:     "ReasonGroup filtering",
			opts:     ListPermissionSyncJobOpts{ReasonGroup: PermissionsSyncJobReasonGroupSourcegraph},
			wantJobs: jobs[2:3],
		},
		{
			name:     "Reason and ReasonGroup filtering (reason filtering wins)",
			opts:     ListPermissionSyncJobOpts{Reason: ReasonManualUserSync, ReasonGroup: PermissionsSyncJobReasonGroupSchedule},
			wantJobs: jobs[1:2],
		},
		{
			name:     "Search doesn't work without SearchType",
			opts:     ListPermissionSyncJobOpts{Query: "where's the search type, Lebowski?"},
			wantJobs: jobs,
		},
		{
			name:     "SearchType alone works as a filter by sync job subject (repository)",
			opts:     ListPermissionSyncJobOpts{SearchType: PermissionsSyncSearchTypeRepo},
			wantJobs: []*PermissionSyncJob{jobs[0], jobs[3], jobs[4], jobs[5]},
		},
		{
			name:     "Repo name search, case-insensitivity",
			opts:     ListPermissionSyncJobOpts{Query: "TeST", SearchType: PermissionsSyncSearchTypeRepo},
			wantJobs: []*PermissionSyncJob{jobs[0], jobs[3], jobs[4], jobs[5]},
		},
		{
			name:     "Repo name search",
			opts:     ListPermissionSyncJobOpts{Query: "1", SearchType: PermissionsSyncSearchTypeRepo},
			wantJobs: jobs[:1],
		},
		{
			name:     "SearchType alone works as a filter by sync job subject (user)",
			opts:     ListPermissionSyncJobOpts{SearchType: PermissionsSyncSearchTypeUser},
			wantJobs: jobs[1:3],
		},
		{
			name:     "User display name search, case-insensitivity",
			opts:     ListPermissionSyncJobOpts{Query: "3", SearchType: PermissionsSyncSearchTypeUser},
			wantJobs: jobs[1:2],
		},
		{
			name:     "User name search",
			opts:     ListPermissionSyncJobOpts{Query: "user-2", SearchType: PermissionsSyncSearchTypeUser},
			wantJobs: jobs[2:3],
		},
		{
			name:     "User name search with pagination",
			opts:     ListPermissionSyncJobOpts{Query: "user-2", SearchType: PermissionsSyncSearchTypeUser, PaginationArgs: &PaginationArgs{First: pointers.Ptr(1)}},
			wantJobs: jobs[2:3],
		},
		{
			name:     "User name search with default OrderBy",
			opts:     ListPermissionSyncJobOpts{Query: "user-2", SearchType: PermissionsSyncSearchTypeUser, PaginationArgs: &PaginationArgs{OrderBy: OrderBy{{Field: "id"}}}},
			wantJobs: jobs[2:3],
		},
	}

	for _, tt := range listTests {
		t.Run(tt.name, func(t *testing.T) {
			have, err := store.List(ctx, tt.opts)
			require.NoError(t, err)
			if len(have) != len(tt.wantJobs) {
				t.Fatalf("wrong number of jobs returned. want=%d, have=%d", len(tt.wantJobs), len(have))
			}
			if diff := cmp.Diff(have, tt.wantJobs); diff != "" {
				t.Fatalf("unexpected jobs. diff: %s", diff)
			}
		})
	}
}

func TestPermissionSyncJobs_GetLatestSyncJob(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	clock := timeutil.NewFakeClock(time.Now(), 0)

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	store := PermissionSyncJobsWith(logger, db)
	usersStore := UsersWith(logger, db)
	reposStore := ReposWith(logger, db)

	// Create users.
	user1, err := usersStore.Create(ctx, NewUser{Username: "test-user-1", DisplayName: "t0pc0d3r"})
	require.NoError(t, err)
	user2, err := usersStore.Create(ctx, NewUser{Username: "test-user-2"})
	require.NoError(t, err)

	// Create repos.
	repo1 := types.Repo{Name: "test-repo-1", ID: 101}
	err = reposStore.Create(ctx, &repo1)
	require.NoError(t, err)
	repo2 := types.Repo{Name: "test-repo-2", ID: 201}
	err = reposStore.Create(ctx, &repo2)
	require.NoError(t, err)

	t.Run("No jobs", func(t *testing.T) {
		job, err := store.GetLatestFinishedSyncJob(ctx, ListPermissionSyncJobOpts{})
		require.NoError(t, err)
		require.Nil(t, job, "should not return any job")
	})

	t.Run("One finished job", func(t *testing.T) {
		createSyncJob(t, store, ctx, user1.ID, 0) // id = 1
		createSyncJob(t, store, ctx, user2.ID, 0) // id = 2

		finishSyncJob(t, db, ctx, 1, clock.Now())

		job, err := store.GetLatestFinishedSyncJob(ctx, ListPermissionSyncJobOpts{})
		require.NoError(t, err)
		require.NotNil(t, job, "should return a job")
		require.Equal(t, 1, job.ID, "wrong job ID")
	})

	t.Run("Two finished jobs", func(t *testing.T) {
		t.Cleanup(func() { cleanupSyncJobs(t, db, ctx) })

		createSyncJob(t, store, ctx, user1.ID, 0) // id = 1
		createSyncJob(t, store, ctx, user2.ID, 0) // id = 2

		finishSyncJob(t, db, ctx, 1, clock.Now().Add(-1*time.Hour))
		finishSyncJob(t, db, ctx, 2, clock.Now().Add(-1*time.Minute))

		job, err := store.GetLatestFinishedSyncJob(ctx, ListPermissionSyncJobOpts{})
		require.NoError(t, err)
		require.NotNil(t, job, "should return a job")
		require.Equal(t, 2, job.ID, "wrong job ID")
	})

	t.Run("Three finished jobs, but one cancelled", func(t *testing.T) {
		t.Cleanup(func() { cleanupSyncJobs(t, db, ctx) })

		createSyncJob(t, store, ctx, user1.ID, 0) // id = 1
		createSyncJob(t, store, ctx, user2.ID, 0) // id = 2
		createSyncJob(t, store, ctx, 0, repo1.ID) // id = 3

		finishSyncJob(t, db, ctx, 1, clock.Now().Add(-1*time.Hour))
		finishSyncJobWithCancel(t, db, ctx, 2, clock.Now().Add(-1*time.Minute))
		finishSyncJob(t, db, ctx, 3, clock.Now().Add(-10*time.Minute))

		job, err := store.GetLatestFinishedSyncJob(ctx, ListPermissionSyncJobOpts{
			NotCanceled: true,
		})
		require.NoError(t, err)
		require.NotNil(t, job, "should return a job")
		require.Equal(t, 3, job.ID, "wrong job ID")
	})

	t.Run("Two finished jobs for each user, pick userIDs latest", func(t *testing.T) {
		t.Cleanup(func() { cleanupSyncJobs(t, db, ctx) })

		createSyncJob(t, store, ctx, user1.ID, 0) // id = 1
		createSyncJob(t, store, ctx, user2.ID, 0) // id = 2
		finishSyncJob(t, db, ctx, 1, clock.Now().Add(-1*time.Hour))
		finishSyncJob(t, db, ctx, 2, clock.Now().Add(-10*time.Minute))

		createSyncJob(t, store, ctx, user2.ID, 0) // id = 3
		createSyncJob(t, store, ctx, user1.ID, 0) // id = 4
		finishSyncJob(t, db, ctx, 3, clock.Now().Add(-1*time.Minute))
		finishSyncJob(t, db, ctx, 4, clock.Now())

		job, err := store.GetLatestFinishedSyncJob(ctx, ListPermissionSyncJobOpts{
			UserID: int(user2.ID),
		})
		require.NoError(t, err)
		require.NotNil(t, job, "should return a job")
		require.Equal(t, 3, job.ID, "wrong job ID")
	})
}

func TestPermissionSyncJobs_Deduplication(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	clock := timeutil.NewFakeClock(time.Now(), 0)

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	user1, err := db.Users().Create(ctx, NewUser{Username: "horse"})
	require.NoError(t, err)

	user2, err := db.Users().Create(ctx, NewUser{Username: "graph"})
	require.NoError(t, err)

	store := PermissionSyncJobsWith(logger, db)

	// 1) Insert low priority job without process_after for user1.
	user1LowPrioJob := PermissionSyncJobOpts{Reason: ReasonManualUserSync, TriggeredByUserID: user1.ID}
	err = store.CreateUserSyncJob(ctx, 1, user1LowPrioJob)
	require.NoError(t, err)

	allJobs, err := store.List(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)
	// Check that we have 1 job with userID=1.
	require.Len(t, allJobs, 1)
	require.Equal(t, 1, allJobs[0].UserID)

	// 2) Insert low priority job without process_after for user2.
	user2LowPrioJob := PermissionSyncJobOpts{Reason: ReasonManualUserSync, TriggeredByUserID: user2.ID}
	err = store.CreateUserSyncJob(ctx, 2, user2LowPrioJob)
	require.NoError(t, err)

	allJobs, err = store.List(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)
	// Check that we have 2 jobs including job for userID=2. Job ID should match user ID.
	require.Len(t, allJobs, 2)
	require.Equal(t, allJobs[0].ID, allJobs[0].UserID)
	require.Equal(t, allJobs[1].ID, allJobs[1].UserID)

	// 3) Another low priority job without process_after for user1 is dropped.
	err = store.CreateUserSyncJob(ctx, 1, user1LowPrioJob)
	require.NoError(t, err)

	allJobs, err = store.List(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)
	// Check that we still have 2 jobs. Job ID should match user ID.
	require.Len(t, allJobs, 2)
	require.Equal(t, allJobs[0].ID, allJobs[0].UserID)
	require.Equal(t, allJobs[1].ID, allJobs[1].UserID)

	// 4) Insert some low priority jobs with process_after for both users. All of them should be inserted.
	fiveMinutesLater := clock.Now().Add(5 * time.Minute)
	tenMinutesLater := clock.Now().Add(10 * time.Minute)
	user1LowPrioDelayedJob := PermissionSyncJobOpts{ProcessAfter: fiveMinutesLater, Reason: ReasonManualUserSync, TriggeredByUserID: user1.ID}
	user2LowPrioDelayedJob := PermissionSyncJobOpts{ProcessAfter: tenMinutesLater, Reason: ReasonManualUserSync, TriggeredByUserID: user1.ID}

	err = store.CreateUserSyncJob(ctx, 1, user1LowPrioDelayedJob)
	require.NoError(t, err)

	err = store.CreateUserSyncJob(ctx, 2, user2LowPrioDelayedJob)
	require.NoError(t, err)

	allDelayedJobs, err := store.List(ctx, ListPermissionSyncJobOpts{NotNullProcessAfter: true})
	require.NoError(t, err)
	// Check that we have 2 delayed jobs in total.
	require.Len(t, allDelayedJobs, 2)
	// UserID of the job should be (jobID - 2).
	require.Equal(t, allDelayedJobs[0].UserID, allDelayedJobs[0].ID-2)
	require.Equal(t, allDelayedJobs[1].UserID, allDelayedJobs[1].ID-2)

	// 5) Insert *medium* priority job without process_after for user1. Check that low priority job is canceled.
	user1MediumPrioJob := PermissionSyncJobOpts{Priority: MediumPriorityPermissionsSync, Reason: ReasonManualUserSync, TriggeredByUserID: user1.ID}
	err = store.CreateUserSyncJob(ctx, 1, user1MediumPrioJob)
	require.NoError(t, err)

	allUser1Jobs, err := store.List(ctx, ListPermissionSyncJobOpts{UserID: 1})
	require.NoError(t, err)
	// Check that we have 3 jobs for userID=1 in total (low prio (canceled), delayed, medium prio).
	require.Len(t, allUser1Jobs, 3)
	// Check that low prio job (ID=1) is canceled and others are not.
	for _, job := range allUser1Jobs {
		if job.ID == 1 {
			require.True(t, job.Cancel)
			require.Equal(t, PermissionsSyncJobStateCanceled, job.State)
		} else {
			require.False(t, job.Cancel)
		}
	}

	// 6) Insert some medium priority jobs with process_after for both users. All of them should be inserted.
	user1MediumPrioDelayedJob := PermissionSyncJobOpts{Priority: MediumPriorityPermissionsSync, ProcessAfter: fiveMinutesLater, Reason: ReasonManualUserSync, TriggeredByUserID: user1.ID}
	user2MediumPrioDelayedJob := PermissionSyncJobOpts{Priority: MediumPriorityPermissionsSync, ProcessAfter: tenMinutesLater, Reason: ReasonManualUserSync, TriggeredByUserID: user1.ID}

	err = store.CreateUserSyncJob(ctx, 1, user1MediumPrioDelayedJob)
	require.NoError(t, err)

	err = store.CreateUserSyncJob(ctx, 2, user2MediumPrioDelayedJob)
	require.NoError(t, err)

	allDelayedJobs, err = store.List(ctx, ListPermissionSyncJobOpts{NotNullProcessAfter: true})
	require.NoError(t, err)
	// Check that we have 2 delayed jobs in total.
	require.Len(t, allDelayedJobs, 4)
	// UserID of the job should be (jobID - 2).
	require.Equal(t, allDelayedJobs[0].UserID, allDelayedJobs[0].ID-2)
	require.Equal(t, allDelayedJobs[1].UserID, allDelayedJobs[1].ID-2)
	require.Equal(t, allDelayedJobs[2].UserID, allDelayedJobs[1].ID-3)
	require.Equal(t, allDelayedJobs[3].UserID, allDelayedJobs[1].ID-2)

	// 5) Insert *high* priority job without process_after for user1. Check that medium and low priority job is canceled.
	user1HighPrioJob := PermissionSyncJobOpts{Priority: HighPriorityPermissionsSync, Reason: ReasonManualUserSync, TriggeredByUserID: user1.ID}
	err = store.CreateUserSyncJob(ctx, 1, user1HighPrioJob)
	require.NoError(t, err)

	allUser1Jobs, err = store.List(ctx, ListPermissionSyncJobOpts{UserID: 1})
	require.NoError(t, err)
	// Check that we have 3 jobs for userID=1 in total (medium prio (canceled), delayed, high prio).
	require.Len(t, allUser1Jobs, 5)
	// Check that medium prio job (ID=3) is canceled and others are not.
	for _, job := range allUser1Jobs {
		if job.ID == 1 || job.ID == 5 {
			require.True(t, job.Cancel)
		} else {
			require.False(t, job.Cancel)
		}
	}

	// 6) Insert another low and high priority jobs without process_after for user1.
	// Check that all of them are dropped since we already have a high prio job.
	err = store.CreateUserSyncJob(ctx, 1, user1LowPrioJob)
	require.NoError(t, err)

	err = store.CreateUserSyncJob(ctx, 1, user1HighPrioJob)
	require.NoError(t, err)

	allUser1Jobs, err = store.List(ctx, ListPermissionSyncJobOpts{UserID: 1})
	require.NoError(t, err)
	// Check that we still have 3 jobs for userID=1 in total (low prio (canceled), medium prio (cancelled), high prio).
	require.Len(t, allUser1Jobs, 5)

	// 7) Check that not "queued" jobs doesn't affect duplicates check: let's change high prio job to "processing"
	// and insert one low prio after that.
	result, err := db.ExecContext(ctx, "UPDATE permission_sync_jobs SET state='processing' WHERE id=7")
	require.NoError(t, err)
	updatedRows, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), updatedRows)

	// Now we're good to insert new low prio job.
	err = store.CreateUserSyncJob(ctx, 1, user1LowPrioJob)
	require.NoError(t, err)

	allUser1Jobs, err = store.List(ctx, ListPermissionSyncJobOpts{UserID: 1})
	require.NoError(t, err)
	// Check that we now have 4 jobs for userID=1 in total (low prio (canceled), delayed, high prio (processing), NEW low prio).
	require.Len(t, allUser1Jobs, 5)
}

func TestPermissionSyncJobs_CancelQueuedJob(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	store := PermissionSyncJobsWith(logger, db)
	reposStore := ReposWith(logger, db)

	// Create a repo.
	repo1 := types.Repo{Name: "test-repo-1", ID: 101}
	err := reposStore.Create(ctx, &repo1)
	require.NoError(t, err)

	// Test that cancelling non-existent job errors out.
	err = store.CancelQueuedJob(ctx, CancellationReasonHigherPriority, 1)
	require.True(t, errcode.IsNotFound(err))

	// Adding a job.
	err = store.CreateRepoSyncJob(ctx, repo1.ID, PermissionSyncJobOpts{Reason: ReasonManualUserSync})
	require.NoError(t, err)

	// Cancelling a job should be successful now.
	err = store.CancelQueuedJob(ctx, CancellationReasonHigherPriority, 1)
	require.NoError(t, err)
	// Checking that cancellation reason is set.
	cancelledJob, err := store.List(ctx, ListPermissionSyncJobOpts{RepoID: int(repo1.ID)})
	require.NoError(t, err)
	require.Len(t, cancelledJob, 1)
	require.Equal(t, CancellationReasonHigherPriority, *cancelledJob[0].CancellationReason)

	// Cancelling already cancelled job doesn't make sense and errors out as well.
	err = store.CancelQueuedJob(ctx, CancellationReasonHigherPriority, 1)
	require.True(t, errcode.IsNotFound(err))

	// Adding another job and setting it to "processing" state.
	err = store.CreateRepoSyncJob(ctx, repo1.ID, PermissionSyncJobOpts{Reason: ReasonManualRepoSync})
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE permission_sync_jobs SET state='processing' WHERE id=2")
	require.NoError(t, err)

	// Cancelling it errors out because it is in a state different from "queued".
	err = store.CancelQueuedJob(ctx, CancellationReasonHigherPriority, 2)
	require.True(t, errcode.IsNotFound(err))
}

func TestPermissionSyncJobs_SaveSyncResult(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	store := PermissionSyncJobsWith(logger, db)
	reposStore := ReposWith(logger, db)

	// Create repo.
	repo1 := types.Repo{Name: "test-repo-1", ID: 101}
	err := reposStore.Create(ctx, &repo1)
	require.NoError(t, err)

	// Creating result.
	result := SetPermissionsResult{
		Added:   1,
		Removed: 2,
		Found:   5,
	}

	// Creating code host states.
	codeHostStates := getSampleCodeHostStates()
	// Adding a job.
	err = store.CreateRepoSyncJob(ctx, repo1.ID, PermissionSyncJobOpts{Reason: ReasonManualUserSync})
	require.NoError(t, err)

	// Saving result should be successful.
	err = store.SaveSyncResult(ctx, 1, true, &result, codeHostStates)
	require.NoError(t, err)

	// Checking that all the results are set.
	jobs, err := store.List(ctx, ListPermissionSyncJobOpts{RepoID: int(repo1.ID)})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	theJob := jobs[0]
	require.Equal(t, 1, theJob.PermissionsAdded)
	require.Equal(t, 2, theJob.PermissionsRemoved)
	require.Equal(t, 5, theJob.PermissionsFound)
	require.Equal(t, codeHostStates, theJob.CodeHostStates)
	require.True(t, theJob.IsPartialSuccess)

	// Saving nil result (in case of errors from code host) should be also successful.
	err = store.SaveSyncResult(ctx, 1, false, nil, codeHostStates[1:])
	require.NoError(t, err)

	// Checking that all the results are set.
	jobs, err = store.List(ctx, ListPermissionSyncJobOpts{RepoID: int(repo1.ID)})
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	theJob = jobs[0]
	require.Equal(t, 0, theJob.PermissionsAdded)
	require.Equal(t, 0, theJob.PermissionsRemoved)
	require.Equal(t, 0, theJob.PermissionsFound)
	require.Equal(t, codeHostStates[1:], theJob.CodeHostStates)
	require.False(t, theJob.IsPartialSuccess)
}

func TestPermissionSyncJobs_CascadeOnRepoDelete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	store := PermissionSyncJobsWith(logger, db)
	reposStore := ReposWith(logger, db)

	// Create a repo.
	repo1 := types.Repo{Name: "test-repo-1", ID: 101}
	err := reposStore.Create(ctx, &repo1)
	require.NoError(t, err)

	// Adding a job.
	err = store.CreateRepoSyncJob(ctx, repo1.ID, PermissionSyncJobOpts{Reason: ReasonManualRepoSync})
	require.NoError(t, err)

	// Checking that the job is created.
	jobs, err := store.List(ctx, ListPermissionSyncJobOpts{RepoID: int(repo1.ID)})
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	// Deleting repo.
	_, err = db.ExecContext(context.Background(), fmt.Sprintf(`DELETE FROM repo WHERE id = %d`, int(repo1.ID)))
	require.NoError(t, err)

	// Checking that the job is deleted.
	jobs, err = store.List(ctx, ListPermissionSyncJobOpts{RepoID: int(repo1.ID)})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

func TestPermissionSyncJobs_CascadeOnUserDelete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	store := PermissionSyncJobsWith(logger, db)
	usersStore := UsersWith(logger, db)

	// Create a user.
	user1, err := usersStore.Create(ctx, NewUser{Username: "test-user-1"})
	require.NoError(t, err)

	// Adding a job.
	err = store.CreateUserSyncJob(ctx, user1.ID, PermissionSyncJobOpts{Reason: ReasonManualRepoSync})
	require.NoError(t, err)

	// Checking that the job is created.
	jobs, err := store.List(ctx, ListPermissionSyncJobOpts{UserID: int(user1.ID)})
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	// Deleting user.
	err = usersStore.HardDelete(ctx, user1.ID)
	require.NoError(t, err)

	// Checking that the job is deleted.
	jobs, err = store.List(ctx, ListPermissionSyncJobOpts{UserID: int(user1.ID)})
	require.NoError(t, err)
	require.Empty(t, jobs)
}

func TestPermissionSyncJobs_Pagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	user, err := db.Users().Create(ctx, NewUser{Username: "horse"})
	require.NoError(t, err)

	store := PermissionSyncJobsWith(logger, db)

	// Create 10 sync jobs.
	createSyncJobs(t, ctx, user.ID, store)

	jobs, err := store.List(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)

	paginationTests := []struct {
		name           string
		paginationArgs PaginationArgs
		wantJobs       []*PermissionSyncJob
	}{
		{
			name:           "After",
			paginationArgs: PaginationArgs{OrderBy: []OrderByOption{{Field: "user_id"}}, Ascending: true, After: []any{int32(1)}},
			wantJobs:       []*PermissionSyncJob{},
		},
		{
			name:           "Before",
			paginationArgs: PaginationArgs{OrderBy: []OrderByOption{{Field: "user_id"}}, Ascending: true, Before: []any{int32(2)}},
			wantJobs:       jobs,
		},
		{
			name:           "First",
			paginationArgs: PaginationArgs{Ascending: true, First: pointers.Ptr(5)},
			wantJobs:       jobs[:5],
		},
		{
			name:           "OrderBy",
			paginationArgs: PaginationArgs{OrderBy: []OrderByOption{{Field: "queued_at"}}, Ascending: false},
			wantJobs:       reverse(jobs),
		},
	}

	for _, tt := range paginationTests {
		t.Run(tt.name, func(t *testing.T) {
			have, err := store.List(ctx, ListPermissionSyncJobOpts{PaginationArgs: &tt.paginationArgs})
			require.NoError(t, err)
			if len(have) != len(tt.wantJobs) {
				t.Fatalf("wrong number of jobs returned. want=%d, have=%d", len(tt.wantJobs), len(have))
			}
			if len(tt.wantJobs) > 0 {
				if diff := cmp.Diff(tt.wantJobs, have); diff != "" {
					t.Fatalf("unexpected jobs. diff: %s", diff)
				}
			}
		})
	}
}

func TestPermissionSyncJobs_Count(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	user, err := db.Users().Create(ctx, NewUser{Username: "horse"})
	require.NoError(t, err)

	store := PermissionSyncJobsWith(logger, db)

	// Create 10 sync jobs.
	createSyncJobs(t, ctx, user.ID, store)

	_, err = store.List(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)

	count, err := store.Count(ctx, ListPermissionSyncJobOpts{})
	require.NoError(t, err)
	require.Equal(t, 10, count)

	// Create 10 more sync jobs.
	createSyncJobs(t, ctx, user.ID, store)
	// Now we will count only the ReasonManualUserSync jobs (which should be a half
	// of all jobs).
	count, err = store.Count(ctx, ListPermissionSyncJobOpts{Reason: ReasonManualUserSync})
	require.NoError(t, err)
	require.Equal(t, 10, count)

	// Counting with user search.
	count, err = store.Count(ctx, ListPermissionSyncJobOpts{SearchType: PermissionsSyncSearchTypeUser, Query: "hors"})
	require.NoError(t, err)
	require.Equal(t, 20, count)

	// Counting with repo search.
	count, err = store.Count(ctx, ListPermissionSyncJobOpts{SearchType: PermissionsSyncSearchTypeRepo, Query: "no :("})
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestPermissionSyncJobs_CountUsersWithFailingSyncJob(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	clock := timeutil.NewFakeClock(time.Now(), 0)

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	store := PermissionSyncJobsWith(logger, db)
	usersStore := UsersWith(logger, db)

	// Create users.
	user1, err := usersStore.Create(ctx, NewUser{Username: "test-user-1", DisplayName: "t0pc0d3r"})
	require.NoError(t, err)
	user2, err := usersStore.Create(ctx, NewUser{Username: "test-user-2"})
	require.NoError(t, err)

	t.Run("No jobs", func(t *testing.T) {
		count, err := store.CountUsersWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(0), count, "wrong count")
	})

	t.Run("No failining sync job", func(t *testing.T) {
		cleanupSyncJobs(t, db, ctx)
		createSyncJob(t, store, ctx, user1.ID, 0) // id = 1
		createSyncJob(t, store, ctx, user2.ID, 0) // id = 2

		finishSyncJob(t, db, ctx, 1, clock.Now())

		count, err := store.CountUsersWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(0), count, "wrong count")
	})

	t.Run("No latest failing sync job", func(t *testing.T) {
		cleanupSyncJobs(t, db, ctx)
		createSyncJob(t, store, ctx, user1.ID, 0) // id = 1
		finishSyncJob(t, db, ctx, 1, clock.Now().Add(-1*time.Minute))

		createSyncJob(t, store, ctx, user1.ID, 0) // id = 2
		finishSyncJobWithFailure(t, db, ctx, 2, clock.Now().Add(-1*time.Hour))

		createSyncJob(t, store, ctx, user2.ID, 0) // id = 3
		finishSyncJob(t, db, ctx, 3, clock.Now())

		count, err := store.CountUsersWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(0), count, "wrong count")
	})

	t.Run("With latest failing sync job", func(t *testing.T) {
		cleanupSyncJobs(t, db, ctx)
		createSyncJob(t, store, ctx, user1.ID, 0) // id = 1
		finishSyncJob(t, db, ctx, 1, clock.Now().Add(-1*time.Hour))

		createSyncJob(t, store, ctx, user1.ID, 0) // id = 2
		finishSyncJobWithFailure(t, db, ctx, 2, clock.Now().Add(-1*time.Minute))

		createSyncJob(t, store, ctx, user1.ID, 0) // id = 3
		finishSyncJobWithCancel(t, db, ctx, 3, clock.Now().Add(-1*time.Minute))

		createSyncJob(t, store, ctx, user2.ID, 0) // id = 4
		finishSyncJobWithFailure(t, db, ctx, 4, clock.Now())

		count, err := store.CountUsersWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(2), count, "wrong count")
	})
}

func TestPermissionSyncJobs_CountReposWithFailingSyncJob(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	clock := timeutil.NewFakeClock(time.Now(), 0)

	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	store := PermissionSyncJobsWith(logger, db)
	reposStore := ReposWith(logger, db)

	// Create repos.
	repo1 := types.Repo{Name: "test-repo-1", ID: 101}
	err := reposStore.Create(ctx, &repo1)
	require.NoError(t, err)
	repo2 := types.Repo{Name: "test-repo-2", ID: 201}
	err = reposStore.Create(ctx, &repo2)
	require.NoError(t, err)

	t.Run("No jobs", func(t *testing.T) {
		count, err := store.CountReposWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(0), count, "wrong count")
	})

	t.Run("No failining sync job", func(t *testing.T) {
		cleanupSyncJobs(t, db, ctx)
		createSyncJob(t, store, ctx, 0, repo1.ID) // id = 1
		createSyncJob(t, store, ctx, 0, repo2.ID) // id = 2

		finishSyncJob(t, db, ctx, 1, clock.Now())

		count, err := store.CountReposWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(0), count, "wrong count")
	})

	t.Run("No latest failing sync job", func(t *testing.T) {
		cleanupSyncJobs(t, db, ctx)
		createSyncJob(t, store, ctx, 0, repo1.ID) // id = 1
		finishSyncJob(t, db, ctx, 1, clock.Now().Add(-1*time.Minute))

		createSyncJob(t, store, ctx, 0, repo1.ID) // id = 2
		finishSyncJobWithFailure(t, db, ctx, 2, clock.Now().Add(-1*time.Hour))

		createSyncJob(t, store, ctx, 0, repo2.ID) // id = 3
		finishSyncJob(t, db, ctx, 3, clock.Now())

		count, err := store.CountReposWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(0), count, "wrong count")
	})

	t.Run("With latest failing sync job", func(t *testing.T) {
		cleanupSyncJobs(t, db, ctx)
		createSyncJob(t, store, ctx, 0, repo1.ID) // id = 1
		finishSyncJob(t, db, ctx, 1, clock.Now().Add(-1*time.Hour))

		createSyncJob(t, store, ctx, 0, repo1.ID) // id = 2
		finishSyncJobWithFailure(t, db, ctx, 2, clock.Now().Add(-1*time.Minute))

		createSyncJob(t, store, ctx, 0, repo1.ID) // id = 3
		finishSyncJobWithCancel(t, db, ctx, 3, clock.Now().Add(-1*time.Minute))

		createSyncJob(t, store, ctx, 0, repo2.ID) // id = 4
		finishSyncJobWithFailure(t, db, ctx, 4, clock.Now())

		count, err := store.CountReposWithFailingSyncJob(ctx)
		require.NoError(t, err)
		require.Equal(t, int32(2), count, "wrong count")
	})
}

// createSyncJobs creates 10 sync jobs, half with the ReasonManualUserSync reason
// and half with the ReasonGitHubUserMembershipRemovedEvent reason.
func createSyncJobs(t *testing.T, ctx context.Context, userID int32, store PermissionSyncJobStore) {
	t.Helper()
	clock := timeutil.NewFakeClock(time.Now(), 0)
	for i := range 10 {
		processAfter := clock.Now().Add(5 * time.Minute)
		reason := ReasonManualUserSync
		if i%2 == 0 {
			reason = ReasonGitHubUserMembershipRemovedEvent
		}
		opts := PermissionSyncJobOpts{Priority: MediumPriorityPermissionsSync, InvalidateCaches: true, ProcessAfter: processAfter, Reason: reason}
		err := store.CreateUserSyncJob(ctx, userID, opts)
		require.NoError(t, err)
	}
}

func createSyncJob(t *testing.T, store PermissionSyncJobStore, ctx context.Context, userID int32, repoID api.RepoID) {
	t.Helper()

	opts := PermissionSyncJobOpts{Priority: HighPriorityPermissionsSync, InvalidateCaches: true, Reason: ReasonUserNoPermissions, NoPerms: true}
	if userID != 0 {
		err := store.CreateUserSyncJob(ctx, userID, opts)
		require.NoError(t, err)
	}
	if repoID != 0 {
		err := store.CreateRepoSyncJob(ctx, repoID, opts)
		require.NoError(t, err)
	}
}

func finishSyncJobWithState(t *testing.T, db DB, ctx context.Context, id int, finishedAt time.Time, state PermissionsSyncJobState, statuses CodeHostStatusesSet) {
	t.Helper()

	query := sqlf.Sprintf("UPDATE permission_sync_jobs SET finished_at = %s, state = %s WHERE id = %d", finishedAt, state, id)

	_, err := db.ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	require.NoError(t, err)

	err = db.PermissionSyncJobs().SaveSyncResult(ctx, id, state == PermissionsSyncJobStateCompleted, nil, statuses)
	require.NoError(t, err)
}

func finishSyncJobWithFailure(t *testing.T, db DB, ctx context.Context, id int, finishedAt time.Time) {
	t.Helper()

	query := sqlf.Sprintf("UPDATE permission_sync_jobs SET finished_at = %s, state = %s WHERE id = %d", finishedAt, PermissionsSyncJobStateFailed, id)

	_, err := db.ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	require.NoError(t, err)
}

func finishSyncJobWithCancel(t *testing.T, db DB, ctx context.Context, id int, finishedAt time.Time) {
	t.Helper()

	query := sqlf.Sprintf("UPDATE permission_sync_jobs SET finished_at = %s, state = %s, cancel = true WHERE id = %d", finishedAt, PermissionsSyncJobStateCanceled, id)

	_, err := db.ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	require.NoError(t, err)
}

func finishSyncJob(t *testing.T, db DB, ctx context.Context, id int, finishedAt time.Time) {
	t.Helper()

	query := sqlf.Sprintf("UPDATE permission_sync_jobs SET finished_at = %s, state = %s WHERE id = %d", finishedAt, PermissionsSyncJobStateCompleted, id)

	_, err := db.ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	require.NoError(t, err)
}

func cleanupSyncJobs(t *testing.T, db DB, ctx context.Context) {
	t.Helper()

	if t.Failed() {
		return
	}

	_, err := db.ExecContext(ctx, "TRUNCATE TABLE permission_sync_jobs; ALTER SEQUENCE permission_sync_jobs_id_seq RESTART WITH 1")
	require.NoError(t, err)
}

func reverse(jobs []*PermissionSyncJob) []*PermissionSyncJob {
	reversed := make([]*PermissionSyncJob, 0, len(jobs))
	for i := range len(jobs) {
		reversed = append(reversed, jobs[len(jobs)-i-1])
	}
	return reversed
}

func getSampleCodeHostStates() []PermissionSyncCodeHostState {
	return []PermissionSyncCodeHostState{
		{
			ProviderID:   "ID",
			ProviderType: "Type",
			Status:       CodeHostStatusSuccess,
			Message:      "successful success",
		},
		{
			ProviderID:   "ID",
			ProviderType: "Type",
			Status:       CodeHostStatusError,
			Message:      "unsuccessful unsuccess :(",
		},
	}

}
