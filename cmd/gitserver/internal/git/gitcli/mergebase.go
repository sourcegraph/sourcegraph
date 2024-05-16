package gitcli

import (
	"bytes"
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) MergeBase(ctx context.Context, baseRevspec, headRevspec string) (api.CommitID, error) {
	out, err := g.NewCommand(
		ctx,
		"merge-base",
		WithArguments(
			FlagArgument{"--"},
			SpecSafeValueArgument{baseRevspec},
			SpecSafeValueArgument{headRevspec},
		),
	)
	if err != nil {
		return "", err
	}

	defer out.Close()

	stdout, err := io.ReadAll(out)
	if err != nil {
		// Exit code 1 and empty output most likely means that no common merge-base was found.
		var e *CommandFailedError
		if errors.As(err, &e) {
			if e.ExitStatus == 1 {
				if len(e.Stderr) == 0 {
					return "", nil
				}
			} else if e.ExitStatus == 128 && bytes.Contains(e.Stderr, []byte("fatal: Not a valid object name")) {
				spec := headRevspec
				if bytes.Contains(e.Stderr, []byte(baseRevspec)) {
					spec = baseRevspec
				}
				return "", &gitdomain.RevisionNotFoundError{
					Repo: g.repoName,
					Spec: spec,
				}
			}
		}

		return "", err
	}

	return api.CommitID(bytes.TrimSpace(stdout)), nil
}
