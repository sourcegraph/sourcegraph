package repos_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestStoreEnqueueSyncJobs(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	ctx := context.Background()
	clock := timeutil.NewFakeClock(time.Now(), 0)
	now := clock.Now()

	services := generateExternalServices(10, mkExternalServices(now)...)

	type testCase struct {
		name            string
		stored          types.ExternalServices
		queued          func(types.ExternalServices) []int64
		ignoreSiteAdmin bool
		err             error
	}

	var testCases []testCase

	testCases = append(testCases, testCase{
		name: "enqueue everything",
		stored: services.With(func(s *types.ExternalService) {
			s.NextSyncAt = now.Add(-10 * time.Second)
		}),
		queued: func(svcs types.ExternalServices) []int64 { return svcs.IDs() },
	})

	testCases = append(testCases, testCase{
		name: "nothing to enqueue",
		stored: services.With(func(s *types.ExternalService) {
			s.NextSyncAt = now.Add(10 * time.Second)
		}),
		queued: func(svcs types.ExternalServices) []int64 { return []int64{} },
	})

	testCases = append(testCases, testCase{
		name: "ignore siteadmin repos",
		stored: services.With(func(s *types.ExternalService) {
			s.NextSyncAt = now.Add(10 * time.Second)
		}),
		ignoreSiteAdmin: true,
		queued:          func(svcs types.ExternalServices) []int64 { return []int64{} },
	})

	{
		i := 0
		testCases = append(testCases, testCase{
			name: "some to enqueue",
			stored: services.With(func(s *types.ExternalService) {
				if i%2 == 0 {
					s.NextSyncAt = now.Add(10 * time.Second)
				} else {
					s.NextSyncAt = now.Add(-10 * time.Second)
				}
				i++
			}),
			queued: func(svcs types.ExternalServices) []int64 {
				var ids []int64
				for i := range svcs {
					if i%2 != 0 {
						ids = append(ids, svcs[i].ID)
					}
				}
				return ids
			},
		})
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				q := sqlf.Sprintf("DELETE FROM external_service_sync_jobs;DELETE FROM external_services")
				if _, err := store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
					t.Fatal(err)
				}
			})
			stored := tc.stored.Clone()

			if err := store.ExternalServiceStore().Upsert(ctx, stored...); err != nil {
				t.Fatalf("failed to setup store: %v", err)
			}

			err := store.EnqueueSyncJobs(ctx, tc.ignoreSiteAdmin)
			if have, want := fmt.Sprint(err), fmt.Sprint(tc.err); have != want {
				t.Errorf("error:\nhave: %v\nwant: %v", have, want)
			}

			jobs, err := store.ListSyncJobs(ctx)
			if err != nil {
				t.Fatal(err)
			}

			gotIDs := make([]int64, 0, len(jobs))
			for _, job := range jobs {
				gotIDs = append(gotIDs, job.ExternalServiceID)
			}

			want := tc.queued(stored)
			sort.Slice(gotIDs, func(i, j int) bool {
				return gotIDs[i] < gotIDs[j]
			})
			sort.Slice(want, func(i, j int) bool {
				return want[i] < want[j]
			})

			if diff := cmp.Diff(want, gotIDs); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestStoreEnqueueSingleSyncJob(t *testing.T) {
	t.Parallel()
	store := getTestRepoStore(t)

	logger := logtest.Scoped(t)
	clock := timeutil.NewFakeClock(time.Now(), 0)
	now := clock.Now()

	ctx := context.Background()
	t.Cleanup(func() {
		q := sqlf.Sprintf("DELETE FROM external_service_sync_jobs;DELETE FROM external_services")
		if _, err := store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
			t.Fatal(err)
		}
	})
	service := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create a new external service
	confGet := func() *conf.Unified { return &conf.Unified{} }

	err := database.ExternalServicesWith(logger, store).Create(ctx, confGet, &service)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncJobCount(t, store, 0)

	err = store.EnqueueSingleSyncJob(ctx, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncJobCount(t, store, 1)

	// Doing it again should not fail or add a new row
	err = store.EnqueueSingleSyncJob(ctx, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncJobCount(t, store, 1)

	// If we change status to processing it should not add a new row
	q := sqlf.Sprintf("UPDATE external_service_sync_jobs SET state='processing'")
	if _, err := store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
		t.Fatal(err)
	}
	err = store.EnqueueSingleSyncJob(ctx, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncJobCount(t, store, 1)

	// If we change status to completed we should be able to enqueue another one
	q = sqlf.Sprintf("UPDATE external_service_sync_jobs SET state='completed'")
	if _, err = store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
		t.Fatal(err)
	}
	err = store.EnqueueSingleSyncJob(ctx, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncJobCount(t, store, 2)

	// Test that cloud default external services don't get jobs enqueued (no-ops instead of errors)
	q = sqlf.Sprintf("UPDATE external_service_sync_jobs SET state='completed'")
	if _, err = store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
		t.Fatal(err)
	}

	service.CloudDefault = true
	err = store.ExternalServiceStore().Upsert(ctx, &service)
	if err != nil {
		t.Fatal(err)
	}

	err = store.EnqueueSingleSyncJob(ctx, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncJobCount(t, store, 2)

	// Test that cloud default external services don't get jobs enqueued also when there are no job rows.
	q = sqlf.Sprintf("DELETE FROM external_service_sync_jobs")
	if _, err = store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
		t.Fatal(err)
	}

	err = store.EnqueueSingleSyncJob(ctx, service.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertSyncJobCount(t, store, 0)
}

func assertSyncJobCount(t *testing.T, store repos.Store, want int) {
	t.Helper()
	var count int
	q := sqlf.Sprintf("SELECT COUNT(*) FROM external_service_sync_jobs")
	if err := store.Handle().QueryRowContext(context.Background(), q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("Expected %d rows, got %d", want, count)
	}
}

func TestStoreEnqueuingSyncJobsWhileExtSvcBeingDeleted(t *testing.T) {
	// This test tests two methods: EnqueueSingleSyncJob and EnqueueSyncJobs.
	// It makes sure that both can't enqueue a sync job while an external
	// service is locked for deletion.

	t.Parallel()
	store := getTestRepoStore(t)

	logger := logtest.Scoped(t)
	clock := timeutil.NewFakeClock(time.Now(), 0)
	now := clock.Now()
	ctx := context.Background()
	confGet := func() *conf.Unified { return &conf.Unified{} }

	type testSubject func(*testing.T, context.Context, repos.Store, *types.ExternalService)

	enqueuingFuncs := map[string]testSubject{
		"EnqueueSingleSyncJob": func(t *testing.T, ctx context.Context, store repos.Store, service *types.ExternalService) {
			t.Helper()
			if err := store.EnqueueSingleSyncJob(ctx, service.ID); err != nil {
				t.Fatal(err)
			}
		},
		"EnqueueSyncJobs": func(t *testing.T, ctx context.Context, store repos.Store, _ *types.ExternalService) {
			t.Helper()
			if err := store.EnqueueSyncJobs(ctx, false); err != nil {
				t.Fatal(err)
			}
		},
	}

	for name, enqueuingFunc := range enqueuingFuncs {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() {
				q := sqlf.Sprintf("DELETE FROM external_service_sync_jobs;DELETE FROM external_services")
				if _, err := store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
					t.Fatal(err)
				}
			})

			service := types.ExternalService{
				Kind:        extsvc.KindGitHub,
				DisplayName: "Github - Test",
				Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			// Create a new external service
			err := database.ExternalServicesWith(logger, store).Create(ctx, confGet, &service)
			if err != nil {
				t.Fatal(err)
			}

			// Sanity check: can create sync jobs when service is not locked/being deleted
			assertSyncJobCount(t, store, 0)
			enqueuingFunc(t, ctx, store, &service)
			assertSyncJobCount(t, store, 1)

			// Mark jobs as completed so we can enqueue new one (see test above)
			q := sqlf.Sprintf("UPDATE external_service_sync_jobs SET state='completed'")
			if _, err = store.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
				t.Fatal(err)
			}

			// If the external service is selected with FOR UPDATE in another
			// transaction it shouldn't enqueue a job
			//
			// Open a transaction and lock external service in there
			tx, err := store.Transact(ctx)
			if err != nil {
				t.Fatalf("Transact error: %s", err)
			}
			q = sqlf.Sprintf("SELECT id FROM external_services WHERE id = %d FOR UPDATE", service.ID)
			if _, err = tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
				t.Fatal(err)
			}

			// In the background delete the external service and commit
			done := make(chan struct{})
			go func() {
				time.Sleep(500 * time.Millisecond)

				q := sqlf.Sprintf("UPDATE external_services SET deleted_at = now() WHERE id = %d", service.ID)
				if _, err = tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
					logger.Error("deleting external service failed", log.Error(err))
				}
				if err = tx.Done(err); err != nil {
					logger.Error("commit transaction failed", log.Error(err))
				}
				close(done)
			}()

			// This blocks until transaction is commited and should NOT have enqueued a
			// job because external service is deleted
			enqueuingFunc(t, ctx, store, &service)
			assertSyncJobCount(t, store, 1)

			// Sanity check: wait for transaction to commit
			select {
			case <-time.After(5 * time.Second):
				t.Fatalf("background goroutine deleting external service timed out!")
			case <-done:
			}
			assertSyncJobCount(t, store, 1)
		})
	}
}

