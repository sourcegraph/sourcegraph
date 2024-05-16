package gitcli

import (
	"context"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGitCLIBackend_Blame(t *testing.T) {
	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		"echo 'hello\nworld\nfrom\nblame\n' > foo.txt",
		"git add foo.txt",
		"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		// Add a second commit with a different author.
		"echo 'hello\nworld\nfrom\nthe best blame\n' > foo.txt",
		"git add foo.txt",
		"git commit -m bar --author='Bar Author <bar@sourcegraph.com>'",
	)

	ctx := context.Background()

	commit, err := backend.RevParseHead(ctx)
	require.NoError(t, err)

	t.Run("bad input", func(t *testing.T) {
		// Bad commit triggers error.
		_, err := backend.Blame(ctx, "-very badarg", "foo.txt", git.BlameOptions{})
		require.Error(t, err)
	})

	t.Run("stream hunks", func(t *testing.T) {
		// Verify that the blame output is correct and that the hunk reader correctly
		// terminates.
		hr, err := backend.Blame(ctx, commit, "foo.txt", git.BlameOptions{})
		require.NoError(t, err)

		h, err := hr.Read()
		require.NoError(t, err)

		require.Equal(t, &gitdomain.Hunk{
			StartLine: 4,
			EndLine:   5,
			CommitID:  "53e63d6dd6e61a58369bbc637b0ead2ee58d993c",
			PreviousCommit: &gitdomain.PreviousCommit{
				CommitID: "51f8be07ed2090b76e77b096c9d0737fc8ac70f4",
				Filename: "foo.txt",
			},
			Author: gitdomain.Signature{
				Name:  "Bar Author",
				Email: "bar@sourcegraph.com",
				Date:  h.Author.Date, // Hard to compare.
			},
			Message:  "bar",
			Filename: "foo.txt",
		}, h)

		h, err = hr.Read()
		require.NoError(t, err)

		require.Equal(t, &gitdomain.Hunk{
			StartLine: 1,
			EndLine:   4,
			CommitID:  "51f8be07ed2090b76e77b096c9d0737fc8ac70f4",
			Author: gitdomain.Signature{
				Name:  "Foo Author",
				Email: "foo@sourcegraph.com",
				Date:  h.Author.Date, // Hard to compare.
			},
			Message:  "foo",
			Filename: "foo.txt",
		}, h)

		h, err = hr.Read()
		require.NoError(t, err)

		require.Equal(t, &gitdomain.Hunk{
			StartLine: 5,
			EndLine:   6,
			CommitID:  "51f8be07ed2090b76e77b096c9d0737fc8ac70f4",
			Author: gitdomain.Signature{
				Name:  "Foo Author",
				Email: "foo@sourcegraph.com",
				Date:  h.Author.Date, // Hard to compare.
			},
			Message:  "foo",
			Filename: "foo.txt",
		}, h)

		_, err = hr.Read()
		require.Equal(t, io.EOF, err)

		require.NoError(t, hr.Close())
	})

	// Verify that if the context is canceled, the hunk reader returns an error.
	t.Run("context cancelation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		hr, err := backend.Blame(ctx, commit, "foo.txt", git.BlameOptions{})
		require.NoError(t, err)

		cancel()

		_, err = hr.Read()
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "unexpected error: %v", err)

		require.True(t, errors.Is(hr.Close(), context.Canceled), "unexpected error: %v", err)
	})

	t.Run("commit not found", func(t *testing.T) {
		// Ambiguous ref, could be commit, could be a ref.
		_, err := backend.Blame(ctx, "deadbeef", "foo.txt", git.BlameOptions{})
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))

		// Definitely a commit (yes, those yield different errors from git).
		_, err = backend.Blame(ctx, "e3889dff4263a2273459471739aafabc10269885", "foo.txt", git.BlameOptions{})
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := backend.Blame(ctx, commit, "notfoundfile", git.BlameOptions{})
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})
}

