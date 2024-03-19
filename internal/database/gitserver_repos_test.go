package database

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/keegancsmith/sqlf"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
)

const shardID = "test"

func TestIterateRepoGitserverStatus(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	repos := types.Repos{
		&types.Repo{Name: "github.com/sourcegraph/repo1"},
		&types.Repo{Name: "github.com/sourcegraph/repo2"},
		&types.Repo{Name: "github.com/sourcegraph/repo3"},
	}
	createTestRepos(ctx, t, db, repos)

	// Soft delete one of the repos
	if err := db.Repos().Delete(ctx, repos[2].ID); err != nil {
		t.Fatal(err)
	}

	if err := db.GitserverRepos().Update(ctx, &types.GitserverRepo{
		RepoID:      repos[0].ID,
		ShardID:     "shard-0",
		CloneStatus: types.CloneStatusCloned,
	}); err != nil {
		t.Fatal(err)
	}

	assert := func(t *testing.T, wantRepoCount, wantStatusCount int, options IterateRepoGitserverStatusOptions) {
		var statusCount int
		var seen []api.RepoName
		var iterationCount int
		// Test iteration path with 1 per page.
		options.BatchSize = 1
		for {

			repos, nextCursor, err := db.GitserverRepos().IterateRepoGitserverStatus(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			for _, repo := range repos {
				seen = append(seen, repo.Name)
				statusCount++
				if repo.GitserverRepo.RepoID == 0 {
					t.Fatal("GitServerRepo has zero id")
				}
			}
			if nextCursor == 0 {
				break
			}
			options.NextCursor = nextCursor

			iterationCount++
			if iterationCount > 50 {
				t.Fatal("infinite iteration loop")
			}
		}

		t.Logf("Seen: %v", seen)
		if len(seen) != wantRepoCount {
			t.Fatalf("Expected %d repos, got %d", wantRepoCount, len(seen))
		}

		if statusCount != wantStatusCount {
			t.Fatalf("Expected %d statuses, got %d", wantStatusCount, statusCount)
		}
	}

	t.Run("iterate with default options", func(t *testing.T) {
		assert(t, 2, 2, IterateRepoGitserverStatusOptions{})
	})
	t.Run("iterate only repos without shard", func(t *testing.T) {
		assert(t, 1, 1, IterateRepoGitserverStatusOptions{OnlyWithoutShard: true})
	})
	t.Run("include deleted", func(t *testing.T) {
		assert(t, 3, 3, IterateRepoGitserverStatusOptions{IncludeDeleted: true})
	})
	t.Run("include deleted, but still only without shard", func(t *testing.T) {
		assert(t, 2, 2, IterateRepoGitserverStatusOptions{OnlyWithoutShard: true, IncludeDeleted: true})
	})
}

func TestListPurgeableRepos(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	store := basestore.NewWithHandle(db.Handle())

	normalRepo := &types.Repo{Name: "normal"}
	blockedRepo := &types.Repo{Name: "blocked"}
	deletedRepo := &types.Repo{Name: "deleted"}
	notCloned := &types.Repo{Name: "notCloned"}

	createTestRepos(ctx, t, db, types.Repos{
		normalRepo,
		blockedRepo,
		deletedRepo,
		notCloned,
	})
	for _, repo := range []*types.Repo{normalRepo, blockedRepo, deletedRepo} {
		updateTestGitserverRepos(ctx, t, db, false, types.CloneStatusCloned, repo.ID)
	}
	// Delete & load soft-deleted name of repo
	if err := db.Repos().Delete(ctx, deletedRepo.ID); err != nil {
		t.Fatal(err)
	}
	deletedRepoNameStr, _, err := basestore.ScanFirstString(store.Query(ctx, sqlf.Sprintf("SELECT name FROM repo WHERE id = %s", deletedRepo.ID)))
	if err != nil {
		t.Fatal(err)
	}
	deletedRepoName := api.RepoName(deletedRepoNameStr)

	// Blocking a repo is currently done manually
	if _, err := db.ExecContext(ctx, `UPDATE repo set blocked = '{}' WHERE id = $1`, blockedRepo.ID); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name      string
		options   ListPurgableReposOptions
		wantRepos []api.RepoName
	}{
		{
			name: "zero deletedBefore",
			options: ListPurgableReposOptions{
				DeletedBefore: time.Time{},
				Limit:         0,
			},
			wantRepos: []api.RepoName{deletedRepoName, blockedRepo.Name},
		},
		{
			name: "deletedBefore now",
			options: ListPurgableReposOptions{
				DeletedBefore: time.Now(),
				Limit:         0,
			},

			wantRepos: []api.RepoName{deletedRepoName, blockedRepo.Name},
		},
		{
			name: "deletedBefore 5 minutes ago",
			options: ListPurgableReposOptions{
				DeletedBefore: time.Now().Add(-5 * time.Minute),
				Limit:         0,
			},
			wantRepos: []api.RepoName{blockedRepo.Name},
		},
		{
			name: "test limit",
			options: ListPurgableReposOptions{
				DeletedBefore: time.Time{},
				Limit:         1,
			},
			wantRepos: []api.RepoName{deletedRepoName},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			have, err := db.GitserverRepos().ListPurgeableRepos(ctx, tt.options)
			require.NoError(t, err)

			sort.Slice(have, func(i, j int) bool { return have[i] < have[j] })

			if diff := cmp.Diff(have, tt.wantRepos); diff != "" {
				t.Fatalf("wrong iterated: %s", diff)
			}
		})
	}
}

