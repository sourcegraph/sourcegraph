package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/zoekt"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestZoektRepos_GetZoektRepo(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &zoektReposStore{Store: basestore.NewWithHandle(db.Handle())}

	repo1, _ := createTestRepo(ctx, t, db, "repo1")
	repo2, _ := createTestRepo(ctx, t, db, "repo2")
	repo3, _ := createTestRepo(ctx, t, db, "repo3")

	assertZoektRepos(t, ctx, s, map[api.RepoID]*ZoektRepo{
		repo1.ID: {RepoID: repo1.ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
		repo2.ID: {RepoID: repo2.ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
		repo3.ID: {RepoID: repo3.ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
	})
}

func TestZoektRepos_UpdateIndexStatuses(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &zoektReposStore{Store: basestore.NewWithHandle(db.Handle())}
	timeUnix := int64(1686763487)

	var repos types.MinimalRepos
	for _, name := range []api.RepoName{
		"repo1",
		"repo2",
		"repo3",
	} {
		r, _ := createTestRepo(ctx, t, db, name)
		repos = append(repos, types.MinimalRepo{ID: r.ID, Name: r.Name})
	}

	// No repo is indexed
	assertZoektRepoStatistics(t, ctx, s, ZoektRepoStatistics{Total: 3, NotIndexed: 3})

	assertZoektRepos(t, ctx, s, map[api.RepoID]*ZoektRepo{
		repos[0].ID: {RepoID: repos[0].ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
		repos[1].ID: {RepoID: repos[1].ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
		repos[2].ID: {RepoID: repos[2].ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
	})

	// 1/3 repo is indexed
	indexed := zoekt.ReposMap{
		uint32(repos[0].ID): {
			Branches:      []zoekt.RepositoryBranch{{Name: "main", Version: "d34db33f"}},
			IndexTimeUnix: timeUnix,
		},
	}

	if err := s.UpdateIndexStatuses(ctx, indexed); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	assertZoektRepoStatistics(t, ctx, s, ZoektRepoStatistics{Total: 3, Indexed: 1, NotIndexed: 2})

	assertZoektRepos(t, ctx, s, map[api.RepoID]*ZoektRepo{
		repos[0].ID: {
			RepoID:        repos[0].ID,
			IndexStatus:   "indexed",
			Branches:      []zoekt.RepositoryBranch{{Name: "main", Version: "d34db33f"}},
			LastIndexedAt: time.Unix(timeUnix, 0),
		},
		repos[1].ID: {RepoID: repos[1].ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
		repos[2].ID: {RepoID: repos[2].ID, IndexStatus: "not_indexed", Branches: []zoekt.RepositoryBranch{}},
	})

	// Index all repositories
	indexed = zoekt.ReposMap{
		// different commit
		uint32(repos[0].ID): {Branches: []zoekt.RepositoryBranch{{Name: "main", Version: "f00b4r"}}},
		// new
		uint32(repos[1].ID): {Branches: []zoekt.RepositoryBranch{{Name: "main-2", Version: "b4rf00"}}},
		// new
		uint32(repos[2].ID): {Branches: []zoekt.RepositoryBranch{{Name: "main", Version: "d00d00"}}},
	}

	if err := s.UpdateIndexStatuses(ctx, indexed); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	assertZoektRepoStatistics(t, ctx, s, ZoektRepoStatistics{Total: 3, Indexed: 3})

	assertZoektRepos(t, ctx, s, map[api.RepoID]*ZoektRepo{
		repos[0].ID: {
			RepoID:      repos[0].ID,
			IndexStatus: "indexed",
			Branches:    []zoekt.RepositoryBranch{{Name: "main", Version: "f00b4r"}},
		},
		repos[1].ID: {
			RepoID:      repos[1].ID,
			IndexStatus: "indexed",
			Branches:    []zoekt.RepositoryBranch{{Name: "main-2", Version: "b4rf00"}},
		},
		repos[2].ID: {
			RepoID:      repos[2].ID,
			IndexStatus: "indexed",
			Branches:    []zoekt.RepositoryBranch{{Name: "main", Version: "d00d00"}},
		},
	})

	// Add an additional branch to a single repository
	indexed = zoekt.ReposMap{
		// additional branch
		uint32(repos[2].ID): {Branches: []zoekt.RepositoryBranch{
			{Name: "main", Version: "d00d00"},
			{Name: "v15.3.1", Version: "b4rf00"},
		}},
	}

	if err := s.UpdateIndexStatuses(ctx, indexed); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	wantZoektRepos := map[api.RepoID]*ZoektRepo{
		repos[0].ID: {
			RepoID:      repos[0].ID,
			IndexStatus: "indexed",
			Branches:    []zoekt.RepositoryBranch{{Name: "main", Version: "f00b4r"}},
		},
		repos[1].ID: {
			RepoID:      repos[1].ID,
			IndexStatus: "indexed",
			Branches:    []zoekt.RepositoryBranch{{Name: "main-2", Version: "b4rf00"}},
		},
		repos[2].ID: {
			RepoID:      repos[2].ID,
			IndexStatus: "indexed",
			Branches: []zoekt.RepositoryBranch{
				{Name: "main", Version: "d00d00"},
				{Name: "v15.3.1", Version: "b4rf00"},
			},
		},
	}
	assertZoektRepos(t, ctx, s, wantZoektRepos)

	// Now we update the indexing status of a repository that doesn't exist and
	// check that the index status in unchanged:
	indexed = zoekt.ReposMap{
		9999: {Branches: []zoekt.RepositoryBranch{{Name: "main", Version: "d00d00"}}},
	}
	if err := s.UpdateIndexStatuses(ctx, indexed); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Should still be the same
	assertZoektRepos(t, ctx, s, wantZoektRepos)
}

func assertZoektRepoStatistics(t *testing.T, ctx context.Context, s *zoektReposStore, wantZoektStats ZoektRepoStatistics) {
	t.Helper()

	stats, err := s.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("zoektRepoStore.GetStatistics failed: %s", err)
	}

	if diff := cmp.Diff(stats, wantZoektStats); diff != "" {
		t.Errorf("ZoektRepoStatistics differ: %s", diff)
	}
}

func assertZoektRepos(t *testing.T, ctx context.Context, s *zoektReposStore, want map[api.RepoID]*ZoektRepo) {
	t.Helper()

	for repoID, w := range want {
		have, err := s.GetZoektRepo(ctx, repoID)
		if err != nil {
			t.Fatalf("unexpected error from GetZoektRepo: %s", err)
		}

		assert.NotZero(t, have.UpdatedAt)
		assert.NotZero(t, have.CreatedAt)

		w.UpdatedAt = have.UpdatedAt
		w.CreatedAt = have.CreatedAt

		if diff := cmp.Diff(have, w); diff != "" {
			t.Errorf("ZoektRepo for repo %d differs: %s", repoID, diff)
		}
	}
}

func benchmarkUpdateIndexStatus(b *testing.B, numRepos int) {
	logger := logtest.Scoped(b)
	db := NewDB(logger, dbtest.NewDB(b))
	ctx := context.Background()
	s := &zoektReposStore{Store: basestore.NewWithHandle(db.Handle())}

	b.Logf("Creating %d repositories...", numRepos)

	var (
		indexedAll         = make(zoekt.ReposMap, numRepos)
		indexedAllBranches = []zoekt.RepositoryBranch{{Name: "main", Version: "d00d00"}}

		indexedHalf         = make(zoekt.ReposMap, numRepos/2)
		indexedHalfBranches = []zoekt.RepositoryBranch{{Name: "main-2", Version: "f00b4r"}}
	)

	inserter := batch.NewInserter(ctx, db.Handle(), "repo", batch.MaxNumPostgresParameters, "name")
	for i := range numRepos {
		if err := inserter.Insert(ctx, fmt.Sprintf("repo-%d", i)); err != nil {
			b.Fatal(err)
		}

		indexedAll[uint32(i+1)] = zoekt.MinimalRepoListEntry{Branches: indexedAllBranches}
		if i%2 == 0 {
			indexedHalf[uint32(i+1)] = zoekt.MinimalRepoListEntry{Branches: indexedHalfBranches}
		}
	}
	if err := inserter.Flush(ctx); err != nil {
		b.Fatal(err)
	}

	b.Logf("Done creating %d repositories.", numRepos)
	b.ResetTimer()

	b.Run("update-all", func(b *testing.B) {
		for range b.N {
			if err := s.UpdateIndexStatuses(ctx, indexedAll); err != nil {
				b.Fatalf("unexpected error: %s", err)
			}
		}
	})

	b.Run("update-half", func(b *testing.B) {
		for range b.N {
			if err := s.UpdateIndexStatuses(ctx, indexedHalf); err != nil {
				b.Fatalf("unexpected error: %s", err)
			}
		}
	})

	b.Run("update-none", func(b *testing.B) {
		for range b.N {
			if err := s.UpdateIndexStatuses(ctx, make(zoekt.ReposMap)); err != nil {
				b.Fatalf("unexpected error: %s", err)
			}
		}
	})
}

// 21 Oct 2022 - MacBook Pro M1 Max
//
// φ go test -v -timeout=900s -run=XXX -benchtime=10s -bench BenchmarkZoektRepos ./internal/database
// goos: darwin
// goarch: arm64
// pkg: github.com/sourcegraph/sourcegraph/internal/database
// BenchmarkZoektRepos_UpdateIndexStatus_10000/update-all-10                   1102          16114459 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_10000/update-half-10                   848          15444057 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_10000/update-none-10                  5642           2446603 ns/op
//
// BenchmarkZoektRepos_UpdateIndexStatus_50000/update-all-10                     36         328577991 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_50000/update-half-10                    58         200992639 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_50000/update-none-10                  5430           2369568 ns/op
//
// BenchmarkZoektRepos_UpdateIndexStatus_100000/update-all-10                    19         611171364 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_100000/update-half-10                   32         360921643 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_100000/update-none-10                 5775           2299364 ns/op
//
// BenchmarkZoektRepos_UpdateIndexStatus_200000/update-all-10                     9        1193084662 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_200000/update-half-10                   16         674584125 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_200000/update-none-10                 5733           2170722 ns/op
//
// BenchmarkZoektRepos_UpdateIndexStatus_500000/update-all-10                     4        2885609312 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_500000/update-half-10                    7        1648433833 ns/op
// BenchmarkZoektRepos_UpdateIndexStatus_500000/update-none-10                 5858           2377811 ns/op

func BenchmarkZoektRepos_UpdateIndexStatus_10000(b *testing.B) {
	benchmarkUpdateIndexStatus(b, 10_000)
}

func BenchmarkZoektRepos_UpdateIndexStatus_50000(b *testing.B) {
	benchmarkUpdateIndexStatus(b, 50_000)
}

func BenchmarkZoektRepos_UpdateIndexStatus_100000(b *testing.B) {
	benchmarkUpdateIndexStatus(b, 100_000)
}

func BenchmarkZoektRepos_UpdateIndexStatus_200000(b *testing.B) {
	benchmarkUpdateIndexStatus(b, 200_000)
}

func BenchmarkZoektRepos_UpdateIndexStatus_500000(b *testing.B) {
	benchmarkUpdateIndexStatus(b, 500_000)
}