func TestBuildBlameArgs(t *testing.T) {
	commit := "deadbeef"
	path := "foo.txt"

	t.Run("default options", func(t *testing.T) {
		want := []Argument{FlagArgument{"--porcelain"}, FlagArgument{"--incremental"}, SpecSafeValueArgument{commit}, FlagArgument{"--"}, SpecSafeValueArgument{"foo.txt"}}
		opt := git.BlameOptions{}
		got := buildBlameArgs(api.CommitID(commit), path, opt)
		require.Equal(t, want, got)
	})

	t.Run("with ignore whitespace", func(t *testing.T) {
		want := []Argument{FlagArgument{"--porcelain"}, FlagArgument{"--incremental"}, FlagArgument{"-w"}, SpecSafeValueArgument{commit}, FlagArgument{"--"}, SpecSafeValueArgument{"foo.txt"}}
		opt := git.BlameOptions{IgnoreWhitespace: true}
		got := buildBlameArgs(api.CommitID(commit), path, opt)
		require.Equal(t, want, got)
	})

	t.Run("with line range", func(t *testing.T) {
		want := []Argument{FlagArgument{"--porcelain"}, FlagArgument{"--incremental"}, FlagArgument{"-L5,10"}, SpecSafeValueArgument{commit}, FlagArgument{"--"}, SpecSafeValueArgument{"foo.txt"}}
		opt := git.BlameOptions{Range: &git.BlameRange{StartLine: 5, EndLine: 10}}
		got := buildBlameArgs(api.CommitID(commit), path, opt)
		require.Equal(t, want, got)
	})
}

// testGitBlameOutputIncremental is produced by running
//
//	git blame -w --porcelain release.sh
//
// `sourcegraph/src-cli`
var testGitBlameOutputIncremental = `8a75c6f8b4cbe2a2f3c8be0f2c50bc766499f498 15 15 1
author Adam Harvey
author-mail <adam@adamharvey.name>
author-time 1660860583
author-tz -0700
committer GitHub
committer-mail <noreply@github.com>
committer-time 1660860583
committer-tz +0000
summary release.sh: allow -rc.X suffixes (#829)
previous e6e03e850770dd0ba745f0fa4b23127e9d72ad30 release.sh
filename release.sh
fbb98e0b7ff0752798463d9f49d922858a4188f6 5 5 10
author Adam Harvey
author-mail <aharvey@sourcegraph.com>
author-time 1602630694
author-tz -0700
committer GitHub
committer-mail <noreply@github.com>
committer-time 1602630694
committer-tz -0700
summary release: add a prompt about DEVELOPMENT.md (#349)
previous 18f59760f4260518c29f0f07056245ed5d1d0f08 release.sh
filename release.sh
67b7b725a7ff913da520b997d71c840230351e30 10 20 1
author Thorsten Ball
author-mail <mrnugget@gmail.com>
author-time 1600334460
author-tz +0200
committer Thorsten Ball
committer-mail <mrnugget@gmail.com>
committer-time 1600334460
committer-tz +0200
summary Fix goreleaser GitHub action setup and release script
previous 6e931cc9745502184ce32d48b01f9a8706a4dfe8 release.sh
filename release.sh
67b7b725a7ff913da520b997d71c840230351e30 12 22 2
previous 6e931cc9745502184ce32d48b01f9a8706a4dfe8 release.sh
filename release.sh
3f61310114082d6179c23f75950b88d1842fe2de 1 1 4
author Thorsten Ball
author-mail <mrnugget@gmail.com>
author-time 1592827635
author-tz +0200
committer GitHub
committer-mail <noreply@github.com>
committer-time 1592827635
committer-tz +0200
summary Check that $VERSION is in MAJOR.MINOR.PATCH format in release.sh (#227)
previous ec809e79094cbcd05825446ee14c6d072466a0b7 release.sh
filename release.sh
3f61310114082d6179c23f75950b88d1842fe2de 6 16 4
previous ec809e79094cbcd05825446ee14c6d072466a0b7 release.sh
filename release.sh
3f61310114082d6179c23f75950b88d1842fe2de 10 21 1
previous ec809e79094cbcd05825446ee14c6d072466a0b7 release.sh
filename release.sh
`