func mkRepos(n int, base ...*types.Repo) types.Repos {
	if len(base) == 0 {
		return nil
	}

	rs := make(types.Repos, 0, n)
	for i := range n {
		id := strconv.Itoa(i)
		r := base[i%len(base)].Clone()
		r.Name += api.RepoName(id)
		r.ExternalRepo.ID += id
		rs = append(rs, r)
	}
	return rs
}

func generateExternalServices(n int, base ...*types.ExternalService) types.ExternalServices {
	if len(base) == 0 {
		return nil
	}
	es := make(types.ExternalServices, 0, n)
	for i := range n {
		id := strconv.Itoa(i)
		r := base[i%len(base)].Clone()
		r.DisplayName += id
		es = append(es, r)
	}
	return es
}

// This error is passed to txstore.Done in order to always
// roll-back the transaction a test case executes in.
// This is meant to ensure each test case has a clean slate.
var errRollback = errors.New("tx: rollback")

func transact(ctx context.Context, s repos.Store, test func(testing.TB, repos.Store)) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()

		var err error
		txStore := s

		if !s.Handle().InTransaction() {
			txStore, err = s.Transact(ctx)
			if err != nil {
				t.Fatalf("failed to start transaction: %v", err)
			}
			defer txStore.Done(errRollback)
		}

		test(t, txStore)
	}
}