func TestListReposWithLastError(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	type testRepo struct {
		name         string
		cloudDefault bool
		hasLastError bool
		blocked      bool
	}
	type testCase struct {
		name               string
		testRepos          []testRepo
		expectedReposFound []api.RepoName
	}
	testCases := []testCase{
		{
			name: "get repos with last error",
			testRepos: []testRepo{
				{
					name:         "github.com/sourcegraph/repo1",
					cloudDefault: true,
					hasLastError: true,
				},
				{
					name:         "github.com/sourcegraph/repo2",
					cloudDefault: true,
				},
			},
			expectedReposFound: []api.RepoName{"github.com/sourcegraph/repo1"},
		},
		{
			name: "filter out non cloud_default repos",
			testRepos: []testRepo{
				{
					name:         "github.com/sourcegraph/repo1",
					cloudDefault: false,
					hasLastError: true,
				},
				{
					name:         "github.com/sourcegraph/repo2",
					cloudDefault: true,
					hasLastError: true,
				},
			},
			expectedReposFound: []api.RepoName{"github.com/sourcegraph/repo2"},
		},
		{
			name: "no cloud_default repos with non-empty last errors",
			testRepos: []testRepo{
				{
					name:         "github.com/sourcegraph/repo1",
					cloudDefault: false,
					hasLastError: true,
				},
				{
					name:         "github.com/sourcegraph/repo2",
					cloudDefault: true,
					hasLastError: false,
				},
			},
			expectedReposFound: nil,
		},
		{
			name: "filter out blocked repos",
			testRepos: []testRepo{
				{
					name:         "github.com/sourcegraph/repo1",
					cloudDefault: true,
					hasLastError: true,
					blocked:      true,
				},
				{
					name:         "github.com/sourcegraph/repo2",
					cloudDefault: true,
					hasLastError: true,
				},
			},
			expectedReposFound: []api.RepoName{"github.com/sourcegraph/repo2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			logger := logtest.Scoped(t)
			db := NewDB(logger, dbtest.NewDB(t))
			now := time.Now()

			cloudDefaultService := createTestExternalService(ctx, t, now, db, true)
			nonCloudDefaultService := createTestExternalService(ctx, t, now, db, false)
			for i, tr := range tc.testRepos {
				testRepo := &types.Repo{
					Name: api.RepoName(tr.name),
					URI:  tr.name,
					ExternalRepo: api.ExternalRepoSpec{
						ID:          fmt.Sprintf("repo%d-external", i),
						ServiceType: extsvc.TypeGitHub,
						ServiceID:   "https://github.com",
					},
				}
				if tr.cloudDefault {
					testRepo = testRepo.With(
						typestest.Opt.RepoSources(cloudDefaultService.URN()),
					)
				} else {
					testRepo = testRepo.With(
						typestest.Opt.RepoSources(nonCloudDefaultService.URN()),
					)
				}
				createTestRepos(ctx, t, db, types.Repos{testRepo})

				if tr.hasLastError {
					if err := db.GitserverRepos().SetLastError(ctx, testRepo.Name, "an error", "test"); err != nil {
						t.Fatal(err)
					}
				}

				if tr.blocked {
					q := sqlf.Sprintf(`UPDATE repo SET blocked = %s WHERE name = %s`, []byte(`{"reason": "test"}`), testRepo.Name)
					if _, err := db.ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
						t.Fatal(err)
					}
				}
			}

			// Iterate and collect repos
			foundRepos, err := db.GitserverRepos().ListReposWithLastError(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expectedReposFound, foundRepos); diff != "" {
				t.Fatalf("mismatch in expected repos with last_error, (-want, +got)\n%s", diff)
			}

			total, err := db.GitserverRepos().TotalErroredCloudDefaultRepos(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if total != len(tc.expectedReposFound) {
				t.Fatalf("expected %d total errored repos, got %d instead", len(tc.expectedReposFound), total)
			}
		})
	}
}

