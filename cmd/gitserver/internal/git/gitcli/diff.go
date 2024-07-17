package gitcli

import (
	"bufio"
	"context"
	"io"
	"sync"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/byteutils"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) RawDiff(ctx context.Context, base string, head string, typ git.GitDiffComparisonType, paths ...string) (io.ReadCloser, error) {
	baseOID, err := g.revParse(ctx, base)
	if err != nil {
		return nil, err
	}
	headOID, err := g.revParse(ctx, head)
	if err != nil {
		return nil, err
	}

	switch typ {
	case git.GitDiffComparisonTypeIntersection:
		// From the git docs on diff:
		// git diff [<options>] <commit>...<commit> [--] [<path>...]
		// This form is to view the changes on the branch containing and up to the second <commit>, starting at a common
		// ancestor of both <commit>.  git diff A...B is equivalent to git diff $(git merge-base A B) B. You can omit
		// any one of <commit>, which has the same effect as using HEAD instead.
		baseOID, err = g.MergeBase(ctx, baseOID.String(), headOID.String())
		if err != nil {
			return nil, err
		}
	case git.GitDiffComparisonTypeOnlyInHead:
		// From the git docs on diff:
		// 	git diff [<options>] <commit> <commit>... <commit> [--] [<path>...]
		// 	This form is to view the results of a merge commit. The first listed <commit> must be the merge itself; the
		// 	remaining two or more commits should be its parents. Convenient ways to produce the desired set of revisions
		// 	are to use the suffixes ^@ and ^!. If A is a merge commit, then git diff A A^@, git diff A^! and git show A
		// 	all give the same combined diff.

		// git diff [<options>] <commit>..<commit> [--] [<path>...]
		// 	This is synonymous to the earlier form (without the ..) for viewing the changes between two arbitrary
		// 	<commit>. If <commit> on one side is omitted, it will have the same effect as using HEAD instead.
		// So: Nothing to do, passing `base head` as two arguments like this is what
		// we want.
	}

	args := buildRawDiffArgs(baseOID, headOID, paths)

	return g.NewCommand(ctx, WithArguments(args...))
}

func buildRawDiffArgs(base, head gitdomain.OID, paths []string) []string {
	return append([]string{
		// Note: We use git diff-tree instead of git diff because git diff lets
		// you diff any arbitrary files on disk, which is a security risk, diffing
		// /etc/passwd to /dev/null is crazy.
		"diff-tree",
		"--patch",
		"--find-renames",
		"--full-index",
		"--inter-hunk-context=3",
		"--no-prefix",
		base.String(),
		head.String(),
		"--",
	}, paths...)
}

func (g *gitCLIBackend) ChangedFiles(ctx context.Context, base, head string) (git.ChangedFilesIterator, error) {
	args := []string{
		"diff-tree",
		"-r",
		"--root",
		"--format=format:",
		"--no-prefix",
		"--name-status",
		"--no-renames",
		"-z",
	}

	if base != "" {
		baseOID, err := g.revParse(ctx, base)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve base commit %q", base)
		}

		args = append(args, baseOID.String())
	}

	headOID, err := g.revParse(ctx, head)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve head commit %q", head)
	}

	args = append(args, headOID.String())

	rc, err := g.NewCommand(ctx, WithArguments(args...))
	if err != nil {
		return nil, errors.Wrap(err, "failed to run git diff-tree command")
	}

	return newChangedFilesIterator(rc), nil
}

func newChangedFilesIterator(rc io.ReadCloser) git.ChangedFilesIterator {
	scanner := bufio.NewScanner(rc)
	scanner.Split(byteutils.ScanNullLines)

	closeChan := make(chan struct{})
	closer := sync.OnceValue(func() error {
		err := rc.Close()
		close(closeChan)

		return err
	})

	return &changedFilesIterator{
		rc:             rc,
		scanner:        scanner,
		closeChan:      closeChan,
		onceFuncCloser: closer,
	}
}

type changedFilesIterator struct {
	rc      io.ReadCloser
	scanner *bufio.Scanner

	closeChan      chan struct{}
	onceFuncCloser func() error
}

func (i *changedFilesIterator) Next() (gitdomain.PathStatus, error) {
	select {
	case <-i.closeChan:
		return gitdomain.PathStatus{}, io.EOF
	default:
	}

	for i.scanner.Scan() {
		select {
		case <-i.closeChan:
			return gitdomain.PathStatus{}, io.EOF
		default:
		}

		status := i.scanner.Text()
		if len(status) == 0 {
			continue
		}

		if !i.scanner.Scan() {
			return gitdomain.PathStatus{}, errors.New("uneven pairs")
		}
		path := i.scanner.Text()

		switch status[0] {
		case 'A':
			return gitdomain.PathStatus{Path: path, Status: gitdomain.StatusAdded}, nil
		case 'M':
			return gitdomain.PathStatus{Path: path, Status: gitdomain.StatusModified}, nil
		case 'D':
			return gitdomain.PathStatus{Path: path, Status: gitdomain.StatusDeleted}, nil
		case 'T':
			return gitdomain.PathStatus{Path: path, Status: gitdomain.StatusTypeChanged}, nil
		default:
			return gitdomain.PathStatus{}, errors.Errorf("encountered unexpected file status %q for file %q", status, path)
		}
	}

	if err := i.scanner.Err(); err != nil {
		return gitdomain.PathStatus{}, errors.Wrap(err, "failed to scan git diff output")
	}

	return gitdomain.PathStatus{}, io.EOF
}

func (i *changedFilesIterator) Close() error {
	return i.onceFuncCloser()
}

var _ git.ChangedFilesIterator = &changedFilesIterator{}
