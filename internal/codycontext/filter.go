package codycontext

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/zoekt/ignore"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

const codyIgnoreFile = ".cody/ignore"

var (
	emptyMatcher ignore.Matcher = ignore.Matcher{}
)

type repoRevision struct {
	Repo   types.RepoIDName
	Commit api.CommitID
}

type filterFunc func(string) bool
type FileChunkFilterFunc func([]FileChunkContext) []FileChunkContext
type safeCache struct {
	cache *lru.Cache[repoRevision, ignore.Matcher]
}

func (c *safeCache) Add(key repoRevision, value ignore.Matcher) (evicted bool) {
	if c.cache != nil {
		return c.cache.Add(key, value)
	}
	return false
}

func (c *safeCache) Get(key repoRevision) (ignore.Matcher, bool) {
	if c.cache != nil {
		return c.cache.Get(key)

	}
	return ignore.Matcher{}, false
}

type repoFilter struct {
	cache  safeCache
	client gitserver.Client

	mu      sync.RWMutex
	enabled bool
}

func (f *repoFilter) SetEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled = enabled
}

func (f *repoFilter) GetEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

func (f *repoFilter) GetFilter(repos []types.RepoIDName, logger log.Logger) ([]types.RepoIDName, FileChunkFilterFunc) {
	if !f.GetEnabled() {
		return repos, func(fcc []FileChunkContext) []FileChunkContext {
			return fcc
		}
	}
	return f.getFilter(repos, logger)
}

// getFilter returns the list of repos that can be filtered
// their .cody/ignore files (or don't have one). If an error
// occurs that repo will be excluded.
func (f *repoFilter) getFilter(repos []types.RepoIDName, logger log.Logger) ([]types.RepoIDName, FileChunkFilterFunc) {
	filters := make(map[api.RepoName]filterFunc, len(repos))
	filterableRepos := make([]types.RepoIDName, 0, len(repos))
	// use the internal actor to ensure access to repo and ignore files
	ctx := actor.WithInternalActor(context.Background())
	for _, repo := range repos {

		_, commit, err := f.client.GetDefaultBranch(ctx, repo.ID, true)
		if err != nil {
			logger.Warn("repoContextFilter: couldn't get default branch, removing repo", log.Int32("repo", int32(repo.ID)), log.Error(err))
			continue
		}
		// No commit signals an empty repo, should be nothing to filter
		// Also we can't lookup the ignore file without a commit
		if commit == "" {
			logger.Info("repoContextFilter: empty repo removing", log.Int32("repo", int32(repo.ID)))
			continue
		}
		matcher, err := getIgnoreMatcher(ctx, f.cache, f.client, repo, commit)
		if err != nil {
			logger.Warn("repoContextFilter: unable to process ignore file", log.Int32("repo", int32(repo.ID)), log.Error(err))
			continue
		}

		filters[repo.Name] = matcher.Match
		filterableRepos = append(filterableRepos, repo)
	}

	return filterableRepos, func(fcc []FileChunkContext) []FileChunkContext {
		filtered := make([]FileChunkContext, 0, len(fcc))
		for _, fc := range fcc {
			remove, ok := filters[fc.RepoName]
			if !ok {
				filtered = append(filtered, fc)
				continue
			}
			if !remove(fc.Path) {
				filtered = append(filtered, fc)
			}
		}
		return filtered
	}
}

type RepoContentFilter interface {
	GetFilter(repos []types.RepoIDName, logger log.Logger) ([]types.RepoIDName, FileChunkFilterFunc)
}

// NewCodyIgnoreFilter creates a new RepoContentFilter that filters out
// content based on the .cody/ignore file at the head of the default branch
// for the given repositories.
func NewCodyIgnoreFilter(client gitserver.Client) RepoContentFilter {
	enabled := isEnabled(conf.Get())
	c, _ := lru.New[repoRevision, ignore.Matcher](128)
	ignoreFilter := &repoFilter{
		cache:   safeCache{c},
		client:  client,
		enabled: enabled,
	}

	go conf.Watch(func() {
		ignoreFilter.SetEnabled(isEnabled(conf.Get()))
	})

	return ignoreFilter
}

func isEnabled(c *conf.Unified) bool {
	if c != nil && c.ExperimentalFeatures != nil && c.ExperimentalFeatures.CodyContextIgnore != nil {
		return *c.ExperimentalFeatures.CodyContextIgnore
	}
	return false
}

func getIgnoreMatcher(ctx context.Context, cache safeCache, client gitserver.Client, repo types.RepoIDName, commit api.CommitID) (*ignore.Matcher, error) {
	cached, ok := cache.Get(repoRevision{Repo: repo, Commit: commit})
	if ok {
		return &cached, nil
	}

	fr, err := client.NewFileReader(
		ctx,
		repo.Name,
		commit,
		codyIgnoreFile,
	)
	if err != nil {
		// We do not ignore anything if the ignore file does not exist.
		if os.IsNotExist(err) {
			cache.Add(repoRevision{Repo: repo, Commit: commit}, emptyMatcher)
			return &emptyMatcher, nil
		}
		return nil, err
	}
	defer fr.Close()

	ignoreFileBytes, err := io.ReadAll(fr)
	if err != nil {
		return nil, err
	}
	ig, err := ignore.ParseIgnoreFile(bytes.NewReader(ignoreFileBytes))
	if err != nil {
		return nil, err
	}
	cache.Add(repoRevision{Repo: repo, Commit: commit}, *ig)
	return ig, nil
}
