package gitcli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/mail"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) ContributorCounts(ctx context.Context, opt git.ContributorCountsOpts) ([]*gitdomain.ContributorCount, error) {
	if opt.Range == "" {
		opt.Range = "HEAD"
	}

	args := []Argument{
		FlagArgument{"--summary"},
		FlagArgument{"--numbered"},
		FlagArgument{"--email"},
		FlagArgument{"--no-merges"},
	}
	if !opt.After.IsZero() {
		args = append(args, FlagArgument{fmt.Sprintf("--after=%d", opt.After.Unix())})
	}
	args = append(args, SpecSafeValueArgument{opt.Range}, FlagArgument{"--"})
	if opt.Path != "" {
		args = append(args, SpecSafeValueArgument{opt.Path})
	}

	r, err := g.NewCommand(ctx, "shortlog", WithArguments(args...))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	counts, err := parseShortLog(r)
	if err != nil {
		// If exit code is 128 and `fatal: bad object` is part of stderr, most likely we
		// are referencing a range that does not exist.
		// We want to return a gitdomain.RevisionNotFoundError in that case.
		var e *CommandFailedError
		if errors.As(err, &e) && e.ExitStatus == 128 && (bytes.Contains(e.Stderr, []byte("fatal: bad object")) ||
			bytes.Contains(e.Stderr, []byte("fatal: bad revision"))) {
			return nil, &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: string(opt.Range)}
		}

		return nil, err
	}
	return counts, nil
}

func parseShortLog(stdout io.Reader) ([]*gitdomain.ContributorCount, error) {
	counts := []*gitdomain.ContributorCount{}
	sc := bufio.NewScanner(stdout)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		// example line: "1125\tJane Doe <jane@sourcegraph.com>"
		count, author, found := bytes.Cut(line, []byte("\t"))
		if !found {
			return nil, errors.Newf("invalid git shortlog line: %q", line)
		}
		countI, err := strconv.Atoi(string(bytes.TrimSpace(count)))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid git shortlog line: %q", line)
		}
		addr, err := lenientParseAddress(string(author))
		if err != nil || addr == nil {
			addr = &mail.Address{Name: string(author)}
		}
		counts = append(counts, &gitdomain.ContributorCount{
			Count: int32(countI),
			Name:  addr.Name,
			Email: addr.Address,
		})
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	return counts, nil
}

// lenientParseAddress is just like mail.ParseAddress, except that it treats
// the following somewhat-common malformed syntax where a user has misconfigured
// their email address as their name:
//
//	foo@gmail.com <foo@gmail.com>
//
// As a valid name, whereas mail.ParseAddress would return an error:
//
//	mail: expected single address, got "<foo@gmail.com>"
func lenientParseAddress(address string) (*mail.Address, error) {
	addr, err := mail.ParseAddress(address)
	if err != nil && strings.Contains(err.Error(), "expected single address") {
		p := strings.LastIndex(address, "<")
		if p == -1 {
			return addr, err
		}
		return &mail.Address{
			Name:    strings.TrimSpace(address[:p]),
			Address: strings.Trim(address[p:], " <>"),
		}, nil
	}
	return addr, err
}
