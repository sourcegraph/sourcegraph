package gitcli

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/format/config"
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
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
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
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
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

	t.Run("bad input", func(t *testing.T) {
		_, err := backend.Stat(ctx, commitID, "-file0")
		require.Error(t, err)
		require.Contains(t, err.Error(), "(begins with '-')")
	})

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
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})

	t.Run("stat root", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "",
			Mode_: os.ModeDir,
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi)
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, ".")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "",
			Mode_: os.ModeDir,
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi)
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
	})

	t.Run("stat file", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "file1")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "file1",
			Mode_: gitdomain.ModeFile,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, "nested/../file1")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "file1",
			Mode_: gitdomain.ModeFile,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, "/file1")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "file1",
			Mode_: gitdomain.ModeFile,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
	})

	t.Run("stat symlink", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "link")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "link",
			Mode_: gitdomain.ModeSymlink,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())

		// TODO: test for not found.
		cfg, err := backend.(*gitCLIBackend).gitModulesConfig(ctx, commitID)
		require.NoError(t, err)
		require.Equal(t, config.Config{
			Sections: config.Sections{
				{},
			},
		}, cfg)
	})

	t.Run("stat submodule", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "submodule")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "submodule",
			Mode_: gitdomain.ModeSubmodule,
			Size_: 0,
			Sys_: gitdomain.Submodule{
				URL:      string(submodDir),
				Path:     "submodule",
				CommitID: "405b565ed446e271bc1998a91dbf4fb50dbfabfe",
			},
		}, fi)
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
	})

	t.Run("stat dir", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "nested")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "nested",
			Mode_: os.ModeDir & gitdomain.ModeDir,
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi)
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, "nested/")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "nested",
			Mode_: os.ModeDir & gitdomain.ModeDir,
			Size_: 0,
			Sys_:  fi.Sys(),
		}, fi)
		require.True(t, fi.IsDir())
		require.False(t, fi.Mode().IsRegular())
	})

	t.Run("stat nested file", func(t *testing.T) {
		fi, err := backend.Stat(ctx, commitID, "nested/file")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "nested/file",
			Mode_: gitdomain.ModeFile,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
	})
}

func TestGitCLIBackend_Stat_specialchars(t *testing.T) {
	ctx := context.Background()

	backendQuoteChars := BackendWithRepoCommands(t,
		"git config core.quotepath on",
		`touch ⊗.txt '".txt' \\.txt`,
		`git add ⊗.txt '".txt' \\.txt`,
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)
	backendNoQuoteChars := BackendWithRepoCommands(t,
		"git config core.quotepath off",
		`touch ⊗.txt '".txt' \\.txt`,
		`git add ⊗.txt '".txt' \\.txt`,
		"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
	)

	commitID, err := backendQuoteChars.RevParseHead(ctx)
	require.NoError(t, err)

	test := func(t *testing.T, backend git.GitBackend) {
		fi, err := backend.Stat(ctx, commitID, "⊗.txt")
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: "⊗.txt",
			Mode_: gitdomain.ModeFile,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, `".txt`)
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: `".txt`,
			Mode_: gitdomain.ModeFile,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
		fi, err = backend.Stat(ctx, commitID, `\.txt`)
		require.NoError(t, err)
		require.Equal(t, &fileutil.FileInfo{
			Name_: `\.txt`,
			Mode_: gitdomain.ModeFile,
			Size_: 5,
			Sys_:  fi.Sys(),
		}, fi)
		require.False(t, fi.IsDir())
		require.True(t, fi.Mode().IsRegular())
	}

	t.Run("quote chars on", func(t *testing.T) {
		test(t, backendQuoteChars)
	})

	t.Run("quote chars off", func(t *testing.T) {
		test(t, backendNoQuoteChars)
	})
}

