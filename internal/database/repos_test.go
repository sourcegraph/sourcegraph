package database

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/zoekt"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

/*
 * Helpers
 */

func sortedRepoNames(repos []*types.Repo) []api.RepoName {
	names := repoNames(repos)
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}

func repoNames(repos []*types.Repo) []api.RepoName {
	var names []api.RepoName
	for _, repo := range repos {
		names = append(names, repo.Name)
	}
	return names
}

func createRepo(ctx context.Context, t *testing.T, db DB, repo *types.Repo) {
	t.Helper()

	op := createInsertRepoOp(repo, 0)

	if err := upsertRepo(ctx, db, op); err != nil {
		t.Fatal(err)
	}
}

func createRepoWithSize(ctx context.Context, t *testing.T, db DB, repo *types.Repo, size int64) {
	t.Helper()

	op := createInsertRepoOp(repo, size)

	if err := upsertRepo(ctx, db, op); err != nil {
		t.Fatal(err)
	}
}

func createInsertRepoOp(repo *types.Repo, size int64) InsertRepoOp {
	return InsertRepoOp{
		Name:              repo.Name,
		Private:           repo.Private,
		ExternalRepo:      repo.ExternalRepo,
		Description:       repo.Description,
		Fork:              repo.Fork,
		Archived:          repo.Archived,
		GitserverRepoSize: size,
	}
}

func mustCreate(ctx context.Context, t *testing.T, db DB, repo *types.Repo) *types.Repo {
	t.Helper()

	createRepo(ctx, t, db, repo)
	repo, err := db.Repos().GetByName(ctx, repo.Name)
	if err != nil {
		t.Fatal(err)
	}

	return repo
}

func setGitserverRepoCloneStatus(t *testing.T, db DB, name api.RepoName, s types.CloneStatus) {
	t.Helper()

	switch s {
	case types.CloneStatusCloned:
		if err := db.GitserverRepos().SetCloned(context.Background(), name, shardID); err != nil {
			t.Fatalf("failed to set clone status for repo %s: %s", name, err)
		}
	case types.CloneStatusCloning:
		if err := db.GitserverRepos().SetCloning(context.Background(), name, shardID); err != nil {
			t.Fatalf("failed to set clone status for repo %s: %s", name, err)
		}
	case types.CloneStatusNotCloned:
		if err := db.GitserverRepos().SetNotCloned(context.Background(), name); err != nil {
			t.Fatalf("failed to set clone status for repo %s: %s", name, err)
		}
	}
}

func setGitserverRepoLastChanged(t *testing.T, db DB, name api.RepoName, last time.Time) {
	t.Helper()

	if err := db.GitserverRepos().SetLastFetched(context.Background(), name, GitserverFetchData{LastFetched: last, LastChanged: last}); err != nil {
		t.Fatal(err)
	}
}

func setGitserverRepoLastError(t *testing.T, db DB, name api.RepoName, msg string) {
	t.Helper()

	err := db.GitserverRepos().SetLastError(context.Background(), name, msg, shardID)
	if err != nil {
		t.Fatalf("failed to set last error: %s", err)
	}
}

func logRepoCorruption(t *testing.T, db DB, name api.RepoName, logOutput string) {
	t.Helper()

	err := db.GitserverRepos().LogCorruption(context.Background(), name, logOutput, shardID)
	if err != nil {
		t.Fatalf("failed to log repo corruption: %s", err)
	}
}

func setZoektIndexed(t *testing.T, db DB, name api.RepoName) {
	t.Helper()
	ctx := context.Background()
	repo, err := db.Repos().GetByName(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	err = db.ZoektRepos().UpdateIndexStatuses(ctx, zoekt.ReposMap{
		uint32(repo.ID): {},
	})
	if err != nil {
		t.Fatalf("failed to set indexed status of %q: %s", name, err)
	}
}

func repoNamesFromRepos(repos []*types.Repo) []types.MinimalRepo {
	rnames := make([]types.MinimalRepo, 0, len(repos))
	for _, repo := range repos {
		rnames = append(rnames, types.MinimalRepo{ID: repo.ID, Name: repo.Name})
	}

	return rnames
}

func reposFromRepoNames(names []types.MinimalRepo) []*types.Repo {
	repos := make([]*types.Repo, 0, len(names))
	for _, name := range names {
		repos = append(repos, &types.Repo{ID: name.ID, Name: name.Name})
	}

	return repos
}

// InsertRepoOp represents an operation to insert a repository.
type InsertRepoOp struct {
	Name              api.RepoName
	Description       string
	Fork              bool
	Archived          bool
	Private           bool
	ExternalRepo      api.ExternalRepoSpec
	GitserverRepoSize int64
}

const upsertSQL = `
WITH upsert AS (
  UPDATE repo
  SET
    name                  = $1,
    description           = $2,
    fork                  = $3,
    external_id           = NULLIF(BTRIM($4), ''),
    external_service_type = NULLIF(BTRIM($5), ''),
    external_service_id   = NULLIF(BTRIM($6), ''),
    archived              = $7,
    private               = $8
  WHERE name = $1 OR (
    external_id IS NOT NULL
    AND external_service_type IS NOT NULL
    AND external_service_id IS NOT NULL
    AND NULLIF(BTRIM($4), '') IS NOT NULL
    AND NULLIF(BTRIM($5), '') IS NOT NULL
    AND NULLIF(BTRIM($6), '') IS NOT NULL
    AND external_id = NULLIF(BTRIM($4), '')
    AND external_service_type = NULLIF(BTRIM($5), '')
    AND external_service_id = NULLIF(BTRIM($6), '')
  )
  RETURNING repo.name
)

INSERT INTO repo (
  name,
  description,
  fork,
  external_id,
  external_service_type,
  external_service_id,
  archived,
  private
) (
  SELECT
    $1 AS name,
    $2 AS description,
    $3 AS fork,
    NULLIF(BTRIM($4), '') AS external_id,
    NULLIF(BTRIM($5), '') AS external_service_type,
    NULLIF(BTRIM($6), '') AS external_service_id,
    $7 AS archived,
    $8 AS private
  WHERE NOT EXISTS (SELECT 1 FROM upsert)
) RETURNING id`

// upsertRepo updates the repository if it already exists (keyed on name) and
// inserts it if it does not.
func upsertRepo(ctx context.Context, db DB, op InsertRepoOp) error {
	s := db.Repos()
	insert := false

	// We optimistically assume the repo is already in the table, so first
	// check if it is. We then fallback to the upsert functionality. The
	// upsert is logged as a modification to the DB, even if it is a no-op. So
	// we do this check to avoid log spam if postgres is configured with
	// log_statement='mod'.
	r, err := s.GetByName(ctx, op.Name)
	if err != nil {
		if !errors.HasType(err, &RepoNotFoundErr{}) {
			return err
		}
		insert = true // missing
	} else {
		insert = (op.Description != r.Description) ||
			(op.Fork != r.Fork) ||
			(!op.ExternalRepo.Equal(&r.ExternalRepo))
	}

	if !insert {
		return nil
	}

	qrc := s.Handle().QueryRowContext(
		ctx,
		upsertSQL,
		op.Name,
		op.Description,
		op.Fork,
		op.ExternalRepo.ID,
		op.ExternalRepo.ServiceType,
		op.ExternalRepo.ServiceID,
		op.Archived,
		op.Private,
	)
	err = qrc.Err()

	// Set size if specified
	if op.GitserverRepoSize > 0 {
		var lastInsertId int64
		err2 := qrc.Scan(&lastInsertId)
		if err2 != nil {
			return err2
		}
		_, err = s.Handle().ExecContext(ctx, `UPDATE gitserver_repos set repo_size_bytes = $1 where repo_id = $2`,
			op.GitserverRepoSize, lastInsertId)

	}

	return err
}

/*
 * Tests
 */

// TestRepos_createRepo_dupe tests the test helper createRepo.
func TestRepos_createRepo_dupe(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	// Add a repo.
	createRepo(ctx, t, db, &types.Repo{Name: "a/b"})

	// Add another repo with the same name.
	createRepo(ctx, t, db, &types.Repo{Name: "a/b"})
}

// TestRepos_createRepo tests the test helper createRepo.
func TestRepos_createRepo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	// Add a repo.
	createRepo(ctx, t, db, &types.Repo{
		Name:        "a/b",
		Description: "test",
	})

	repo, err := db.Repos().GetByName(ctx, "a/b")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := repo.Name, api.RepoName("a/b"); got != want {
		t.Fatalf("got Name %q, want %q", got, want)
	}
	if got, want := repo.Description, "test"; got != want {
		t.Fatalf("got Description %q, want %q", got, want)
	}
}

func TestRepos_Get(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	now := time.Now()

	service := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create a new external service
	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	err := db.ExternalServices().Create(ctx, confGet, &service)
	if err != nil {
		t.Fatal(err)
	}

	want := mustCreate(ctx, t, db, &types.Repo{
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "r",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com",
		},
		Name:        "name",
		Private:     true,
		URI:         "uri",
		Description: "description",
		Fork:        true,
		Archived:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    new(github.Repository),
		Sources: map[string]*types.SourceInfo{
			service.URN(): {
				ID:       service.URN(),
				CloneURL: "git@github.com:foo/bar.git",
			},
		},
	})

	repo, err := db.Repos().Get(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(t, repo, want) {
		t.Errorf("got %v, want %v", repo, want)
	}
}

