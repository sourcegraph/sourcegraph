package inference

import (
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/luasandbox"
)

type SandboxService interface {
	CreateSandbox(ctx context.Context, opts luasandbox.CreateOptions) (*luasandbox.Sandbox, error)
}

type GitService interface {
	LsFiles(ctx context.Context, repo api.RepoID, commit string, pathspecs ...gitdomain.Pathspec) ([]string, error)
	Archive(ctx context.Context, repo api.RepoID, opts gitserver.ArchiveOptions) (io.ReadCloser, error)
}

type gitService struct {
	checker authz.SubRepoPermissionChecker
	client  gitserver.Client
}

func NewDefaultGitService(checker authz.SubRepoPermissionChecker) GitService {
	if checker == nil {
		checker = authz.DefaultSubRepoPermsChecker
	}

	return &gitService{
		checker: checker,
		client:  gitserver.NewClient("codeintel.inference"),
	}
}

func (s *gitService) LsFiles(ctx context.Context, repo api.RepoID, commit string, pathspecs ...gitdomain.Pathspec) ([]string, error) {
	return s.client.LsFiles(ctx, repo, api.CommitID(commit), pathspecs...)
}

func (s *gitService) Archive(ctx context.Context, repo api.RepoID, opts gitserver.ArchiveOptions) (io.ReadCloser, error) {
	return s.client.ArchiveReader(ctx, repo, opts)
}