// This test-data includes the boundary keyword, which is not present in the previous one.
var testGitBlameOutputIncremental2 = `bbca6551549492486ca1b0f8dee45553dd6aa6d7 16 16 1
author French Ben
author-mail <frenchben@docker.com>
author-time 1517407262
author-tz +0100
committer French Ben
committer-mail <frenchben@docker.com>
committer-time 1517407262
committer-tz +0100
summary Update error output to be clean
previous b7773ae218740a7be65057fc60b366a49b538a44 format.go
filename format.go
bbca6551549492486ca1b0f8dee45553dd6aa6d7 25 25 2
previous b7773ae218740a7be65057fc60b366a49b538a44 format.go
filename format.go
2c87fda17de1def6ea288141b8e7600b888e535b 15 15 1
author David Tolnay
author-mail <dtolnay@gmail.com>
author-time 1478451741
author-tz -0800
committer David Tolnay
committer-mail <dtolnay@gmail.com>
committer-time 1478451741
committer-tz -0800
summary Singular message for a single error
previous 8c5f0ad9360406a3807ce7de6bc73269a91a6e51 format.go
filename format.go
2c87fda17de1def6ea288141b8e7600b888e535b 17 17 2
previous 8c5f0ad9360406a3807ce7de6bc73269a91a6e51 format.go
filename format.go
31fee45604949934710ada68f0b307c4726fb4e8 1 1 14
author Mitchell Hashimoto
author-mail <mitchell.hashimoto@gmail.com>
author-time 1418673320
author-tz -0800
committer Mitchell Hashimoto
committer-mail <mitchell.hashimoto@gmail.com>
committer-time 1418673320
committer-tz -0800
summary Initial commit
boundary
filename format.go
31fee45604949934710ada68f0b307c4726fb4e8 15 19 6
filename format.go
31fee45604949934710ada68f0b307c4726fb4e8 23 27 1
filename format.go
`

var testGitBlameOutputHunks = []*gitdomain.Hunk{
	{
		StartLine: 1, EndLine: 5, StartByte: 0, EndByte: 41,
		CommitID: "3f61310114082d6179c23f75950b88d1842fe2de",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "ec809e79094cbcd05825446ee14c6d072466a0b7",
			Filename: "release.sh",
		},
		Author: gitdomain.Signature{
			Name:  "Thorsten Ball",
			Email: "mrnugget@gmail.com",
			Date:  mustParseTime(time.RFC3339, "2020-06-22T12:07:15Z"),
		},
		Message:  "Check that $VERSION is in MAJOR.MINOR.PATCH format in release.sh (#227)",
		Filename: "release.sh",
	},
	{
		StartLine: 5, EndLine: 15, StartByte: 41, EndByte: 249,
		CommitID: "fbb98e0b7ff0752798463d9f49d922858a4188f6",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "18f59760f4260518c29f0f07056245ed5d1d0f08",
			Filename: "release.sh",
		},
		Author: gitdomain.Signature{
			Name:  "Adam Harvey",
			Email: "aharvey@sourcegraph.com",
			Date:  mustParseTime(time.RFC3339, "2020-10-13T23:11:34Z"),
		},
		Message:  "release: add a prompt about DEVELOPMENT.md (#349)",
		Filename: "release.sh",
	},
	{
		StartLine: 15, EndLine: 16, StartByte: 249, EndByte: 328,
		CommitID: "8a75c6f8b4cbe2a2f3c8be0f2c50bc766499f498",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "e6e03e850770dd0ba745f0fa4b23127e9d72ad30",
			Filename: "release.sh",
		},
		Author: gitdomain.Signature{
			Name:  "Adam Harvey",
			Email: "adam@adamharvey.name",
			Date:  mustParseTime(time.RFC3339, "2022-08-18T22:09:43Z"),
		},
		Message:  "release.sh: allow -rc.X suffixes (#829)",
		Filename: "release.sh",
	},
	{
		StartLine: 16, EndLine: 20, StartByte: 328, EndByte: 394,
		CommitID: "3f61310114082d6179c23f75950b88d1842fe2de",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "ec809e79094cbcd05825446ee14c6d072466a0b7",
			Filename: "release.sh",
		},
		Author: gitdomain.Signature{
			Name:  "Thorsten Ball",
			Email: "mrnugget@gmail.com",
			Date:  mustParseTime(time.RFC3339, "2020-06-22T12:07:15Z"),
		},
		Message:  "Check that $VERSION is in MAJOR.MINOR.PATCH format in release.sh (#227)",
		Filename: "release.sh",
	},
	{
		StartLine: 20, EndLine: 21, StartByte: 394, EndByte: 504,
		CommitID: "67b7b725a7ff913da520b997d71c840230351e30",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "6e931cc9745502184ce32d48b01f9a8706a4dfe8",
			Filename: "release.sh",
		},
		Author: gitdomain.Signature{
			Name:  "Thorsten Ball",
			Email: "mrnugget@gmail.com",
			Date:  mustParseTime(time.RFC3339, "2020-09-17T09:21:00Z"),
		},
		Message:  "Fix goreleaser GitHub action setup and release script",
		Filename: "release.sh",
	},
	{
		StartLine: 21, EndLine: 22, StartByte: 504, EndByte: 553,
		CommitID: "3f61310114082d6179c23f75950b88d1842fe2de",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "ec809e79094cbcd05825446ee14c6d072466a0b7",
			Filename: "release.sh",
		},
		Author: gitdomain.Signature{
			Name:  "Thorsten Ball",
			Email: "mrnugget@gmail.com",
			Date:  mustParseTime(time.RFC3339, "2020-06-22T12:07:15Z"),
		},
		Message:  "Check that $VERSION is in MAJOR.MINOR.PATCH format in release.sh (#227)",
		Filename: "release.sh",
	},
	{
		StartLine: 22, EndLine: 24, StartByte: 553, EndByte: 695,
		CommitID: "67b7b725a7ff913da520b997d71c840230351e30",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "6e931cc9745502184ce32d48b01f9a8706a4dfe8",
			Filename: "release.sh",
		},
		Author: gitdomain.Signature{
			Name:  "Thorsten Ball",
			Email: "mrnugget@gmail.com",
			Date:  mustParseTime(time.RFC3339, "2020-09-17T09:21:00Z"),
		},
		Message:  "Fix goreleaser GitHub action setup and release script",
		Filename: "release.sh",
	},
}

