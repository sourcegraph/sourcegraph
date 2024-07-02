package backend

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"strconv"

	"golang.org/x/sync/semaphore"

	"github.com/sourcegraph/sourcegraph/internal/trace"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/inventory"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Feature flag for enhanced (but much slower) language detection that uses file contents, not just
// filenames. Enabled by default.
var useEnhancedLanguageDetection, _ = strconv.ParseBool(env.Get("USE_ENHANCED_LANGUAGE_DETECTION", "true", "Enable more accurate but slower language detection that uses file contents"))

var inventoryCache = rcache.New(fmt.Sprintf("inv:v2:enhanced_%v", useEnhancedLanguageDetection))

var gitServerConcurrency, _ = strconv.Atoi(env.Get("GET_INVENTORY_GIT_SERVER_CONCURRENCY", "4", "Changes the number of concurrent requests against the gitserver for getInventory requests."))

// Raising this value to 50 or higher lead to the following error on my dev machine
// lvl=warn msg="failed to execute redis command" cmd=GET error="dial tcp *********:6379: connect: can't assign requested address"
var redisConcurrency, _ = strconv.Atoi(env.Get("GET_INVENTORY_REDIS_CONCURRENCY", "20", "Changes the number of concurrent requests against the redis cache for getInventory requests."))

type semaphoredReadCloser struct {
	io.ReadCloser
	releaseSemaphore func()
}

func (s *semaphoredReadCloser) Close() error {
	defer s.releaseSemaphore()
	return s.ReadCloser.Close()
}

// InventoryContext returns the inventory context for computing the inventory for the repository at
// the given commit.
func InventoryContext(logger log.Logger, repo api.RepoName, gsClient gitserver.Client, commitID api.CommitID, forceEnhancedLanguageDetection bool) (inventory.Context, error) {
	if !gitdomain.IsAbsoluteRevision(string(commitID)) {
		return inventory.Context{}, errors.Errorf("refusing to compute inventory for non-absolute commit ID %q", commitID)
	}

	gitServerSemaphore := semaphore.NewWeighted(int64(gitServerConcurrency))
	redisSemaphore := semaphore.NewWeighted(int64(redisConcurrency))

	logger = logger.Scoped("InventoryContext").
		With(log.String("repo", string(repo)), log.String("commitID", string(commitID)))
	invCtx := inventory.Context{
		Repo: repo,
		ReadTree: func(ctx context.Context, path string) ([]fs.FileInfo, error) {
			trc, ctx := trace.New(ctx, "ReadTree waits for semaphore")
			err := gitServerSemaphore.Acquire(ctx, 1)
			trc.End()
			if err != nil {
				return nil, err
			}
			defer gitServerSemaphore.Release(1)
			// Using recurse=true does not yield a significant performance improvement. See https://github.com/sourcegraph/sourcegraph/pull/62011/files#r1577513913.

			fds := make([]fs.FileInfo, 0)
			it, err := gsClient.ReadDir(ctx, repo, commitID, path, false)
			if err != nil {
				return nil, err
			}
			defer it.Close()
			for {
				fd, err := it.Next()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return nil, err
				}
				fds = append(fds, fd)
			}
			return fds, nil
		},
		NewFileReader: func(ctx context.Context, path string) (io.ReadCloser, error) {
			trc, ctx := trace.New(ctx, "NewFileReader waits for semaphore")
			err := gitServerSemaphore.Acquire(ctx, 1)
			trc.End()
			if err != nil {
				return nil, err
			}
			reader, err := gsClient.NewFileReader(ctx, repo, commitID, path)
			if err != nil {
				return nil, err
			}
			return &semaphoredReadCloser{ReadCloser: reader, releaseSemaphore: func() {
				gitServerSemaphore.Release(1)
			}}, nil
		},
		CacheKey: func(e fs.FileInfo) string {
			info, ok := e.Sys().(gitdomain.ObjectInfo)
			if !ok {
				return "" // not cacheable
			}
			return info.OID().String()
		},
		CacheGet: func(ctx context.Context, cacheKey string) (inventory.Inventory, bool) {
			if cacheKey == "" {
				return inventory.Inventory{}, false // not cacheable
			}

			if err := redisSemaphore.Acquire(ctx, 1); err != nil {
				logger.Warn("Failed to acquire semaphore for redis cache.", log.String("cacheKey", cacheKey), log.Error(err))
				return inventory.Inventory{}, false
			}
			defer redisSemaphore.Release(1)

			if b, ok := inventoryCache.Get(cacheKey); ok {
				var inv inventory.Inventory
				if err := json.Unmarshal(b, &inv); err != nil {
					logger.Warn("Failed to unmarshal cached JSON inventory.", log.String("cacheKey", cacheKey), log.Error(err))
					return inventory.Inventory{}, false
				}
				return inv, true
			}
			return inventory.Inventory{}, false
		},
		CacheSet: func(ctx context.Context, cacheKey string, inv inventory.Inventory) {
			if cacheKey == "" {
				return // not cacheable
			}
			b, err := json.Marshal(&inv)
			if err != nil {
				logger.Warn("Failed to marshal JSON inventory for cache.", log.String("cacheKey", cacheKey), log.Error(err))
				return
			}

			if err := redisSemaphore.Acquire(ctx, 1); err != nil {
				logger.Warn("Failed to acquire semaphore for redis cache.", log.String("cacheKey", cacheKey), log.Error(err))
				return
			}
			defer redisSemaphore.Release(1)
			inventoryCache.Set(cacheKey, b)
		},
		NewTarReader: func(reader io.ReadCloser) *tar.Reader {
			return tar.NewReader(reader)
		},
		ShouldSkipEnhancedLanguageDetection: !useEnhancedLanguageDetection && !forceEnhancedLanguageDetection,
		GitServerClient:                     gsClient,
		CommitID:                            commitID,
	}

	return invCtx, nil
}
