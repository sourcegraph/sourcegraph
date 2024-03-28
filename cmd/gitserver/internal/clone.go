package internal

import (
	"context"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type CloneStatus struct {
	CloneInProgress bool
	CloneProgress   string
}

// MaybeStartClone checks if a given repository is cloned on disk. If not, it starts
// cloning the repository in the background and returns a CloneStatus.
// Note: If disableAutoGitUpdates is set in the site config, no operation is taken and
// a NotFound error is returned.
func (s *Server) MaybeStartClone(ctx context.Context, repo api.RepoName) (cloned bool, status CloneStatus, _ error) {
	cloned, err := s.fs.RepoCloned(repo)
	if err != nil {
		return false, CloneStatus{}, errors.Wrap(err, "determine clone status")
	}

	if cloned {
		return true, CloneStatus{}, nil
	}

	if conf.Get().DisableAutoGitUpdates {
		s.logger.Debug("not cloning on demand as DisableAutoGitUpdates is set")
		return false, CloneStatus{}, nil
	}

	cloneProgress, err := s.CloneRepo(ctx, repo, CloneOptions{})
	if err != nil {
		s.logger.Warn("error starting repo clone", log.String("repo", string(repo)), log.Error(err))
		return false, CloneStatus{}, nil
	}

	return false, CloneStatus{
		CloneInProgress: true,
		CloneProgress:   cloneProgress,
	}, nil
}