func TestBlameHunkReader(t *testing.T) {
	t.Run("OK matching hunks", func(t *testing.T) {
		rc := io.NopCloser(strings.NewReader(testGitBlameOutputIncremental))
		reader := newBlameHunkReader(rc)
		defer reader.Close()

		hunks := []*gitdomain.Hunk{}
		for {
			hunk, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("blameHunkReader.Read failed: %s", err)
			}
			hunks = append(hunks, hunk)
		}

		sortFn := func(x []*gitdomain.Hunk) func(i, j int) bool {
			return func(i, j int) bool {
				return x[i].Author.Date.After(x[j].Author.Date)
			}
		}

		// We're not giving back bytes, as the output of --incremental only gives back annotations.
		expectedHunks := make([]*gitdomain.Hunk, 0, len(testGitBlameOutputHunks))
		for _, h := range testGitBlameOutputHunks {
			dup := *h
			dup.EndByte = 0
			dup.StartByte = 0
			expectedHunks = append(expectedHunks, &dup)
		}

		// Sort expected hunks by the most recent first, as --incremental does.
		sort.SliceStable(expectedHunks, sortFn(expectedHunks))

		if d := cmp.Diff(expectedHunks, hunks); d != "" {
			t.Fatalf("unexpected hunks (-want, +got):\n%s", d)
		}
	})

	t.Run("OK parsing hunks", func(t *testing.T) {
		rc := io.NopCloser(strings.NewReader(testGitBlameOutputIncremental2))
		reader := newBlameHunkReader(rc)
		defer reader.Close()

		for {
			_, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("blameHunkReader.Read failed: %s", err)
			}
		}
	})
}

var testGitBlameMovedFile = `9b3fbcf3fd859a4fa7f97e6056138307c57fb949 39 39 1
author Petri-Johan Last
author-mail <petri.last@sourcegraph.com>
author-time 1712302218
author-tz +0200
committer Petri-Johan Last
committer-mail <petri.last@sourcegraph.com>
committer-time 1712302218
committer-tz +0200
summary Move commit
previous bae93ddeeba0cc0099c322e2e46f60ad368c6e37 blame_test.go
filename another_test.go
9b3fbcf3fd859a4fa7f97e6056138307c57fb949 111 111 1
previous bae93ddeeba0cc0099c322e2e46f60ad368c6e37 blame_test.go
filename another_test.go
9b3fbcf3fd859a4fa7f97e6056138307c57fb949 178 178 2
previous bae93ddeeba0cc0099c322e2e46f60ad368c6e37 blame_test.go
filename another_test.go
9b3fbcf3fd859a4fa7f97e6056138307c57fb949 190 190 1
previous bae93ddeeba0cc0099c322e2e46f60ad368c6e37 blame_test.go
filename another_test.go
bae93ddeeba0cc0099c322e2e46f60ad368c6e37 1 1 38
author Petri-Johan Last
author-mail <petri.last@sourcegraph.com>
author-time 1712302156
author-tz +0200
committer Petri-Johan Last
committer-mail <petri.last@sourcegraph.com>
committer-time 1712302156
committer-tz +0200
summary Initial commit
boundary
filename blame_test.go
bae93ddeeba0cc0099c322e2e46f60ad368c6e37 39 40 71
filename blame_test.go
bae93ddeeba0cc0099c322e2e46f60ad368c6e37 110 112 66
filename blame_test.go
bae93ddeeba0cc0099c322e2e46f60ad368c6e37 178 180 10
filename blame_test.go
bae93ddeeba0cc0099c322e2e46f60ad368c6e37 188 191 303
filename blame_test.go`

