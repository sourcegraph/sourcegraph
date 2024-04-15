package codycontext

import (
	"context"
	"os"
	"sync"

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

type filterFunc func(string) bool

type dotcomRepoFilter struct {
	mu      sync.RWMutex
	logger  log.Logger
	client  gitserver.Client
	enabled bool
}

func (f *dotcomRepoFilter) setEnabled(enabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enabled = enabled
}

func (f *dotcomRepoFilter) getEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

func (f *dotcomRepoFilter) GetFilter(repos []types.RepoIDName) ([]types.RepoIDName, FileChunkFilterFunc, error) {
	if !f.getEnabled() {
		return repos, func(fcc []FileChunkContext) []FileChunkContext {
			return fcc
		}, nil
	}
	return f.getFilter(repos)
}

// getFilter returns the list of repos that can be filtered
// their .cody/ignore files (or don't have one). If an error
// occurs that repo will be excluded.
func (f *dotcomRepoFilter) getFilter(repos []types.RepoIDName) ([]types.RepoIDName, FileChunkFilterFunc, error) {
	filters := make(map[api.RepoID]filterFunc, len(repos))
	filterableRepos := make([]types.RepoIDName, 0, len(repos))
	// use the internal actor to ensure access to repo and ignore files
	ctx := actor.WithInternalActor(context.Background())
	for _, repo := range repos {

		_, commit, err := f.client.GetDefaultBranch(ctx, repo.Name, true)
		if err != nil {
			f.logger.Warn("couldn't get default branch, removing repo", log.Int32("repo", int32(repo.ID)), log.Error(err))
			continue
		}
		// No commit signals an empty repo, should be nothing to filter
		// Also we can't lookup the ignore file without a commit
		if commit == "" {
			f.logger.Info("empty repo removing", log.Int32("repo", int32(repo.ID)))
			continue
		}
		matcher, err := getIgnoreMatcher(ctx, f.client, repo, commit)
		if err != nil {
			f.logger.Warn("unable to process ignore file", log.Int32("repo", int32(repo.ID)), log.Error(err))
			continue
		}

		filters[repo.ID] = matcher.Match
		filterableRepos = append(filterableRepos, repo)
	}

	return filterableRepos, func(fcc []FileChunkContext) []FileChunkContext {
		filtered := make([]FileChunkContext, 0, len(fcc))
		for _, fc := range fcc {
			ignore, ok := filters[fc.RepoID]
			if !ok {
				filtered = append(filtered, fc)
				continue
			}
			if !ignore(fc.Path) {
				filtered = append(filtered, fc)
			}
		}
		return filtered
	}, nil
}

// newDotcomFilter creates a new RepoContentFilter that filters out
// content based on the .cody/ignore file at the head of the default branch
// for the given repositories.
func newDotcomFilter(logger log.Logger, client gitserver.Client) RepoContentFilter {
	enabled := isEnabled(conf.Get())
	ignoreFilter := &dotcomRepoFilter{
		logger:  logger.Scoped("filter"),
		client:  client,
		enabled: enabled,
	}

	conf.Watch(func() {
		ignoreFilter.setEnabled(isEnabled(conf.Get()))
	})

	return ignoreFilter
}

func isEnabled(c *conf.Unified) bool {
	if c != nil && c.ExperimentalFeatures != nil && c.ExperimentalFeatures.CodyContextIgnore != nil {
		return *c.ExperimentalFeatures.CodyContextIgnore
	}
	return false
}

func getIgnoreMatcher(ctx context.Context, client gitserver.Client, repo types.RepoIDName, commit api.CommitID) (*ignore.Matcher, error) {
	fr, err := client.NewFileReader(
		ctx,
		repo.Name,
		commit,
		codyIgnoreFile,
	)
	if err != nil {
		// We do not ignore anything if the ignore file does not exist.
		if os.IsNotExist(err) {
			return &emptyMatcher, nil
		}
		return nil, err
	}
	defer fr.Close()
	ig, err := ignore.ParseIgnoreFile(fr)
	if err != nil {
		return nil, err
	}
	return ig, nil
}