func TestRepos_GetByIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	want := mustCreate(ctx, t, db, &types.Repo{
		Name: "r",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "a",
			ServiceType: "b",
			ServiceID:   "c",
		},
	})

	repos, err := db.Repos().GetByIDs(ctx, want.ID, 404)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("got %d repos, but want 1", len(repos))
	}

	if !jsonEqual(t, repos[0], want) {
		t.Errorf("got %v, want %v", repos[0], want)
	}
}

func TestRepos_GetByIDs_EmptyIDs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	repos, err := db.Repos().GetByIDs(ctx, []api.RepoID{}...)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Fatalf("got %d repos, but want 0", len(repos))
	}

}

func TestRepos_GetRepoDescriptionsByIDs(t *testing.T) {
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	created := mustCreate(ctx, t, db, &types.Repo{
		Name:        "Kafka by the Shore",
		Description: "A novel by Haruki Murakami",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "a",
			ServiceType: "b",
			ServiceID:   "c",
		},
	})
	want := map[api.RepoID]string{
		created.ID: "A novel by Haruki Murakami",
	}

	repos, err := db.Repos().GetRepoDescriptionsByIDs(ctx, created.ID, 404)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 1 {
		t.Errorf("got %d repos, want 1", len(repos))
	}
	if diff := cmp.Diff(repos, want); diff != "" {
		t.Errorf("unexpected result (-want, +got)\n%s", diff)
	}
}

func TestRepos_List(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	now := time.Now()

	service := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create a new external service
	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	err := db.ExternalServices().Create(ctx, confGet, &service)
	if err != nil {
		t.Fatal(err)
	}

	want := mustCreate(ctx, t, db, &types.Repo{
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "r",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com",
		},
		Name:        "name",
		Private:     true,
		URI:         "uri",
		Description: "description",
		Fork:        true,
		Archived:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    new(github.Repository),
		Sources: map[string]*types.SourceInfo{
			service.URN(): {
				ID:       service.URN(),
				CloneURL: "git@github.com:foo/bar.git",
			},
		},
	})

	repos, err := db.Repos().List(ctx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(t, repos, []*types.Repo{want}) {
		t.Errorf("got %v, want %v", repos, want)
	}
}

func TestRepos_List_fork(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	mine := mustCreate(ctx, t, db, &types.Repo{Name: "a/r", Fork: false})
	yours := mustCreate(ctx, t, db, &types.Repo{Name: "b/r", Fork: true})

	for _, tt := range []struct {
		opts ReposListOptions
		want []*types.Repo
	}{
		{opts: ReposListOptions{}, want: []*types.Repo{mine, yours}},
		{opts: ReposListOptions{OnlyForks: true}, want: []*types.Repo{yours}},
		{opts: ReposListOptions{NoForks: true}, want: []*types.Repo{mine}},
		{opts: ReposListOptions{OnlyForks: true, NoForks: true}, want: nil},
	} {
		have, err := db.Repos().List(ctx, tt.opts)
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, tt.want, have)
	}
}

func TestRepos_List_FailedSync(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	assertCount := func(t *testing.T, opts ReposListOptions, want int) {
		t.Helper()
		count, err := db.Repos().Count(ctx, opts)
		if err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("Expected %d repos, got %d", want, count)
		}
	}

	repo := mustCreate(ctx, t, db, &types.Repo{Name: "repo1"})
	setGitserverRepoCloneStatus(t, db, repo.Name, types.CloneStatusCloned)
	assertCount(t, ReposListOptions{}, 1)
	assertCount(t, ReposListOptions{FailedFetch: true}, 0)

	setGitserverRepoLastError(t, db, repo.Name, "Oops")
	assertCount(t, ReposListOptions{FailedFetch: true}, 1)
	assertCount(t, ReposListOptions{}, 1)
}

func TestRepos_List_OnlyCorrupted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	assertCount := func(t *testing.T, opts ReposListOptions, want int) {
		t.Helper()
		count, err := db.Repos().Count(ctx, opts)
		if err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Fatalf("Expected %d repos, got %d", want, count)
		}
	}

	repo := mustCreate(ctx, t, db, &types.Repo{Name: "repo1"})
	setGitserverRepoCloneStatus(t, db, repo.Name, types.CloneStatusCloned)
	assertCount(t, ReposListOptions{}, 1)
	assertCount(t, ReposListOptions{OnlyCorrupted: true}, 0)

	logCorruption(t, db, repo.Name, shardID, "some corruption")
	assertCount(t, ReposListOptions{OnlyCorrupted: true}, 1)
	assertCount(t, ReposListOptions{}, 1)
}