func createExternalServices(t *testing.T, store repos.Store, opts ...func(*types.ExternalService)) map[string]*types.ExternalService {
	clock := timeutil.NewFakeClock(time.Now(), 0)
	now := clock.Now()

	svcs := mkExternalServices(now)
	for _, svc := range svcs {
		for _, opt := range opts {
			opt(svc)
		}
	}

	// create a few external services
	if err := store.ExternalServiceStore().Upsert(context.Background(), svcs...); err != nil {
		t.Fatalf("failed to insert external services: %v", err)
	}

	services, err := store.ExternalServiceStore().List(context.Background(), database.ExternalServicesListOptions{})
	if err != nil {
		t.Fatal("failed to list external services")
	}

	servicesPerKind := make(map[string]*types.ExternalService)
	for _, svc := range services {
		servicesPerKind[svc.Kind] = svc
	}

	return servicesPerKind
}

func mkExternalServices(now time.Time) types.ExternalServices {
	githubSvc := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		Config:      extsvc.NewUnencryptedConfig(basicGitHubConfig),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	gitlabSvc := types.ExternalService{
		Kind:        extsvc.KindGitLab,
		DisplayName: "GitLab - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://gitlab.com", "token": "abc", "projectQuery": ["none"]}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	bitbucketServerSvc := types.ExternalService{
		Kind:        extsvc.KindBitbucketServer,
		DisplayName: "Bitbucket Server - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://bitbucket.org", "token": "abc", "username": "user", "repos": ["owner/name"]}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	bitbucketCloudSvc := types.ExternalService{
		Kind:        extsvc.KindBitbucketCloud,
		DisplayName: "Bitbucket Cloud - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://bitbucket.org", "username": "user", "appPassword": "password"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	awsSvc := types.ExternalService{
		Kind:        extsvc.KindAWSCodeCommit,
		DisplayName: "AWS Code - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"region": "us-east-1", "accessKeyID": "abc", "secretAccessKey": "abc", "gitCredentials": {"username": "user", "password": "pass"}}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	otherSvc := types.ExternalService{
		Kind:        extsvc.KindOther,
		DisplayName: "Other - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://other.com", "repos": ["repo"]}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	gitoliteSvc := types.ExternalService{
		Kind:        extsvc.KindGitolite,
		DisplayName: "Gitolite - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"prefix": "pre", "host": "host.com"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return []*types.ExternalService{
		&githubSvc,
		&gitlabSvc,
		&bitbucketServerSvc,
		&bitbucketCloudSvc,
		&awsSvc,
		&otherSvc,
		&gitoliteSvc,
	}
}

// get a test store. When in short mode, the test will be skipped as it accesses
// the database.
func getTestRepoStore(t *testing.T) repos.Store {
	t.Helper()

	if testing.Short() {
		t.Skip(t)
	}

	logger := logtest.Scoped(t)
	store := repos.NewStore(logtest.Scoped(t), database.NewDB(logger, dbtest.NewDB(t)))
	store.SetMetrics(repos.NewStoreMetrics())
	return store
}
