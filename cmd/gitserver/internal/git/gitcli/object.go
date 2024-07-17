package gitcli

import (
	"bytes"
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) GetObject(ctx context.Context, objectName string) (*gitdomain.GitObject, error) {
	if err := checkSpecArgSafety(objectName); err != nil {
		return nil, err
	}

	oid, err := g.revParse(ctx, objectName)
	if err != nil {
		return nil, errors.Wrap(err, "getting object ID")
	}

	objectType, err := g.getObjectType(ctx, oid.String())
	if err != nil {
		return nil, errors.Wrap(err, "getting object type")
	}

	return &gitdomain.GitObject{
		ID:   oid,
		Type: objectType,
	}, nil
}

// getObjectType returns the object type given an objectID.
func (g *gitCLIBackend) getObjectType(ctx context.Context, objectID string) (gitdomain.ObjectType, error) {
	r, err := g.NewCommand(ctx, WithArguments("cat-file", "-t", "--", objectID))
	if err != nil {
		return "", err
	}

	stdout, err := io.ReadAll(r)
	if err != nil {
		var e *commandFailedError
		if errors.As(err, &e) && e.ExitStatus == 128 && (bytes.Contains(e.Stderr, []byte("Not a valid object name")) ||
			bytes.Contains(e.Stderr, []byte("git cat-file: could not get object info"))) {
			return "", &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: objectID}
		}

		return "", err
	}

	return gitdomain.ObjectType(bytes.TrimSpace(stdout)), nil
}