func TestRepos_List_cloned(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	var repos []*types.Repo
	for _, data := range []struct {
		repo        *types.Repo
		cloneStatus types.CloneStatus
	}{
		{repo: &types.Repo{Name: "repo-0"}, cloneStatus: types.CloneStatusNotCloned},
		{repo: &types.Repo{Name: "repo-1"}, cloneStatus: types.CloneStatusCloned},
		{repo: &types.Repo{Name: "repo-2"}, cloneStatus: types.CloneStatusCloning},
	} {
		repo := mustCreate(ctx, t, db, data.repo)
		setGitserverRepoCloneStatus(t, db, repo.Name, data.cloneStatus)
		repos = append(repos, repo)
	}

	tests := []struct {
		name string
		opt  ReposListOptions
		want []*types.Repo
	}{
		{"OnlyCloned", ReposListOptions{OnlyCloned: true}, []*types.Repo{repos[1]}},
		{"NoCloned", ReposListOptions{NoCloned: true}, []*types.Repo{repos[0], repos[2]}},
		{"NoCloned && OnlyCloned", ReposListOptions{NoCloned: true, OnlyCloned: true}, nil},
		{"Default", ReposListOptions{}, repos},
		{"CloneStatus=Cloned", ReposListOptions{CloneStatus: types.CloneStatusCloned}, []*types.Repo{repos[1]}},
		{"CloneStatus=NotCloned", ReposListOptions{CloneStatus: types.CloneStatusNotCloned}, []*types.Repo{repos[0]}},
		{"CloneStatus=Cloning", ReposListOptions{CloneStatus: types.CloneStatusCloning}, []*types.Repo{repos[2]}},
		// These don't make sense, but we test that both conditions are used
		{"OnlyCloned && CloneStatus=Cloning", ReposListOptions{OnlyCloned: true, CloneStatus: types.CloneStatusCloning}, nil},
		{"NoCloned && CloneStatus=Cloned", ReposListOptions{NoCloned: true, CloneStatus: types.CloneStatusCloned}, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().List(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_List_indexed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	var repos []*types.Repo
	for _, data := range []struct {
		repo    *types.Repo
		indexed bool
	}{
		{repo: &types.Repo{Name: "repo-0"}, indexed: true},
		{repo: &types.Repo{Name: "repo-1"}, indexed: false},
	} {
		repo := mustCreate(ctx, t, db, data.repo)
		if data.indexed {
			setZoektIndexed(t, db, repo.Name)
		}
		repos = append(repos, repo)
	}

	tests := []struct {
		name string
		opt  ReposListOptions
		want []*types.Repo
	}{
		{"Default", ReposListOptions{}, repos},
		{"OnlyIndexed", ReposListOptions{OnlyIndexed: true}, repos[0:1]},
		{"NoIndexed", ReposListOptions{NoIndexed: true}, repos[1:2]},
		{"NoIndexed && OnlyIndexed", ReposListOptions{NoIndexed: true, OnlyIndexed: true}, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().List(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_List_LastChanged(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	repos := db.Repos()

	// Insert a repo which should never be returned since we always specify
	// OnlyCloned.
	if err := upsertRepo(ctx, db, InsertRepoOp{Name: "not-on-gitserver"}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	r1 := mustCreate(ctx, t, db, &types.Repo{Name: "old"})
	setGitserverRepoCloneStatus(t, db, r1.Name, types.CloneStatusCloned)
	setGitserverRepoLastChanged(t, db, r1.Name, now.Add(-time.Hour))
	r2 := mustCreate(ctx, t, db, &types.Repo{Name: "new"})
	setGitserverRepoCloneStatus(t, db, r2.Name, types.CloneStatusCloned)
	setGitserverRepoLastChanged(t, db, r2.Name, now)

	// we create a repo that has recently had new page rank scores committed to the database
	r3 := mustCreate(ctx, t, db, &types.Repo{Name: "ranked"})
	setGitserverRepoCloneStatus(t, db, r3.Name, types.CloneStatusCloned)
	setGitserverRepoLastChanged(t, db, r3.Name, now.Add(-time.Hour))
	{
		if _, err := db.Handle().ExecContext(ctx, `
			INSERT INTO codeintel_path_ranks(graph_key, repository_id, updated_at, payload)
			VALUES ('test', $1, NOW() + '1 day'::interval, '{}'::jsonb)
		`,
			r3.ID,
		); err != nil {
			t.Fatal(err)
		}

		if _, err := db.Handle().ExecContext(ctx, `
			INSERT INTO codeintel_ranking_progress(graph_key, max_export_id, mappers_started_at, reducer_completed_at)
			VALUES ('test', 1000, NOW(), $1)
		`, now,
		); err != nil {
			t.Fatal(err)
		}
	}

	// Our test helpers don't do updated_at, so manually doing it.
	_, err := db.Handle().ExecContext(ctx, "update repo set updated_at = $1", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// will have update_at set to now, so should be included as often as new.
	r4 := mustCreate(ctx, t, db, &types.Repo{Name: "newMeta"})
	setGitserverRepoCloneStatus(t, db, r4.Name, types.CloneStatusCloned)
	setGitserverRepoLastChanged(t, db, r4.Name, now.Add(-24*time.Hour))
	_, err = db.Handle().ExecContext(ctx, "update repo set updated_at = $1 where name = 'newMeta'", now)
	if err != nil {
		t.Fatal(err)
	}

	// we create two search contexts, with one being updated recently only
	// including "newSearchContext".
	r5 := mustCreate(ctx, t, db, &types.Repo{Name: "newSearchContext"})
	setGitserverRepoCloneStatus(t, db, r5.Name, types.CloneStatusCloned)
	setGitserverRepoLastChanged(t, db, r5.Name, now.Add(-24*time.Hour))
	{
		mkSearchContext := func(name string, opts ReposListOptions) {
			t.Helper()
			var revs []*types.SearchContextRepositoryRevisions
			err := repos.StreamMinimalRepos(ctx, opts, func(repo *types.MinimalRepo) {
				revs = append(revs, &types.SearchContextRepositoryRevisions{
					Repo:      *repo,
					Revisions: []string{"HEAD"},
				})
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.SearchContexts().CreateSearchContextWithRepositoryRevisions(ctx, &types.SearchContext{Name: name}, revs)
			if err != nil {
				t.Fatal(err)
			}
		}
		mkSearchContext("old", ReposListOptions{})
		_, err = db.Handle().ExecContext(ctx, "update search_contexts set updated_at = $1", now.Add(-24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		mkSearchContext("new", ReposListOptions{
			Names: []string{"newSearchContext"},
		})
	}

	tests := []struct {
		Name           string
		MinLastChanged time.Time
		Want           []string
	}{{
		Name: "not specified",
		Want: []string{"old", "new", "ranked", "newMeta", "newSearchContext"},
	}, {
		Name:           "old",
		MinLastChanged: now.Add(-24 * time.Hour),
		Want:           []string{"old", "new", "ranked", "newMeta", "newSearchContext"},
	}, {
		Name:           "new",
		MinLastChanged: now.Add(-time.Minute),
		Want:           []string{"new", "ranked", "newMeta", "newSearchContext"},
	}, {
		Name:           "none",
		MinLastChanged: now.Add(time.Minute),
	}}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			repos, err := repos.List(ctx, ReposListOptions{
				OnlyCloned:     true,
				MinLastChanged: test.MinLastChanged,
			})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range repos {
				got = append(got, string(r.Name))
			}
			if d := cmp.Diff(test.Want, got); d != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", d)
			}
		})
	}
}

func TestRepos_List_ids(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	mine := types.Repos{mustCreate(ctx, t, db, typestest.MakeGithubRepo())}
	mine = append(mine, mustCreate(ctx, t, db, typestest.MakeGitlabRepo()))

	yours := types.Repos{mustCreate(ctx, t, db, typestest.MakeGitoliteRepo())}
	all := append(mine, yours...)

	tests := []struct {
		name string
		opt  ReposListOptions
		want []*types.Repo
	}{
		{"Subset", ReposListOptions{IDs: mine.IDs()}, mine},
		{"All", ReposListOptions{IDs: all.IDs()}, all},
		{"Default", ReposListOptions{}, all},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().List(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_List_pagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "r1"},
		{Name: "r2"},
		{Name: "r3"},
	}
	for _, repo := range createdRepos {
		mustCreate(ctx, t, db, repo)
	}

	type testcase struct {
		limit  int
		offset int
		exp    []api.RepoName
	}
	tests := []testcase{
		{limit: 1, offset: 0, exp: []api.RepoName{"r1"}},
		{limit: 1, offset: 1, exp: []api.RepoName{"r2"}},
		{limit: 1, offset: 2, exp: []api.RepoName{"r3"}},
		{limit: 2, offset: 0, exp: []api.RepoName{"r1", "r2"}},
		{limit: 2, offset: 2, exp: []api.RepoName{"r3"}},
		{limit: 3, offset: 0, exp: []api.RepoName{"r1", "r2", "r3"}},
		{limit: 3, offset: 3, exp: nil},
		{limit: 4, offset: 0, exp: []api.RepoName{"r1", "r2", "r3"}},
		{limit: 4, offset: 4, exp: nil},
	}
	for _, test := range tests {
		repos, err := db.Repos().List(ctx, ReposListOptions{LimitOffset: &LimitOffset{Limit: test.limit, Offset: test.offset}})
		if err != nil {
			t.Fatal(err)
		}
		if got := sortedRepoNames(repos); !reflect.DeepEqual(got, test.exp) {
			t.Errorf("for test case %v, got %v (want %v)", test, got, test.exp)
		}
	}
}

// TestRepos_List_query tests the behavior of Repos.List when called with
// a query.
// Test batch 1 (correct filtering)
func TestRepos_List_query1(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "abc/def"},
		{Name: "def/ghi"},
		{Name: "jkl/mno/pqr"},
		{Name: "github.com/abc/xyz"},
	}
	for _, repo := range createdRepos {
		createRepo(ctx, t, db, repo)
	}

	abcDefRepo, err := db.Repos().GetByName(ctx, "abc/def")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		query string
		want  []api.RepoName
	}{
		{"def", []api.RepoName{"abc/def", "def/ghi"}},
		{"ABC/DEF", []api.RepoName{"abc/def"}},
		{"xyz", []api.RepoName{"github.com/abc/xyz"}},
		{"mno/p", []api.RepoName{"jkl/mno/pqr"}},

		// Test if we match by ID
		{strconv.Itoa(int(abcDefRepo.ID)), []api.RepoName{"abc/def"}},
		{string(relay.MarshalID("Repository", abcDefRepo.ID)), []api.RepoName{"abc/def"}},
	}
	for _, test := range tests {
		repos, err := db.Repos().List(ctx, ReposListOptions{Query: test.query})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(repos); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%q: got repos %q, want %q", test.query, got, test.want)
		}
	}
}

// Test batch 2 (correct ranking)
func TestRepos_List_correct_ranking(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "a/def"},
		{Name: "b/def"},
		{Name: "c/def"},
		{Name: "def/ghi"},
		{Name: "def/jkl"},
		{Name: "def/mno"},
		{Name: "abc/m"},
	}
	for _, repo := range createdRepos {
		createRepo(ctx, t, db, repo)
	}
	tests := []struct {
		query string
		want  []api.RepoName
	}{
		{"def", []api.RepoName{"a/def", "b/def", "c/def", "def/ghi", "def/jkl", "def/mno"}},
		{"b/def", []api.RepoName{"b/def"}},
		{"def/", []api.RepoName{"def/ghi", "def/jkl", "def/mno"}},
		{"def/m", []api.RepoName{"def/mno"}},
	}
	for _, test := range tests {
		repos, err := db.Repos().List(ctx, ReposListOptions{Query: test.query})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(repos); !reflect.DeepEqual(got, test.want) {
			t.Errorf("Unexpected repo result for query %q:\ngot:  %q\nwant: %q", test.query, got, test.want)
		}
	}
}

type repoAndSize struct {
	repo *types.Repo
	size int64
}

// Test sort
func TestRepos_List_sort(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	reposAndSizes := []*repoAndSize{
		{repo: &types.Repo{Name: "c/def"}, size: 20},
		{repo: &types.Repo{Name: "def/mno"}, size: 30},
		{repo: &types.Repo{Name: "b/def"}, size: 40},
		{repo: &types.Repo{Name: "abc/m"}, size: 50},
		{repo: &types.Repo{Name: "abc/def"}, size: 60},
		{repo: &types.Repo{Name: "def/jkl"}, size: 70},
		{repo: &types.Repo{Name: "def/ghi"}, size: 10},
	}
	for _, repoAndSize := range reposAndSizes {
		createRepoWithSize(ctx, t, db, repoAndSize.repo, repoAndSize.size)
	}
	tests := []struct {
		query   string
		orderBy RepoListOrderBy
		want    []api.RepoName
	}{
		{
			query: "",
			orderBy: RepoListOrderBy{{
				Field: RepoListName,
			}},
			want: []api.RepoName{"abc/def", "abc/m", "b/def", "c/def", "def/ghi", "def/jkl", "def/mno"},
		},
		{
			query: "",
			orderBy: RepoListOrderBy{{
				Field: RepoListCreatedAt,
			}},
			want: []api.RepoName{"c/def", "def/mno", "b/def", "abc/m", "abc/def", "def/jkl", "def/ghi"},
		},
		{
			query: "",
			orderBy: RepoListOrderBy{{
				Field:      RepoListCreatedAt,
				Descending: true,
			}},
			want: []api.RepoName{"def/ghi", "def/jkl", "abc/def", "abc/m", "b/def", "def/mno", "c/def"},
		},
		{
			query: "def",
			orderBy: RepoListOrderBy{{
				Field:      RepoListCreatedAt,
				Descending: true,
			}},
			want: []api.RepoName{"def/ghi", "def/jkl", "abc/def", "b/def", "def/mno", "c/def"},
		},
		{
			query: "",
			orderBy: RepoListOrderBy{{
				Field:      RepoListSize,
				Descending: false,
			}},
			want: []api.RepoName{"def/ghi", "c/def", "def/mno", "b/def", "abc/m", "abc/def", "def/jkl"},
		},
	}
	for _, test := range tests {
		repos, err := db.Repos().List(ctx, ReposListOptions{Query: test.query, OrderBy: test.orderBy})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(repos); !reflect.DeepEqual(got, test.want) {
			t.Errorf("Unexpected repo result for query %q, orderBy %v:\ngot:  %q\nwant: %q", test.query, test.orderBy, got, test.want)
		}
	}
}

// TestRepos_List_patterns tests the behavior of Repos.List when called with
// IncludePaths and ExcludePaths.
func TestRepos_List_patterns(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "a/b"},
		{Name: "c/d"},
		{Name: "e/f"},
		{Name: "g/h"},
		{Name: "I/J"},
	}
	for _, repo := range createdRepos {
		createRepo(ctx, t, db, repo)
	}
	tests := []struct {
		includePatterns []string
		excludePattern  string
		caseSensitive   bool
		want            []api.RepoName
	}{
		{
			includePatterns: []string{"(a|c)"},
			want:            []api.RepoName{"a/b", "c/d"},
		},
		{
			includePatterns: []string{"(a|c)", "b"},
			want:            []api.RepoName{"a/b"},
		},
		{
			includePatterns: []string{"(a|c)"},
			excludePattern:  "d",
			want:            []api.RepoName{"a/b"},
		},
		{
			excludePattern: "(d|e)",
			want:           []api.RepoName{"a/b", "g/h", "I/J"},
		},
		{
			includePatterns: []string{"(A|c|I)"},
			want:            []api.RepoName{"a/b", "c/d", "I/J"},
		},
		{
			includePatterns: []string{"I", "J"},
			caseSensitive:   true,
			want:            []api.RepoName{"I/J"},
		},
	}
	for _, test := range tests {
		repos, err := db.Repos().List(ctx, ReposListOptions{
			IncludePatterns:       test.includePatterns,
			ExcludePattern:        test.excludePattern,
			CaseSensitivePatterns: test.caseSensitive,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(repos); !reflect.DeepEqual(got, test.want) {
			t.Errorf("include %q exclude %q: got repos %q, want %q", test.includePatterns, test.excludePattern, got, test.want)
		}
	}
}

func TestRepos_List_queryAndPatternsMutuallyExclusive(t *testing.T) {
	ctx := actor.WithInternalActor(context.Background())
	wantErr := "Query and IncludePaths/ExcludePaths options are mutually exclusive"

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	t.Run("Query and IncludePaths", func(t *testing.T) {
		_, err := db.Repos().List(ctx, ReposListOptions{Query: "x", IncludePatterns: []string{"y"}})
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("got error %v, want it to contain %q", err, wantErr)
		}
	})

	t.Run("Query and ExcludePaths", func(t *testing.T) {
		_, err := db.Repos().List(ctx, ReposListOptions{Query: "x", ExcludePattern: "y"})
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("got error %v, want it to contain %q", err, wantErr)
		}
	})
}

func TestRepos_List_useOr(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	archived := mustCreate(ctx, t, db, types.Repos{typestest.MakeGitlabRepo()}.With(func(r *types.Repo) { r.Archived = true })[0])
	forks := mustCreate(ctx, t, db, types.Repos{typestest.MakeGitoliteRepo()}.With(func(r *types.Repo) { r.Fork = true })[0])
	cloned := mustCreate(ctx, t, db, types.Repos{typestest.MakeGithubRepo()}[0])
	setGitserverRepoCloneStatus(t, db, cloned.Name, types.CloneStatusCloned)

	archivedAndForks := append(types.Repos{}, archived, forks)
	sort.Sort(archivedAndForks)
	all := append(archivedAndForks, cloned)
	sort.Sort(all)

	tests := []struct {
		name string
		opt  ReposListOptions
		want []*types.Repo
	}{
		{"Archived or Forks", ReposListOptions{OnlyArchived: true, OnlyForks: true, UseOr: true}, archivedAndForks},
		{"Archived or Forks Or Cloned", ReposListOptions{OnlyArchived: true, OnlyForks: true, OnlyCloned: true, UseOr: true}, all},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().List(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_List_externalServiceID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	services := typestest.MakeExternalServices()
	service1 := services[0]
	service2 := services[1]
	if err := db.ExternalServices().Create(ctx, confGet, service1); err != nil {
		t.Fatal(err)
	}
	if err := db.ExternalServices().Create(ctx, confGet, service2); err != nil {
		t.Fatal(err)
	}

	mine := types.Repos{typestest.MakeGithubRepo(service1)}
	if err := db.Repos().Create(ctx, mine...); err != nil {
		t.Fatal(err)
	}

	yours := types.Repos{typestest.MakeGitlabRepo(service2)}
	if err := db.Repos().Create(ctx, yours...); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opt  ReposListOptions
		want []*types.Repo
	}{
		{"Some", ReposListOptions{ExternalServiceIDs: []int64{service1.ID}}, mine},
		{"Default", ReposListOptions{}, append(mine, yours...)},
		{"NonExistant", ReposListOptions{ExternalServiceIDs: []int64{1000}}, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().List(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_List_topics(t *testing.T) {
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())
	confGet := func() *conf.Unified { return &conf.Unified{} }

	services := typestest.MakeExternalServices()
	githubService := services[0]
	gitlabService := services[1]
	if err := db.ExternalServices().Create(ctx, confGet, githubService); err != nil {
		t.Fatal(err)
	}
	if err := db.ExternalServices().Create(ctx, confGet, gitlabService); err != nil {
		t.Fatal(err)
	}

	setTopics := func(topics ...string) func(r *types.Repo) {
		return func(r *types.Repo) {
			if ghr, ok := r.Metadata.(*github.Repository); ok {
				for _, topic := range topics {
					ghr.RepositoryTopics.Nodes = append(ghr.RepositoryTopics.Nodes, github.RepositoryTopic{
						Topic: github.Topic{Name: topic},
					})
					// In production, topics are only set on read. Here we add them manually to be
					// able to compare them with the repos returned by the db.
					r.Topics = append(r.Topics, topic)
				}
			}
		}
	}

	ids := func(id int) func(r *types.Repo) {
		return func(r *types.Repo) {
			r.ExternalRepo.ID = strconv.Itoa(id)
			r.Name = api.RepoName(strconv.Itoa(id))
		}
	}

	r1 := typestest.MakeGithubRepo().With(ids(1), setTopics("topic1", "topic2"))
	r2 := typestest.MakeGithubRepo().With(ids(2), setTopics("topic2", "topic3"))
	r3 := typestest.MakeGithubRepo().With(ids(3), setTopics("topic1", "topic2", "topic3"))
	r4 := typestest.MakeGitlabRepo().With(ids(4))
	r5 := typestest.MakeGithubRepo().With(ids(5))
	if err := db.Repos().Create(ctx, r1, r2, r3, r4, r5); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opt  ReposListOptions
		want []*types.Repo
	}{
		{"topic1", ReposListOptions{TopicFilters: []RepoTopicFilter{{Topic: "topic1"}}}, []*types.Repo{r1, r3}},
		{"topic2", ReposListOptions{TopicFilters: []RepoTopicFilter{{Topic: "topic2"}}}, []*types.Repo{r1, r2, r3}},
		{"topic3", ReposListOptions{TopicFilters: []RepoTopicFilter{{Topic: "topic3"}}}, []*types.Repo{r2, r3}},
		{"not topic1", ReposListOptions{TopicFilters: []RepoTopicFilter{{Topic: "topic1", Negated: true}}}, []*types.Repo{r2, r4, r5}},
		{
			"topic3 not topic1",
			ReposListOptions{TopicFilters: []RepoTopicFilter{{Topic: "topic3"}, {Topic: "topic1", Negated: true}}},
			[]*types.Repo{r2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().List(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			require.Equal(t, test.want, repos)
		})
	}
}

func TestRepos_ListMinimalRepos(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	now := time.Now()

	service := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"url": "https://github.com", "repositoryQuery": ["none"], "token": "abc"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create a new external service
	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	err := db.ExternalServices().Create(ctx, confGet, &service)
	if err != nil {
		t.Fatal(err)
	}

	repo := mustCreate(ctx, t, db, &types.Repo{
		Name: "name",
	})
	want := []types.MinimalRepo{{ID: repo.ID, Name: repo.Name}}

	repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(t, repos, want) {
		t.Errorf("got %v, want %v", repos, want)
	}
}

func TestRepos_ListMinimalRepos_fork(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	mine := repoNamesFromRepos([]*types.Repo{mustCreate(ctx, t, db, &types.Repo{Name: "a/r", Fork: false})})
	yours := repoNamesFromRepos([]*types.Repo{mustCreate(ctx, t, db, &types.Repo{Name: "b/r", Fork: true})})

	{
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{OnlyForks: true})
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, yours, repos)
	}
	{
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{NoForks: true})
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, mine, repos)
	}
	{
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{NoForks: true, OnlyForks: true})
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, []types.MinimalRepo{}, repos)
	}
	{
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{})
		if err != nil {
			t.Fatal(err)
		}
		assertJSONEqual(t, append(append([]types.MinimalRepo{}, mine...), yours...), repos)
	}
}

