package discovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hexops/autogold/v2"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// TestAllReposIterator tests the AllReposIterator in the common use cases.
func TestAllReposIterator(t *testing.T) {
	ctx := context.Background()
	repoStore := NewMockRepoStore()
	var timeOffset time.Duration
	clock := func() time.Time { return time.Now().Add(timeOffset) }

	// Mock the repo store listing, and confirm calls to it are cached.
	var (
		repoStoreListCalls []database.ReposListOptions
		nextRepoID         api.RepoID
	)
	repoStore.ListFunc.SetDefaultHook(func(ctx context.Context, opt database.ReposListOptions) ([]*types.Repo, error) {
		repoStoreListCalls = append(repoStoreListCalls, opt)
		var result []*types.Repo
		for range 3 {
			nextRepoID++
			result = append(result, &types.Repo{ID: nextRepoID, Name: api.RepoName(fmt.Sprint(nextRepoID))})
		}
		if nextRepoID > 10 {
			return nil, nil
		}
		return result, nil
	})

	iter := NewAllReposIterator(repoStore, clock, false, 15*time.Minute, &prometheus.CounterOpts{Name: "fake_name123"})
	{
		// Do we get all 9 repositories?
		var each []string
		iter.ForEach(ctx, func(repoName string, id api.RepoID) error {
			each = append(each, repoName)
			return nil
		})
		autogold.Expect([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}).Equal(t, each)
	}

	// Were the RepoStore.List calls as we expected?
	autogold.Expect([]database.ReposListOptions{
		{
			LimitOffset: &database.LimitOffset{Limit: 1000},
		},
		{
			LimitOffset: &database.LimitOffset{
				Limit:  1000,
				Offset: 3,
			},
		},
		{
			LimitOffset: &database.LimitOffset{
				Limit:  1000,
				Offset: 6,
			},
		},
		{
			LimitOffset: &database.LimitOffset{
				Limit:  1000,
				Offset: 9,
			},
		},
	}).Equal(t, repoStoreListCalls)

	// Again: do we get all 9 repositories, but this time all RepoStore.List calls were cached?
	repoStoreListCalls = nil
	nextRepoID = 0
	{
		var each []string
		iter.ForEach(ctx, func(repoName string, id api.RepoID) error {
			each = append(each, repoName)
			return nil
		})
		autogold.Expect([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}).Equal(t, each)
		autogold.Expect([]database.ReposListOptions{}).Equal(t, repoStoreListCalls)
	}

	// If the clock moves forward, does the cache expire and new RepoStore.List calls are made?
	timeOffset += iter.RepositoryListCacheTime
	repoStoreListCalls = nil
	nextRepoID = 0
	{
		var each []string
		iter.ForEach(ctx, func(repoName string, id api.RepoID) error {
			each = append(each, repoName)
			return nil
		})
		autogold.Expect([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}).Equal(t, each)
		autogold.Expect([]database.ReposListOptions{
			{
				LimitOffset: &database.LimitOffset{Limit: 1000},
			},
			{
				LimitOffset: &database.LimitOffset{
					Limit:  1000,
					Offset: 3,
				},
			},
			{
				LimitOffset: &database.LimitOffset{
					Limit:  1000,
					Offset: 6,
				},
			},
			{
				LimitOffset: &database.LimitOffset{
					Limit:  1000,
					Offset: 9,
				},
			},
		}).Equal(t, repoStoreListCalls)
	}
}
