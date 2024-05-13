package gitcli

import (
	"context"
	"os"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
)

func TestGitCLIBackend_WriteCommitGraph(t *testing.T) {
	// We create a simple repo and generate a commit graph for it.
	// This test is simply meant to verify that the command runs without
	// error and that the commit graph is written to the expected location.
	rcf := wrexec.NewNoOpRecordingCommandFactory()
	dir := RepoWithCommands(
		t,
		"echo 'hello world' > foo.txt",
		"git add foo.txt",
		"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
	)

	backend := NewBackend(logtest.Scoped(t), rcf, dir, api.RepoName(t.Name()))

	ctx := context.Background()
	err := backend.WriteCommitGraph(ctx, true)
	require.NoError(t, err)

	// Verify that the commit graph index file exists.
	commitGraphPath := dir.Path("objects", "info", "commit-graphs", "commit-graph-chain")
	_, err = os.Stat(commitGraphPath)
	require.NoError(t, err)

	// Now run without replace and check that it also succeeds and the graph
	// files still exist.
	err = backend.WriteCommitGraph(ctx, false)
	require.NoError(t, err)

	// Verify that the commit graph index file exists.
	_, err = os.Stat(commitGraphPath)
	require.NoError(t, err)
}

func TestBuildCommitGraphArgs(t *testing.T) {
	t.Run("with replace chain", func(t *testing.T) {
		args := buildCommitGraphArgs(true)
		require.Equal(t, []string{
			"commit-graph",
			"write",
			"--reachable",
			"--changed-paths",
			"--size-multiple=4",
			"--split=replace",
		}, args)
	})

	t.Run("without replace chain", func(t *testing.T) {
		args := buildCommitGraphArgs(false)
		require.Equal(t, []string{
			"commit-graph",
			"write",
			"--reachable",
			"--changed-paths",
			"--size-multiple=4",
			"--split",
		}, args)
	})
}
