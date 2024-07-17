package gitcli

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGitCLIBackend_ReadFile(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		// simple file
		"echo abcd > file1",
		"git add file1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",

		// test we handle file names with .. (git show by default interprets
		// this). Ensure past the .. exists as a branch. Then if we use git
		// show it would return a diff instead of file contents.
		"mkdir subdir",
		"echo old > subdir/name",
		"echo old > subdir/name..dev",
		"git add subdir",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
		"echo dotdot > subdir/name..dev",
		"git add subdir",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
		"git branch dev",
	)

	commitID, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	t.Run("read simple file", func(t *testing.T) {
		r, err := backend.ReadFile(ctx, commitID, "file1")
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		contents, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "abcd\n", string(contents))
	})

	t.Run("non existent file", func(t *testing.T) {
		_, err := backend.ReadFile(ctx, commitID, "filexyz")
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("non existent commit", func(t *testing.T) {
		_, err := backend.ReadFile(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "file1")
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	t.Run("special file paths", func(t *testing.T) {
		// File with .. in path name:
		{
			r, err := backend.ReadFile(ctx, commitID, "subdir/name..dev")
			require.NoError(t, err)
			t.Cleanup(func() { r.Close() })
			contents, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "dotdot\n", string(contents))
		}
		// File with .. in path name that doesn't exist:
		{
			_, err := backend.ReadFile(ctx, commitID, "subdir/404..dev")
			require.Error(t, err)
			require.True(t, os.IsNotExist(err))
		}
		// This test case ensures we do not return a log with diff for the
		// specially crafted "git show HASH:..branch". IE a way to bypass
		// sub-repo permissions.
		{
			_, err := backend.ReadFile(ctx, commitID, "..dev")
			require.Error(t, err)
			require.True(t, os.IsNotExist(err))
		}

		// 3 dots ... as a prefix when using git show will return an error like
		// error: object b5462a7c880ce339ba3f93ac343706c0fa35babc is a tree, not a commit
		// fatal: Invalid symmetric difference expression 269e2b9bda9a95ad4181a7a6eb2058645d9bad82:...dev
		{
			_, err := backend.ReadFile(ctx, commitID, "...dev")
			require.Error(t, err)
			require.True(t, os.IsNotExist(err))
		}
	})

	t.Run("submodule", func(t *testing.T) {
		submodDir := RepoWithCommands(t,
			// simple file
			"echo abcd > file1",
			"git add file1",
			"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
		)

		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			// simple file
			"echo abcd > file1",
			"git add file1",
			"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",

			// Add submodule
			"git -c protocol.file.allow=always submodule add "+filepath.ToSlash(string(submodDir))+" submod",
			"git commit -m 'add submodule' --author='Foo Author <foo@sourcegraph.com>'",
		)

		commitID, err := backend.RevParseHead(ctx)
		require.NoError(t, err)

		r, err := backend.ReadFile(ctx, commitID, "submod")
		require.NoError(t, err)
		t.Cleanup(func() { r.Close() })
		contents, err := io.ReadAll(r)
		require.NoError(t, err)
		// A submodule should read like an empty file for now.
		require.Equal(t, "", string(contents))
	})
}

func TestGitCLIBackend_ReadFile_GoroutineLeak(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		// simple file
		"echo abcd > file1",
		"git add file1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	commitID, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	routinesBefore := runtime.NumGoroutine()

	r, err := backend.ReadFile(ctx, commitID, "file1")
	require.NoError(t, err)

	// Read just a few bytes, but not enough to complete.
	buf := make([]byte, 2)
	n, err := r.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	// Don't complete reading all the output, instead, bail and close the reader.
	require.NoError(t, r.Close())

	time.Sleep(time.Millisecond)

	// Expect no leaked routines.
	routinesAfter := runtime.NumGoroutine()
	require.Equal(t, routinesBefore, routinesAfter)
}

func TestGitCLIBackend_GetCommit(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		// simple file
		"echo abcd > file1",
		"git add file1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
		"echo efgh > file2",
		"git add file2",
		"git commit -m commit2 --author='Foo Author <foo@sourcegraph.com>'",
	)

	commitID, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	t.Run("non existent commit", func(t *testing.T) {
		_, err := backend.GetCommit(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", false)
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	// This test only exists because we sometimes pass non-commit ID strings to the
	// api.CommitID input of GetCommit. Once we get to clean that up, we can remove
	// this test here.
	t.Run("bad revision", func(t *testing.T) {
		_, err := backend.GetCommit(ctx, "nonexisting", false)
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	// This test only exists because we sometimes pass non-commit ID strings to the
	// api.CommitID input of GetCommit. Once we get to clean that up, we can remove
	// this test here.
	t.Run("empty repo", func(t *testing.T) {
		backend := BackendWithRepoCommands(t)
		_, err := backend.GetCommit(ctx, "HEAD", false)
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	t.Run("read commit", func(t *testing.T) {
		c, err := backend.GetCommit(ctx, commitID, false)
		require.NoError(t, err)
		require.Equal(t, &git.GitCommitWithFiles{
			Commit: &gitdomain.Commit{
				ID:      commitID,
				Message: "commit2",
				Author: gitdomain.Signature{
					Name:  "Foo Author",
					Email: "foo@sourcegraph.com",
					Date:  c.Author.Date, // Hard to test
				},
				Committer: &gitdomain.Signature{
					Name:  "a",
					Email: "a@a.com",
					Date:  c.Committer.Date, // Hard to test
				},
				Parents: []api.CommitID{"405b565ed446e271bc1998a91dbf4fb50dbfabfe"},
			},
		}, c)
		// This should not exist but callers currently pass HEAD as a commitID
		// to gitserver. Until we get a handle on this, we want to verify that
		// this works properly.
		c, err = backend.GetCommit(ctx, "HEAD", false)
		require.NoError(t, err)
		require.Equal(t, &git.GitCommitWithFiles{
			Commit: &gitdomain.Commit{
				ID:      commitID,
				Message: "commit2",
				Author: gitdomain.Signature{
					Name:  "Foo Author",
					Email: "foo@sourcegraph.com",
					Date:  c.Author.Date, // Hard to test
				},
				Committer: &gitdomain.Signature{
					Name:  "a",
					Email: "a@a.com",
					Date:  c.Committer.Date, // Hard to test
				},
				Parents: []api.CommitID{"405b565ed446e271bc1998a91dbf4fb50dbfabfe"},
			},
		}, c)

		c2, err := backend.GetCommit(ctx, c.Parents[0], false)
		require.NoError(t, err)
		require.Equal(t, &git.GitCommitWithFiles{
			Commit: &gitdomain.Commit{
				ID:      c.Parents[0],
				Message: "commit",
				Author: gitdomain.Signature{
					Name:  "Foo Author",
					Email: "foo@sourcegraph.com",
					Date:  c2.Author.Date, // Hard to test
				},
				Committer: &gitdomain.Signature{
					Name:  "a",
					Email: "a@a.com",
					Date:  c2.Committer.Date, // Hard to test
				},
				Parents: nil,
			},
		}, c2)
	})

	t.Run("include modified files", func(t *testing.T) {
		c, err := backend.GetCommit(ctx, commitID, true)
		require.NoError(t, err)
		require.Equal(t, &git.GitCommitWithFiles{
			Commit: &gitdomain.Commit{
				ID:      commitID,
				Message: "commit2",
				Author: gitdomain.Signature{
					Name:  "Foo Author",
					Email: "foo@sourcegraph.com",
					Date:  c.Author.Date, // Hard to test
				},
				Committer: &gitdomain.Signature{
					Name:  "a",
					Email: "a@a.com",
					Date:  c.Committer.Date, // Hard to test
				},
				Parents: []api.CommitID{"405b565ed446e271bc1998a91dbf4fb50dbfabfe"},
			},
			ModifiedFiles: []string{"file2"},
		}, c)
	})
}

func TestRepository_FirstEverCommit(t *testing.T) {
	testCases := []struct {
		commitDates []string
		want        string
	}{
		{
			commitDates: []string{
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:05Z",
				"2008-01-02T15:04:05Z",
			},
			want: "2006-01-02T15:04:05Z",
		},
		{
			commitDates: []string{
				"2007-01-02T15:04:05Z", // Don't think this is possible, but if it is we still want the first commit (not strictly "oldest")
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:06Z",
			},
			want: "2007-01-02T15:04:05Z",
		},
	}

	t.Run("basic", func(t *testing.T) {
		for _, tc := range testCases {
			ctx := context.Background()

			gitCommands := make([]string, len(tc.commitDates))
			for i, date := range tc.commitDates {
				gitCommands[i] = fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=%s git commit --allow-empty -m foo --author='a <a@a.com>'", date)
			}

			backend := BackendWithRepoCommands(
				t,
				gitCommands...,
			)

			id, err := backend.FirstEverCommit(ctx)
			if err != nil {
				t.Fatal(err)
			}

			commit, err := backend.GetCommit(ctx, api.CommitID(id.String()), false)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.want, commit.Committer.Date.Format(time.RFC3339)); diff != "" {
				t.Fatalf("unexpected commit date (-want +got):\n%s", diff)
			}
		}
	})

	// Added for awareness if this error message changes.
	// Insights skip over empty repos and check against this error type
	t.Run("empty repo", func(t *testing.T) {
		backend := BackendWithRepoCommands(
			t,
		)

		_, err := backend.FirstEverCommit(context.Background())

		var repoErr *gitdomain.RevisionNotFoundError
		if !errors.As(err, &repoErr) {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGitCLIBackend_GetBehindAhead(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		// This is the commit graph we are creating
		//
		//         +-----> 3  -----> 4           (branch1)
		//         |
		//         |
		// 0 ----> 1  ----> 2 -----> 5  -----> 6  (master)
		//
		"echo abcd > file0",
		"git add file0",
		"git commit -m commit0 --author='Foo Author <foo@sourcegraph.com>'",

		"echo abcd > file1",
		"git add file1",
		"git commit -m commit1 --author='Foo Author <foo@sourcegraph.com>'",

		"git branch branch1",

		"echo efgh > file2",
		"git add file2",
		"git commit -m commit2 --author='Foo Author <foo@sourcegraph.com>'",

		"git checkout branch1",

		"echo ijkl > file3",
		"git add file3",
		"git commit -m commit3 --author='Foo Author <foo@sourcegraph.com>'",

		"echo ijkl > file4",
		"git add file4",
		"git commit -m commit4 --author='Foo Author <foo@sourcegraph.com>'",

		"git checkout master",

		"echo ijkl > file5",
		"git add file5",
		"git commit -m commit5 --author='Foo Author <foo@sourcegraph.com>'",

		"echo ijkl > file6",
		"git add file6",
		"git commit -m commit6 --author='Foo Author <foo@sourcegraph.com>'",
	)

	left := "branch1"
	right := "master"

	t.Run("valid branches", func(t *testing.T) {
		behindAhead, err := backend.BehindAhead(ctx, left, right)
		require.NoError(t, err)
		require.Equal(t, &gitdomain.BehindAhead{Behind: 2, Ahead: 3}, behindAhead)
	})

	t.Run("missing left branch", func(t *testing.T) {
		_, err := backend.BehindAhead(ctx, left, "")
		require.NoError(t, err) // Should compare to HEAD
	})

	t.Run("missing right branch", func(t *testing.T) {
		_, err := backend.BehindAhead(ctx, "", right)
		require.NoError(t, err) // Should compare to HEAD
	})

	t.Run("invalid left branch", func(t *testing.T) {
		_, err := backend.BehindAhead(ctx, "invalid-branch", right)
		require.Error(t, err)
		var e *gitdomain.RevisionNotFoundError
		require.True(t, errors.As(err, &e))
	})

	t.Run("invalid right branch", func(t *testing.T) {
		_, err := backend.BehindAhead(ctx, left, "invalid-branch")
		require.Error(t, err)
		var e *gitdomain.RevisionNotFoundError
		require.True(t, errors.As(err, &e))
	})

	t.Run("same branch", func(t *testing.T) {
		behindAhead, err := backend.BehindAhead(ctx, left, left)
		require.NoError(t, err)
		require.Equal(t, &gitdomain.BehindAhead{Behind: 0, Ahead: 0}, behindAhead)
	})

	t.Run("invalid object id", func(t *testing.T) {
		_, err := backend.BehindAhead(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", right)
		require.Error(t, err)
		var e *gitdomain.RevisionNotFoundError
		require.True(t, errors.As(err, &e))
	})
}
func TestGitCLIBackend_Stat(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	submodDir := RepoWithCommands(t,
		// simple file
		"echo abcd > file1",
		"git add file1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	backend := BackendWithRepoCommands(t,
		"echo abcd > file1",
		"git add file1",
		"mkdir nested",
		"echo efgh > nested/file",
		"git add nested/file",
		"ln -s nested/file link",
		"git add link",
		"git -c protocol.file.allow=always submodule add "+filepath.ToSlash(string(submodDir))+" submodule",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
		"echo defg > file2",
		"git add file2",
		"git commit -m commit2 --author='Foo Author <foo@sourcegraph.com>'",
	)

	commitID, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	c, err := backend.GetCommit(ctx, commitID, false)
	require.NoError(t, err)
	require.Equal(t, 1, len(c.Parents))

	t.Run("non existent file", func(t *testing.T) {
		_, err := backend.Stat(ctx, commitID, "file0")
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("file exists but not at commit", func(t *testing.T) {
		_, err := backend.Stat(ctx, commitID, "file2")
		require.NoError(t, err)

		_, err = backend.Stat(ctx, c.Parents[0], "file0")
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("non existent commit", func(t *testing.T) {
		_, err := backend.Stat(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "file1")
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	t.Run("stat root", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "",
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, ".")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "",
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
	})

	t.Run("stat file", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "file1")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "file1",
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, "nested/../file1")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "file1",
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, "/file1")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "file1",
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
	})

	t.Run("stat symlink", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "link")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "link",
			Size_: 11,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.False(t, fi.IsDir())

		cfg, err := backend.(*gitCLIBackend).gitModulesConfig(ctx, commitID)
		require.NoError(t, err)
		require.Equal(t, config.Config{
			Sections: config.Sections{
				{
					Name: "submodule",
					Subsections: config.Subsections{
						{
							Name: "submodule",
							Options: config.Options{
								{
									Key:   "path",
									Value: "submodule",
								},
								{
									Key:   "url",
									Value: string(submodDir),
								},
							},
						},
					},
				},
			},
		}, cfg)
	})

	t.Run("stat submodule", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "submodule")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "submodule",
			Size_: 0,
			Sys_: gitdomain.Submodule{
				URL:      string(submodDir),
				Path:     "submodule",
				CommitID: "405b565ed446e271bc1998a91dbf4fb50dbfabfe",
			},
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.Equal(t, gitdomain.ModeSubmodule, fi.Mode())
		require.False(t, fi.Mode().IsRegular())
	})

	t.Run("stat dir", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "nested")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "nested",
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, "nested/")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "nested",
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
	})

	t.Run("stat nested file", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "nested/file")
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(&fileutil.FileInfo{
			Name_: "nested/file",
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
	})
}

func TestGitCLIBackend_Stat_specialchars(t *testing.T) {
	ctx := context.Background()

	backend := BackendWithRepoCommands(t,
		`touch ⊗.txt '".txt' \\.txt`,
		`git add ⊗.txt '".txt' \\.txt`,
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	commitID, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	fi, err := backend.Stat(ctx, commitID, "⊗.txt")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(&fileutil.FileInfo{
		Name_: "⊗.txt",
		Size_: 0,
		Sys_:  fi.Sys(),
	}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
	require.False(t, fi.IsDir())
	require.True(t, fi.Mode().IsRegular())
	fi, err = backend.Stat(ctx, commitID, `".txt`)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(&fileutil.FileInfo{
		Name_: `".txt`,
		Size_: 0,
		Sys_:  fi.Sys(),
	}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
	require.False(t, fi.IsDir())
	require.True(t, fi.Mode().IsRegular())
	fi, err = backend.Stat(ctx, commitID, `\.txt`)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(&fileutil.FileInfo{
		Name_: `\.txt`,
		Size_: 0,
		Sys_:  fi.Sys(),
	}, fi, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
	require.False(t, fi.IsDir())
	require.True(t, fi.Mode().IsRegular())
}

func TestGitCLIBackend_ReadDir(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	submodDir := RepoWithCommands(t,
		// simple file
		"echo abcd > file1",
		"git add file1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	backend := BackendWithRepoCommands(t,
		"echo abcd > file1",
		"git add file1",
		"mkdir nested",
		"echo efgh > nested/file",
		"git add nested/file",
		"ln -s nested/file link",
		"git add link",
		"git -c protocol.file.allow=always submodule add "+filepath.ToSlash(string(submodDir))+" submodule",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	commitID, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	t.Run("bad input", func(t *testing.T) {
		_, err := backend.ReadDir(ctx, "-commit", "file", false)
		require.Error(t, err)
	})

	t.Run("non existent path", func(t *testing.T) {
		it, err := backend.ReadDir(ctx, commitID, "404dir", false)
		require.NoError(t, err)
		t.Cleanup(func() { it.Close() })
		_, err = it.Next()
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("non existent tree-ish", func(t *testing.T) {
		it, err := backend.ReadDir(ctx, "notfound", "nested", false)
		require.NoError(t, err)
		t.Cleanup(func() { it.Close() })
		_, err = it.Next()
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))

		// Read no entries:
		it, err = backend.ReadDir(ctx, "notfound", "nested", false)
		require.NoError(t, err)
		err = it.Close()
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	t.Run("non existent commit", func(t *testing.T) {
		it, err := backend.ReadDir(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "nested", false)
		require.NoError(t, err)
		t.Cleanup(func() { it.Close() })
		_, err = it.Next()
		require.Error(t, err)

		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	t.Run("read root", func(t *testing.T) {
		it, err := backend.ReadDir(ctx, commitID, "", false)
		require.NoError(t, err)
		fis := make([]fs.FileInfo, 0)
		for {
			fi, err := it.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			fis = append(fis, fi)
		}
		require.Empty(t, cmp.Diff([]fs.FileInfo{
			&fileutil.FileInfo{
				Name_: ".gitmodules",
				Size_: fis[0].Size(),
				Sys_:  fis[0].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "file1",
				Size_: 5,
				Sys_:  fis[1].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "link",
				Size_: 11,
				Sys_:  fis[2].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "nested",
				Size_: 0,
				Sys_:  fis[3].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "submodule",
				Size_: 0,
				Sys_:  fis[4].Sys(),
			},
		}, fis, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.True(t, fis[3].IsDir())
		it, err = backend.ReadDir(ctx, commitID, ".", false)
		require.NoError(t, err)
		dotFis := make([]fs.FileInfo, 0)
		for {
			fi, err := it.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			dotFis = append(dotFis, fi)
		}
		require.Empty(t, cmp.Diff(fis, dotFis, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
	})

	t.Run("read root recursive", func(t *testing.T) {
		it, err := backend.ReadDir(ctx, commitID, "", true)
		require.NoError(t, err)
		fis := make([]fs.FileInfo, 0)
		for {
			fi, err := it.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			fis = append(fis, fi)
		}
		require.Empty(t, cmp.Diff([]fs.FileInfo{
			&fileutil.FileInfo{
				Name_: ".gitmodules",
				Size_: fis[0].Size(),
				Sys_:  fis[0].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "file1",
				Size_: 5,
				Sys_:  fis[1].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "link",
				Size_: 11,
				Sys_:  fis[2].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "nested",
				Size_: 0,
				Sys_:  fis[3].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "nested/file",
				Size_: 5,
				Sys_:  fis[4].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "submodule",
				Size_: 0,
				Sys_:  fis[5].Sys(),
			},
		}, fis, cmpopts.IgnoreFields(fileutil.FileInfo{}, "Mode_")))
		require.True(t, fis[3].IsDir())
	})
}

func TestLogPartsPerCommitInSync(t *testing.T) {
	require.Equal(t, partsPerCommit-1, strings.Count(logFormatWithoutRefs, "%x00"))
}

func TestGitCLIBackend_fileSizeReader(t *testing.T) {
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		"echo abcd > file1",
		"git add file1",
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	sizeReader, err := backend.(*gitCLIBackend).newFileSizeReader(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		sizeReader.Close()
	})

	it, err := backend.ReadDir(ctx, "HEAD", "", false)
	t.Cleanup(func() {
		it.Close()
	})
	require.NoError(t, err)
	fd, err := it.Next()
	require.NoError(t, err)
	oid := fd.Sys().(objectInfo)

	size, err := sizeReader.Size(api.CommitID(oid.OID().String()))
	require.NoError(t, err)
	// "abcd\n" -> 5 bytes
	require.Equal(t, int64(5), size)
}