func TestRepos_ListMinimalRepos_cloned(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	mine := repoNamesFromRepos([]*types.Repo{mustCreate(ctx, t, db, &types.Repo{Name: "a/r"})})
	yourRepo := mustCreate(ctx, t, db, &types.Repo{Name: "b/r"})
	setGitserverRepoCloneStatus(t, db, yourRepo.Name, types.CloneStatusCloned)
	yours := repoNamesFromRepos([]*types.Repo{yourRepo})

	tests := []struct {
		name string
		opt  ReposListOptions
		want []types.MinimalRepo
	}{
		{"OnlyCloned", ReposListOptions{OnlyCloned: true}, yours},
		{"NoCloned", ReposListOptions{NoCloned: true}, mine},
		{"NoCloned && OnlyCloned", ReposListOptions{NoCloned: true, OnlyCloned: true}, []types.MinimalRepo{}},
		{"Default", ReposListOptions{}, append(append([]types.MinimalRepo{}, mine...), yours...)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().ListMinimalRepos(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_ListMinimalRepos_ids(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	mine := types.Repos{mustCreate(ctx, t, db, typestest.MakeGithubRepo())}
	mine = append(mine, mustCreate(ctx, t, db, typestest.MakeGitlabRepo()))

	yours := types.Repos{mustCreate(ctx, t, db, typestest.MakeGitoliteRepo())}
	all := append(mine, yours...)

	tests := []struct {
		name string
		opt  ReposListOptions
		want []types.MinimalRepo
	}{
		{"Subset", ReposListOptions{IDs: mine.IDs()}, repoNamesFromRepos(mine)},
		{"All", ReposListOptions{IDs: all.IDs()}, repoNamesFromRepos(all)},
		{"Default", ReposListOptions{}, repoNamesFromRepos(all)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().ListMinimalRepos(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_ListMinimalRepos_pagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "r1"},
		{Name: "r2"},
		{Name: "r3"},
	}
	for _, repo := range createdRepos {
		mustCreate(ctx, t, db, repo)
	}

	type testcase struct {
		limit  int
		offset int
		exp    []api.RepoName
	}
	tests := []testcase{
		{limit: 1, offset: 0, exp: []api.RepoName{"r1"}},
		{limit: 1, offset: 1, exp: []api.RepoName{"r2"}},
		{limit: 1, offset: 2, exp: []api.RepoName{"r3"}},
		{limit: 2, offset: 0, exp: []api.RepoName{"r1", "r2"}},
		{limit: 2, offset: 2, exp: []api.RepoName{"r3"}},
		{limit: 3, offset: 0, exp: []api.RepoName{"r1", "r2", "r3"}},
		{limit: 3, offset: 3, exp: nil},
		{limit: 4, offset: 0, exp: []api.RepoName{"r1", "r2", "r3"}},
		{limit: 4, offset: 4, exp: nil},
	}
	for _, test := range tests {
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{LimitOffset: &LimitOffset{Limit: test.limit, Offset: test.offset}})
		if err != nil {
			t.Fatal(err)
		}
		if got := sortedRepoNames(reposFromRepoNames(repos)); !reflect.DeepEqual(got, test.exp) {
			t.Errorf("for test case %v, got %v (want %v)", test, got, test.exp)
		}
	}
}

// TestRepos_ListMinimalRepos_query tests the behavior of Repos.ListMinimalRepos when called with
// a query.
// Test batch 1 (correct filtering)
func TestRepos_ListMinimalRepos_correctFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "abc/def"},
		{Name: "def/ghi"},
		{Name: "jkl/mno/pqr"},
		{Name: "github.com/abc/xyz"},
	}
	for _, repo := range createdRepos {
		createRepo(ctx, t, db, repo)
	}
	tests := []struct {
		query string
		want  []api.RepoName
	}{
		{"def", []api.RepoName{"abc/def", "def/ghi"}},
		{"ABC/DEF", []api.RepoName{"abc/def"}},
		{"xyz", []api.RepoName{"github.com/abc/xyz"}},
		{"mno/p", []api.RepoName{"jkl/mno/pqr"}},
	}
	for _, test := range tests {
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{Query: test.query})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(reposFromRepoNames(repos)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%q: got repos %q, want %q", test.query, got, test.want)
		}
	}
}

