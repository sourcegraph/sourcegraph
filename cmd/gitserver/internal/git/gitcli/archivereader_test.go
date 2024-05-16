package gitcli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func readFileContentsFromTar(t *testing.T, tr *tar.Reader, name string) string {
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		if h.Name == name {
			contents, err := io.ReadAll(tr)
			require.NoError(t, err)
			return string(contents)
		}
	}

	t.Fatalf("File %q not found in tar archive", name)
	return ""
}

func readFileContentsFromZip(t *testing.T, zr *zip.Reader, name string) string {
	f, err := zr.Open(name)
	if err != nil {
		t.Fatalf("File %q not found in zip archive", name)
	}

	contents, err := io.ReadAll(f)
	require.NoError(t, err)
	return string(contents)
}

func TestBuildArchiveArgs(t *testing.T) {
	t.Run("no paths", func(t *testing.T) {
		args := buildArchiveArgs(git.ArchiveFormatTar, "HEAD", nil)
		require.Equal(t, []Argument{FlagArgument{"--worktree-attributes"}, ValueFlagArgument{Flag: "--format", Value: "tar"}, SpecSafeValueArgument{"HEAD"}, FlagArgument{"--"}}, args)
	})

	t.Run("with paths", func(t *testing.T) {
		args := buildArchiveArgs(git.ArchiveFormatTar, "HEAD", []string{"file1", "file2"})
		require.Equal(t, []Argument{FlagArgument{"--worktree-attributes"}, ValueFlagArgument{Flag: "--format", Value: "tar"}, SpecSafeValueArgument{"HEAD"}, FlagArgument{"--"}, FlagArgument{":(literal)file1"}, FlagArgument{":(literal)file2"}}, args)
	})

	t.Run("zip adds -0", func(t *testing.T) {
		args := buildArchiveArgs(git.ArchiveFormatZip, "HEAD", nil)
		require.Equal(t, []Argument{FlagArgument{"--worktree-attributes"}, ValueFlagArgument{Flag: "--format", Value: "zip"}, FlagArgument{"-0"}, SpecSafeValueArgument{"HEAD"}, FlagArgument{"--"}}, args)
	})
}

func TestGitCLIBackend_ArchiveReader(t *testing.T) {
	ctx := context.Background()

	backend := BackendWithRepoCommands(t,
		"echo abcd > file1",
		"mkdir dir1",
		"echo efgh > dir1/file2",
		`echo ijkl > " file3"`,
		`echo mnop > "dir1/file with spaces"`,
		"echo qrst > 我的工作",
		"git add 我的工作",
		"git add file1",
		`git add " file3"`,
		"git add dir1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	commitID, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	t.Run("read simple tar archive", func(t *testing.T) {
		r, err := backend.ArchiveReader(ctx, "tar", string(commitID), nil)
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		tr := tar.NewReader(r)
		contents := readFileContentsFromTar(t, tr, "file1")
		require.Equal(t, "abcd\n", contents)
	})

	t.Run("read simple zip archive", func(t *testing.T) {
		r, err := backend.ArchiveReader(ctx, "zip", string(commitID), nil)
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		contents, err := io.ReadAll(r)
		require.NoError(t, err)
		zr, err := zip.NewReader(bytes.NewReader([]byte(contents)), int64(len(contents)))
		require.NoError(t, err)
		fileContents := readFileContentsFromZip(t, zr, "file1")
		require.Equal(t, "abcd\n", fileContents)
	})

	t.Run("read multiple files from tar archive using paths", func(t *testing.T) {
		r, err := backend.ArchiveReader(ctx, "tar", string(commitID), []string{"file1", "dir1/file2"})
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		tr := tar.NewReader(r)
		contents := readFileContentsFromTar(t, tr, "dir1/file2")
		require.Equal(t, "efgh\n", contents)
		r, err = backend.ArchiveReader(ctx, "tar", string(commitID), []string{"file1", "dir1/file2"})
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		tr = tar.NewReader(r)
		contents = readFileContentsFromTar(t, tr, "file1")
		require.Equal(t, "abcd\n", contents)
	})

	t.Run("read file in directory", func(t *testing.T) {
		r, err := backend.ArchiveReader(ctx, "tar", string(commitID), nil)
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		tr := tar.NewReader(r)
		contents := readFileContentsFromTar(t, tr, "dir1/file2")
		require.Equal(t, "efgh\n", contents)
	})

	t.Run("read file with space in name", func(t *testing.T) {
		r, err := backend.ArchiveReader(ctx, "tar", string(commitID), []string{" file3", "dir1/file with spaces"})
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		tr := tar.NewReader(r)
		contents := readFileContentsFromTar(t, tr, " file3")
		require.Equal(t, "ijkl\n", contents)

		r, err = backend.ArchiveReader(ctx, "tar", string(commitID), []string{" file3", "dir1/file with spaces"})
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		tr = tar.NewReader(r)
		contents = readFileContentsFromTar(t, tr, "dir1/file with spaces")
		require.Equal(t, "mnop\n", contents)
	})

	t.Run("read non-ascii filename", func(t *testing.T) {
		r, err := backend.ArchiveReader(ctx, "tar", string(commitID), []string{" file3", "我的工作"})
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		tr := tar.NewReader(r)
		contents := readFileContentsFromTar(t, tr, "我的工作")
		require.Equal(t, "qrst\n", contents)
	})

	t.Run("non existent commit", func(t *testing.T) {
		_, err := backend.ArchiveReader(ctx, "tar", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})

	t.Run("non existent ref", func(t *testing.T) {
		_, err := backend.ArchiveReader(ctx, "tar", "head-2", nil)
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})

	// Verify that if the context is canceled, the reader returns an error.
	t.Run("context cancelation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		r, err := backend.ArchiveReader(ctx, git.ArchiveFormatTar, string(commitID), nil)
		require.NoError(t, err)

		cancel()

		tr := tar.NewReader(r)
		_, err = tr.Next()
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "unexpected error: %v", err)

		require.True(t, errors.Is(r.Close(), context.Canceled), "unexpected error: %v", err)
	})
}