func TestReposWithLastOutput(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	type testRepo struct {
		title      string
		name       string
		lastOutput string
	}
	testRepos := []testRepo{
		{
			title:      "1kb-last-output",
			name:       "github.com/sourcegraph/repo1",
			lastOutput: "Lorem ipsum dolor sit amet, consectetur adipiscing elit.\nNulla tincidunt at turpis ut rhoncus.\nQuisque sollicitudin bibendum libero a interdum.\nMauris efficitur, nunc ac consectetur dapibus, tortor velit sollicitudin justo, varius faucibus purus tellus eu ex.\nProin bibendum feugiat ornare..\nDonec placerat vestibulum hendrerit.\nInteger quis mattis justo.\nFusce eu arcu mollis magna rutrum porttitor.\nUt quis tristique enim..\nDonec suscipit nisl sit amet nulla cursus, ac vulputate justo ornare.\nNam non nisl aliquam, porta ligula vitae, sodales sapien.\nVestibulum et dictum tortor.\nAenean nec risus ac justo luctus posuere et in massa.\nVivamus nec ultricies est, a pulvinar ante.\nSed semper rutrum lorem.\nNulla ut metus ornare, dapibus justo et, sagittis lacus.\nIn massa felis, pellentesque pretium mauris id, pretium pellentesque augue.\nNulla feugiat est sit amet ex rhoncus, ut dapibus massa viverra.\nSuspendisse ullamcorper orci nec mauris vulputate vestibulum.\nInteger luctus tincidunt augue, ut congue neque dapibus sit amet.\nEtiam eu justo in dui ornare ultricies.\nNam fermentum ultricies sagittis.\nMorbi ultricies maximus tortor ut aliquet.\nNullam eget venenatis nunc.\nNam ultricies neque ac blandit eleifend.\nPhasellus pharetra, augue ac semper feugiat, lorem nulla consectetur purus, nec malesuada nisi sem id erat.\nFusce mollis, est vel maximus convallis, eros magna convallis turpis, ac fermentum ipsum nulla in mi.",
		},
		{
			title:      "56b-last-output",
			name:       "github.com/sourcegraph/repo2",
			lastOutput: "Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		},
		{
			title:      "empty-last-output",
			name:       "github.com/sourcegraph/repo3",
			lastOutput: "",
		},
	}
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	now := time.Now()
	cloudDefaultService := createTestExternalService(ctx, t, now, db, true)
	for i, tr := range testRepos {
		t.Run(tr.title, func(t *testing.T) {
			testRepo := &types.Repo{
				Name:        api.RepoName(tr.name),
				URI:         tr.name,
				Description: "",
				ExternalRepo: api.ExternalRepoSpec{
					ID:          fmt.Sprintf("repo%d-external", i),
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com",
				},
			}
			testRepo = testRepo.With(
				typestest.Opt.RepoSources(cloudDefaultService.URN()),
			)
			createTestRepos(ctx, t, db, types.Repos{testRepo})
			if err := db.GitserverRepos().SetLastOutput(ctx, testRepo.Name, tr.lastOutput); err != nil {
				t.Fatal(err)
			}
			haveOut, ok, err := db.GitserverRepos().GetLastSyncOutput(ctx, testRepo.Name)
			if err != nil {
				t.Fatal(err)
			}
			if tr.lastOutput == "" && ok {
				t.Fatalf("last output is not empty")
			}
			if have, want := haveOut, tr.lastOutput; have != want {
				t.Fatalf("wrong last output returned, have=%s want=%s", have, want)
			}
		})
	}
}

func createTestExternalService(ctx context.Context, t *testing.T, now time.Time, db DB, cloudDefault bool) types.ExternalService {
	service := types.ExternalService{
		Kind:         extsvc.KindGitHub,
		DisplayName:  "Github - Test",
		Config:       extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
		CreatedAt:    now,
		UpdatedAt:    now,
		CloudDefault: cloudDefault,
	}

	// Create a new external service
	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	err := db.ExternalServices().Create(ctx, confGet, &service)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestGitserverReposGetByID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Create one test repo
	_, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo")

	// GetByID should now work
	fromDB, err := db.GitserverRepos().GetByID(ctx, gitserverRepo.RepoID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	_, err = db.GitserverRepos().GetByID(ctx, gitserverRepo.RepoID+1)
	if !errcode.IsNotFound(err) {
		t.Fatal("expected not found error for non-existant ID", err)
	}
}

func TestGitserverReposGetByName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Create one test repo
	repo, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo")

	// GetByName should now work
	fromDB, err := db.GitserverRepos().GetByName(ctx, repo.Name)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	_, err = db.GitserverRepos().GetByName(ctx, repo.Name+"404")
	if !errcode.IsNotFound(err) {
		t.Fatal("expected not found error for non-existant repo name", err)
	}
}

func TestGitserverReposGetByNames(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	gitserverRepoStore := &gitserverRepoStore{Store: basestore.NewWithHandle(db.Handle())}

	// Creating a few repos
	repoNames := make([]api.RepoName, 5)
	gitserverRepos := make([]*types.GitserverRepo, 5)
	for i := range len(repoNames) {
		repoName := fmt.Sprintf("github.com/sourcegraph/repo%d", i)
		repo, gitserverRepo := createTestRepo(ctx, t, db, api.RepoName(repoName))
		repoNames[i] = repo.Name
		gitserverRepos[i] = gitserverRepo
	}

	for i := range len(repoNames) {
		have, err := gitserverRepoStore.GetByNames(ctx, repoNames[:i+1]...)
		if err != nil {
			t.Fatal(err)
		}
		haveRepos := make([]*types.GitserverRepo, 0, len(have))
		for _, r := range have {
			haveRepos = append(haveRepos, r)
		}
		sort.Slice(haveRepos, func(i, j int) bool {
			return haveRepos[i].RepoID < haveRepos[j].RepoID
		})
		if diff := cmp.Diff(gitserverRepos[:i+1], haveRepos, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestSetCloneStatus(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Create one test repo
	repo, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo")

	// Set cloned
	setGitserverRepoCloneStatus(t, db, repo.Name, types.CloneStatusCloned)

	fromDB, err := db.GitserverRepos().GetByID(ctx, gitserverRepo.RepoID)
	if err != nil {
		t.Fatal(err)
	}

	gitserverRepo.CloneStatus = types.CloneStatusCloned
	gitserverRepo.ShardID = shardID
	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Setting clone status should work even if no row exists in gitserver table
	repo2 := &types.Repo{
		Name:         "github.com/sourcegraph/repo2",
		URI:          "github.com/sourcegraph/repo2",
		ExternalRepo: api.ExternalRepoSpec{},
	}

	// Create one test repo
	err = db.Repos().Create(ctx, repo2)
	if err != nil {
		t.Fatal(err)
	}

	setGitserverRepoCloneStatus(t, db, repo2.Name, types.CloneStatusCloned)
	fromDB, err = db.GitserverRepos().GetByID(ctx, repo2.ID)
	if err != nil {
		t.Fatal(err)
	}
	gitserverRepo2 := &types.GitserverRepo{
		RepoID:      repo2.ID,
		ShardID:     shardID,
		CloneStatus: types.CloneStatusCloned,
	}
	if diff := cmp.Diff(gitserverRepo2, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "LastFetched", "LastChanged", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Setting the same status again should not touch the row
	setGitserverRepoCloneStatus(t, db, repo2.Name, types.CloneStatusCloned)
	after, err := db.GitserverRepos().GetByID(ctx, repo2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(fromDB, after); diff != "" {
		t.Fatal(diff)
	}
}

func TestCloningProgress(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	t.Run("Default", func(t *testing.T) {
		repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/defaultcloningprogress")
		gotRepo, err := db.GitserverRepos().GetByName(ctx, repo.Name)
		if err != nil {
			t.Fatalf("GetByName: %s", err)
		}
		if got := gotRepo.CloningProgress; got != "" {
			t.Errorf("GetByName.CloningProgress, got %q, want empty string", got)
		}
	})

	t.Run("Set", func(t *testing.T) {
		repo, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/updatedcloningprogress")

		gitserverRepo.CloningProgress = "Receiving objects: 97% (97/100)"
		if err := db.GitserverRepos().SetCloningProgress(ctx, repo.Name, gitserverRepo.CloningProgress); err != nil {
			t.Fatalf("SetCloningProgress: %s", err)
		}
		gotRepo, err := db.GitserverRepos().GetByName(ctx, repo.Name)
		if err != nil {
			t.Fatalf("GetByName: %s", err)
		}
		if diff := cmp.Diff(gitserverRepo, gotRepo, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt")); diff != "" {
			t.Errorf("SetCloningProgress->GetByName -want+got: %s", diff)
		}
	})
}

func TestLogCorruption(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	t.Run("log repo corruption sets corrupted_at time", func(t *testing.T) {
		repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo1")
		logRepoCorruption(t, db, repo.Name, "test")

		fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
		if err != nil {
			t.Fatalf("failed to get repo by id: %s", err)
		}

		if fromDB.CorruptedAt.IsZero() {
			t.Errorf("Expected corruptedAt time to be set. Got zero value for time %q", fromDB.CorruptedAt)
		}
		// We should have one corruption log entry
		if len(fromDB.CorruptionLogs) != 1 {
			t.Errorf("Wanted 1 Corruption log entries,  got %d entries", len(fromDB.CorruptionLogs))
		}
		if fromDB.CorruptionLogs[0].Timestamp.IsZero() {
			t.Errorf("Corruption Log entry expected to have non zero timestamp. Got %q", fromDB.CorruptionLogs[0])
		}
		if fromDB.CorruptionLogs[0].Reason != "test" {
			t.Errorf("Wanted Corruption Log reason %q got %q", "test", fromDB.CorruptionLogs[0].Reason)
		}
	})
	t.Run("setting clone status clears corruptedAt time", func(t *testing.T) {
		repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo2")
		logRepoCorruption(t, db, repo.Name, "test 2")

		setGitserverRepoCloneStatus(t, db, repo.Name, types.CloneStatusCloned)

		fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
		if err != nil {
			t.Fatalf("failed to get repo by id: %s", err)
		}
		if !fromDB.CorruptedAt.IsZero() {
			t.Errorf("Setting clone status should set corrupt_at value to zero time value. Got non zero value for time %q", fromDB.CorruptedAt)
		}
	})
	t.Run("setting last error does not clear corruptedAt time", func(t *testing.T) {
		repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo3")
		logRepoCorruption(t, db, repo.Name, "test 3")

		setGitserverRepoLastChanged(t, db, repo.Name, time.Now())

		fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
		if err != nil {
			t.Fatalf("failed to get repo by id: %s", err)
		}
		if !fromDB.CorruptedAt.IsZero() {
			t.Errorf("Setting Last Changed should set corrupted at value to zero time value. Got non zero value for time %q", fromDB.CorruptedAt)
		}
	})
	t.Run("setting clone status clears corruptedAt time", func(t *testing.T) {
		repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo4")
		logRepoCorruption(t, db, repo.Name, "test 3")

		setGitserverRepoLastError(t, db, repo.Name, "This is a TEST ERAWR")

		fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
		if err != nil {
			t.Fatalf("failed to get repo by id: %s", err)
		}
		if fromDB.CorruptedAt.IsZero() {
			t.Errorf("Setting Last Error should not clear the corruptedAt value")
		}
	})
	t.Run("consecutive corruption logs appends", func(t *testing.T) {
		repo, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo5")
		for i := range 12 {
			logRepoCorruption(t, db, repo.Name, fmt.Sprintf("test %d", i))
			// We set the Clone status so that the 'corrupted_at' time gets cleared
			// otherwise we cannot log corruption for a repo that is already corrupt
			gitserverRepo.CloneStatus = types.CloneStatusCloned
			gitserverRepo.CorruptedAt = time.Time{}
			if err := db.GitserverRepos().Update(ctx, gitserverRepo); err != nil {
				t.Fatal(err)
			}

		}

		fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
		if err != nil {
			t.Fatalf("failed to retrieve repo from db: %s", err)
		}

		// We added 12 entries but we only keep 10
		if len(fromDB.CorruptionLogs) != 10 {
			t.Errorf("expected 10 corruption log entries but got %d", len(fromDB.CorruptionLogs))
		}

		// A log entry gets prepended to the json array, so:
		// first entry = most recent log entry
		// last entry = oldest log entry
		// Our most recent log entry (idx 0!) should have "test 11" as the reason ie. the last element the loop
		// that we added
		wanted := "test 11"
		if fromDB.CorruptionLogs[0].Reason != wanted {
			t.Errorf("Wanted %q for last corruption log entry but got %q", wanted, fromDB.CorruptionLogs[9].Reason)
		}

	})
	t.Run("large reason gets truncated", func(t *testing.T) {
		repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo6")

		largeReason := make([]byte, MaxReasonSizeInMB*2)
		for i := range len(largeReason) {
			largeReason[i] = 'a'
		}

		logRepoCorruption(t, db, repo.Name, string(largeReason))

		fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
		if err != nil {
			t.Fatalf("failed to retrieve repo from db: %s", err)
		}

		if len(fromDB.CorruptionLogs[0].Reason) == len(largeReason) {
			t.Errorf("expected reason to be truncated - got length=%d, wanted=%d", len(fromDB.CorruptionLogs[0].Reason), MaxReasonSizeInMB)
		}
	})
	t.Run("logging corruption from wrong shard does not log corruption", func(t *testing.T) {
		// Create repo
		repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo7")

		// Mark it as cloned on shard-1
		err := db.GitserverRepos().SetCloneStatus(ctx, repo.Name, types.CloneStatusCloned, "shard-1")
		require.NoError(t, err)

		// Log corruption on shard-2
		err = db.GitserverRepos().LogCorruption(ctx, repo.Name, "corrupt lol", "shard-2")
		if err == nil || err.Error() != "repo not found or already corrupt" {
			t.Fatalf("expected not-found error but got: %s", err)
		}

		// This should not result in corruption being logged
		fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
		if err != nil {
			t.Fatalf("failed to get repo by id: %s", err)
		}
		require.True(t, fromDB.CorruptedAt.IsZero(), "corrupted_at should not be set, but it was")
	})
}

func TestSetLastError(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Create one test repo
	repo, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo")

	// Set error.
	//
	// We are using a null terminated string for the last_error column. See
	// https://stackoverflow.com/a/38008565/1773961 on how to set null terminated strings in Go.
	err := db.GitserverRepos().SetLastError(ctx, repo.Name, "oops\x00", "")
	if err != nil {
		t.Fatal(err)
	}

	fromDB, err := db.GitserverRepos().GetByID(ctx, gitserverRepo.RepoID)
	if err != nil {
		t.Fatal(err)
	}

	gitserverRepo.LastError = "oops"
	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Remove error
	const emptyErr = ""
	err = db.GitserverRepos().SetLastError(ctx, repo.Name, emptyErr, "")
	if err != nil {
		t.Fatal(err)
	}

	fromDB, err = db.GitserverRepos().GetByID(ctx, gitserverRepo.RepoID)
	if err != nil {
		t.Fatal(err)
	}

	gitserverRepo.LastError = emptyErr
	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Set again to same value, updated_at should not change
	err = db.GitserverRepos().SetLastError(ctx, repo.Name, emptyErr, shardID)
	if err != nil {
		t.Fatal(err)
	}

	after, err := db.GitserverRepos().GetByID(ctx, gitserverRepo.RepoID)
	if err != nil {
		t.Fatal(err)
	}

	gitserverRepo.LastError = emptyErr
	if diff := cmp.Diff(fromDB, after); diff != "" {
		t.Fatal(diff)
	}

	// Setting to empty error should set the column to null
	count, _, err := basestore.ScanFirstInt(db.QueryContext(ctx, "SELECT COUNT(*) FROM gitserver_repos WHERE last_error IS NULL"))
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("Want %d, got %d", 1, count)
	}
}

func TestSetRepoSize(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Create one test repo
	repo, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo")

	// Set repo size
	err := db.GitserverRepos().SetRepoSize(ctx, repo.Name, 200, "")
	if err != nil {
		t.Fatal(err)
	}

	fromDB, err := db.GitserverRepos().GetByID(ctx, gitserverRepo.RepoID)
	if err != nil {
		t.Fatal(err)
	}

	gitserverRepo.RepoSizeBytes = 200
	// If we have size, we can assume it's cloned
	gitserverRepo.CloneStatus = types.CloneStatusCloned
	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Setting repo size should work even if no row exists
	repo2 := &types.Repo{
		Name:         "github.com/sourcegraph/repo2",
		URI:          "github.com/sourcegraph/repo2",
		ExternalRepo: api.ExternalRepoSpec{},
	}

	// Create one test repo
	err = db.Repos().Create(ctx, repo2)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.GitserverRepos().SetRepoSize(ctx, repo2.Name, 300, ""); err != nil {
		t.Fatal(err)
	}
	fromDB, err = db.GitserverRepos().GetByID(ctx, repo2.ID)
	if err != nil {
		t.Fatal(err)
	}
	gitserverRepo2 := &types.GitserverRepo{
		RepoID:        repo2.ID,
		ShardID:       "",
		RepoSizeBytes: 300,
		// If we have size, we can assume it's cloned
		CloneStatus: types.CloneStatusCloned,
	}
	if diff := cmp.Diff(gitserverRepo2, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "LastFetched", "LastChanged", "CloneStatus", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Setting the same size should not touch the row
	if err := db.GitserverRepos().SetRepoSize(ctx, repo2.Name, 300, ""); err != nil {
		t.Fatal(err)
	}
	after, err := db.GitserverRepos().GetByID(ctx, repo2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(fromDB, after); diff != "" {
		t.Fatal(diff)
	}
}

func TestGitserverRepo_Update(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Create one test repo
	repo, gitserverRepo := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo")

	// Change clone status
	gitserverRepo.CloneStatus = types.CloneStatusCloned
	if err := db.GitserverRepos().Update(ctx, gitserverRepo); err != nil {
		t.Fatal(err)
	}
	fromDB, err := db.GitserverRepos().GetByID(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Change error

	// We want to test if update can handle the writing a null character to the last_error
	// column. See https://stackoverflow.com/a/38008565/1773961 on how to set null terminated
	// strings in Go.
	gitserverRepo.LastError = "Oops\x00"
	if err := db.GitserverRepos().Update(ctx, gitserverRepo); err != nil {
		t.Fatal(err)
	}
	fromDB, err = db.GitserverRepos().GetByID(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Set LastError to the expected error string but without the null character, because we expect
	// our code to work and strip it before writing to the DB.
	gitserverRepo.LastError = "Oops"
	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	// Remove error
	gitserverRepo.LastError = ""
	if err := db.GitserverRepos().Update(ctx, gitserverRepo); err != nil {
		t.Fatal(err)
	}
	fromDB, err = db.GitserverRepos().GetByID(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(gitserverRepo, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}
}

func TestGitserverRepoUpdateMany(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Create two test repos
	repo1, gitserverRepo1 := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo1")
	repo2, gitserverRepo2 := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo2")

	// Change their clone statuses
	gitserverRepo1.CloneStatus = types.CloneStatusCloned
	gitserverRepo2.CloneStatus = types.CloneStatusCloning
	if err := db.GitserverRepos().Update(ctx, gitserverRepo1, gitserverRepo2); err != nil {
		t.Fatal(err)
	}

	// Confirm
	t.Run("repo1", func(t *testing.T) {
		fromDB, err := db.GitserverRepos().GetByID(ctx, repo1.ID)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(gitserverRepo1, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
			t.Fatal(diff)
		}
	})
	t.Run("repo2", func(t *testing.T) {
		fromDB, err := db.GitserverRepos().GetByID(ctx, repo2.ID)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(gitserverRepo2, fromDB, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
			t.Fatal(diff)
		}
	})
}

func TestSanitizeToUTF8(t *testing.T) {
	testSet := map[string]string{
		"test\x00":     "test",
		"test\x00test": "testtest",
		"\x00test":     "test",
	}

	for input, expected := range testSet {
		got := sanitizeToUTF8(input)
		if got != expected {
			t.Fatalf("Failed to sanitize to UTF-8, got %q but wanted %q", got, expected)
		}
	}
}

func TestGitserverUpdateRepoSizes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	repo1, gitserverRepo1 := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo1")
	repo2, gitserverRepo2 := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo2")
	repo3, gitserverRepo3 := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo3")

	// Setting repo sizes in DB
	sizes := map[api.RepoName]int64{
		repo1.Name: 100,
		repo2.Name: 500,
		repo3.Name: 800,
	}
	numUpdated, err := db.GitserverRepos().UpdateRepoSizes(ctx, logger, shardID, sizes)
	if err != nil {
		t.Fatal(err)
	}
	if have, want := numUpdated, len(sizes); have != want {
		t.Fatalf("wrong number of repos updated. have=%d, want=%d", have, want)
	}

	// Updating sizes in test data for further diff comparison
	gitserverRepo1.RepoSizeBytes = sizes[repo1.Name]
	gitserverRepo2.RepoSizeBytes = sizes[repo2.Name]
	gitserverRepo3.RepoSizeBytes = sizes[repo3.Name]

	// Checking repo diffs, excluding UpdatedAt. This is to verify that nothing except repo_size_bytes
	// has changed
	for _, repo := range []*types.GitserverRepo{
		gitserverRepo1,
		gitserverRepo2,
		gitserverRepo3,
	} {
		reloaded, err := db.GitserverRepos().GetByID(ctx, repo.RepoID)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(repo, reloaded, cmpopts.IgnoreFields(types.GitserverRepo{}, "UpdatedAt", "CorruptionLogs")); diff != "" {
			t.Fatal(diff)
		}
		// Separately make sure UpdatedAt has changed, though
		if repo.UpdatedAt.Equal(reloaded.UpdatedAt) {
			t.Fatalf("UpdatedAt of GitserverRepo should be updated but was not. before=%s, after=%s", repo.UpdatedAt, reloaded.UpdatedAt)
		}
	}

	// update again to make sure they're not updated again
	numUpdated, err = db.GitserverRepos().UpdateRepoSizes(ctx, logger, shardID, sizes)
	if err != nil {
		t.Fatal(err)
	}
	if have, want := numUpdated, 0; have != want {
		t.Fatalf("wrong number of repos updated. have=%d, want=%d", have, want)
	}

	// update subset
	sizes = map[api.RepoName]int64{
		repo3.Name: 900,
	}
	numUpdated, err = db.GitserverRepos().UpdateRepoSizes(ctx, logger, shardID, sizes)
	if err != nil {
		t.Fatal(err)
	}
	if have, want := numUpdated, 1; have != want {
		t.Fatalf("wrong number of repos updated. have=%d, want=%d", have, want)
	}

	// update with different batch sizes
	gitserverRepoStore := &gitserverRepoStore{Store: basestore.NewWithHandle(db.Handle())}
	for _, batchSize := range []int64{1, 2, 3, 6} {
		sizes = map[api.RepoName]int64{
			repo1.Name: 123 + batchSize,
			repo2.Name: 456 + batchSize,
			repo3.Name: 789 + batchSize,
		}

		numUpdated, err = gitserverRepoStore.updateRepoSizesWithBatchSize(ctx, logger, sizes, int(batchSize))
		if err != nil {
			t.Fatal(err)
		}
		if have, want := numUpdated, 3; have != want {
			t.Fatalf("wrong number of repos updated. have=%d, want=%d", have, want)
		}
	}
}

func BenchmarkGitserverUpdateRepoSizes_LargeAmountOfRepos(b *testing.B) {
	logger := logtest.Scoped(b)
	db := NewDB(logger, dbtest.NewDB(b))
	ctx := context.Background()

	// Large number of repositories.
	// NOTE: Tweak this to ensure the code runs fast. When you modify the code,
	// you should change this 20k or 60k.
	const count = 6_000
	namesToSize := make(map[api.RepoName]int64, count)

	reposBatch := make([]*types.Repo, 0, 1000)
	for i := range count {
		name := fmt.Sprintf("github.com/sourcegraph/repo-%d", i)
		r := &types.Repo{Name: api.RepoName(name)}
		namesToSize[r.Name] = int64(i + 1)

		reposBatch = append(reposBatch, r)
		if i%5000 == 0 || i == count-1 {
			createTestRepos(ctx, b, db, types.Repos(reposBatch))
			reposBatch = reposBatch[0:0]
			b.Logf("Creating %d repos. %d to go", i, count-i)
		}
	}

	b.Run(fmt.Sprintf("count=%d", count), func(b *testing.B) {
		numUpdated, err := db.GitserverRepos().UpdateRepoSizes(ctx, logger, shardID, namesToSize)
		if err != nil {
			b.Fatal(err)
		}
		if have, want := numUpdated, len(namesToSize); have != want {
			b.Fatalf("wrong number of repos updated. have=%d, want=%d", have, want)
		}
	})
}

func createTestRepo(ctx context.Context, t *testing.T, db DB, name api.RepoName) (*types.Repo, *types.GitserverRepo) {
	t.Helper()

	repo := &types.Repo{Name: name}

	// Create Repo
	err := db.Repos().Create(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}

	// Get the gitserver repo
	gitserverRepo, err := db.GitserverRepos().GetByID(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}

	want := &types.GitserverRepo{
		RepoID:         repo.ID,
		CloneStatus:    types.CloneStatusNotCloned,
		CorruptionLogs: []types.RepoCorruptionLog{},
	}
	if diff := cmp.Diff(want, gitserverRepo, cmpopts.IgnoreFields(types.GitserverRepo{}, "LastFetched", "LastChanged", "UpdatedAt", "CorruptionLogs")); diff != "" {
		t.Fatal(diff)
	}

	return repo, gitserverRepo
}

func createTestRepos(ctx context.Context, t testing.TB, db DB, repos types.Repos) {
	t.Helper()
	err := db.Repos().Create(ctx, repos...)
	if err != nil {
		t.Fatal(err)
	}
}

func updateTestGitserverRepos(ctx context.Context, t *testing.T, db DB, hasLastError bool, cloneStatus types.CloneStatus, repoID api.RepoID) {
	t.Helper()
	gitserverRepo := &types.GitserverRepo{
		RepoID:      repoID,
		ShardID:     fmt.Sprintf("gitserver%d", repoID),
		CloneStatus: cloneStatus,
	}
	if hasLastError {
		gitserverRepo.LastError = "an error occurred"
	}
	if err := db.GitserverRepos().Update(ctx, gitserverRepo); err != nil {
		t.Fatal(err)
	}
}

func TestGitserverRepos_GetGitserverGitDirSize(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	assertSize := func(want int64) {
		t.Helper()

		have, err := db.GitserverRepos().GetGitserverGitDirSize(ctx)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, want, have)
	}

	// Expect exactly 0 bytes used when no repos exist yet.
	assertSize(0)

	// Create one test repo.
	repo, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo")

	// Now, we should see an uncloned test repo that takes no space.
	assertSize(0)

	// Set repo size and mark repo as cloned.
	require.NoError(t, db.GitserverRepos().SetRepoSize(ctx, repo.Name, 200, "test-gitserver"))

	// Now the total should be 200 bytes.
	assertSize(200)

	// Now add a second repo to make sure it aggregates properly.
	repo2, _ := createTestRepo(ctx, t, db, "github.com/sourcegraph/repo2")
	require.NoError(t, db.GitserverRepos().SetRepoSize(ctx, repo2.Name, 500, "test-gitserver"))

	// 200 from the first repo and another 500 from the newly created repo.
	assertSize(700)

	// Now mark the repo as uncloned, that should exclude it from statistics.
	require.NoError(t, db.GitserverRepos().SetCloneStatus(ctx, repo.Name, types.CloneStatusNotCloned, "test-gitserver"))

	// only repo2 which is 500 bytes should cont now.
	assertSize(500)
}
