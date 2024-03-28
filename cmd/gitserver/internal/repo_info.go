package internal

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func repoCloneProgress(fs gitserverfs.FS, locker RepositoryLocker, repo api.RepoName) (*protocol.RepoCloneProgress, error) {
	cloned, err := fs.RepoCloned(repo)
	if err != nil {
		return nil, errors.Wrap(err, "determine clone status")
	}

	resp := protocol.RepoCloneProgress{
		Cloned: cloned,
	}
	resp.CloneProgress, resp.CloneInProgress = locker.Status(repo)
	if isAlwaysCloningTest(repo) {
		resp.CloneInProgress = true
		resp.CloneProgress = "This will never finish cloning"
	}
	return &resp, nil
}

func deleteRepo(
	ctx context.Context,
	db database.DB,
	shardID string,
	fs gitserverfs.FS,
	repo api.RepoName,
) error {
	err := fs.RemoveRepo(repo)
	if err != nil {
		return errors.Wrap(err, "removing repo directory")
	}

	err = db.GitserverRepos().SetCloneStatus(ctx, repo, types.CloneStatusNotCloned, shardID)
	if err != nil {
		return errors.Wrap(err, "setting clone status after delete")
	}
	return nil
}
