package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
)

func (g *gitCLIBackend) RevAtTime(ctx context.Context, spec string, t time.Time) (gitdomain.OID, error) {
	// First, try to resolve the revspec so we can return a useful RevisionNotFound error
	sha, err := g.ResolveRevision(ctx, spec)
	if err != nil {
		return gitdomain.OID{}, err
	}

	key := revAtTimeCacheKey{g.repoName, sha, t}
	if entry, ok := g.caches.revAtTimeCache.Get(key); ok {
		return entry, nil
	}

	r, err := g.NewCommand(ctx, WithArguments(
		"log",
		"--format=format:%H", // only hash
		"--first-parent",     // linearize history
		fmt.Sprintf("--before=%d", t.Unix()),
		"--max-count=1", // only one commit
		sha.String(),
	))
	if err != nil {
		return gitdomain.OID{}, err
	}
	defer r.Close()

	stdout, err := io.ReadAll(r)
	if err != nil {
		return gitdomain.OID{}, err
	}

	commitOID, err := gitdomain.NewOID(api.CommitID(bytes.TrimSpace(stdout)))
	if err != nil {
		return gitdomain.OID{}, err
	}
	g.caches.revAtTimeCache.Add(key, commitOID)
	return commitOID, nil
}
