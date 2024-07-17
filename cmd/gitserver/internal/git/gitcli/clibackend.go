package gitcli

import (
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
)

func NewBackend(logger log.Logger, rcf *wrexec.RecordingCommandFactory, dir common.GitDir, repoName api.RepoName) git.GitBackend {
	return &gitCLIBackend{
		logger:   logger,
		rcf:      rcf,
		dir:      dir,
		repoName: repoName,
		caches:   makeGlobalCache(),
	}
}

type gitCLIBackend struct {
	logger   log.Logger
	rcf      *wrexec.RecordingCommandFactory
	dir      common.GitDir
	repoName api.RepoName
	caches   *caches
}
