package gitcli

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGitCLIBackend_SymbolicRefHead(t *testing.T) {
	ctx := context.Background()

	for _, short := range []bool{false, true} {
		t.Run(fmt.Sprintf("short=%v", short), func(t *testing.T) {
			t.Run("resolves master", func(t *testing.T) {
				// Prepare repo state:
				backend := BackendWithRepoCommands(t,
					"echo 'hello world' > foo.txt",
					"git add foo.txt",
					"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
				)

				head, err := backend.SymbolicRefHead(ctx, short)
				require.NoError(t, err)
				if short {
					require.Equal(t, "master", head)
				} else {
					require.Equal(t, "refs/heads/master", head)
				}
			})

			t.Run("empty repo", func(t *testing.T) {
				// Prepare repo state:
				backend := BackendWithRepoCommands(t)

				_, err := backend.SymbolicRefHead(ctx, short)
				require.NoError(t, err)
			})
		})
	}
}

func TestGitCLIBackend_RevParseHead(t *testing.T) {
	ctx := context.Background()

	t.Run("resolves master", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo 'hello world' > foo.txt",
			"git add foo.txt",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		)

		head, err := backend.RevParseHead(ctx)
		require.NoError(t, err)
		require.Equal(t, api.CommitID("7ec733d1fd20d9d73db4d6df8939bef6bd9a057d"), head)
	})

	t.Run("empty repo", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t)

		_, err := backend.RevParseHead(ctx)
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})
}

func BenchmarkQuickRevParseHeadQuickSymbolicRefHead_packed_refs(b *testing.B) {
	tmp := b.TempDir()

	dir := filepath.Join(tmp, ".git")
	gitDir := common.GitDir(dir)
	if err := os.Mkdir(dir, 0o700); err != nil {
		b.Fatal(err)
	}

	masterRef := "refs/heads/master"
	// This simulates the most amount of work QuickRevParseHead has to do, and
	// is also the most common in prod. That is where the final rev is in
	// packed-refs.
	err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte(fmt.Sprintf("ref: %s\n", masterRef)), 0o600)
	if err != nil {
		b.Fatal(err)
	}
	// in prod the kubernetes repo has a packed-refs file that is 62446 lines
	// long. Simulate something like that with everything except master
	masterRev := "4d5092a09bca95e0153c423d76ef62d4fcd168ec"
	{
		f, err := os.Create(filepath.Join(dir, "packed-refs"))
		if err != nil {
			b.Fatal(err)
		}
		writeRef := func(refBase string, num int) {
			_, err := fmt.Fprintf(f, "%016x%016x%08x %s-%d\n", rand.Uint64(), rand.Uint64(), rand.Uint32(), refBase, num)
			if err != nil {
				b.Fatal(err)
			}
		}
		for i := range 32 {
			writeRef("refs/heads/feature-branch", i)
		}
		_, err = fmt.Fprintf(f, "%s refs/heads/master\n", masterRev)
		if err != nil {
			b.Fatal(err)
		}
		for i := range 10000 {
			// actual format is refs/pull/${i}/head, but doesn't actually
			// matter for testing
			writeRef("refs/pull/head", i)
			writeRef("refs/pull/merge", i)
		}
		err = f.Close()
		if err != nil {
			b.Fatal(err)
		}
	}

	// Exclude setup
	b.ResetTimer()

	for range b.N {
		rev, err := QuickRevParseHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if rev != masterRev {
			b.Fatal("unexpected rev: ", rev)
		}
		ref, err := QuickSymbolicRefHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if ref != masterRef {
			b.Fatal("unexpected ref: ", ref)
		}
	}

	// Exclude cleanup (defers)
	b.StopTimer()
}

func BenchmarkQuickRevParseHeadQuickSymbolicRefHead_unpacked_refs(b *testing.B) {
	tmp := b.TempDir()

	dir := filepath.Join(tmp, ".git")
	gitDir := common.GitDir(dir)
	if err := os.Mkdir(dir, 0o700); err != nil {
		b.Fatal(err)
	}

	// This simulates the usual case for a repo that HEAD is often
	// updated. The master ref will be unpacked.
	masterRef := "refs/heads/master"
	masterRev := "4d5092a09bca95e0153c423d76ef62d4fcd168ec"
	files := map[string]string{
		"HEAD":              fmt.Sprintf("ref: %s\n", masterRef),
		"refs/heads/master": masterRev + "\n",
	}
	for path, content := range files {
		path = filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(path), 0o700)
		if err != nil {
			b.Fatal(err)
		}
		err = os.WriteFile(path, []byte(content), 0o600)
		if err != nil {
			b.Fatal(err)
		}
	}

	// Exclude setup
	b.ResetTimer()

	for range b.N {
		rev, err := QuickRevParseHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if rev != masterRev {
			b.Fatal("unexpected rev: ", rev)
		}
		ref, err := QuickSymbolicRefHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if ref != masterRef {
			b.Fatal("unexpected ref: ", ref)
		}
	}

	// Exclude cleanup (defers)
	b.StopTimer()
}
