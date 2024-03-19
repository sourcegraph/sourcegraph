package shared

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/embeddings"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestGetCachedRepoEmbeddingIndex(t *testing.T) {
	mockRepoEmbeddingJobsStore := repo.NewMockRepoEmbeddingJobsStore()
	mockRepoStore := dbmocks.NewMockRepoStore()

	mockRepoStore.GetByNameFunc.SetDefaultHook(func(ctx context.Context, name api.RepoName) (*types.Repo, error) { return &types.Repo{ID: 1}, nil })

	finishedAt := time.Now()
	mockRepoEmbeddingJobsStore.GetLastCompletedRepoEmbeddingJobFunc.SetDefaultHook(func(ctx context.Context, id api.RepoID) (*repo.RepoEmbeddingJob, error) {
		return &repo.RepoEmbeddingJob{FinishedAt: &finishedAt}, nil
	})

	cacheSize := 10 * 1024 * 1024
	hasDownloadedRepoEmbeddingIndex := false
	downloadLargeCount := 0
	indexGetter, err := NewCachedEmbeddingIndexGetter(
		mockRepoStore,
		mockRepoEmbeddingJobsStore,
		func(ctx context.Context, repoID api.RepoID, _ api.RepoName) (*embeddings.RepoEmbeddingIndex, error) {
			switch repoID {
			case 7:
				hasDownloadedRepoEmbeddingIndex = true
				return &embeddings.RepoEmbeddingIndex{}, nil
			default:
				downloadLargeCount += 1
				return &embeddings.RepoEmbeddingIndex{
					CodeIndex: embeddings.EmbeddingIndex{
						Embeddings: make([]int8, cacheSize*10), // too large to fit in cache
					},
				}, nil
			}
		},
		uint64(cacheSize),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	repo := types.Repo{
		Name: "a",
		ID:   7,
	}

	tooLarge := types.Repo{
		Name: "tooLarge",
		ID:   42,
	}

	// Initial request should download and cache the index.
	_, err = indexGetter.Get(ctx, repo.ID, repo.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDownloadedRepoEmbeddingIndex {
		t.Fatal("expected to download the index on initial request")
	}

	// Subsequent requests should read from the cache.
	hasDownloadedRepoEmbeddingIndex = false
	_, err = indexGetter.Get(ctx, repo.ID, repo.Name)
	if err != nil {
		t.Fatal(err)
	}
	if hasDownloadedRepoEmbeddingIndex {
		t.Fatal("expected to not download the index on subsequent request")
	}

	// Simulate a newer completed repo embedding job.
	finishedAt = finishedAt.Add(time.Hour)
	_, err = indexGetter.Get(ctx, repo.ID, repo.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !hasDownloadedRepoEmbeddingIndex {
		t.Fatal("expected to download the index after a newer embedding job is completed")
	}

	_, err = indexGetter.Get(ctx, tooLarge.ID, tooLarge.Name)
	require.NoError(t, err)
	require.Equal(t, 1, downloadLargeCount)

	// Fetching a second time should trigger a second download since it's
	// too large to fit in the cache
	_, err = indexGetter.Get(ctx, tooLarge.ID, tooLarge.Name)
	require.NoError(t, err)
	require.Equal(t, 2, downloadLargeCount)
}

func Test_embeddingsIndexCache(t *testing.T) {
	entryWithSize := func(size int) repoEmbeddingIndexCacheEntry {
		return repoEmbeddingIndexCacheEntry{
			index: &embeddings.RepoEmbeddingIndex{
				CodeIndex: embeddings.EmbeddingIndex{
					Embeddings: make([]int8, size),
				},
			},
		}
	}

	c, err := newEmbeddingsIndexCache(1024)
	require.NoError(t, err)

	tooBig := entryWithSize(2048)
	fitsOne := entryWithSize(700)

	c.Add("a", tooBig)
	_, ok := c.Get("a")
	require.False(t, ok, "a cache entry that is too large should enter the cache")

	c.Add("fitsOne", fitsOne)
	_, ok = c.Get("fitsOne")
	require.True(t, ok, "a cache entry that fits should always get added to the cache")

	c.Add("fitsOneAgain", fitsOne)
	_, ok = c.Get("fitsOneAgain")
	require.True(t, ok, "a cache entry should evict other cache entries until it fits")

	_, ok = c.Get("fitsOne")
	require.False(t, ok, "after being evicted, a cache entry should not exist")
}

func TestConcurrentGetCachedRepoEmbeddingIndex(t *testing.T) {
	t.Parallel()

	mockRepoEmbeddingJobsStore := repo.NewMockRepoEmbeddingJobsStore()
	mockRepoStore := dbmocks.NewMockRepoStore()

	mockRepoStore.GetByNameFunc.SetDefaultHook(func(ctx context.Context, name api.RepoName) (*types.Repo, error) { return &types.Repo{ID: 1}, nil })

	finishedAt := time.Now()
	mockRepoEmbeddingJobsStore.GetLastCompletedRepoEmbeddingJobFunc.SetDefaultHook(func(ctx context.Context, id api.RepoID) (*repo.RepoEmbeddingJob, error) {
		return &repo.RepoEmbeddingJob{FinishedAt: &finishedAt}, nil
	})

	var mu sync.Mutex
	hasDownloadedRepoEmbeddingIndex := false
	indexGetter, err := NewCachedEmbeddingIndexGetter(
		mockRepoStore,
		mockRepoEmbeddingJobsStore,
		func(ctx context.Context, _ api.RepoID, _ api.RepoName) (*embeddings.RepoEmbeddingIndex, error) {
			mu.Lock()
			defer mu.Unlock()

			if hasDownloadedRepoEmbeddingIndex {
				t.Fatal("index already downloaded")
			}
			hasDownloadedRepoEmbeddingIndex = true
			// Simulate the download time.
			time.Sleep(time.Millisecond * 500)
			return &embeddings.RepoEmbeddingIndex{}, nil
		},
		10*1024*1024,
	)
	if err != nil {
		t.Fatal(err)
	}

	repo := types.Repo{
		Name: "a",
		ID:   7,
	}

	numRequests := 4
	var wg sync.WaitGroup
	wg.Add(numRequests)
	for range numRequests {
		ctx := context.Background()
		go func() {
			defer wg.Done()
			indexGetter.Get(ctx, repo.ID, repo.Name)
		}()
	}
	wg.Wait()
}