// Test batch 2 (correct ranking)
func TestRepos_ListMinimalRepos_query2(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "a/def"},
		{Name: "b/def"},
		{Name: "c/def"},
		{Name: "def/ghi"},
		{Name: "def/jkl"},
		{Name: "def/mno"},
		{Name: "abc/m"},
	}
	for _, repo := range createdRepos {
		createRepo(ctx, t, db, repo)
	}
	tests := []struct {
		query string
		want  []api.RepoName
	}{
		{"def", []api.RepoName{"a/def", "b/def", "c/def", "def/ghi", "def/jkl", "def/mno"}},
		{"b/def", []api.RepoName{"b/def"}},
		{"def/", []api.RepoName{"def/ghi", "def/jkl", "def/mno"}},
		{"def/m", []api.RepoName{"def/mno"}},
	}
	for _, test := range tests {
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{Query: test.query})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(reposFromRepoNames(repos)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("Unexpected repo result for query %q:\ngot:  %q\nwant: %q", test.query, got, test.want)
		}
	}
}

// Test sort
func TestRepos_ListMinimalRepos_sort(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "c/def"},
		{Name: "def/mno"},
		{Name: "b/def"},
		{Name: "abc/m"},
		{Name: "abc/def"},
		{Name: "def/jkl"},
		{Name: "def/ghi"},
	}
	for _, repo := range createdRepos {
		createRepo(ctx, t, db, repo)
	}
	tests := []struct {
		query   string
		orderBy RepoListOrderBy
		want    []api.RepoName
	}{
		{
			query: "",
			orderBy: RepoListOrderBy{{
				Field: RepoListName,
			}},
			want: []api.RepoName{"abc/def", "abc/m", "b/def", "c/def", "def/ghi", "def/jkl", "def/mno"},
		},
		{
			query: "",
			orderBy: RepoListOrderBy{{
				Field: RepoListCreatedAt,
			}},
			want: []api.RepoName{"c/def", "def/mno", "b/def", "abc/m", "abc/def", "def/jkl", "def/ghi"},
		},
		{
			query: "",
			orderBy: RepoListOrderBy{{
				Field:      RepoListCreatedAt,
				Descending: true,
			}},
			want: []api.RepoName{"def/ghi", "def/jkl", "abc/def", "abc/m", "b/def", "def/mno", "c/def"},
		},
		{
			query: "def",
			orderBy: RepoListOrderBy{{
				Field:      RepoListCreatedAt,
				Descending: true,
			}},
			want: []api.RepoName{"def/ghi", "def/jkl", "abc/def", "b/def", "def/mno", "c/def"},
		},
	}
	for _, test := range tests {
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{Query: test.query, OrderBy: test.orderBy})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(reposFromRepoNames(repos)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("Unexpected repo result for query %q, orderBy %v:\ngot:  %q\nwant: %q", test.query, test.orderBy, got, test.want)
		}
	}
}

// TestRepos_ListMinimalRepos_patterns tests the behavior of Repos.List when called with
// IncludePaths and ExcludePaths.
func TestRepos_ListMinimalRepos_patterns(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	createdRepos := []*types.Repo{
		{Name: "a/b"},
		{Name: "c/d"},
		{Name: "e/f"},
		{Name: "g/h"},
	}
	for _, repo := range createdRepos {
		createRepo(ctx, t, db, repo)
	}
	tests := []struct {
		includePatterns []string
		excludePattern  string
		want            []api.RepoName
	}{
		{
			includePatterns: []string{"(a|c)"},
			want:            []api.RepoName{"a/b", "c/d"},
		},
		{
			includePatterns: []string{"(a|c)", "b"},
			want:            []api.RepoName{"a/b"},
		},
		{
			includePatterns: []string{"(a|c)"},
			excludePattern:  "d",
			want:            []api.RepoName{"a/b"},
		},
		{
			excludePattern: "(d|e)",
			want:           []api.RepoName{"a/b", "g/h"},
		},
	}
	for _, test := range tests {
		repos, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{
			IncludePatterns: test.includePatterns,
			ExcludePattern:  test.excludePattern,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := repoNames(reposFromRepoNames(repos)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("include %q exclude %q: got repos %q, want %q", test.includePatterns, test.excludePattern, got, test.want)
		}
	}
}

func TestRepos_ListMinimalRepos_queryAndPatternsMutuallyExclusive(t *testing.T) {
	ctx := actor.WithInternalActor(context.Background())
	wantErr := "Query and IncludePaths/ExcludePaths options are mutually exclusive"

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	t.Run("Query and IncludePaths", func(t *testing.T) {
		_, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{Query: "x", IncludePatterns: []string{"y"}})
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("got error %v, want it to contain %q", err, wantErr)
		}
	})

	t.Run("Query and ExcludePaths", func(t *testing.T) {
		_, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{Query: "x", ExcludePattern: "y"})
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("got error %v, want it to contain %q", err, wantErr)
		}
	})
}

func TestRepos_ListMinimalRepos_SearchContextIDAndExternalServiceIDsMutuallyExclusive(t *testing.T) {
	ctx := actor.WithInternalActor(context.Background())
	wantErr := "options ExternalServiceIDs and SearchContextID are mutually exclusive"

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	_, err := db.Repos().ListMinimalRepos(ctx, ReposListOptions{SearchContextID: 1, ExternalServiceIDs: []int64{2}})
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Fatalf("got error %v, want it to contain %q", err, wantErr)
	}
}

