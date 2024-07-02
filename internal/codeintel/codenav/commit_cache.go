package codenav

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type CommitCache interface {
	ExistsBatch(ctx context.Context, commits []RepositoryCommit) ([]bool, error)
	SetResolvableCommit(repositoryID api.RepoID, commit api.CommitID)
}

type RepositoryCommit struct {
	RepositoryID api.RepoID
	Commit       api.CommitID
}

type commitCache struct {
	repoStore       database.RepoStore
	gitserverClient gitserver.Client
	mutex           sync.RWMutex
	cache           map[api.RepoID]map[api.CommitID]bool
}

func NewCommitCache(repoStore database.RepoStore, client gitserver.Client) CommitCache {
	return &commitCache{
		repoStore:       repoStore,
		gitserverClient: client,
		cache:           map[api.RepoID]map[api.CommitID]bool{},
	}
}

// ExistsBatch determines if the given commits are resolvable for the given repositories.
// If we do not know the answer from a previous call to set or existsBatch, we ask gitserver
// to resolve the remaining commits and store the results for subsequent calls. This method
// returns a slice of the same size as the input slice, true indicating that the commit at
// the symmetric index exists.
func (c *commitCache) ExistsBatch(ctx context.Context, commits []RepositoryCommit) ([]bool, error) {
	exists := make([]bool, len(commits))
	rcIndexMap := make([]int, 0, len(commits))
	rcs := make([]RepositoryCommit, 0, len(commits))

	for i, rc := range commits {
		if e, ok := c.getInternal(rc.RepositoryID, rc.Commit); ok {
			exists[i] = e
		} else {
			rcIndexMap = append(rcIndexMap, i)
			rcs = append(rcs, RepositoryCommit{
				RepositoryID: rc.RepositoryID,
				Commit:       rc.Commit,
			})
		}
	}

	if len(rcs) == 0 {
		return exists, nil
	}

	// Perform heavy work outside of critical section
	e, err := c.commitsExist(ctx, rcs)
	if err != nil {
		return nil, errors.Wrap(err, "gitserverClient.CommitsExist")
	}
	if len(e) != len(rcs) {
		panic(strings.Join([]string{
			fmt.Sprintf("Expected slice returned from CommitsExist to have len %d, but has len %d.", len(rcs), len(e)),
			"If this panic occurred during a test, your test is missing a mock definition for CommitsExist.",
			"If this is occurred during runtime, please file a bug.",
		}, " "))
	}

	for i, rc := range rcs {
		exists[rcIndexMap[i]] = e[i]
		c.setInternal(rc.RepositoryID, rc.Commit, e[i])
	}

	return exists, nil
}

// commitsExist determines if the given commits exists in the given repositories. This method returns a
// slice of the same size as the input slice, true indicating that the commit at the symmetric index exists.
func (c *commitCache) commitsExist(ctx context.Context, commits []RepositoryCommit) (_ []bool, err error) {
	repositoryIDSet := collections.NewSet[api.RepoID]()
	for _, rc := range commits {
		repositoryIDSet.Add(rc.RepositoryID)
	}
	repositoryIDs := repositoryIDSet.Values()
	repos, err := c.repoStore.GetReposSetByIDs(ctx, repositoryIDs...)
	if err != nil {
		return nil, err
	}
	repositoryNames := make(map[api.RepoID]api.RepoName, len(repos))
	for _, v := range repos {
		repositoryNames[v.ID] = v.Name
	}

	// Build the batch request to send to gitserver. Because we only add repo/commit
	// pairs that are resolvable to a repo name, we may end up skipping inputs for an
	// unresolvable repo. We also ensure that we only represent each repo/commit pair
	// ONCE in the input slice.

	repoCommits := make([]repoCommit, 0, len(commits))                 // input to CommitsExist
	indexMapping := make(map[int]int, len(commits))                    // map commits[i] to relevant repoCommits[i]
	commitsRepresentedInInput := map[api.RepoID]map[api.CommitID]int{} // used to populate index mapping

	for i, rc := range commits {
		repoName, ok := repositoryNames[rc.RepositoryID]
		if !ok {
			// insert a sentinel value we explicitly check below for any repositories
			// that we're unable to resolve
			indexMapping[i] = -1
			continue
		}

		// Ensure our second-level mapping exists
		if _, ok := commitsRepresentedInInput[rc.RepositoryID]; !ok {
			commitsRepresentedInInput[rc.RepositoryID] = map[api.CommitID]int{}
		}

		if n, ok := commitsRepresentedInInput[rc.RepositoryID][rc.Commit]; ok {
			// repoCommits[n] already represents this pair
			indexMapping[i] = n
		} else {
			// pair is not yet represented in the input, so we'll stash the index of input
			// object we're _about_ to insert
			n := len(repoCommits)
			indexMapping[i] = n
			commitsRepresentedInInput[rc.RepositoryID][rc.Commit] = n

			repoCommits = append(repoCommits, repoCommit{
				repoName: repoName,
				commitID: rc.Commit,
			})
		}
	}

	exists := make([]bool, len(commits))
	for i, rc := range repoCommits {
		_, err := c.gitserverClient.GetCommit(ctx, rc.repoName, rc.commitID)
		if err != nil {
			if errors.HasType[*gitdomain.RevisionNotFoundError](err) {
				exists[i] = false
				continue
			}
			return nil, err
		}
		exists[i] = true
	}

	// Spread the response back to the correct indexes the caller is expecting. Each value in the
	// response from gitserver belongs to some index in the original commits slice. We re-map these
	// values and leave all other values implicitly false (these repo name were not resolvable).
	out := make([]bool, len(commits))
	for i := range commits {
		if indexMapping[i] != -1 {
			out[i] = exists[indexMapping[i]]
		}
	}

	return out, nil
}

type repoCommit struct {
	repoName api.RepoName
	commitID api.CommitID
}

// set marks the given repository and commit as valid and resolvable by gitserver.
func (c *commitCache) SetResolvableCommit(repositoryID api.RepoID, commit api.CommitID) {
	c.setInternal(repositoryID, commit, true)
}

func (c *commitCache) getInternal(repositoryID api.RepoID, commit api.CommitID) (bool, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if repositoryMap, ok := c.cache[repositoryID]; ok {
		if exists, ok := repositoryMap[commit]; ok {
			return exists, true
		}
	}

	return false, false
}

func (c *commitCache) setInternal(repositoryID api.RepoID, commit api.CommitID, exists bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, ok := c.cache[repositoryID]; !ok {
		c.cache[repositoryID] = map[api.CommitID]bool{}
	}

	c.cache[repositoryID][commit] = exists
}
