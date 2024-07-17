package gitcli

import (
	"bytes"
	"context"
	"io"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) ResolveRevision(ctx context.Context, spec string) (gitdomain.OID, error) {
	if spec == "" {
		spec = "HEAD"
	}
	if spec != "HEAD" {
		// "git rev-parse HEAD^0" is slower than "git rev-parse HEAD"
		// since it checks that the resolved git object exists. We can
		// assume it exists for HEAD, but for other commits we should
		// check.
		spec = spec + "^0"
	}

	if err := checkSpecArgSafety(spec); err != nil {
		return gitdomain.OID{}, err
	}

	return g.revParse(ctx, spec)
}

func (g *gitCLIBackend) revParse(ctx context.Context, spec string) (gitdomain.OID, error) {
	r, err := g.NewCommand(ctx, WithArguments("rev-parse", spec, "--"))
	if err != nil {
		return gitdomain.OID{}, err
	}

	stdout, err := io.ReadAll(r)
	if err != nil {
		var e *commandFailedError
		if errors.As(err, &e) && e.ExitStatus == 128 && (bytes.Contains(e.Stderr, []byte("bad revision")) ||
			bytes.Contains(e.Stderr, []byte("unknown revision")) ||
			bytes.Contains(e.Stderr, []byte("expected commit type"))) {
			return gitdomain.OID{}, &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: spec}
		}

		return gitdomain.OID{}, err
	}

	line, _, _ := bytes.Cut(stdout, []byte{'\n'})
	commit := api.CommitID(bytes.TrimSpace(line))
	if !gitdomain.IsAbsoluteRevision(string(commit)) {
		if commit == "HEAD" {
			// We don't verify the existence of HEAD (see above comments), but
			// if HEAD doesn't point to anything git just returns `HEAD` as the
			// output of rev-parse. An example where this occurs is an empty
			// repository.
			return gitdomain.OID{}, &gitdomain.RevisionNotFoundError{Repo: g.repoName, Spec: spec}
		}
		// TODO: When can this happen?
		badCommitErrorCounter.Inc()
		return gitdomain.OID{}, &gitdomain.BadCommitError{Spec: spec, Commit: commit, Repo: g.repoName}
	}
	return gitdomain.NewOID(commit)
}

// TODO: Remove, just temporary to check if BadCommitError ever happens in practice:
var (
	badCommitErrorCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_gitserver_bad_commit_error",
		Help: "number of times a bad commit error is returned",
	})
)