func TestRepos_ListMinimalRepos_useOr(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	archived := mustCreate(ctx, t, db, types.Repos{typestest.MakeGitlabRepo()}.With(func(r *types.Repo) { r.Archived = true })[0])
	forks := mustCreate(ctx, t, db, types.Repos{typestest.MakeGitoliteRepo()}.With(func(r *types.Repo) { r.Fork = true })[0])
	cloned := mustCreate(ctx, t, db, types.Repos{typestest.MakeGithubRepo()}[0])
	setGitserverRepoCloneStatus(t, db, cloned.Name, types.CloneStatusCloned)

	archivedAndForks := append(types.Repos{}, archived, forks)
	sort.Sort(archivedAndForks)
	all := append(archivedAndForks, cloned)
	sort.Sort(all)

	tests := []struct {
		name string
		opt  ReposListOptions
		want []types.MinimalRepo
	}{
		{"Archived or Forks", ReposListOptions{OnlyArchived: true, OnlyForks: true, UseOr: true}, repoNamesFromRepos(archivedAndForks)},
		{"Archived or Forks Or Cloned", ReposListOptions{OnlyArchived: true, OnlyForks: true, OnlyCloned: true, UseOr: true}, repoNamesFromRepos(all)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().ListMinimalRepos(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestRepos_ListMinimalRepos_externalServiceID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	services := typestest.MakeExternalServices()
	service1 := services[0]
	service2 := services[1]
	if err := db.ExternalServices().Create(ctx, confGet, service1); err != nil {
		t.Fatal(err)
	}
	if err := db.ExternalServices().Create(ctx, confGet, service2); err != nil {
		t.Fatal(err)
	}

	mine := types.Repos{typestest.MakeGithubRepo(service1)}
	if err := db.Repos().Create(ctx, mine...); err != nil {
		t.Fatal(err)
	}

	yours := types.Repos{typestest.MakeGitlabRepo(service2)}
	if err := db.Repos().Create(ctx, yours...); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opt  ReposListOptions
		want []types.MinimalRepo
	}{
		{"Some", ReposListOptions{ExternalServiceIDs: []int64{service1.ID}}, repoNamesFromRepos(mine)},
		{"Default", ReposListOptions{}, repoNamesFromRepos(append(mine, yours...))},
		{"NonExistant", ReposListOptions{ExternalServiceIDs: []int64{1000}}, []types.MinimalRepo{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().ListMinimalRepos(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

// This function tests for both individual uses of ExternalRepoIncludeContains,
// ExternalRepoExcludeContains as well as combination of these two options.
func TestRepos_ListMinimalRepos_externalRepoContains(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	svc := &types.ExternalService{
		Kind:        extsvc.KindPerforce,
		DisplayName: "Perforce - Test",
		Config:      extsvc.NewUnencryptedConfig(`{"p4.port": "ssl:111.222.333.444:1666", "p4.user": "admin", "p4.passwd": "pa$$word", "depots": [], "repositoryPathPattern": "perforce/{depot}"}`),
	}
	if err := db.ExternalServices().Create(ctx, confGet, svc); err != nil {
		t.Fatal(err)
	}

	var (
		perforceMarketing = &types.Repo{
			Name:    api.RepoName("perforce/Marketing"),
			URI:     "Marketing",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "//Marketing/",
				ServiceType: extsvc.TypePerforce,
				ServiceID:   "ssl:111.222.333.444:1666",
			},
		}
		perforceEngineering = &types.Repo{
			Name:    api.RepoName("perforce/Engineering"),
			URI:     "Engineering",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "//Engineering/",
				ServiceType: extsvc.TypePerforce,
				ServiceID:   "ssl:111.222.333.444:1666",
			},
		}
		perforceEngineeringFrontend = &types.Repo{
			Name:    api.RepoName("perforce/Engineering/Frontend"),
			URI:     "Engineering/Frontend",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "//Engineering/Frontend/",
				ServiceType: extsvc.TypePerforce,
				ServiceID:   "ssl:111.222.333.444:1666",
			},
		}
		perforceEngineeringBackend = &types.Repo{
			Name:    api.RepoName("perforce/Engineering/Backend"),
			URI:     "Engineering/Backend",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "//Engineering/Backend/",
				ServiceType: extsvc.TypePerforce,
				ServiceID:   "ssl:111.222.333.444:1666",
			},
		}
		perforceEngineeringHandbookFrontend = &types.Repo{
			Name:    api.RepoName("perforce/Engineering/Handbook/Frontend"),
			URI:     "Engineering/Handbook/Frontend",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "//Engineering/Handbook/Frontend/",
				ServiceType: extsvc.TypePerforce,
				ServiceID:   "ssl:111.222.333.444:1666",
			},
		}
		perforceEngineeringHandbookBackend = &types.Repo{
			Name:    api.RepoName("perforce/Engineering/Handbook/Backend"),
			URI:     "Engineering/Handbook/Backend",
			Private: true,
			ExternalRepo: api.ExternalRepoSpec{
				ID:          "//Engineering/Handbook/Backend/",
				ServiceType: extsvc.TypePerforce,
				ServiceID:   "ssl:111.222.333.444:1666",
			},
		}
	)
	if err := db.Repos().Create(ctx,
		perforceMarketing,
		perforceEngineering,
		perforceEngineeringFrontend,
		perforceEngineeringBackend,
		perforceEngineeringHandbookFrontend,
		perforceEngineeringHandbookBackend); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		opt  ReposListOptions
		want []types.MinimalRepo
	}{
		{
			name: "only apply ExternalRepoIncludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceEngineering, perforceEngineeringFrontend, perforceEngineeringBackend, perforceEngineeringHandbookFrontend, perforceEngineeringHandbookBackend}),
		},
		{
			name: "only apply transformed '...' Perforce wildcard ExternalRepoIncludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//%/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceEngineeringBackend, perforceEngineeringHandbookBackend}),
		},
		{
			name: "only apply multiple transformed '...' Perforce wildcard ExternalRepoIncludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//%/%/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			// Only match this specific nested folder, and not the other Backends
			want: repoNamesFromRepos([]*types.Repo{perforceEngineeringHandbookBackend}),
		},
		{
			name: "only apply transformed '*' Perforce wildcard ExternalRepoIncludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//[^/]+/[^/]+/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			// Only match this specific nested folder, and not the other Backends
			want: repoNamesFromRepos([]*types.Repo{perforceEngineeringHandbookBackend}),
		},
		{
			name: "only apply transformed '*' Perforce partial wildcard ExternalRepoIncludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//[^/]+/Back[^/]+/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			// Only match this specific nested folder, and not the other Backends
			want: repoNamesFromRepos([]*types.Repo{perforceEngineeringBackend}),
		},
		{
			name: "only apply ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceMarketing}),
		},
		{
			name: "only apply transformed '...' Perforce wildcard ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//%/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceMarketing, perforceEngineering, perforceEngineeringFrontend, perforceEngineeringHandbookFrontend}),
		},
		{
			name: "only apply transformed '*' Perforce wildcard ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//[^/]+/[^/]+/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			// Only filter this very specific nesting level
			want: repoNamesFromRepos([]*types.Repo{perforceMarketing, perforceEngineering, perforceEngineeringFrontend, perforceEngineeringBackend, perforceEngineeringHandbookFrontend}),
		},
		{
			name: "apply both ExternalRepoIncludeContains and ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
					{
						ID:          "//Engineering/Handbook/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceEngineering, perforceEngineeringFrontend}),
		},
		{
			name: "apply both ExternalRepoIncludeContains and transformed '...' Perforce wildcard ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//%/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceEngineering, perforceEngineeringFrontend, perforceEngineeringHandbookFrontend}),
		},
		{
			name: "apply both ExternalRepoIncludeContains and transformed '*' Perforce wildcard ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/[^/]+/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceEngineering, perforceEngineeringFrontend, perforceEngineeringBackend, perforceEngineeringHandbookFrontend}),
		},
		{
			name: "apply both transformed '...' Perforce wildcard ExternalRepoIncludeContains and ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//%/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/Handbook/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: repoNamesFromRepos([]*types.Repo{perforceEngineeringBackend}),
		},
		{
			name: "apply both transformed '*' Perforce wildcard ExternalRepoIncludeContains and ExternalRepoExcludeContains",
			opt: ReposListOptions{
				ExternalRepoIncludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/[^/]+/Backend/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
				ExternalRepoExcludeContains: []api.ExternalRepoSpec{
					{
						ID:          "//Engineering/%",
						ServiceType: extsvc.TypePerforce,
						ServiceID:   "ssl:111.222.333.444:1666",
					},
				},
			},
			want: []types.MinimalRepo{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repos, err := db.Repos().ListMinimalRepos(ctx, test.opt)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, test.want, repos)
		})
	}
}

func TestGetFirstRepoNamesByCloneURL(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := actor.WithInternalActor(context.Background())

	confGet := func() *conf.Unified {
		return &conf.Unified{}
	}

	services := typestest.MakeExternalServices()
	service1 := services[0]
	if err := db.ExternalServices().Create(ctx, confGet, service1); err != nil {
		t.Fatal(err)
	}

	repo1 := typestest.MakeGithubRepo(service1)
	if err := db.Repos().Create(ctx, repo1); err != nil {
		t.Fatal(err)
	}

	_, err := db.ExecContext(ctx, "UPDATE external_service_repos SET clone_url = 'https://github.com/foo/bar' WHERE repo_id = $1", repo1.ID)
	if err != nil {
		t.Fatal(err)
	}

	name, err := db.Repos().GetFirstRepoNameByCloneURL(ctx, "https://github.com/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	if name != "github.com/foo/bar" {
		t.Fatalf("Want %q, got %q", "github.com/foo/bar", name)
	}
}

func TestParseIncludePattern(t *testing.T) {
	tests := map[string]struct {
		exact  []string
		like   []string
		regexp string

		pattern []*sqlf.Query // only tested if non-nil
	}{
		`^$`:              {exact: []string{""}},
		`(^$)`:            {exact: []string{""}},
		`((^$))`:          {exact: []string{""}},
		`^((^$))$`:        {exact: []string{""}},
		`^()$`:            {exact: []string{""}},
		`^(())$`:          {exact: []string{""}},
		`^a$`:             {exact: []string{"a"}},
		`(^a$)`:           {exact: []string{"a"}},
		`((^a$))`:         {exact: []string{"a"}},
		`^((^a$))$`:       {exact: []string{"a"}},
		`^(a)$`:           {exact: []string{"a"}},
		`^((a))$`:         {exact: []string{"a"}},
		`^a|b$`:           {like: []string{"a%", "%b"}}, // "|" has higher precedence than "^" or "$"
		`^(a)|(b)$`:       {like: []string{"a%", "%b"}}, // "|" has higher precedence than "^" or "$"
		`^(a|b)$`:         {exact: []string{"a", "b"}},
		`(^a$)|(^b$)`:     {exact: []string{"a", "b"}},
		`((^a$)|(^b$))`:   {exact: []string{"a", "b"}},
		`^((^a$)|(^b$))$`: {exact: []string{"a", "b"}},
		`^((a)|(b))$`:     {exact: []string{"a", "b"}},
		`abc`:             {like: []string{"%abc%"}},
		`a|b`:             {like: []string{"%a%", "%b%"}},
		`^a(b|c)$`:        {exact: []string{"ab", "ac"}},

		`^github\.com/foo/bar`: {like: []string{"github.com/foo/bar%"}},

		`github.com`:  {regexp: `github.com`},
		`github\.com`: {like: []string{`%github.com%`}},

		// https://github.com/sourcegraph/sourcegraph/issues/9146
		`github.com/.*/ini$`:      {regexp: `github.com/.*/ini$`},
		`github\.com/.*/ini$`:     {regexp: `github\.com/.*/ini$`},
		`github\.com/go-ini/ini$`: {like: []string{`%github.com/go-ini/ini`}},

		// https://github.com/sourcegraph/sourcegraph/issues/4166
		`golang/oauth.*`:                    {like: []string{"%golang/oauth%"}},
		`^golang/oauth.*`:                   {like: []string{"golang/oauth%"}},
		`golang/(oauth.*|bla)`:              {like: []string{"%golang/oauth%", "%golang/bla%"}},
		`golang/(oauth|bla)`:                {like: []string{"%golang/oauth%", "%golang/bla%"}},
		`^github.com/(golang|go-.*)/oauth$`: {regexp: `^github.com/(golang|go-.*)/oauth$`},
		`^github.com/(go.*lang|go)/oauth$`:  {regexp: `^github.com/(go.*lang|go)/oauth$`},

		// https://github.com/sourcegraph/sourcegraph/issues/20389
		`^github\.com/sourcegraph/(sourcegraph-atom|sourcegraph)$`: {
			exact: []string{"github.com/sourcegraph/sourcegraph", "github.com/sourcegraph/sourcegraph-atom"},
		},

		// Ensure we don't lose foo/.*. In the past we returned exact for bar only.
		`(^foo/.+$|^bar$)`:     {regexp: `(^foo/.+$|^bar$)`},
		`^foo/.+$|^bar$`:       {regexp: `^foo/.+$|^bar$`},
		`((^foo/.+$)|(^bar$))`: {regexp: `((^foo/.+$)|(^bar$))`},
		`((^foo/.+)|(^bar$))`:  {regexp: `((^foo/.+)|(^bar$))`},

		`(^github\.com/Microsoft/vscode$)|(^github\.com/sourcegraph/go-langserver$)`: {
			exact: []string{"github.com/Microsoft/vscode", "github.com/sourcegraph/go-langserver"},
		},

		// Avoid DoS when there are too many possible matches to enumerate.
		`^(a|b)(c|d)(e|f)(g|h)(i|j)(k|l)(m|n)$`: {regexp: `^(a|b)(c|d)(e|f)(g|h)(i|j)(k|l)(m|n)$`},
		`^[0-a]$`:                               {regexp: `^[0-a]$`},
		`sourcegraph|^github\.com/foo/bar$`: {
			like:  []string{`%sourcegraph%`},
			exact: []string{"github.com/foo/bar"},
			pattern: []*sqlf.Query{
				sqlf.Sprintf(`(name = ANY (%s) OR lower(name) LIKE %s)`, "%!s(*pq.StringArray=&[github.com/foo/bar])", "%sourcegraph%"),
			},
		},

		// Recognize perl character class shorthand syntax.
		`\s`: {regexp: `\s`},
	}

	tr, _ := trace.New(context.Background(), "")
	defer tr.End()

	for pattern, want := range tests {
		exact, like, regexp, err := parseIncludePattern(pattern)
		if err != nil {
			t.Fatal(pattern, err)
		}
		if !reflect.DeepEqual(exact, want.exact) {
			t.Errorf("got exact %q, want %q for %s", exact, want.exact, pattern)
		}
		if !reflect.DeepEqual(like, want.like) {
			t.Errorf("got like %q, want %q for %s", like, want.like, pattern)
		}
		if regexp != want.regexp {
			t.Errorf("got regexp %q, want %q for %s", regexp, want.regexp, pattern)
		}
		if qs, err := parsePattern(tr, pattern, false); err != nil {
			t.Fatal(pattern, err)
		} else {
			if testing.Verbose() {
				q := sqlf.Join(qs, "AND")
				t.Log(pattern, q.Query(sqlf.PostgresBindVar), q.Args())
			}

			if want.pattern != nil {
				want := queriesToString(want.pattern)
				q := queriesToString(qs)
				if want != q {
					t.Errorf("got pattern %q, want %q for %s", q, want, pattern)
				}
			}
		}
	}
}

func queriesToString(qs []*sqlf.Query) string {
	q := sqlf.Join(qs, "AND")
	return fmt.Sprintf("%s %s", q.Query(sqlf.PostgresBindVar), q.Args())
}

func TestRepos_Count(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})

	if count, err := db.Repos().Count(ctx, ReposListOptions{}); err != nil {
		t.Fatal(err)
	} else if want := 0; count != want {
		t.Errorf("got %d, want %d", count, want)
	}

	if err := upsertRepo(ctx, db, InsertRepoOp{Name: "myrepo", Description: "", Fork: false}); err != nil {
		t.Fatal(err)
	}

	if count, err := db.Repos().Count(ctx, ReposListOptions{}); err != nil {
		t.Fatal(err)
	} else if want := 1; count != want {
		t.Errorf("got %d, want %d", count, want)
	}

	t.Run("order and limit options are ignored", func(t *testing.T) {
		opts := ReposListOptions{
			OrderBy:     []RepoListSort{{Field: RepoListID}},
			LimitOffset: &LimitOffset{Limit: 1},
		}
		if count, err := db.Repos().Count(ctx, opts); err != nil {
			t.Fatal(err)
		} else if want := 1; count != want {
			t.Errorf("got %d, want %d", count, want)
		}
	})

	repos, err := db.Repos().List(ctx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Repos().Delete(ctx, repos[0].ID); err != nil {
		t.Fatal(err)
	}

	if count, err := db.Repos().Count(ctx, ReposListOptions{}); err != nil {
		t.Fatal(err)
	} else if want := 0; count != want {
		t.Errorf("got %d, want %d", count, want)
	}
}

