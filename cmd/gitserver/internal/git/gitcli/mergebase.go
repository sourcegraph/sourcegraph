package gitcli

import (
	"bytes"
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) MergeBase(ctx context.Context, baseRevspec, headRevspec string) (gitdomain.OID, error) {
	out, err := g.NewCommand(
		ctx,
		WithArguments("merge-base", "--", baseRevspec, headRevspec),
	)
	if err != nil {
		return gitdomain.OID{}, err
	}

	defer out.Close()

	stdout, err := io.ReadAll(out)
	if err != nil {
		// Exit code 1 and empty output most likely means that no common merge-base was found.
		var e *commandFailedError
		if errors.As(err, &e) {
			if e.ExitStatus == 1 {
				if len(e.Stderr) == 0 {
					return gitdomain.OID{}, nil
				}
			} else if e.ExitStatus == 128 && bytes.Contains(e.Stderr, []byte("fatal: Not a valid object name")) {
				spec := headRevspec
				if bytes.Contains(e.Stderr, []byte(baseRevspec)) {
					spec = baseRevspec
				}
				return gitdomain.OID{}, &gitdomain.RevisionNotFoundError{
					Repo: g.repoName,
					Spec: spec,
				}
			}
		}

		return gitdomain.OID{}, err
	}

	return gitdomain.NewOID(api.CommitID(bytes.TrimSpace(stdout)))
}
