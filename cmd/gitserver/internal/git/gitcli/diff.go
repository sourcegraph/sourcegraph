package gitcli

import (
	"bufio"
	"context"
	"io"
	"sync"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
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

	return g.NewCommand(ctx, "diff", WithArguments(buildRawDiffArgs(baseOID, headOID, typ, paths)...))
}

func buildRawDiffArgs(base, head api.CommitID, typ git.GitDiffComparisonType, paths []string) []Argument {
	var rangeType string
	switch typ {
	case git.GitDiffComparisonTypeIntersection:
		rangeType = "..."
	case git.GitDiffComparisonTypeOnlyInHead:
		rangeType = ".."
	}
	rangeSpec := string(base) + rangeType + string(head)

	args := []Argument{
		FlagArgument{"--find-renames"},
		FlagArgument{"--full-index"},
		FlagArgument{"--inter-hunk-context=3"},
		FlagArgument{"--no-prefix"},
		SpecSafeValueArgument{rangeSpec},
		FlagArgument{"--"},
	}
	for _, path := range paths {
		args = append(args, SpecSafeValueArgument{path})
	}

	return args
}

func (g *gitCLIBackend) ChangedFiles(ctx context.Context, base, head string) (git.ChangedFilesIterator, error) {
	args := []Argument{
		FlagArgument{"-r"},
		FlagArgument{"--root"},
		ValueFlagArgument{Flag: "--format", Value: "format:"},
		FlagArgument{"--no-prefix"},
		FlagArgument{"--name-status"},
		FlagArgument{"--no-renames"},
		FlagArgument{"-z"},
	}

	if base != "" {
		baseOID, err := g.revParse(ctx, base)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve base commit %q", base)
		}

		args = append(args, SpecSafeValueArgument{string(baseOID)})
	}

	headOID, err := g.revParse(ctx, head)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve head commit %q", head)
	}

	args = append(args, SpecSafeValueArgument{string(headOID)})

	rc, err := g.NewCommand(ctx, "diff-tree", WithArguments(args...))
	if err != nil {
		return nil, errors.Wrap(err, "failed to run git diff-tree command")
	}

	return newGitDiffIterator(rc), nil
}

func newGitDiffIterator(rc io.ReadCloser) git.ChangedFilesIterator {
	scanner := bufio.NewScanner(rc)
	scanner.Split(byteutils.ScanNullLines)

	closeChan := make(chan struct{})
	closer := sync.OnceValue(func() error {
		err := rc.Close()
		close(closeChan)

		return err
	})

	return &gitDiffIterator{
		rc:             rc,
		scanner:        scanner,
		closeChan:      closeChan,
		onceFuncCloser: closer,
	}
}

type gitDiffIterator struct {
	rc      io.ReadCloser
	scanner *bufio.Scanner

	closeChan      chan struct{}
	onceFuncCloser func() error
}

func (i *gitDiffIterator) Next() (gitdomain.PathStatus, error) {
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

func (i *gitDiffIterator) Close() error {
	return i.onceFuncCloser()
}

var _ git.ChangedFilesIterator = &gitDiffIterator{}
