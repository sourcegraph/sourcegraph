package gitresolvers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	numRoutines     = 5
	numRepositories = 10
	numCommits      = 10 // per repo
	numPaths        = 10 // per commit
)

func TestCachedLocationResolver(t *testing.T) {
	repos := dbmocks.NewStrictMockRepoStore()
	repos.GetFunc.SetDefaultHook(func(v0 context.Context, id api.RepoID) (*types.Repo, error) {
		return &types.Repo{ID: id, CreatedAt: time.Now()}, nil
	})

	gsClient := gitserver.NewMockClient()
	gsClient.ResolveRevisionFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, spec string, _ gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		return api.CommitID(spec), nil
	})

	var commitCalls uint32
	factory := NewCachedLocationResolverFactory(repos, gsClient)
	locationResolver := factory.Create()

	var repositoryIDs []api.RepoID
	for i := 1; i <= numRepositories; i++ {
		repositoryIDs = append(repositoryIDs, api.RepoID(i))
	}

	var commits []string
	for i := 1; i <= numCommits; i++ {
		commits = append(commits, fmt.Sprintf("%040d", i))
	}

	var paths []string
	for i := 1; i <= numPaths; i++ {
		paths = append(paths, fmt.Sprintf("/foo/%d/bar/baz.go", i))
	}

	type resolverPair struct {
		key      string
		resolver resolverstubs.GitTreeEntryResolver
	}
	resolvers := make(chan resolverPair, numRoutines*len(repositoryIDs)*len(commits)*len(paths))

	var wg sync.WaitGroup
	errs := make(chan error, numRoutines)
	for range numRoutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for _, repositoryID := range repositoryIDs {
				repositoryResolver, err := locationResolver.Repository(context.Background(), repositoryID)
				if err != nil {
					errs <- err
					return
				}
				repoID, err := resolverstubs.UnmarshalID[api.RepoID](repositoryResolver.ID())
				if err != nil {
					errs <- err
					return
				}
				if repoID != repositoryID {
					errs <- errors.Errorf("unexpected repository id. want=%d have=%d", repositoryID, repoID)
					return
				}
			}

			for _, repositoryID := range repositoryIDs {
				for _, commit := range commits {
					commitResolver, err := locationResolver.Commit(context.Background(), repositoryID, commit)
					if err != nil {
						errs <- err
						return
					}
					if commitResolver.OID() != resolverstubs.GitObjectID(commit) {
						errs <- errors.Errorf("unexpected commit. want=%s have=%s", commit, commitResolver.OID())
						return
					}
				}
			}

			for _, repositoryID := range repositoryIDs {
				for _, commit := range commits {
					for _, path := range paths {
						treeResolver, err := locationResolver.Path(context.Background(), repositoryID, commit, path, false)
						if err != nil {
							errs <- err
							return
						}
						if treeResolver.Path() != path {
							errs <- errors.Errorf("unexpected path. want=%s have=%s", path, treeResolver.Path())
							return
						}

						resolvers <- resolverPair{key: fmt.Sprintf("%d:%s:%s", repositoryID, commit, path), resolver: treeResolver}
					}
				}
			}
		}()
	}
	wg.Wait()

	close(errs)
	for err := range errs {
		t.Error(err)
	}

	mockrequire.CalledN(t, repos.GetFunc, len(repositoryIDs))

	// We don't need to load commits from git-server unless we ask for fields like author or committer.
	// Since we already know this commit exists, and we only need it's already known commit ID, we assert
	// that zero calls to git.GetCommit where done. Check the gitCommitResolver lazy loading logic.
	if val := atomic.LoadUint32(&commitCalls); val != 0 {
		t.Errorf("unexpected number of commit calls. want=%d have=%d", 0, val)
	}

	close(resolvers)
	resolversByKey := map[string][]resolverstubs.GitTreeEntryResolver{}
	for pair := range resolvers {
		resolversByKey[pair.key] = append(resolversByKey[pair.key], pair.resolver)
	}

	for _, vs := range resolversByKey {
		for _, v := range vs {
			if v != vs[0] {
				t.Errorf("resolvers for same key unexpectedly have differing addresses: %p and %p", v, vs[0])
			}
		}
	}
}

func TestCachedLocationResolverUnknownRepository(t *testing.T) {
	repos := dbmocks.NewStrictMockRepoStore()
	repos.GetFunc.SetDefaultHook(func(_ context.Context, id api.RepoID) (*types.Repo, error) {
		return nil, &database.RepoNotFoundErr{ID: id}
	})

	gsClient := gitserver.NewMockClient()

	factory := NewCachedLocationResolverFactory(repos, gsClient)
	locationResolver := factory.Create()

	repositoryResolver, err := locationResolver.Repository(context.Background(), 50)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if repositoryResolver != nil {
		t.Errorf("unexpected non-nil resolver")
	}

	// Ensure no dereference in child resolvers either
	pathResolver, err := locationResolver.Path(context.Background(), 50, "deadbeef", "main.go", false)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pathResolver != nil {
		t.Errorf("unexpected non-nil resolver")
	}
	mockrequire.Called(t, repos.GetFunc)
}

func TestCachedLocationResolverUnknownCommit(t *testing.T) {
	repos := dbmocks.NewStrictMockRepoStore()
	repos.GetFunc.SetDefaultHook(func(_ context.Context, id api.RepoID) (*types.Repo, error) {
		return &types.Repo{ID: id}, nil
	})

	gsClient := gitserver.NewMockClient()
	gsClient.ResolveRevisionFunc.SetDefaultReturn("", &gitdomain.RevisionNotFoundError{})

	factory := NewCachedLocationResolverFactory(repos, gsClient)
	locationResolver := factory.Create()

	commitResolver, err := locationResolver.Commit(context.Background(), 50, "deadbeef")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if commitResolver != nil {
		t.Errorf("unexpected non-nil resolver")
	}

	// Ensure no dereference in child resolvers either
	pathResolver, err := locationResolver.Path(context.Background(), 50, "deadbeef", "main.go", false)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if pathResolver != nil {
		t.Errorf("unexpected non-nil resolver")
	}
	mockrequire.Called(t, repos.GetFunc)
}
