package gitserver

import (
	"bytes"
	"context"
	"io"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type GitserverClient interface {
	// FetchTar returns an io.ReadCloser to a tar archive of a repository at the specified Git
	// remote URL and commit ID. If the error implements "BadRequest() bool", it will be used to
	// determine if the error is a bad request (eg invalid repo).
	FetchTar(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error)

	// GitDiff returns the paths that have changed between two commits.
	GitDiff(context.Context, api.RepoName, api.CommitID, api.CommitID) (Changes, error)

	// NewFileReader returns an io.ReadCloser reading from the named file at commit.
	// The caller should always close the reader after use.
	// (If you just need to check a file's existence, use Stat, not a file reader.)
	NewFileReader(ctx context.Context, repoCommitPath types.RepoCommitPath) (io.ReadCloser, error)

	// LogReverseEach runs git log in reverse order and calls the given callback for each entry.
	LogReverseEach(ctx context.Context, repo api.RepoID, commit string, n int, onLogEntry func(entry gitdomain.LogEntry) error) error

	// RevList makes a git rev-list call and iterates through the resulting commits, calling the provided
	// onCommit function for each.
	RevList(ctx context.Context, repo api.RepoID, commit string, onCommit func(commit string) (shouldContinue bool, err error)) error
}

// Changes are added, deleted, and modified paths.
type Changes struct {
	Added    []string
	Modified []string
	Deleted  []string
}

type gitserverClient struct {
	innerClient gitserver.Client
	operations  *operations
}

func NewClient(observationCtx *observation.Context, db database.DB) GitserverClient {
	return &gitserverClient{
		innerClient: gitserver.NewClient("symbols"),
		operations:  newOperations(observationCtx),
	}
}

func (c *gitserverClient) FetchTar(ctx context.Context, repo api.RepoName, commit api.CommitID, paths []string) (_ io.ReadCloser, err error) {
	ctx, _, endObservation := c.operations.fetchTar.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		repo.Attr(),
		commit.Attr(),
		attribute.Int("paths", len(paths)),
	}})
	defer endObservation(1, observation.Args{})

	opts := gitserver.ArchiveOptions{
		Treeish: string(commit),
		Format:  gitserver.ArchiveFormatTar,
		Paths:   paths,
	}

	// Note: the sub-repo perms checker is nil here because we do the sub-repo filtering at a higher level
	return c.innerClient.ArchiveReader(ctx, repo, opts)
}

func (c *gitserverClient) GitDiff(ctx context.Context, repo api.RepoName, commitA, commitB api.CommitID) (_ Changes, err error) {
	ctx, _, endObservation := c.operations.gitDiff.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		repo.Attr(),
		attribute.String("commitA", string(commitA)),
		attribute.String("commitB", string(commitB)),
	}})
	defer endObservation(1, observation.Args{})

	output, err := c.innerClient.DiffSymbols(ctx, repo, commitA, commitB)

	changes, err := parseGitDiffOutput(output)
	if err != nil {
		return Changes{}, errors.Wrap(err, "failed to parse git diff output")
	}

	return changes, nil
}

func (c *gitserverClient) NewFileReader(ctx context.Context, repoCommitPath types.RepoCommitPath) (io.ReadCloser, error) {
	return c.innerClient.NewFileReader(ctx, api.RepoName(repoCommitPath.Repo), api.CommitID(repoCommitPath.Commit), repoCommitPath.Path)
}

func (c *gitserverClient) LogReverseEach(ctx context.Context, repo api.RepoID, commit string, n int, onLogEntry func(entry gitdomain.LogEntry) error) error {
	return c.innerClient.LogReverseEach(ctx, repo, commit, n, onLogEntry)
}

func (c *gitserverClient) RevList(ctx context.Context, repo api.RepoID, commit string, onCommit func(commit string) (shouldContinue bool, err error)) error {
	return c.innerClient.RevList(ctx, repo, commit, onCommit)
}

var NUL = []byte{0}

// parseGitDiffOutput parses the output of a git diff command, which consists
// of a repeated sequence of `<status> NUL <path> NUL` where NUL is the 0 byte.
func parseGitDiffOutput(output []byte) (changes Changes, _ error) {
	if len(output) == 0 {
		return Changes{}, nil
	}

	slices := bytes.Split(bytes.TrimRight(output, string(NUL)), NUL)
	if len(slices)%2 != 0 {
		return changes, errors.Newf("uneven pairs")
	}

	for i := 0; i < len(slices); i += 2 {
		switch slices[i][0] {
		case 'A':
			changes.Added = append(changes.Added, string(slices[i+1]))
		case 'M':
			changes.Modified = append(changes.Modified, string(slices[i+1]))
		case 'D':
			changes.Deleted = append(changes.Deleted, string(slices[i+1]))
		}
	}

	return changes, nil
}
