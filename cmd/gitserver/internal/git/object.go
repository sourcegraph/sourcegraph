package git

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func GetObject(ctx context.Context, rcf *wrexec.RecordingCommandFactory, reposDir string, repo api.RepoName, objectName string) (_ *gitdomain.GitObject, err error) {
	return getObject(ctx, rcf, reposDir, getObjectType, revParse, repo, objectName)
}

func getObject(ctx context.Context, rcf *wrexec.RecordingCommandFactory, reposDir string, getObjectType getObjectTypeFunc, revParse revParseFunc, repo api.RepoName, objectName string) (_ *gitdomain.GitObject, err error) {
	tr, ctx := trace.New(ctx, "GetObject",
		attribute.String("objectName", objectName))
	defer tr.EndWithErr(&err)

	if err := CheckSpecArgSafety(objectName); err != nil {
		return nil, err
	}

	dir := gitserverfs.RepoDirFromName(reposDir, repo)

	sha, err := revParse(ctx, rcf, repo, dir, objectName)
	if err != nil {
		if gitdomain.IsRepoNotExist(err) {
			return nil, err
		}
		if strings.Contains(sha, "unknown revision") {
			return nil, &gitdomain.RevisionNotFoundError{Repo: repo, Spec: objectName}
		}
		return nil, err
	}

	sha = strings.TrimSpace(sha)
	if !gitdomain.IsAbsoluteRevision(sha) {
		if sha == "HEAD" {
			// We don't verify the existence of HEAD, but if HEAD doesn't point to anything
			// git just returns `HEAD` as the output of rev-parse. An example where this
			// occurs is an empty repository.
			return nil, &gitdomain.RevisionNotFoundError{Repo: repo, Spec: objectName}
		}
		return nil, &gitdomain.BadCommitError{Spec: objectName, Commit: api.CommitID(sha), Repo: repo}
	}

	oid, err := decodeOID(sha)
	if err != nil {
		return nil, errors.Wrap(err, "decoding oid")
	}

	objectType, err := getObjectType(ctx, rcf, repo, dir, oid.String())
	if err != nil {
		return nil, errors.Wrap(err, "getting object type")
	}

	return &gitdomain.GitObject{
		ID:   oid,
		Type: objectType,
	}, nil

}

type getObjectTypeFunc func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, objectID string) (gitdomain.ObjectType, error)

// getObjectType returns the object type given an objectID.
func getObjectType(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, objectID string) (gitdomain.ObjectType, error) {
	r, err := git.PlainOpen(dir.Path())
	if err != nil {
		return "", err
	}

	o, err := r.Object(plumbing.AnyObject, plumbing.NewHash(objectID))
	if err != nil {
		return "", err
	}

	switch o.Type() {
	case plumbing.InvalidObject:
		return "", errors.Newf("invalid object type")
	case plumbing.CommitObject:
		return gitdomain.ObjectTypeCommit, nil
	case plumbing.TreeObject:
		return gitdomain.ObjectTypeTree, nil
	case plumbing.BlobObject:
		return gitdomain.ObjectTypeBlob, nil
	case plumbing.TagObject:
		return gitdomain.ObjectTypeTag, nil
	default:
		return "", errors.Newf("unknown object type %s", o.Type())
	}

	// cmd := exec.Command("git", "cat-file", "-t", "--", objectID)
	// dir.Set(cmd)
	// wrappedCmd := rcf.WrapWithRepoName(context.Background(), log.NoOp(), repo, cmd)
	// out, err := wrappedCmd.CombinedOutput()
	// if err != nil {
	// 	return "", errors.WithMessage(err, fmt.Sprintf("git command %v failed (output: %q)", wrappedCmd.Args, out))
	// }

	// objectType := gitdomain.ObjectType(bytes.TrimSpace(out))
	// return objectType, nil
}

type revParseFunc func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, rev string) (string, error)

// revParse will run rev-parse on the given rev.
// rev should have been checked by the caller to be safe to pass to git rev-parse.
func revParse(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, rev string) (string, error) {
	r, err := git.PlainOpen(dir.Path())
	if err != nil {
		return "", err
	}

	h, err := r.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return "", err
	}

	return h.String(), nil

	// cmd := exec.Command("git", "rev-parse", rev)
	// dir.Set(cmd)
	// wrappedCmd := rcf.WrapWithRepoName(context.Background(), log.NoOp(), repo, cmd)
	// out, err := wrappedCmd.CombinedOutput()
	// if err != nil {
	// 	return "", errors.WithMessage(err, fmt.Sprintf("git command %v failed (output: %q)", wrappedCmd.Args, out))
	// }

	// return string(out), nil
}

func decodeOID(sha string) (gitdomain.OID, error) {
	oidBytes, err := hex.DecodeString(sha)
	if err != nil {
		return gitdomain.OID{}, err
	}
	var oid gitdomain.OID
	copy(oid[:], oidBytes)
	return oid, nil
}
