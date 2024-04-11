package database

import (
	"context"
	"fmt"
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
