package gitcli

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) SymbolicRefHead(ctx context.Context, short bool) (refName string, err error) {
	// TODO: implement refs_shorten_unambiguous_ref from git: https://sourcegraph.com/github.com/git/git/-/blob/refs.c?L1376,
	// so QuickSymbolicRefHead can also be used when short=true.
	if !short {
		refName, err = quickSymbolicRefHead(g.dir)
		if err == nil {
			return refName, err
		}
	}

	// If our optimized version didn't work, fall back to asking git directly.
	args := []string{"symbolic-ref", "HEAD"}
	if short {
		args = append(args, "--short")
	}

	r, err := g.NewCommand(ctx, WithArguments(args...))
	if err != nil {
		return "", err
	}
	defer r.Close()
	stdout, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	refName = string(bytes.TrimSpace(stdout))

	return refName, nil
}

func (g *gitCLIBackend) RevParseHead(ctx context.Context) (api.CommitID, error) {
	shaStr, err := quickRevParseHead(g.dir)
	if err == nil {
		return api.CommitID(shaStr), nil
	}

	// If our optimized version didn't work, fall back to asking git directly.
	args := []string{"rev-parse", "HEAD"}

	r, err := g.NewCommand(ctx, WithArguments(args...))
	if err != nil {
		return "", err
	}
	defer r.Close()

	stdout, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	sha := string(bytes.TrimSpace(stdout))
	if sha == "HEAD" {
		// If HEAD doesn't point to anything git just returns `HEAD` as the
		// output of rev-parse. An example where this occurs is an empty
		// repository.
		return "", &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: "HEAD"}
	}

	return api.CommitID(sha), nil
}

const headFileRefPrefix = "ref: "

// quickSymbolicRefHead best-effort mimics the execution of `git symbolic-ref HEAD`, but doesn't exec a child process.
// It just reads the .git/HEAD file from the bare git repository directory.
func quickSymbolicRefHead(dir common.GitDir) (string, error) {
	// See if HEAD contains a commit hash and fail if so.
	head, err := os.ReadFile(dir.Path("HEAD"))
	if err != nil {
		return "", err
	}
	head = bytes.TrimSpace(head)
	if gitdomain.IsAbsoluteRevision(string(head)) {
		return "", errors.New("ref HEAD is not a symbolic ref")
	}

	// HEAD doesn't contain a commit hash. It contains something like "ref: refs/heads/master".
	if !bytes.HasPrefix(head, []byte(headFileRefPrefix)) {
		return "", errors.New("unrecognized HEAD file format")
	}
	headRef := bytes.TrimPrefix(head, []byte(headFileRefPrefix))
	return string(headRef), nil
}

// quickRevParseHead best-effort mimics the execution of `git rev-parse HEAD`, but doesn't exec a child process.
// It just reads the relevant files from the bare git repository directory.
func quickRevParseHead(dir common.GitDir) (string, error) {
	// See if HEAD contains a commit hash and return it if so.
	head, err := os.ReadFile(dir.Path("HEAD"))
	if err != nil {
		return "", err
	}
	head = bytes.TrimSpace(head)
	if h := string(head); gitdomain.IsAbsoluteRevision(h) {
		return h, nil
	}

	// HEAD doesn't contain a commit hash. It contains something like "ref: refs/heads/master".
	if !bytes.HasPrefix(head, []byte(headFileRefPrefix)) {
		return "", errors.New("unrecognized HEAD file format")
	}
	// Look for the file in refs/heads. If it exists, it contains the commit hash.
	headRef := bytes.TrimPrefix(head, []byte(headFileRefPrefix))
	if bytes.HasPrefix(headRef, []byte("../")) || bytes.Contains(headRef, []byte("/../")) || bytes.HasSuffix(headRef, []byte("/..")) {
		// 🚨 SECURITY: prevent leakage of file contents outside repo dir
		return "", errors.Errorf("invalid ref format: %s", headRef)
	}
	headRefFile := dir.Path(filepath.FromSlash(string(headRef)))
	if refs, err := os.ReadFile(headRefFile); err == nil {
		return string(bytes.TrimSpace(refs)), nil
	}

	// File didn't exist in refs/heads. Look for it in packed-refs.
	f, err := os.Open(dir.Path("packed-refs"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 2 {
			continue
		}
		commit, ref := fields[0], fields[1]
		if bytes.Equal(ref, headRef) {
			return string(commit), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	// Didn't find the refs/heads/$HEAD_BRANCH in packed_refs
	return "", errors.New("could not compute `git rev-parse HEAD` in-process, try running `git` process")
}
