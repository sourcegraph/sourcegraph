package gitcli

import (
	"context"
	"io"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (g *gitCLIBackend) WriteCommitGraph(ctx context.Context, replaceChain bool) error {
	r, err := g.NewCommand(ctx, WithArguments(buildCommitGraphArgs(replaceChain)...))
	if err != nil {
		return err
	}
	// There is no interesting output on stdout, so just discard it.
	_, err = io.Copy(io.Discard, r)
	return errors.Append(err, r.Close())
}

func buildCommitGraphArgs(replaceChain bool) []string {
	args := []string{
		"commit-graph",
		"write",
		"--reachable",
		"--changed-paths", // enable Bloom filters for faster history queries.
		"--size-multiple=4",
	}
	if replaceChain {
		args = append(args, "--split=replace")
	} else {
		args = append(args, "--split")
	}
	return args
}
