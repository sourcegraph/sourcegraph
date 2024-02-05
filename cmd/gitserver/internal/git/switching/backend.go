package switching

import (
	"context"
	"io"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
)

func NewBackend(logger log.Logger, rcf *wrexec.RecordingCommandFactory, dir common.GitDir, repoName api.RepoName, gogitFactory func(common.GitDir) (git.GitBackend, error)) (git.GitBackend, error) {
	gc := gitcli.NewBackend(logger, rcf, dir, repoName)
	gg, err := gogitFactory(dir)
	if err != nil {
		return nil, err
	}
	return &switchingBackend{gogit: gg, gitcli: gc}, nil
}

type switchingBackend struct {
	gogit  git.GitBackend
	gitcli git.GitBackend
}

func (g *switchingBackend) Config() git.GitConfigBackend {
	return g.gogit.Config()
}

func (g *switchingBackend) GetObject(ctx context.Context, objectName string) (*gitdomain.GitObject, error) {
	return g.gogit.GetObject(ctx, objectName)
}

func (g *switchingBackend) MergeBase(ctx context.Context, baseRevspec, headRevspec string) (api.CommitID, error) {
	return g.gogit.MergeBase(ctx, baseRevspec, headRevspec)
}

func (g *switchingBackend) Blame(ctx context.Context, path string, opt git.BlameOptions) (git.BlameHunkReader, error) {
	return g.gogit.Blame(ctx, path, opt)
}

func (g *switchingBackend) SymbolicRefHead(ctx context.Context, short bool) (string, error) {
	return g.gogit.SymbolicRefHead(ctx, short)
}

func (g *switchingBackend) RevParseHead(ctx context.Context) (api.CommitID, error) {
	return g.gogit.RevParseHead(ctx)
}

func (g *switchingBackend) ReadFile(ctx context.Context, commit api.CommitID, path string) (io.ReadCloser, error) {
	return g.gogit.ReadFile(ctx, commit, path)
}

func (g *switchingBackend) GetCommit(ctx context.Context, commit api.CommitID, includeModifiedFiles bool) (*git.GitCommitWithFiles, error) {
	return g.gogit.GetCommit(ctx, commit, includeModifiedFiles)
}

func (g *switchingBackend) Exec(ctx context.Context, args ...string) (io.ReadCloser, error) {
	return g.gitcli.Exec(ctx, args...)
}