func TestRepos_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})

	if err := upsertRepo(ctx, db, InsertRepoOp{Name: "myrepo", Description: "", Fork: false}); err != nil {
		t.Fatal(err)
	}

	if count, err := db.Repos().Count(ctx, ReposListOptions{}); err != nil {
		t.Fatal(err)
	} else if want := 1; count != want {
		t.Errorf("got %d, want %d", count, want)
	}

	repos, err := db.Repos().List(ctx, ReposListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Repos().Delete(ctx, repos[0].ID); err != nil {
		t.Fatal(err)
	}

	if count, err := db.Repos().Count(ctx, ReposListOptions{}); err != nil {
		t.Fatal(err)
	} else if want := 0; count != want {
		t.Errorf("got %d, want %d", count, want)
	}
}

func TestRepos_DeleteReconcilesName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})
	repo := mustCreate(ctx, t, db, &types.Repo{Name: "myrepo"})
	// Artificially set deleted_at but do not modify the name, which all delete code does.
	repo.DeletedAt = time.Date(2020, 10, 12, 12, 0, 0, 0, time.UTC)
	q := sqlf.Sprintf("UPDATE repo SET deleted_at = %s WHERE id = %s", repo.DeletedAt, repo.ID)
	if _, err := db.ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
		t.Fatal(err)
	}
	// Delete repo
	if err := db.Repos().Delete(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	// Check if name is updated to DELETED-...
	repos, err := db.Repos().List(ctx, ReposListOptions{
		IDs:            []api.RepoID{repo.ID},
		IncludeDeleted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("want one repo with given ID, got %v", repos)
	}
	if got := string(repos[0].Name); !strings.HasPrefix(got, "DELETED-") {
		t.Errorf("deleted repo name, got %q, want \"DELETED-..\"", got)
	}
	if got, want := repos[0].DeletedAt, repo.DeletedAt; got != want {
		t.Errorf("deleted_at seems unexpectedly updated, got %s want %s", got, want)
	}
}

func TestRepos_MultipleDeletesKeepTheSameTombstoneData(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})
	repo := mustCreate(ctx, t, db, &types.Repo{Name: "myrepo"})
	// Delete once.
	if err := db.Repos().Delete(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	repos, err := db.Repos().List(ctx, ReposListOptions{
		IDs:            []api.RepoID{repo.ID},
		IncludeDeleted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("want one repo with given ID, got %v", repos)
	}
	afterFirstDelete := repos[0]
	// Delete again
	if err := db.Repos().Delete(ctx, repo.ID); err != nil {
		t.Fatal(err)
	}
	repos, err = db.Repos().List(ctx, ReposListOptions{
		IDs:            []api.RepoID{repo.ID},
		IncludeDeleted: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("want one repo with given ID, got %v", repos)
	}
	afterSecondDelete := repos[0]
	// Check if tombstone data - deleted_at and name are the same.
	if got, want := afterSecondDelete.Name, afterFirstDelete.Name; got != want {
		t.Errorf("name: got %q want %q", got, want)
	}
	if got, want := afterSecondDelete.DeletedAt, afterFirstDelete.DeletedAt; got != want {
		t.Errorf("deleted_at, got %v want %v", got, want)
	}
}

func TestRepos_Upsert(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})

	if _, err := db.Repos().GetByName(ctx, "myrepo"); !errcode.IsNotFound(err) {
		if err == nil {
			t.Fatal("myrepo already present")
		} else {
			t.Fatal(err)
		}
	}

	if err := upsertRepo(ctx, db, InsertRepoOp{Name: "myrepo", Description: "", Fork: false}); err != nil {
		t.Fatal(err)
	}

	rp, err := db.Repos().GetByName(ctx, "myrepo")
	if err != nil {
		t.Fatal(err)
	}

	if rp.Name != "myrepo" {
		t.Fatalf("rp.Name: %s != %s", rp.Name, "myrepo")
	}

	ext := api.ExternalRepoSpec{
		ID:          "ext:id",
		ServiceType: "test",
		ServiceID:   "ext:test",
	}

	if err := upsertRepo(ctx, db, InsertRepoOp{Name: "myrepo", Description: "asdfasdf", Fork: false, ExternalRepo: ext}); err != nil {
		t.Fatal(err)
	}

	rp, err = db.Repos().GetByName(ctx, "myrepo")
	if err != nil {
		t.Fatal(err)
	}

	if rp.Name != "myrepo" {
		t.Fatalf("rp.Name: %s != %s", rp.Name, "myrepo")
	}
	if rp.Description != "asdfasdf" {
		t.Fatalf("rp.Name: %q != %q", rp.Description, "asdfasdf")
	}
	if !reflect.DeepEqual(rp.ExternalRepo, ext) {
		t.Fatalf("rp.ExternalRepo: %s != %s", rp.ExternalRepo, ext)
	}

	// Rename. Detected by external repo
	if err := upsertRepo(ctx, db, InsertRepoOp{Name: "myrepo/renamed", Description: "asdfasdf", Fork: false, ExternalRepo: ext}); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Repos().GetByName(ctx, "myrepo"); !errcode.IsNotFound(err) {
		if err == nil {
			t.Fatal("myrepo should be renamed, but still present as myrepo")
		} else {
			t.Fatal(err)
		}
	}

	rp, err = db.Repos().GetByName(ctx, "myrepo/renamed")
	if err != nil {
		t.Fatal(err)
	}
	if rp.Name != "myrepo/renamed" {
		t.Fatalf("rp.Name: %s != %s", rp.Name, "myrepo/renamed")
	}
	if rp.Description != "asdfasdf" {
		t.Fatalf("rp.Name: %q != %q", rp.Description, "asdfasdf")
	}
	if !reflect.DeepEqual(rp.ExternalRepo, ext) {
		t.Fatalf("rp.ExternalRepo: %s != %s", rp.ExternalRepo, ext)
	}
}