var testGitBlameMovedFileHunks = []*gitdomain.Hunk{
	{
		StartLine: 39, EndLine: 40, StartByte: 0, EndByte: 0,
		CommitID: "9b3fbcf3fd859a4fa7f97e6056138307c57fb949",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
			Filename: "blame_test.go",
		},
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302218, 0),
		},
		Message:  "Move commit",
		Filename: "another_test.go",
	},
	{
		StartLine: 111, EndLine: 112, StartByte: 0, EndByte: 0,
		CommitID: "9b3fbcf3fd859a4fa7f97e6056138307c57fb949",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
			Filename: "blame_test.go",
		},
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302218, 0),
		},
		Message:  "Move commit",
		Filename: "another_test.go",
	},
	{
		StartLine: 178, EndLine: 180, StartByte: 0, EndByte: 0,
		CommitID: "9b3fbcf3fd859a4fa7f97e6056138307c57fb949",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
			Filename: "blame_test.go",
		},
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302218, 0),
		},
		Message:  "Move commit",
		Filename: "another_test.go",
	},
	{
		StartLine: 190, EndLine: 191, StartByte: 0, EndByte: 0,
		CommitID: "9b3fbcf3fd859a4fa7f97e6056138307c57fb949",
		PreviousCommit: &gitdomain.PreviousCommit{
			CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
			Filename: "blame_test.go",
		},
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302218, 0),
		},
		Message:  "Move commit",
		Filename: "another_test.go",
	},
	{
		StartLine: 1, EndLine: 39, StartByte: 0, EndByte: 0,
		CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302156, 0),
		},
		Message:  "Initial commit",
		Filename: "blame_test.go",
	},
	{
		StartLine: 40, EndLine: 111, StartByte: 0, EndByte: 0,
		CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302156, 0),
		},
		Message:  "Initial commit",
		Filename: "blame_test.go",
	},
	{
		StartLine: 112, EndLine: 178, StartByte: 0, EndByte: 0,
		CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302156, 0),
		},
		Message:  "Initial commit",
		Filename: "blame_test.go",
	},
	{
		StartLine: 180, EndLine: 190, StartByte: 0, EndByte: 0,
		CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302156, 0),
		},
		Message:  "Initial commit",
		Filename: "blame_test.go",
	},
	{
		StartLine: 191, EndLine: 494, StartByte: 0, EndByte: 0,
		CommitID: "bae93ddeeba0cc0099c322e2e46f60ad368c6e37",
		Author: gitdomain.Signature{
			Name:  "Petri-Johan Last",
			Email: "petri.last@sourcegraph.com",
			Date:  time.Unix(1712302156, 0),
		},
		Message:  "Initial commit",
		Filename: "blame_test.go",
	},
}

func TestBlameHunkReader_moved_file(t *testing.T) {
	t.Run("OK matching hunks", func(t *testing.T) {
		rc := io.NopCloser(strings.NewReader(testGitBlameMovedFile))
		reader := newBlameHunkReader(rc)
		defer reader.Close()

		hunks := []*gitdomain.Hunk{}
		for {
			hunk, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				t.Fatalf("blameHunkReader.Read failed: %s", err)
			}
			hunks = append(hunks, hunk)
		}

		if d := cmp.Diff(testGitBlameMovedFileHunks, hunks); d != "" {
			t.Fatalf("unexpected hunks (-want, +got):\n%s", d)
		}
	})
}

func BenchmarkBlameBytes(b *testing.B) {
	for range b.N {
		rc := io.NopCloser(strings.NewReader(testGitBlameOutputIncremental2))
		reader := newBlameHunkReader(rc)

		for {
			_, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				b.Fatalf("blameHunkReader.Read failed: %s", err)
			}
		}

		reader.Close()
	}
	b.ReportAllocs()
}

func mustParseTime(layout, value string) time.Time {
	tm, err := time.Parse(layout, value)
	if err != nil {
		panic(err.Error())
	}
	return tm
}
