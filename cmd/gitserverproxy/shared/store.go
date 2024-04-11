package shared

import (
	"context"
	"strconv"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserverproxy/internal/proxy"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type repoLookupStore struct {
	db database.DB
}

var _ proxy.RepoLookupStore = &repoLookupStore{}

func (s *repoLookupStore) RepoByUID(ctx context.Context, uid string) (api.RepoName, string, error) {
	// Our UID is currently the int32 field of the `repo.id` column.
	id, err := strconv.Atoi(uid)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to unmarshal repository id")
	}

	r, err := s.db.Repos().Get(ctx, api.RepoID(id))
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get repository")
	}

	gr, err := s.db.GitserverRepos().GetByID(ctx, api.RepoID(id))
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get gitserver repository")
	}

	// Imagine gr held the directory here.
	// gr.StoragePath should be a globally unique text field that is generated
	// by either using a human readable path, or using a path that encodes the repo ID.
	// If gr.Path is not yet assigned, we should set it here and then return the
	// value.
	return r.Name, gr.ShardID, nil
}

func (s *repoLookupStore) ListRepos(ctx context.Context, pageCursor string) (repos []proxy.ListRepo, nextPage string, err error) {
	rs, err := s.db.Repos().ListMinimalRepos(ctx, database.ReposListOptions{IncludeDeleted: true, IncludeBlocked: true})
	if err != nil {
		return nil, "", err
	}
	repos = make([]proxy.ListRepo, len(rs))
	for i, r := range rs {
		var deleteAfter time.Time
		// TODO: ListMinimalRepos doesn't return deletedAt, blocked right now,
		// need to set deleteAfter here later.
		// This is where we can determine if we want to delete a repo.
		// Blocked should be immediate. Deleted should be deleted after some TTL
		// period.
		repos[i] = proxy.ListRepo{
			UID:         strconv.Itoa(int(r.ID)),
			Name:        r.Name,
			DeleteAfter: deleteAfter,
		}
	}

	// No pagination for now.
	return repos, "", nil
}

// simpleLocalDebuggingRepoLookupStore is a RepoLookupStore implementation that looks up
// repositories by name instead of ID for local debugging purposes.
// Note that this may never be used in production since name lookups are not
// guaranteed to be unique or never changing.
type simpleLocalDebuggingRepoLookupStore struct {
	db database.DB
}

var _ proxy.RepoLookupStore = &simpleLocalDebuggingRepoLookupStore{}

func (s *simpleLocalDebuggingRepoLookupStore) RepoByUID(ctx context.Context, uid string) (api.RepoName, string, error) {
	r, err := s.db.Repos().GetByName(ctx, api.RepoName(uid))
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get repository")
	}

	return r.Name, "", nil
}

func (s *simpleLocalDebuggingRepoLookupStore) ListRepos(ctx context.Context, pageCursor string) (repos []proxy.ListRepo, nextPage string, err error) {
	rs, err := s.db.Repos().ListMinimalRepos(ctx, database.ReposListOptions{})
	if err != nil {
		return nil, "", err
	}
	repos = make([]proxy.ListRepo, len(rs))
	for i, r := range rs {
		repos[i] = proxy.ListRepo{
			UID:  strconv.Itoa(int(r.ID)),
			Name: r.Name,
		}
	}

	// No pagination for now.
	return repos, "", nil
}