// TODO: Test for `directory` and `directory/`.
// TODO: Test for root as `.`.
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
		_, err := backend.ReadDir(ctx, commitID, "-file0", false)
		require.Error(t, err)
		require.Contains(t, err.Error(), "(begins with '-')")
	})

	t.Run("non existent path", func(t *testing.T) {
		_, err := backend.ReadDir(ctx, commitID, "404dir", false)
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("non existent commit", func(t *testing.T) {
		_, err := backend.ReadDir(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "nested", false)
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
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
		require.Equal(t, []fs.FileInfo{
			&fileutil.FileInfo{
				Name_: ".gitmodules",
				Mode_: gitdomain.ModeFile,
				Size_: 148,
				Sys_:  fis[0].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "file1",
				Mode_: gitdomain.ModeFile,
				Size_: 0,
				Sys_:  fis[1].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "nested",
				Mode_: os.ModeDir,
				Size_: 0,
				Sys_:  fis[2].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "link",
				Mode_: gitdomain.ModeSymlink,
				Size_: 0,
				Sys_:  fis[3].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "submodule",
				Mode_: gitdomain.ModeSubmodule,
				Size_: 0,
				Sys_:  fis[4].Sys(),
			},
		}, fis)
		require.True(t, fis[1].IsDir())
		dotFis, err := backend.ReadDir(ctx, commitID, ".", false)
		require.NoError(t, err)
		require.Equal(t, fis, dotFis)
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
		require.Equal(t, []fs.FileInfo{
			&fileutil.FileInfo{
				Name_: ".gitmodules",
				Mode_: gitdomain.ModeFile,
				Size_: 148,
				Sys_:  fis[0].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "file1",
				Mode_: gitdomain.ModeFile,
				Size_: 5,
				Sys_:  fis[1].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "nested",
				Mode_: os.ModeDir,
				Size_: 0,
				Sys_:  fis[2].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "nested/file",
				Mode_: gitdomain.ModeFile,
				Size_: 5,
				Sys_:  fis[3].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "link",
				Mode_: gitdomain.ModeSymlink,
				Size_: 0,
				Sys_:  fis[4].Sys(),
			},
			&fileutil.FileInfo{
				Name_: "submodule",
				Mode_: gitdomain.ModeSubmodule,
				Size_: 0,
				Sys_:  fis[5].Sys(),
			},
		}, fis)
		require.True(t, fis[1].IsDir())
	})

	// t.Run("stat file", func(t *testing.T) {
	// 	fi, err := backend.ReadDir(ctx, commitID, "file1")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "file1",
	// 		Mode_: gitdomain.ModeFile,
	// 		Size_: 5,
	// 		Sys_: fi.Sys(),
	// 	}, fi)
	// 	require.False(t, fi.IsDir())
	// require.True(t, fi.Mode().IsRegular())
	// 	fi, err = backend.ReadDir(ctx, commitID, "nested/../file1")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "file1",
	// 		Mode_: gitdomain.ModeFile,
	// 		Size_: 5,
	// 		Sys_: fi.Sys(),
	// 	}, fi)
	// 	require.False(t, fi.IsDir())
	// require.True(t, fi.Mode().IsRegular())
	// 	fi, err = backend.ReadDir(ctx, commitID, "/file1")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "file1",
	// 		Mode_: gitdomain.ModeFile,
	// 		Size_: 5,
	// 		Sys_: fi.Sys(),
	// 	}, fi)
	// 	require.False(t, fi.IsDir())
	// require.True(t, fi.Mode().IsRegular())
	// })

	// t.Run("stat symlink", func(t *testing.T) {
	// 	fi, err := backend.ReadDir(ctx, commitID, "link")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "link",
	// 		Mode_: gitdomain.ModeSymlink,
	// 		Size_: 5,
	// 		Sys_: fi.Sys(),
	// 	}, fi)
	// 	require.False(t, fi.IsDir())
	// require.True(t, fi.Mode().IsRegular())

	// 	// TODO: test for not found.
	// 	cfg, err := backend.(*gitCLIBackend).gitModulesConfig(ctx, commitID)
	// 	require.NoError(t, err)
	// 	require.Equal(t, config.Config{
	// 		Sections: config.Sections{
	// 			{},
	// 		},
	// 	}, cfg)
	// })

	// t.Run("stat submodule", func(t *testing.T) {
	// 	fi, err := backend.ReadDir(ctx, commitID, "submodule")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "submodule",
	// 		Mode_: gitdomain.ModeSubmodule,
	// 		Size_: 0,
	// 		Sys_: gitdomain.Submodule{
	// 			URL:      string(submodDir),
	// 			Path:     "submodule",
	// 			CommitID: "405b565ed446e271bc1998a91dbf4fb50dbfabfe",
	// 		},
	// 	}, fi)
	// 	require.True(t, fi.IsDir())
	// require.False(t, fi.Mode().IsRegular())
	// })

	// t.Run("stat dir", func(t *testing.T) {
	// 	fi, err := backend.ReadDir(ctx, commitID, "nested")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "nested",
	// 		Mode_: os.ModeDir & gitdomain.ModeDir,
	// 		Size_: 0,
	// 		Sys_: fi.Sys(),
	// 	}, fi)
	// 	require.True(t, fi.IsDir())
	// require.False(t, fi.Mode().IsRegular())
	// 	fi, err = backend.ReadDir(ctx, commitID, "nested/")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "nested",
	// 		Mode_: os.ModeDir & gitdomain.ModeDir,
	// 		Size_: 0,
	// 		Sys_: fi.Sys(),
	// 	}, fi)
	// 	require.True(t, fi.IsDir())
	// require.False(t, fi.Mode().IsRegular())
	// })

	// t.Run("stat nested file", func(t *testing.T) {
	// 	fi, err := backend.ReadDir(ctx, commitID, "nested/file")
	// 	require.NoError(t, err)
	// 	require.Equal(t, &fileutil.FileInfo{
	// 		Name_: "nested/file",
	// 		Mode_: gitdomain.ModeFile,
	// 		Size_: 5,
	// 		Sys_: fi.Sys(),
	// 	}, fi)
	// 	require.False(t, fi.IsDir())
	// require.True(t, fi.Mode().IsRegular())
	// })
}