func TestRepos_UpsertForkAndArchivedFields(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})

	i := 0
	for _, fork := range []bool{true, false} {
		for _, archived := range []bool{true, false} {
			i++
			name := api.RepoName(fmt.Sprintf("myrepo-%d", i))

			if err := upsertRepo(ctx, db, InsertRepoOp{Name: name, Fork: fork, Archived: archived}); err != nil {
				t.Fatal(err)
			}

			rp, err := db.Repos().GetByName(ctx, name)
			if err != nil {
				t.Fatal(err)
			}

			if rp.Fork != fork {
				t.Fatalf("rp.Fork: %v != %v", rp.Fork, fork)
			}
			if rp.Archived != archived {
				t.Fatalf("rp.Archived: %v != %v", rp.Archived, archived)
			}
		}
	}
}

func TestRepos_Create(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})

	svcs := typestest.MakeExternalServices()
	if err := db.ExternalServices().Upsert(ctx, svcs...); err != nil {
		t.Fatalf("Upsert error: %s", err)
	}

	msvcs := typestest.ExternalServicesToMap(svcs)

	repo1 := typestest.MakeGithubRepo(msvcs[extsvc.KindGitHub], msvcs[extsvc.KindBitbucketServer])
	repo2 := typestest.MakeGitlabRepo(msvcs[extsvc.KindGitLab])

	t.Run("no repos should not fail", func(t *testing.T) {
		if err := db.Repos().Create(ctx); err != nil {
			t.Fatalf("Create error: %s", err)
		}
	})

	t.Run("many repos", func(t *testing.T) {
		want := typestest.GenerateRepos(7, repo1, repo2)

		if err := db.Repos().Create(ctx, want...); err != nil {
			t.Fatalf("Create error: %s", err)
		}

		sort.Sort(want)

		if noID := want.Filter(func(r *types.Repo) bool { return r.ID == 0 }); len(noID) > 0 {
			t.Fatalf("Create didn't assign an ID to all repos: %v", noID.Names())
		}

		have, err := db.Repos().List(ctx, ReposListOptions{})
		if err != nil {
			t.Fatalf("List error: %s", err)
		}

		if diff := cmp.Diff(have, []*types.Repo(want), cmpopts.EquateEmpty()); diff != "" {
			t.Fatalf("List:\n%s", diff)
		}
	})
}

func TestListSourcegraphDotComIndexableRepos(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	reposToAdd := []types.Repo{
		{
			ID:    api.RepoID(1),
			Name:  "github.com/foo/bar1",
			Stars: 20,
		},
		{
			ID:    api.RepoID(2),
			Name:  "github.com/baz/bar2",
			Stars: 30,
		},
		{
			ID:      api.RepoID(3),
			Name:    "github.com/baz/bar3",
			Stars:   15,
			Private: true,
		},
		{
			ID:    api.RepoID(4),
			Name:  "github.com/foo/bar4",
			Stars: 1, // Not enough stars
		},
		{
			ID:    api.RepoID(5),
			Name:  "github.com/foo/bar5",
			Stars: 400,
			Blocked: &types.RepoBlock{
				At:     time.Now().UTC().Unix(),
				Reason: "Failed to index too many times.",
			},
		},
	}

	ctx := context.Background()
	// Add an external service
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO external_services(id, kind, display_name, config, cloud_default) VALUES (1, 'github', 'github', '{}', true);`,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range reposToAdd {
		blocked, err := json.Marshal(r.Blocked)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.ExecContext(ctx,
			`INSERT INTO repo(id, name, stars, private, blocked) VALUES ($1, $2, $3, $4, NULLIF($5, 'null'::jsonb))`,
			r.ID, r.Name, r.Stars, r.Private, blocked,
		)
		if err != nil {
			t.Fatal(err)
		}

		if r.Private {
			if _, err := db.ExecContext(ctx, `INSERT INTO external_service_repos VALUES (1, $1, $2);`, r.ID, r.Name); err != nil {
				t.Fatal(err)
			}
		}

		cloned := int(r.ID) > 1
		cloneStatus := types.CloneStatusCloned
		if !cloned {
			cloneStatus = types.CloneStatusNotCloned
		}
		if _, err := db.ExecContext(ctx, `UPDATE gitserver_repos SET clone_status = $2, shard_id = 'test' WHERE repo_id = $1;`, r.ID, cloneStatus); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name string
		opts ListSourcegraphDotComIndexableReposOptions
		want []api.RepoID
	}{
		{
			name: "no opts",
			want: []api.RepoID{2, 1, 3},
		},
		{
			name: "only uncloned",
			opts: ListSourcegraphDotComIndexableReposOptions{CloneStatus: types.CloneStatusNotCloned},
			want: []api.RepoID{1},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repos, err := db.Repos().ListSourcegraphDotComIndexableRepos(ctx, tc.opts)
			if err != nil {
				t.Fatal(err)
			}

			have := make([]api.RepoID, 0, len(repos))
			for _, r := range repos {
				have = append(have, r.ID)
			}

			if diff := cmp.Diff(tc.want, have, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("mismatch (-want +have):\n%s", diff)
			}
		})
	}
}

func TestRepoStore_Metadata(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	ctx := context.Background()

	repos := []*types.Repo{
		{
			ID:          1,
			Name:        "foo",
			Description: "foo 1",
			Fork:        false,
			Archived:    false,
			Private:     false,
			Stars:       10,
			URI:         "foo-uri",
			Sources:     map[string]*types.SourceInfo{},
		},
		{
			ID:          2,
			Name:        "bar",
			Description: "bar 2",
			Fork:        true,
			Archived:    true,
			Private:     true,
			Stars:       20,
			URI:         "bar-uri",
			Sources:     map[string]*types.SourceInfo{},
		},
	}

	r := db.Repos()
	require.NoError(t, r.Create(ctx, repos...))

	d1 := time.Unix(1627945150, 0).UTC()
	d2 := time.Unix(1628945150, 0).UTC()
	gitserverRepos := []*types.GitserverRepo{
		{
			RepoID:      1,
			LastFetched: d1,
			ShardID:     "abc",
		},
		{
			RepoID:      2,
			LastFetched: d2,
			ShardID:     "abc",
		},
	}

	gr := db.GitserverRepos()
	require.NoError(t, gr.Update(ctx, gitserverRepos...))

	expected := []*types.SearchedRepo{
		{
			ID:          1,
			Name:        "foo",
			Description: "foo 1",
			Fork:        false,
			Archived:    false,
			Private:     false,
			Stars:       10,
			LastFetched: &d1,
		},
		{
			ID:          2,
			Name:        "bar",
			Description: "bar 2",
			Fork:        true,
			Archived:    true,
			Private:     true,
			Stars:       20,
			LastFetched: &d2,
		},
	}

	md, err := r.Metadata(ctx, 1, 2)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, md)
}
