package gitcli

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) GetObject(ctx context.Context, objectName string) (*gitdomain.GitObject, error) {
	sha, err := g.revParse(ctx, objectName)
	if err != nil {
		return nil, errors.Wrap(err, "getting object ID")
	}

	oid, err := decodeOID(sha)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode OID")
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
	r, err := g.NewCommand(ctx, "cat-file", WithArguments(
		FlagArgument{"-t"},
		FlagArgument{"--"},
		SpecSafeValueArgument{objectID},
	))
	if err != nil {
		return "", err
	}

	stdout, err := io.ReadAll(r)
	if err != nil {
		var e *CommandFailedError
		if errors.As(err, &e) && e.ExitStatus == 128 && (bytes.Contains(e.Stderr, []byte("Not a valid object name")) ||
			bytes.Contains(e.Stderr, []byte("git cat-file: could not get object info"))) {
			return "", &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: objectID}
		}

		return "", err
	}

	return gitdomain.ObjectType(bytes.TrimSpace(stdout)), nil
}

func decodeOID(sha api.CommitID) (gitdomain.OID, error) {
	oidBytes, err := hex.DecodeString(string(sha))
	if err != nil {
		return gitdomain.OID{}, err
	}
	var oid gitdomain.OID
	copy(oid[:], oidBytes)
	return oid, nil
}
