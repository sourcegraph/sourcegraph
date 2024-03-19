package gitserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/google/go-cmp/cmp"
	godiff "github.com/sourcegraph/go-diff/diff"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Generate a random archive format.
func (f ArchiveFormat) Generate(rand *rand.Rand, _ int) reflect.Value {
	choices := []ArchiveFormat{ArchiveFormatZip, ArchiveFormatTar}
	index := rand.Intn(len(choices))

	return reflect.ValueOf(choices[index])
}

func TestParseShortLog(t *testing.T) {
	tests := []struct {
		name    string
		input   string // in the format of `git shortlog -sne`
		want    []*gitdomain.ContributorCount
		wantErr error
	}{
		{
			name: "basic",
			input: `
  1125	Jane Doe <jane@sourcegraph.com>
   390	Bot Of Doom <bot@doombot.com>
`,
			want: []*gitdomain.ContributorCount{
				{
					Name:  "Jane Doe",
					Email: "jane@sourcegraph.com",
					Count: 1125,
				},
				{
					Name:  "Bot Of Doom",
					Email: "bot@doombot.com",
					Count: 390,
				},
			},
		},
		{
			name: "commonly malformed (email address as name)",
			input: `  1125	jane@sourcegraph.com <jane@sourcegraph.com>
   390	Bot Of Doom <bot@doombot.com>
`,
			want: []*gitdomain.ContributorCount{
				{
					Name:  "jane@sourcegraph.com",
					Email: "jane@sourcegraph.com",
					Count: 1125,
				},
				{
					Name:  "Bot Of Doom",
					Email: "bot@doombot.com",
					Count: 390,
				},
			},
		},
	}
	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			got, gotErr := parseShortLog([]byte(tst.input))
			if (gotErr == nil) != (tst.wantErr == nil) {
				t.Fatalf("gotErr %+v wantErr %+v", gotErr, tst.wantErr)
			}
			if !reflect.DeepEqual(got, tst.want) {
				t.Logf("got %q", got)
				t.Fatalf("want %q", tst.want)
			}
		})
	}
}

func TestDiffWithSubRepoFiltering(t *testing.T) {
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 1,
	})

	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()

	cmds := getGitCommandsWithFileLists([]string{"file0"}, []string{"file1", "file1.1"}, []string{"file2"}, []string{"file3", "file3.3"})
	checker := getTestSubRepoPermsChecker("file1.1", "file2")
	testCases := []struct {
		label               string
		extraGitCommands    []string
		expectedDiffFiles   []string
		expectedFileStat    *godiff.Stat
		rangeOverAllCommits bool
	}{
		{
			label:               "adding files",
			expectedDiffFiles:   []string{"file1", "file3", "file3.3"},
			expectedFileStat:    &godiff.Stat{Added: 3},
			rangeOverAllCommits: true,
		},
		{
			label: "changing filename",
			extraGitCommands: []string{
				"mv file1.1 file_can_access",
				"git add file_can_access",
				makeGitCommit("rename", 7),
			},
			expectedDiffFiles: []string{"file_can_access"},
			expectedFileStat:  &godiff.Stat{Added: 1},
		},
		{
			label: "file modified",
			extraGitCommands: []string{
				"echo new_file_content > file2",
				"echo more_new_file_content > file1",
				"git add file2",
				"git add file1",
				makeGitCommit("edit_files", 7),
			},
			expectedDiffFiles: []string{"file1"}, // file2 is updated but user doesn't have access
			expectedFileStat:  &godiff.Stat{Changed: 1},
		},
		{
			label: "diff for commit w/ no access returns empty result",
			extraGitCommands: []string{
				"echo new_file_content > file2",
				"git add file2",
				makeGitCommit("no_access", 7),
			},
			expectedDiffFiles: []string{},
			expectedFileStat:  &godiff.Stat{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			repo := MakeGitRepository(t, append(cmds, tc.extraGitCommands...)...)
			c := NewTestClient(t)
			commits, err := c.Commits(ctx, repo, CommitsOptions{})
			if err != nil {
				t.Fatalf("err fetching commits: %s", err)
			}
			baseCommit := commits[1]
			headCommit := commits[0]
			if tc.rangeOverAllCommits {
				baseCommit = commits[len(commits)-1]
			}

			c = c.WithChecker(checker)
			iter, err := c.Diff(ctx, DiffOptions{Base: string(baseCommit.ID), Head: string(headCommit.ID), Repo: repo})
			if err != nil {
				t.Fatalf("error fetching diff: %s", err)
			}
			defer iter.Close()

			stat := &godiff.Stat{}
			fileNames := make([]string, 0, 3)
			for {
				file, err := iter.Next()
				if err == io.EOF {
					break
				} else if err != nil {
					t.Error(err)
				}

				fileNames = append(fileNames, file.NewName)

				fileStat := file.Stat()
				stat.Added += fileStat.Added
				stat.Changed += fileStat.Changed
				stat.Deleted += fileStat.Deleted
			}
			if diff := cmp.Diff(fileNames, tc.expectedDiffFiles); diff != "" {
				t.Fatal(diff)
			}
			if diff := cmp.Diff(stat, tc.expectedFileStat); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestDiff(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid bases", func(t *testing.T) {
		for _, input := range []string{
			"",
			"-foo",
			".foo",
		} {
			t.Run("invalid base: "+input, func(t *testing.T) {
				i, err := NewClient("test").Diff(ctx, DiffOptions{Base: input})
				if i != nil {
					t.Errorf("unexpected non-nil iterator: %+v", i)
				}
				if err == nil {
					t.Error("unexpected nil error")
				}
			})
		}
	})

	t.Run("rangeSpec calculation", func(t *testing.T) {
		for _, tc := range []struct {
			opts DiffOptions
			want string
		}{
			{opts: DiffOptions{Base: "foo", Head: "bar"}, want: "foo...bar"},
		} {
			t.Run("rangeSpec: "+tc.want, func(t *testing.T) {
				c := NewMockClientWithExecReader(nil, func(_ context.Context, _ api.RepoName, args []string) (io.ReadCloser, error) {
					// The range spec is the sixth argument.
					if args[5] != tc.want {
						t.Errorf("unexpected rangeSpec: have: %s; want: %s", args[5], tc.want)
					}
					return nil, nil
				})
				_, _ = c.Diff(ctx, tc.opts)
			})
		}
	})

	t.Run("ExecReader error", func(t *testing.T) {
		c := NewMockClientWithExecReader(nil, func(_ context.Context, _ api.RepoName, args []string) (io.ReadCloser, error) {
			return nil, errors.New("ExecReader error")
		})
		i, err := c.Diff(ctx, DiffOptions{Base: "foo", Head: "bar"})
		if i != nil {
			t.Errorf("unexpected non-nil iterator: %+v", i)
		}
		if err == nil {
			t.Error("unexpected nil error")
		}
	})

	t.Run("success", func(t *testing.T) {
		const testDiffFiles = 3
		const testDiff = `diff --git INSTALL.md INSTALL.md
index e5af166..d44c3fc 100644
--- INSTALL.md
+++ INSTALL.md
@@ -3,10 +3,10 @@
 Line 1
 Line 2
 Line 3
-Line 4
+This is cool: Line 4
 Line 5
 Line 6
-Line 7
-Line 8
+Another Line 7
+Foobar Line 8
 Line 9
 Line 10
diff --git JOKES.md JOKES.md
index ea80abf..1b86505 100644
--- JOKES.md
+++ JOKES.md
@@ -4,10 +4,10 @@ Joke #1
 Joke #2
 Joke #3
 Joke #4
-Joke #5
+This is not funny: Joke #5
 Joke #6
-Joke #7
+This one is good: Joke #7
 Joke #8
-Joke #9
+Waffle: Joke #9
 Joke #10
 Joke #11
diff --git README.md README.md
index 9bd8209..d2acfa9 100644
--- README.md
+++ README.md
@@ -1,12 +1,13 @@
 # README

-Line 1
+Foobar Line 1
 Line 2
 Line 3
 Line 4
 Line 5
-Line 6
+Barfoo Line 6
 Line 7
 Line 8
 Line 9
 Line 10
+Another line
`

		testDiffFileNames := []string{
			"INSTALL.md",
			"JOKES.md",
			"README.md",
		}

		c := NewMockClientWithExecReader(nil, func(_ context.Context, _ api.RepoName, args []string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(testDiff)), nil
		})

		i, err := c.Diff(ctx, DiffOptions{Base: "foo", Head: "bar"})
		if i == nil {
			t.Error("unexpected nil iterator")
		}
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
		defer i.Close()

		count := 0
		for {
			diff, err := i.Next()
			if err == io.EOF {
				break
			} else if err != nil {
				t.Errorf("unexpected iteration error: %+v", err)
			}

			if diff.OrigName != testDiffFileNames[count] {
				t.Errorf("unexpected diff file name: have: %s; want: %s", diff.OrigName, testDiffFileNames[count])
			}
			count++
		}
		if count != testDiffFiles {
			t.Errorf("unexpected diff count: have %d; want %d", count, testDiffFiles)
		}
	})
}

func TestDiffPath(t *testing.T) {
	testDiff := `
diff --git a/foo.md b/foo.md
index 51a59ef1c..493090958 100644
--- a/foo.md
+++ b/foo.md
@@ -1 +1 @@
-this is my file content
+this is my file contnent
`
	t.Run("basic", func(t *testing.T) {
		checker := authz.NewMockSubRepoPermissionChecker()
		c := NewMockClientWithExecReader(checker, func(_ context.Context, _ api.RepoName, args []string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(testDiff)), nil
		})
		ctx := actor.WithActor(context.Background(), &actor.Actor{
			UID: 1,
		})
		hunks, err := c.DiffPath(ctx, "", "sourceCommit", "", "file")
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if len(hunks) != 1 {
			t.Errorf("unexpected hunks returned: %d", len(hunks))
		}
	})
	t.Run("with sub-repo permissions enabled", func(t *testing.T) {
		checker := authz.NewMockSubRepoPermissionChecker()
		ctx := actor.WithActor(context.Background(), &actor.Actor{
			UID: 1,
		})
		fileName := "foo"
		checker.EnabledFunc.SetDefaultHook(func() bool {
			return true
		})
		// User doesn't have access to this file
		checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
			if content.Path == fileName {
				return authz.None, nil
			}
			return authz.Read, nil
		})
		usePermissionsForFilePermissionsFunc(checker)
		c := NewMockClientWithExecReader(checker, func(_ context.Context, _ api.RepoName, args []string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(testDiff)), nil
		})
		hunks, err := c.DiffPath(ctx, "", "sourceCommit", "", fileName)
		if !os.IsNotExist(err) {
			t.Errorf("unexpected error: %s", err)
		}
		if hunks != nil {
			t.Errorf("expected DiffPath to return no results, got %v", hunks)
		}
	})
}

func TestLsFiles(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	runFileListingTest(t, func(ctx context.Context, checker authz.SubRepoPermissionChecker, repo api.RepoName, commit string) ([]string, error) {
		client := NewTestClient(t).WithChecker(checker)
		return client.LsFiles(ctx, repo, api.CommitID(commit))
	})
}

// runFileListingTest tests the specified function which must return a list of filenames and an error. The test first
// tests the basic case (all paths returned), then the case with sub-repo permissions specified.
func runFileListingTest(t *testing.T,
	listingFunctionToTest func(context.Context, authz.SubRepoPermissionChecker, api.RepoName, string) ([]string, error),
) {
	t.Helper()
	gitCommands := []string{
		"touch file1",
		"mkdir dir",
		"touch dir/file2",
		"touch dir/file3",
		"git add file1 dir/file2 dir/file3",
		"git commit -m commit1",
	}

	repo, dir := MakeGitRepositoryAndReturnDir(t, gitCommands...)
	headCommit := GetHeadCommitFromGitDir(t, dir)
	ctx := context.Background()

	checker := authz.NewMockSubRepoPermissionChecker()
	// Start disabled
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return false
	})

	files, err := listingFunctionToTest(ctx, checker, repo, headCommit)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"dir/file2", "dir/file3", "file1",
	}
	if diff := cmp.Diff(want, files); diff != "" {
		t.Fatal(diff)
	}

	// With filtering
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if content.Path == "dir/file2" {
			return authz.Read, nil
		}
		return authz.None, nil
	})
	usePermissionsForFilePermissionsFunc(checker)
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 1,
	})
	files, err = listingFunctionToTest(ctx, checker, repo, headCommit)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{
		"dir/file2",
	}
	if diff := cmp.Diff(want, files); diff != "" {
		t.Fatal(diff)
	}
}

func TestParseDirectoryChildrenRoot(t *testing.T) {
	dirnames := []string{""}
	paths := []string{
		".github",
		".gitignore",
		"LICENSE",
		"README.md",
		"cmd",
		"go.mod",
		"go.sum",
		"internal",
		"protocol",
	}

	expected := map[string][]string{
		"": paths,
	}

	if diff := cmp.Diff(expected, parseDirectoryChildren(dirnames, paths)); diff != "" {
		t.Errorf("unexpected directory children result (-want +got):\n%s", diff)
	}
}

func TestParseDirectoryChildrenNonRoot(t *testing.T) {
	dirnames := []string{"cmd/", "protocol/", "cmd/protocol/"}
	paths := []string{
		"cmd/lsif-go",
		"protocol/protocol.go",
		"protocol/writer.go",
	}

	expected := map[string][]string{
		"cmd/":          {"cmd/lsif-go"},
		"protocol/":     {"protocol/protocol.go", "protocol/writer.go"},
		"cmd/protocol/": nil,
	}

	if diff := cmp.Diff(expected, parseDirectoryChildren(dirnames, paths)); diff != "" {
		t.Errorf("unexpected directory children result (-want +got):\n%s", diff)
	}
}

func TestParseDirectoryChildrenDifferentDepths(t *testing.T) {
	dirnames := []string{"cmd/", "protocol/", "cmd/protocol/"}
	paths := []string{
		"cmd/lsif-go",
		"protocol/protocol.go",
		"protocol/writer.go",
		"cmd/protocol/main.go",
	}

	expected := map[string][]string{
		"cmd/":          {"cmd/lsif-go"},
		"protocol/":     {"protocol/protocol.go", "protocol/writer.go"},
		"cmd/protocol/": {"cmd/protocol/main.go"},
	}

	if diff := cmp.Diff(expected, parseDirectoryChildren(dirnames, paths)); diff != "" {
		t.Errorf("unexpected directory children result (-want +got):\n%s", diff)
	}
}

func TestCleanDirectoriesForLsTree(t *testing.T) {
	args := []string{"", "foo", "bar/", "baz"}
	actual := cleanDirectoriesForLsTree(args)
	expected := []string{".", "foo/", "bar/", "baz/"}

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("unexpected ls-tree args (-want +got):\n%s", diff)
	}
}

func TestListDirectoryChildren(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	gitCommands := []string{
		"mkdir -p dir{1..3}/sub{1..3}",
		"touch dir1/sub1/file",
		"touch dir1/sub2/file",
		"touch dir2/sub1/file",
		"touch dir2/sub2/file",
		"touch dir3/sub1/file",
		"touch dir3/sub3/file",
		"git add .",
		"git commit -m commit1",
	}

	repo := MakeGitRepository(t, gitCommands...)

	ctx := context.Background()

	checker := authz.NewMockSubRepoPermissionChecker()
	// Start disabled
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return false
	})
	client1 := NewTestClient(t).WithChecker(checker)

	dirnames := []string{"dir1/", "dir2/", "dir3/"}
	children, err := client1.ListDirectoryChildren(ctx, repo, "HEAD", dirnames)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]string{
		"dir1/": {"dir1/sub1", "dir1/sub2"},
		"dir2/": {"dir2/sub1", "dir2/sub2"},
		"dir3/": {"dir3/sub1", "dir3/sub3"},
	}
	if diff := cmp.Diff(expected, children); diff != "" {
		t.Fatal(diff)
	}

	// With filtering
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if strings.Contains(content.Path, "dir1/") {
			return authz.Read, nil
		}
		return authz.None, nil
	})
	usePermissionsForFilePermissionsFunc(checker)
	client2 := NewTestClient(t).WithChecker(checker)
	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 1,
	})
	children, err = client2.ListDirectoryChildren(ctx, repo, "HEAD", dirnames)
	if err != nil {
		t.Fatal(err)
	}
	expected = map[string][]string{
		"dir1/": {"dir1/sub1", "dir1/sub2"},
		"dir2/": nil,
		"dir3/": nil,
	}
	if diff := cmp.Diff(expected, children); diff != "" {
		t.Fatal(diff)
	}
}

func TestListTags(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()

	dateEnv := "GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z"
	gitCommands := []string{
		dateEnv + " git commit --allow-empty -m foo --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		"git tag t0",
		"git tag t1",
		dateEnv + " git tag --annotate -m foo t2",
		dateEnv + " git commit --allow-empty -m foo --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		"git tag t3",
	}

	repo := MakeGitRepository(t, gitCommands...)
	wantTags := []*gitdomain.Tag{
		{Name: "t0", CommitID: "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8", CreatorDate: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
		{Name: "t1", CommitID: "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8", CreatorDate: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
		{Name: "t2", CommitID: "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8", CreatorDate: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
		{Name: "t3", CommitID: "afeafc4a918c144329807df307e68899e6b65018", CreatorDate: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
	}

	client := NewClient("test")
	tags, err := client.ListTags(context.Background(), repo)
	require.Nil(t, err)

	sort.Sort(sortedTags(tags))
	sort.Sort(sortedTags(wantTags))

	if diff := cmp.Diff(wantTags, tags); diff != "" {
		t.Fatalf("tag mismatch (-want +got):\n%s", diff)
	}

	tags, err = client.ListTags(context.Background(), repo, "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8")
	require.Nil(t, err)
	if diff := cmp.Diff(wantTags[:3], tags); diff != "" {
		t.Fatalf("tag mismatch (-want +got):\n%s", diff)
	}

	tags, err = client.ListTags(context.Background(), repo, "afeafc4a918c144329807df307e68899e6b65018")
	require.Nil(t, err)
	if diff := cmp.Diff([]*gitdomain.Tag{wantTags[3]}, tags); diff != "" {
		t.Fatalf("tag mismatch (-want +got):\n%s", diff)
	}
}

type sortedTags []*gitdomain.Tag

func (p sortedTags) Len() int           { return len(p) }
func (p sortedTags) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p sortedTags) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// See https://github.com/sourcegraph/sourcegraph/issues/5453
func TestParseTags_WithoutCreatorDate(t *testing.T) {
	have, err := parseTags([]byte(
		"9ee1c939d1cb936b1f98e8d81aeffab57bae46ab\x00v2.6.12\x001119037709\n" +
			"c39ae07f393806ccf406ef966e9a15afc43cc36a\x00v2.6.11-tree\x00\n" +
			"c39ae07f393806ccf406ef966e9a15afc43cc36a\x00v2.6.11\x00\n",
	))
	if err != nil {
		t.Fatalf("parseTags: have err %v, want nil", err)
	}

	want := []*gitdomain.Tag{
		{
			Name:        "v2.6.12",
			CommitID:    "9ee1c939d1cb936b1f98e8d81aeffab57bae46ab",
			CreatorDate: time.Unix(1119037709, 0).UTC(),
		},
		{
			Name:     "v2.6.11-tree",
			CommitID: "c39ae07f393806ccf406ef966e9a15afc43cc36a",
		},
		{
			Name:     "v2.6.11",
			CommitID: "c39ae07f393806ccf406ef966e9a15afc43cc36a",
		},
	}

	if diff := cmp.Diff(have, want); diff != "" {
		t.Fatal(diff)
	}
}

func TestRepository_FileSystem_Symlinks(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()

	gitCommands := []string{
		"touch file1",
		"mkdir dir1",
		"ln -s file1 link1",
		"ln -s ../file1 dir1/link2",
		"touch --date=2006-01-02T15:04:05Z file1 link1 dir1/link2 || touch -t " + Times[0] + " file1 link1 dir1/link2",
		"git add link1 file1 dir1/link2",
		"git commit -m commit1",
	}

	// map of path to size of content
	symlinks := map[string]int64{
		"link1":      5, // file1
		"dir1/link2": 8, // ../file1
	}

	dir := InitGitRepository(t, gitCommands...)
	repo := api.RepoName(filepath.Base(dir))

	client := NewClient("test")

	commitID := api.CommitID(ComputeCommitHash(dir, true))

	ctx := context.Background()

	// file1 should be a file.
	file1Info, err := client.Stat(ctx, repo, commitID, "file1")
	if err != nil {
		t.Fatalf("fs.Stat(file1): %s", err)
	}
	if !file1Info.Mode().IsRegular() {
		t.Errorf("file1 Stat !IsRegular (mode: %o)", file1Info.Mode())
	}

	checkSymlinkFileInfo := func(name string, link fs.FileInfo) {
		t.Helper()
		if link.Mode()&os.ModeSymlink == 0 {
			t.Errorf("link mode is not symlink (mode: %o)", link.Mode())
		}
		if link.Name() != name {
			t.Errorf("got link.Name() == %q, want %q", link.Name(), name)
		}
	}

	// Check symlinks are links
	for symlink := range symlinks {
		fi, err := client.Stat(ctx, repo, commitID, symlink)
		if err != nil {
			t.Fatalf("fs.Stat(%s): %s", symlink, err)
		}
		if runtime.GOOS != "windows" {
			// TODO(alexsaveliev) make it work on Windows too
			checkSymlinkFileInfo(symlink, fi)
		}
	}

	// Also check the FileInfo returned by ReadDir to ensure it's
	// consistent with the FileInfo returned by lStat.
	entries, err := client.ReadDir(ctx, repo, commitID, ".", false)
	if err != nil {
		t.Fatalf("fs.ReadDir(.): %s", err)
	}
	found := false
	for _, entry := range entries {
		if entry.Name() == "link1" {
			found = true
			if runtime.GOOS != "windows" {
				checkSymlinkFileInfo("link1", entry)
			}
		}
	}
	if !found {
		t.Fatal("readdir did not return link1")
	}

	for symlink, size := range symlinks {
		fi, err := client.Stat(ctx, repo, commitID, symlink)
		if err != nil {
			t.Fatalf("fs.Stat(%s): %s", symlink, err)
		}
		if fi.Mode()&fs.ModeSymlink == 0 {
			t.Errorf("%s Stat is not a symlink (mode: %o)", symlink, fi.Mode())
		}
		if fi.Name() != symlink {
			t.Errorf("got Name %q, want %q", fi.Name(), symlink)
		}
		if fi.Size() != size {
			t.Errorf("got %s Size %d, want %d", symlink, fi.Size(), size)
		}
	}
}

func TestStat(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()

	gitCommands := []string{
		"mkdir dir1",
		"touch dir1/file1",
		"git add dir1/file1",
		"git commit -m commit1",
	}

	dir := InitGitRepository(t, gitCommands...)
	repo := api.RepoName(filepath.Base(dir))
	checker := authz.NewMockSubRepoPermissionChecker()
	// Start disabled
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return false
	})
	client := NewTestClient(t).WithChecker(checker)

	commitID := api.CommitID(ComputeCommitHash(dir, true))

	ctx := context.Background()

	fileInfo, err := client.Stat(ctx, repo, commitID, "dir1/file1")
	if err != nil {
		t.Fatal(err)
	}
	want := "dir1/file1"
	if diff := cmp.Diff(want, fileInfo.Name()); diff != "" {
		t.Fatal(diff)
	}

	ctx = actor.WithActor(ctx, &actor.Actor{
		UID: 1,
	})

	// With filtering
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if strings.HasPrefix(content.Path, "dir2") {
			return authz.Read, nil
		}
		return authz.None, nil
	})
	usePermissionsForFilePermissionsFunc(checker)
	_, err = client.Stat(ctx, repo, commitID, "dir1/file1")
	if err == nil {
		t.Fatal(err)
	}
	want = "ls-tree dir1/file1: file does not exist"
	if diff := cmp.Diff(want, err.Error()); diff != "" {
		t.Fatal(diff)
	}
}

var NonExistentCommitID = api.CommitID(strings.Repeat("a", 40))

func TestLogPartsPerCommitInSync(t *testing.T) {
	require.Equal(t, partsPerCommit-1, strings.Count(logFormatWithoutRefs, "%x00"))
}

func TestRepository_HasCommitAfter(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})

	testCases := []struct {
		label                 string
		commitDates           []string
		after                 string
		revspec               string
		want, wantSubRepoTest bool
	}{
		{
			label: "after specific date",
			commitDates: []string{
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:05Z",
				"2008-01-02T15:04:05Z",
			},
			after:           "2006-01-02T15:04:05Z",
			revspec:         "master",
			want:            true,
			wantSubRepoTest: true,
		},
		{
			label: "after 1 year ago",
			commitDates: []string{
				"2016-01-02T15:04:05Z",
				"2017-01-02T15:04:05Z",
				"2017-01-02T15:04:06Z",
			},
			after:           "1 year ago",
			revspec:         "master",
			want:            false,
			wantSubRepoTest: false,
		},
		{
			label: "after too recent date",
			commitDates: []string{
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:05Z",
				"2008-01-02T15:04:05Z",
			},
			after:           "2010-01-02T15:04:05Z",
			revspec:         "HEAD",
			want:            false,
			wantSubRepoTest: false,
		},
		{
			label: "commit 1 second after",
			commitDates: []string{
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:05Z",
				"2007-01-02T15:04:06Z",
			},
			after:           "2007-01-02T15:04:05Z",
			revspec:         "HEAD",
			want:            true,
			wantSubRepoTest: false,
		},
		{
			label: "after 10 years ago",
			commitDates: []string{
				"2016-01-02T15:04:05Z",
				"2017-01-02T15:04:05Z",
				"2017-01-02T15:04:06Z",
			},
			after:           "10 years ago",
			revspec:         "HEAD",
			want:            true,
			wantSubRepoTest: true,
		},
	}

	t.Run("basic", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.label, func(t *testing.T) {
				client := NewTestClient(t).WithClientSource(NewTestClientSource(t, []string{"test"}, func(o *TestClientSourceOptions) {
					o.ClientFunc = func(conn *grpc.ClientConn) proto.GitserverServiceClient {
						c := NewMockGitserverServiceClient()
						c.ResolveRevisionFunc.SetDefaultReturn(&proto.ResolveRevisionResponse{
							CommitSha: tc.revspec,
						}, nil)
						return c
					}
				}))

				gitCommands := make([]string, len(tc.commitDates))
				for i, date := range tc.commitDates {
					gitCommands[i] = fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=%s git commit --allow-empty -m foo --author='a <a@a.com>'", date)
				}
				repo := MakeGitRepository(t, gitCommands...)
				got, err := client.HasCommitAfter(ctx, repo, tc.after, tc.revspec)
				if err != nil || got != tc.want {
					t.Errorf("got %t hascommitafter, want %t", got, tc.want)
				}
			})
		}
	})

	t.Run("with sub-repo permissions", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.label, func(t *testing.T) {
				gitCommands := make([]string, len(tc.commitDates))
				for i, date := range tc.commitDates {
					fileName := fmt.Sprintf("file%d", i)
					gitCommands = append(gitCommands, fmt.Sprintf("touch %s", fileName), fmt.Sprintf("git add %s", fileName))
					gitCommands = append(gitCommands, fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=%s git commit -m commit%d --author='a <a@a.com>'", date, i))
				}
				// Case where user can't view commit 2, but can view commits 0 and 1. In each test case the result should match the case where no sub-repo perms enabled
				checker := getTestSubRepoPermsChecker("file2")
				client := NewTestClient(t).WithChecker(checker)
				repo := MakeGitRepository(t, gitCommands...)
				got, err := client.HasCommitAfter(ctx, repo, tc.after, tc.revspec)
				if err != nil {
					t.Errorf("got error: %s", err)
				}
				if got != tc.want {
					t.Errorf("got %t hascommitafter, want %t", got, tc.want)
				}

				// Case where user can't view commit 1 or commit 2, which will mean in some cases since HasCommitAfter will be false due to those commits not being visible.
				checker = getTestSubRepoPermsChecker("file1", "file2")
				client = NewTestClient(t).WithChecker(checker)
				got, err = client.HasCommitAfter(ctx, repo, tc.after, tc.revspec)
				if err != nil {
					t.Errorf("got error: %s", err)
				}
				if got != tc.wantSubRepoTest {
					t.Errorf("got %t hascommitafter, want %t", got, tc.wantSubRepoTest)
				}
			})
		}
	})
}

func TestRepository_FirstEverCommit(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})

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
			gitCommands := make([]string, len(tc.commitDates))
			for i, date := range tc.commitDates {
				gitCommands[i] = fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=%s git commit --allow-empty -m foo --author='a <a@a.com>'", date)
			}

			repo := MakeGitRepository(t, gitCommands...)

			client := NewTestClient(t).WithClientSource(NewTestClientSource(t, []string{"test"}, func(o *TestClientSourceOptions) {
				o.ClientFunc = func(conn *grpc.ClientConn) proto.GitserverServiceClient {
					date, err := time.Parse(time.RFC3339, tc.want)
					require.NoError(t, err)
					c := NewMockGitserverServiceClient()
					c.GetCommitFunc.SetDefaultReturn(&proto.GetCommitResponse{
						Commit: &proto.GitCommit{
							Committer: &proto.GitSignature{
								Date: timestamppb.New(date),
							},
						},
					}, nil)
					return c
				}
			}))

			gotCommit, err := client.FirstEverCommit(ctx, repo)
			if err != nil {
				t.Fatal(err)
			}
			got := gotCommit.Committer.Date.Format(time.RFC3339)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		}
	})

	// Added for awareness if this error message changes. Insights skip over empty repos and check against error message
	t.Run("empty repo", func(t *testing.T) {
		repo := MakeGitRepository(t)
		_, err := NewClient("test").FirstEverCommit(ctx, repo)
		wantErr := `git command [rev-list --reverse --date-order --max-parents=0 HEAD] failed (output: ""): exit status 128`
		if err.Error() != wantErr {
			t.Errorf("expected :%s, got :%s", wantErr, err)
		}
	})
}

func TestRepository_Commits(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})

	// TODO(sqs): test CommitsOptions.Base

	gitCommands := []string{
		"git commit --allow-empty -m foo",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:07Z git commit --allow-empty -m bar --author='a <a@a.com>' --date 2006-01-02T15:04:06Z",
	}
	wantGitCommits := []*gitdomain.Commit{
		{
			ID:        "b266c7e3ca00b1a17ad0b1449825d0854225c007",
			Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:06Z")},
			Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:07Z")},
			Message:   "bar",
			Parents:   []api.CommitID{"ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"},
		},
		{
			ID:        "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8",
			Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
			Committer: &gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
			Message:   "foo",
			Parents:   nil,
		},
	}
	tests := map[string]struct {
		repo        api.RepoName
		id          api.CommitID
		wantCommits []*gitdomain.Commit
		wantTotal   uint
	}{
		"git cmd": {
			repo:        MakeGitRepository(t, gitCommands...),
			id:          "b266c7e3ca00b1a17ad0b1449825d0854225c007",
			wantCommits: wantGitCommits,
			wantTotal:   2,
		},
	}
	client := NewClient("test")
	runCommitsTests := func(checker authz.SubRepoPermissionChecker) {
		for label, test := range tests {
			t.Run(label, func(t *testing.T) {
				testCommits(ctx, label, test.repo, CommitsOptions{Range: string(test.id)}, checker, test.wantCommits, t)

				// Test that trying to get a nonexistent commit returns RevisionNotFoundError.
				if _, err := client.Commits(ctx, test.repo, CommitsOptions{Range: string(NonExistentCommitID)}); !errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
					t.Errorf("%s: for nonexistent commit: got err %v, want RevisionNotFoundError", label, err)
				}
			})
		}
	}
	runCommitsTests(nil)
	checker := getTestSubRepoPermsChecker()
	runCommitsTests(checker)
}

func TestCommits_SubRepoPerms(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})
	gitCommands := []string{
		"touch file1",
		"git add file1",
		"git commit -m commit1",
		"touch file2",
		"git add file2",
		"touch file2.2",
		"git add file2.2",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:07Z git commit -m commit2 --author='a <a@a.com>' --date 2006-01-02T15:04:06Z",
		"touch file3",
		"git add file3",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:07Z git commit -m commit3 --author='a <a@a.com>' --date 2006-01-02T15:04:07Z",
	}
	repo := MakeGitRepository(t, gitCommands...)

	tests := map[string]struct {
		wantCommits   []*gitdomain.Commit
		opt           CommitsOptions
		wantTotal     uint
		noAccessPaths []string
	}{
		"if no read perms on at least one file in the commit should filter out commit": {
			wantTotal: 2,
			wantCommits: []*gitdomain.Commit{
				{
					ID:        "b96d097108fa49e339ca88bc97ab07f833e62131",
					Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:06Z")},
					Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:07Z")},
					Message:   "commit2",
					Parents:   []api.CommitID{"d38233a79e037d2ab8170b0d0bc0aa438473e6da"},
				},
				{
					ID:        "d38233a79e037d2ab8170b0d0bc0aa438473e6da",
					Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
					Committer: &gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
					Message:   "commit1",
				},
			},
			noAccessPaths: []string{"file2", "file3"},
		},
		"sub-repo perms with path (w/ no access) specified should return no commits": {
			wantTotal: 1,
			opt: CommitsOptions{
				Path: "file2",
			},
			wantCommits:   []*gitdomain.Commit{},
			noAccessPaths: []string{"file2", "file3"},
		},
		"sub-repo perms with path (w/ access) specified should return that commit": {
			wantTotal: 1,
			opt: CommitsOptions{
				Path: "file1",
			},
			wantCommits: []*gitdomain.Commit{
				{
					ID:        "d38233a79e037d2ab8170b0d0bc0aa438473e6da",
					Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
					Committer: &gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
					Message:   "commit1",
				},
			},
			noAccessPaths: []string{"file2", "file3"},
		},
	}

	for label, test := range tests {
		t.Run(label, func(t *testing.T) {
			checker := getTestSubRepoPermsChecker(test.noAccessPaths...)
			client := NewTestClient(t).WithChecker(checker)
			commits, err := client.Commits(ctx, repo, test.opt)
			if err != nil {
				t.Errorf("%s: Commits(): %s", label, err)
				return
			}

			if len(commits) != len(test.wantCommits) {
				t.Errorf("%s: got %d commits, want %d", label, len(commits), len(test.wantCommits))
			}

			checkCommits(t, commits, test.wantCommits)
		})
	}
}

func TestCommits_SubRepoPerms_ReturnNCommits(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})
	gitCommands := []string{
		"touch file1",
		"git add file1",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:01Z git commit -m commit1 --author='a <a@a.com>' --date 2006-01-02T15:04:01Z",
		"touch file2",
		"git add file2",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:02Z git commit -m commit2 --author='a <a@a.com>' --date 2006-01-02T15:04:02Z",
		"echo foo > file1",
		"git add file1",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:03Z git commit -m commit3 --author='a <a@a.com>' --date 2006-01-02T15:04:03Z",
		"echo asdf > file1",
		"git add file1",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:04Z git commit -m commit4 --author='a <a@a.com>' --date 2006-01-02T15:04:04Z",
		"echo bar > file1",
		"git add file1",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit -m commit5 --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		"echo asdf2 > file2",
		"git add file2",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:06Z git commit -m commit6 --author='a <a@a.com>' --date 2006-01-02T15:04:06Z",
		"echo bazz > file1",
		"git add file1",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:07Z git commit -m commit7 --author='a <a@a.com>' --date 2006-01-02T15:04:07Z",
		"echo bazz > file2",
		"git add file2",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:08Z git commit -m commit8 --author='a <a@a.com>' --date 2006-01-02T15:04:08Z",
	}

	tests := map[string]struct {
		repo          api.RepoName
		wantCommits   []*gitdomain.Commit
		opt           CommitsOptions
		wantTotal     uint
		noAccessPaths []string
	}{
		"return the requested number of commits": {
			repo:      MakeGitRepository(t, gitCommands...),
			wantTotal: 3,
			opt: CommitsOptions{
				N: 3,
			},
			wantCommits: []*gitdomain.Commit{
				{
					ID:        "61dbc35f719c53810904a2d359309d4e1e98a6be",
					Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:07Z")},
					Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:07Z")},
					Message:   "commit7",
					Parents:   []api.CommitID{"66566c8aa223f3e1b94ebe09e6cdb14c3a5bfb36"},
				},
				{
					ID:        "2e6b2c94293e9e339f781b2a2f7172e15460f88c",
					Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
					Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
					Parents: []api.CommitID{
						"9a7ec70986d657c4c86d6ac476f0c5181ece509a",
					},
					Message: "commit5",
				},
				{
					ID:        "9a7ec70986d657c4c86d6ac476f0c5181ece509a",
					Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:04Z")},
					Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:04Z")},
					Message:   "commit4",
					Parents: []api.CommitID{
						"f3fa8cf6ec56d0469402523385d6ca4b7cb222d8",
					},
				},
			},
			noAccessPaths: []string{"file2"},
		},
	}

	for label, test := range tests {
		t.Run(label, func(t *testing.T) {
			checker := getTestSubRepoPermsChecker(test.noAccessPaths...)
			client := NewTestClient(t).WithChecker(checker)
			commits, err := client.Commits(ctx, test.repo, test.opt)
			if err != nil {
				t.Errorf("%s: Commits(): %s", label, err)
				return
			}

			if diff := cmp.Diff(test.wantCommits, commits); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestRepository_Commits_options(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := context.Background()
	ctx = actor.WithActor(ctx, actor.FromUser(42))

	gitCommands := []string{
		"git commit --allow-empty -m foo",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:07Z git commit --allow-empty -m bar --author='a <a@a.com>' --date 2006-01-02T15:04:06Z",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:08Z git commit --allow-empty -m qux --author='a <a@a.com>' --date 2006-01-02T15:04:08Z",
	}
	wantGitCommits := []*gitdomain.Commit{
		{
			ID:        "b266c7e3ca00b1a17ad0b1449825d0854225c007",
			Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:06Z")},
			Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:07Z")},
			Message:   "bar",
			Parents:   []api.CommitID{"ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"},
		},
	}
	wantGitCommits2 := []*gitdomain.Commit{
		{
			ID:        "ade564eba4cf904492fb56dcd287ac633e6e082c",
			Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:08Z")},
			Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:08Z")},
			Message:   "qux",
			Parents:   []api.CommitID{"b266c7e3ca00b1a17ad0b1449825d0854225c007"},
		},
	}
	tests := map[string]struct {
		opt         CommitsOptions
		wantCommits []*gitdomain.Commit
		wantTotal   uint
	}{
		"git cmd": {
			opt:         CommitsOptions{Range: "ade564eba4cf904492fb56dcd287ac633e6e082c", N: 1, Skip: 1},
			wantCommits: wantGitCommits,
			wantTotal:   1,
		},
		"git cmd Head": {
			opt: CommitsOptions{
				Range: "b266c7e3ca00b1a17ad0b1449825d0854225c007...ade564eba4cf904492fb56dcd287ac633e6e082c",
			},
			wantCommits: wantGitCommits2,
			wantTotal:   1,
		},
		"before": {
			opt: CommitsOptions{
				Before: "2006-01-02T15:04:07Z",
				Range:  "HEAD",
				N:      1,
			},
			wantCommits: []*gitdomain.Commit{
				{
					ID:        "b266c7e3ca00b1a17ad0b1449825d0854225c007",
					Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:06Z")},
					Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:07Z")},
					Message:   "bar",
					Parents:   []api.CommitID{"ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"},
				},
			},
			wantTotal: 1,
		},
	}
	runCommitsTests := func(checker authz.SubRepoPermissionChecker) {
		for label, test := range tests {
			t.Run(label, func(t *testing.T) {
				repo := MakeGitRepository(t, gitCommands...)
				testCommits(ctx, label, repo, test.opt, checker, test.wantCommits, t)
			})
		}
		// Added for awareness if this error message changes. Insights record last repo indexing and consider empty
		// repos a success case.
		subRepo := ""
		if checker != nil {
			subRepo = " sub repo enabled"
		}
		t.Run("empty repo"+subRepo, func(t *testing.T) {
			repo := MakeGitRepository(t)
			before := ""
			after := time.Date(2022, 11, 11, 12, 10, 0, 4, time.UTC).Format(time.RFC3339)
			client := NewTestClient(t).WithChecker(checker)
			_, err := client.Commits(ctx, repo, CommitsOptions{N: 0, DateOrder: true, After: after, Before: before})
			if err == nil {
				t.Error("expected error, got nil")
			}
			wantErr := `git command [git log --format=format:%x1e%H%x00%aN%x00%aE%x00%at%x00%cN%x00%cE%x00%ct%x00%B%x00%P%x00 --after=` + after + " --date-order"
			if subRepo != "" {
				wantErr += " --name-only"
			}
			wantErr += `] failed (output: ""): exit status 128`
			if err.Error() != wantErr {
				t.Errorf("expected:%v got:%v", wantErr, err.Error())
			}
		})
	}
	runCommitsTests(nil)
	checker := getTestSubRepoPermsChecker()
	runCommitsTests(checker)
}

func TestRepository_Commits_options_path(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})

	gitCommands := []string{
		"git commit --allow-empty -m commit1",
		"touch file1",
		"touch --date=2006-01-02T15:04:05Z file1 || touch -t " + Times[0] + " file1",
		"git add file1",
		"git commit -m commit2",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:07Z git commit --allow-empty -m commit3 --author='a <a@a.com>' --date 2006-01-02T15:04:06Z",
	}
	wantGitCommits := []*gitdomain.Commit{
		{
			ID:        "546a3ef26e581624ef997cb8c0ba01ee475fc1dc",
			Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
			Committer: &gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:05Z")},
			Message:   "commit2",
			Parents:   []api.CommitID{"a04652fa1998a0a7d2f2f77ecb7021de943d3aab"},
		},
	}
	tests := map[string]struct {
		opt         CommitsOptions
		wantCommits []*gitdomain.Commit
	}{
		"git cmd Path 0": {
			opt: CommitsOptions{
				Range: "master",
				Path:  "doesnt-exist",
			},
			wantCommits: nil,
		},
		"git cmd Path 1": {
			opt: CommitsOptions{
				Range: "master",
				Path:  "file1",
			},
			wantCommits: wantGitCommits,
		},
		"git cmd non utf8": {
			opt: CommitsOptions{
				Range:  "master",
				Author: "a\xc0rn",
			},
			wantCommits: nil,
		},
	}

	runCommitsTest := func(checker authz.SubRepoPermissionChecker) {
		for label, test := range tests {
			t.Run(label, func(t *testing.T) {
				repo := MakeGitRepository(t, gitCommands...)
				testCommits(ctx, label, repo, test.opt, checker, test.wantCommits, t)
			})
		}
	}
	runCommitsTest(nil)
	checker := getTestSubRepoPermsChecker()
	runCommitsTest(checker)
}

func TestParseCommitsUniqueToBranch(t *testing.T) { // KEEP
	commits, err := parseCommitsUniqueToBranch([]string{
		"c165bfff52e9d4f87891bba497e3b70fea144d89:2020-08-04T08:23:30-05:00",
		"f73ee8ed601efea74f3b734eeb073307e1615606:2020-04-16T16:06:21-04:00",
		"6057f7ed8d331c82030c713b650fc8fd2c0c2347:2020-04-16T16:20:26-04:00",
		"7886287b8758d1baf19cf7b8253856128369a2a7:2020-04-16T16:55:58-04:00",
		"b69f89473bbcc04dc52cafaf6baa504e34791f5a:2020-04-20T12:10:49-04:00",
		"172b7fcf8b8c49b37b231693433586c2bfd1619e:2020-04-20T12:37:36-04:00",
		"5bc35c78fb5fb388891ca944cd12d85fd6dede95:2020-05-05T12:53:18-05:00",
	})
	if err != nil {
		t.Fatalf("unexpected error parsing commits: %s", err)
	}

	expectedCommits := map[string]time.Time{
		"c165bfff52e9d4f87891bba497e3b70fea144d89": *mustParseDate("2020-08-04T08:23:30-05:00", t),
		"f73ee8ed601efea74f3b734eeb073307e1615606": *mustParseDate("2020-04-16T16:06:21-04:00", t),
		"6057f7ed8d331c82030c713b650fc8fd2c0c2347": *mustParseDate("2020-04-16T16:20:26-04:00", t),
		"7886287b8758d1baf19cf7b8253856128369a2a7": *mustParseDate("2020-04-16T16:55:58-04:00", t),
		"b69f89473bbcc04dc52cafaf6baa504e34791f5a": *mustParseDate("2020-04-20T12:10:49-04:00", t),
		"172b7fcf8b8c49b37b231693433586c2bfd1619e": *mustParseDate("2020-04-20T12:37:36-04:00", t),
		"5bc35c78fb5fb388891ca944cd12d85fd6dede95": *mustParseDate("2020-05-05T12:53:18-05:00", t),
	}
	if diff := cmp.Diff(expectedCommits, commits); diff != "" {
		t.Errorf("unexpected commits (-want +got):\n%s", diff)
	}
}

func TestParseBranchesContaining(t *testing.T) { // KEEP
	names := parseBranchesContaining([]string{
		"refs/tags/v0.7.0",
		"refs/tags/v0.5.1",
		"refs/tags/v1.1.4",
		"refs/heads/symbols", "refs/heads/bl/symbols",
		"refs/tags/v1.2.0",
		"refs/tags/v1.1.0",
		"refs/tags/v0.10.0",
		"refs/tags/v1.0.0",
		"refs/heads/garo/index-specific-files",
		"refs/heads/bl/symbols-2",
		"refs/tags/v1.3.1",
		"refs/tags/v0.5.2",
		"refs/tags/v1.1.2",
		"refs/tags/v0.8.0",
		"refs/heads/ef/wtf",
		"refs/tags/v1.5.0",
		"refs/tags/v0.9.0",
		"refs/heads/garo/go-and-typescript-lsif-indexing",
		"refs/heads/master",
		"refs/heads/sg/document-symbols",
		"refs/tags/v1.1.1",
		"refs/tags/v1.4.0",
		"refs/heads/nsc/bump-go-version",
		"refs/heads/nsc/random",
		"refs/heads/nsc/markupcontent",
		"refs/tags/v0.6.0",
		"refs/tags/v1.1.3",
		"refs/tags/v0.5.3",
		"refs/tags/v1.3.0",
	})

	expectedNames := []string{
		"bl/symbols",
		"bl/symbols-2",
		"ef/wtf",
		"garo/go-and-typescript-lsif-indexing",
		"garo/index-specific-files",
		"master",
		"nsc/bump-go-version",
		"nsc/markupcontent",
		"nsc/random",
		"sg/document-symbols",
		"symbols",
		"v0.10.0",
		"v0.5.1",
		"v0.5.2",
		"v0.5.3",
		"v0.6.0",
		"v0.7.0",
		"v0.8.0",
		"v0.9.0",
		"v1.0.0",
		"v1.1.0",
		"v1.1.1",
		"v1.1.2",
		"v1.1.3",
		"v1.1.4",
		"v1.2.0",
		"v1.3.0",
		"v1.3.1",
		"v1.4.0",
		"v1.5.0",
	}
	if diff := cmp.Diff(expectedNames, names); diff != "" {
		t.Errorf("unexpected names (-want +got):\n%s", diff)
	}
}

func TestParseRefDescriptions(t *testing.T) { // KEEP
	refDescriptions, err := parseRefDescriptions(bytes.Join([][]byte{
		[]byte("66a7ac584740245fc523da443a3f540a52f8af72\x00refs/heads/bl/symbols\x00 \x001611017211"),
		[]byte("58537c06cf7ba8a562a3f5208fb7a8efbc971d0e\x00refs/heads/bl/symbols-2\x00 \x001614176480"),
		[]byte("a40716031ae97ee7c5cdf1dec913567a4a7c50c8\x00refs/heads/ef/wtf\x00 \x001612975808"),
		[]byte("e2e283fdaf6ea4a419cdbad142bbfd4b730080f8\x00refs/heads/garo/go-and-typescript-lsif-indexing\x00 \x001588178746"),
		[]byte("c485d92c3d2065041bf29b3fe0b55ffac7e66b2a\x00refs/heads/garo/index-specific-files\x00 \x001614632982"),
		[]byte("ce30aee6cc56f39d0ac6fee03c4c151c08a8cd2e\x00refs/heads/master\x00*\x001623869469"),
		[]byte("ec5cfc8ab33370c698273b1a097af73ea289c92b\x00refs/heads/nsc/bump-go-version\x00 \x001615588397"),
		[]byte("22b2c4f734f62060cae69da856fe3854defdcc87\x00refs/heads/nsc/markupcontent\x00 \x001620082202"),
		[]byte("9df3358a18792fa9dbd40d506f2e0ad23fc11ee8\x00refs/heads/nsc/random\x00 \x001612974546"),
		[]byte("a02b85b63345a1406d7a19727f7a5472c976e053\x00refs/heads/sg/document-symbols\x00 \x001617921183"),
		[]byte("234b0a484519129b251164ecb0674ec27d154d2f\x00refs/heads/symbols\x00 \x001609570315"),
		[]byte("6b5ae2e0ce568a7641174072271d109d7d0977c7\x00refs/tags/v0.0.0\x00 \x00"),
		[]byte("c165bfff52e9d4f87891bba497e3b70fea144d89\x00refs/tags/v0.10.0\x00 \x001596547410"),
		[]byte("f73ee8ed601efea74f3b734eeb073307e1615606\x00refs/tags/v0.5.1\x00 \x001587067581"),
		[]byte("6057f7ed8d331c82030c713b650fc8fd2c0c2347\x00refs/tags/v0.5.2\x00 \x001587068426"),
		[]byte("7886287b8758d1baf19cf7b8253856128369a2a7\x00refs/tags/v0.5.3\x00 \x001587070558"),
		[]byte("b69f89473bbcc04dc52cafaf6baa504e34791f5a\x00refs/tags/v0.6.0\x00 \x001587399049"),
		[]byte("172b7fcf8b8c49b37b231693433586c2bfd1619e\x00refs/tags/v0.7.0\x00 \x001587400656"),
		[]byte("5bc35c78fb5fb388891ca944cd12d85fd6dede95\x00refs/tags/v0.8.0\x00 \x001588701198"),
		[]byte("14faa49ef098df9488536ca3c9b26d79e6bec4d6\x00refs/tags/v0.9.0\x00 \x001594754800"),
		[]byte("0a82af8b6914d8c81326eee5f3a7e1d1106547f1\x00refs/tags/v1.0.0\x00 \x001597883619"),
		[]byte("262defb72b96261a7d56b000d438c5c7ec6d0f3e\x00refs/tags/v1.1.0\x00 \x001598037344"),
		[]byte("806b96eb544e7e632a617c26402eccee6d67faed\x00refs/tags/v1.1.1\x00 \x001598043755"),
		[]byte("5d8865d6feacb4fce3313cade2c61dc29c6271e6\x00refs/tags/v1.1.2\x00 \x001598121926"),
		[]byte("8c45a5635cf0a4968cc8c9dac2d61c388b53251e\x00refs/tags/v1.1.3\x00 \x001598368246"),
		[]byte("fc212da31ce157ef0795e934381509c5a50654f6\x00refs/tags/v1.1.4\x00 \x001598468567"),
		[]byte("4fd8b2c3522df32ffc8be983d42c3a504cc75fbc\x00refs/tags/v1.2.0\x00 \x001599490363"),
		[]byte("9741f54aa0f14be1103b00c89406393ea4d8a08a\x00refs/tags/v1.3.0\x00 \x001612999291"),
		[]byte("b358977103d2d66e2a3fc5f8081075c2834c4936\x00refs/tags/v1.3.1\x00 \x001614197805"),
		[]byte("2882ad236da4b649b4c1259d815bf1a378e3b92f\x00refs/tags/v1.4.0\x00 \x001620920462"),
		[]byte("340b84452286c18000afad9b140a32212a82840a\x00refs/tags/v1.5.0\x00 \x001621554101"),
	}, []byte("\n")))
	if err != nil {
		t.Fatalf("unexpected error parsing ref descriptions: %s", err)
	}

	makeBranch := func(name, createdDate string, isDefaultBranch bool) gitdomain.RefDescription {
		return gitdomain.RefDescription{Name: name, Type: gitdomain.RefTypeBranch, IsDefaultBranch: isDefaultBranch, CreatedDate: mustParseDate(createdDate, t)}
	}

	makeTag := func(name, createdDate string) gitdomain.RefDescription {
		return gitdomain.RefDescription{Name: name, Type: gitdomain.RefTypeTag, IsDefaultBranch: false, CreatedDate: mustParseDate(createdDate, t)}
	}

	expectedRefDescriptions := map[string][]gitdomain.RefDescription{
		"66a7ac584740245fc523da443a3f540a52f8af72": {makeBranch("bl/symbols", "2021-01-18T16:46:51-08:00", false)},
		"58537c06cf7ba8a562a3f5208fb7a8efbc971d0e": {makeBranch("bl/symbols-2", "2021-02-24T06:21:20-08:00", false)},
		"a40716031ae97ee7c5cdf1dec913567a4a7c50c8": {makeBranch("ef/wtf", "2021-02-10T10:50:08-06:00", false)},
		"e2e283fdaf6ea4a419cdbad142bbfd4b730080f8": {makeBranch("garo/go-and-typescript-lsif-indexing", "2020-04-29T16:45:46+00:00", false)},
		"c485d92c3d2065041bf29b3fe0b55ffac7e66b2a": {makeBranch("garo/index-specific-files", "2021-03-01T13:09:42-08:00", false)},
		"ce30aee6cc56f39d0ac6fee03c4c151c08a8cd2e": {makeBranch("master", "2021-06-16T11:51:09-07:00", true)},
		"ec5cfc8ab33370c698273b1a097af73ea289c92b": {makeBranch("nsc/bump-go-version", "2021-03-12T22:33:17+00:00", false)},
		"22b2c4f734f62060cae69da856fe3854defdcc87": {makeBranch("nsc/markupcontent", "2021-05-03T23:50:02+01:00", false)},
		"9df3358a18792fa9dbd40d506f2e0ad23fc11ee8": {makeBranch("nsc/random", "2021-02-10T16:29:06+00:00", false)},
		"a02b85b63345a1406d7a19727f7a5472c976e053": {makeBranch("sg/document-symbols", "2021-04-08T15:33:03-07:00", false)},
		"234b0a484519129b251164ecb0674ec27d154d2f": {makeBranch("symbols", "2021-01-01T22:51:55-08:00", false)},
		"6b5ae2e0ce568a7641174072271d109d7d0977c7": {gitdomain.RefDescription{Name: "v0.0.0", Type: gitdomain.RefTypeTag, IsDefaultBranch: false}},
		"c165bfff52e9d4f87891bba497e3b70fea144d89": {makeTag("v0.10.0", "2020-08-04T08:23:30-05:00")},
		"f73ee8ed601efea74f3b734eeb073307e1615606": {makeTag("v0.5.1", "2020-04-16T16:06:21-04:00")},
		"6057f7ed8d331c82030c713b650fc8fd2c0c2347": {makeTag("v0.5.2", "2020-04-16T16:20:26-04:00")},
		"7886287b8758d1baf19cf7b8253856128369a2a7": {makeTag("v0.5.3", "2020-04-16T16:55:58-04:00")},
		"b69f89473bbcc04dc52cafaf6baa504e34791f5a": {makeTag("v0.6.0", "2020-04-20T12:10:49-04:00")},
		"172b7fcf8b8c49b37b231693433586c2bfd1619e": {makeTag("v0.7.0", "2020-04-20T12:37:36-04:00")},
		"5bc35c78fb5fb388891ca944cd12d85fd6dede95": {makeTag("v0.8.0", "2020-05-05T12:53:18-05:00")},
		"14faa49ef098df9488536ca3c9b26d79e6bec4d6": {makeTag("v0.9.0", "2020-07-14T14:26:40-05:00")},
		"0a82af8b6914d8c81326eee5f3a7e1d1106547f1": {makeTag("v1.0.0", "2020-08-19T19:33:39-05:00")},
		"262defb72b96261a7d56b000d438c5c7ec6d0f3e": {makeTag("v1.1.0", "2020-08-21T14:15:44-05:00")},
		"806b96eb544e7e632a617c26402eccee6d67faed": {makeTag("v1.1.1", "2020-08-21T16:02:35-05:00")},
		"5d8865d6feacb4fce3313cade2c61dc29c6271e6": {makeTag("v1.1.2", "2020-08-22T13:45:26-05:00")},
		"8c45a5635cf0a4968cc8c9dac2d61c388b53251e": {makeTag("v1.1.3", "2020-08-25T10:10:46-05:00")},
		"fc212da31ce157ef0795e934381509c5a50654f6": {makeTag("v1.1.4", "2020-08-26T14:02:47-05:00")},
		"4fd8b2c3522df32ffc8be983d42c3a504cc75fbc": {makeTag("v1.2.0", "2020-09-07T09:52:43-05:00")},
		"9741f54aa0f14be1103b00c89406393ea4d8a08a": {makeTag("v1.3.0", "2021-02-10T23:21:31+00:00")},
		"b358977103d2d66e2a3fc5f8081075c2834c4936": {makeTag("v1.3.1", "2021-02-24T20:16:45+00:00")},
		"2882ad236da4b649b4c1259d815bf1a378e3b92f": {makeTag("v1.4.0", "2021-05-13T10:41:02-05:00")},
		"340b84452286c18000afad9b140a32212a82840a": {makeTag("v1.5.0", "2021-05-20T18:41:41-05:00")},
	}
	if diff := cmp.Diff(expectedRefDescriptions, refDescriptions); diff != "" {
		t.Errorf("unexpected ref descriptions (-want +got):\n%s", diff)
	}
}

func TestFilterRefDescriptions(t *testing.T) { // KEEP
	refDescriptions := map[string][]gitdomain.RefDescription{
		"d38233a79e037d2ab8170b0d0bc0aa438473e6da": {},
		"2775e60f523d3151a2a34ffdc659f500d0e73022": {},
		"2ba4dd2b9a27ec125fea7d72e12b9824ead18631": {},
		"9019942b8b92d5a70a7f546d97c451621c5059a6": {},
	}

	client := NewTestClient(t).WithClientSource(NewTestClientSource(t, []string{"test"}, func(o *TestClientSourceOptions) {
		o.ClientFunc = func(conn *grpc.ClientConn) proto.GitserverServiceClient {
			c := NewMockGitserverServiceClient()
			c.GetCommitFunc.SetDefaultHook(func(ctx context.Context, gcr *proto.GetCommitRequest, co ...grpc.CallOption) (*proto.GetCommitResponse, error) {
				if gcr.GetCommit() == "2775e60f523d3151a2a34ffdc659f500d0e73022" {
					s, err := status.New(codes.NotFound, "bad revision").WithDetails(&proto.RevisionNotFoundPayload{Repo: "repo", Spec: "deadbeef"})
					require.NoError(t, err)
					return nil, s.Err()
				}
				return &proto.GetCommitResponse{}, nil
			})
			return c
		}
	})).(*clientImplementor)
	filtered := client.filterRefDescriptions(context.Background(), "repo", refDescriptions)
	expectedRefDescriptions := map[string][]gitdomain.RefDescription{
		"d38233a79e037d2ab8170b0d0bc0aa438473e6da": {},
		"2ba4dd2b9a27ec125fea7d72e12b9824ead18631": {},
		"9019942b8b92d5a70a7f546d97c451621c5059a6": {},
	}
	if diff := cmp.Diff(expectedRefDescriptions, filtered); diff != "" {
		t.Errorf("unexpected ref descriptions (-want +got):\n%s", diff)
	}
}

func TestRefDescriptions(t *testing.T) { // KEEP
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})
	client := NewClient("test")
	gitCommands := append(getGitCommandsWithFiles("file1", "file2"), "git checkout -b my-other-branch")
	gitCommands = append(gitCommands, getGitCommandsWithFiles("file1-b2", "file2-b2")...)
	gitCommands = append(gitCommands, "git checkout -b my-branch-no-access")
	gitCommands = append(gitCommands, getGitCommandsWithFiles("file", "file-with-no-access")...)
	repo := MakeGitRepository(t, gitCommands...)

	makeBranch := func(name, createdDate string, isDefaultBranch bool) gitdomain.RefDescription {
		return gitdomain.RefDescription{Name: name, Type: gitdomain.RefTypeBranch, IsDefaultBranch: isDefaultBranch, CreatedDate: mustParseDate(createdDate, t)}
	}

	refDescriptions, err := client.RefDescriptions(ctx, repo)
	if err != nil {
		t.Errorf("err calling RefDescriptions: %s", err)
	}
	expectedRefDescriptions := map[string][]gitdomain.RefDescription{
		"2ba4dd2b9a27ec125fea7d72e12b9824ead18631": {makeBranch("master", "2006-01-02T15:04:05Z", false)},
		"9d7a382983098eed6cf911bd933dfacb13116e42": {makeBranch("my-other-branch", "2006-01-02T15:04:05Z", false)},
		"7cf006d0599531db799c08d3b00d7fd06da33015": {makeBranch("my-branch-no-access", "2006-01-02T15:04:05Z", true)},
	}
	if diff := cmp.Diff(expectedRefDescriptions, refDescriptions); diff != "" {
		t.Errorf("unexpected ref descriptions (-want +got):\n%s", diff)
	}
}

func TestCommitsUniqueToBranch(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})
	gitCommands := append([]string{"git checkout -b my-branch"}, getGitCommandsWithFiles("file1", "file2")...)
	gitCommands = append(gitCommands, getGitCommandsWithFiles("file3", "file-with-no-access")...)
	repo := MakeGitRepository(t, gitCommands...)

	client := NewClient("test")
	commits, err := client.CommitsUniqueToBranch(ctx, repo, "my-branch", true, &time.Time{})
	if err != nil {
		t.Errorf("err calling RefDescriptions: %s", err)
	}
	expectedCommits := map[string]time.Time{
		"2775e60f523d3151a2a34ffdc659f500d0e73022": *mustParseDate("2006-01-02T15:04:05-00:00", t),
		"2ba4dd2b9a27ec125fea7d72e12b9824ead18631": *mustParseDate("2006-01-02T15:04:05-00:00", t),
		"791ce7cd8ca2d855e12f47f8692a62bc42477edc": *mustParseDate("2006-01-02T15:04:05-00:00", t),
		"d38233a79e037d2ab8170b0d0bc0aa438473e6da": *mustParseDate("2006-01-02T15:04:05-00:00", t),
	}
	if diff := cmp.Diff(expectedCommits, commits); diff != "" {
		t.Errorf("unexpected ref descriptions (-want +got):\n%s", diff)
	}
}

func TestFilterCommitsUniqueToBranch(t *testing.T) {
	commitMap := map[string]time.Time{
		"d38233a79e037d2ab8170b0d0bc0aa438473e6da": {},
		"2775e60f523d3151a2a34ffdc659f500d0e73022": {},
		"2ba4dd2b9a27ec125fea7d72e12b9824ead18631": {},
		"9019942b8b92d5a70a7f546d97c451621c5059a6": {},
	}

	client := NewTestClient(t).WithClientSource(NewTestClientSource(t, []string{"test"}, func(o *TestClientSourceOptions) {
		o.ClientFunc = func(conn *grpc.ClientConn) proto.GitserverServiceClient {
			c := NewMockGitserverServiceClient()
			c.GetCommitFunc.SetDefaultHook(func(ctx context.Context, gcr *proto.GetCommitRequest, co ...grpc.CallOption) (*proto.GetCommitResponse, error) {
				if gcr.GetCommit() == "2775e60f523d3151a2a34ffdc659f500d0e73022" {
					s, err := status.New(codes.NotFound, "bad revision").WithDetails(&proto.RevisionNotFoundPayload{Repo: "repo", Spec: "deadbeef"})
					require.NoError(t, err)
					return nil, s.Err()
				}
				return &proto.GetCommitResponse{}, nil
			})
			return c
		}
	})).(*clientImplementor)
	filtered := client.filterCommitsUniqueToBranch(context.Background(), "repo", commitMap)
	expected := map[string]time.Time{
		"d38233a79e037d2ab8170b0d0bc0aa438473e6da": {},
		"2ba4dd2b9a27ec125fea7d72e12b9824ead18631": {},
		"9019942b8b92d5a70a7f546d97c451621c5059a6": {},
	}
	if diff := cmp.Diff(expected, filtered); diff != "" {
		t.Errorf("unexpected commits in result (-want +got):\n%s", diff)
	}
}

func testCommits(ctx context.Context, label string, repo api.RepoName, opt CommitsOptions, checker authz.SubRepoPermissionChecker, wantCommits []*gitdomain.Commit, t *testing.T) {
	t.Helper()
	client := NewTestClient(t).WithChecker(checker)
	commits, err := client.Commits(ctx, repo, opt)
	if err != nil {
		t.Errorf("%s: Commits(): %s", label, err)
		return
	}

	if len(commits) != len(wantCommits) {
		t.Errorf("%s: got %d commits, want %d", label, len(commits), len(wantCommits))
	}
	checkCommits(t, commits, wantCommits)
}

func checkCommits(t *testing.T, commits, wantCommits []*gitdomain.Commit) {
	t.Helper()
	for i := 0; i < len(commits) || i < len(wantCommits); i++ {
		var gotC, wantC *gitdomain.Commit
		if i < len(commits) {
			gotC = commits[i]
		}
		if i < len(wantCommits) {
			wantC = wantCommits[i]
		}
		if diff := cmp.Diff(gotC, wantC); diff != "" {
			t.Fatal(diff)
		}
	}
}

// get a test sub-repo permissions checker which allows access to all files (so should be a no-op)
func getTestSubRepoPermsChecker(noAccessPaths ...string) authz.SubRepoPermissionChecker {
	checker := authz.NewMockSubRepoPermissionChecker()
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		for _, noAccessPath := range noAccessPaths {
			if content.Path == noAccessPath {
				return authz.None, nil
			}
		}
		return authz.Read, nil
	})
	usePermissionsForFilePermissionsFunc(checker)
	return checker
}

func getGitCommandsWithFileLists(filenamesPerCommit ...[]string) []string {
	cmds := make([]string, 0, len(filenamesPerCommit)*3)
	for i, filenames := range filenamesPerCommit {
		for _, fn := range filenames {
			cmds = append(cmds,
				fmt.Sprintf("touch %s", fn),
				fmt.Sprintf("echo my_content_%d > %s", i, fn),
				fmt.Sprintf("git add %s", fn))
		}
		cmds = append(cmds,
			fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05=%dZ git commit -m commit%d --author='a <a@a.com>' --date 2006-01-02T15:04:0%dZ", i, i, i))
	}
	return cmds
}

func makeGitCommit(commitMessage string, seconds int) string {
	return fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05=%dZ git commit -m %s --author='a <a@a.com>' --date 2006-01-02T15:04:0%dZ", seconds, commitMessage, seconds)
}

func getGitCommandsWithFiles(fileName1, fileName2 string) []string {
	return []string{
		fmt.Sprintf("touch %s", fileName1),
		fmt.Sprintf("git add %s", fileName1),
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit -m commit1 --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		fmt.Sprintf("touch %s", fileName2),
		fmt.Sprintf("git add %s", fileName2),
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit -m commit2 --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
	}
}

func mustParseDate(s string, t *testing.T) *time.Time {
	t.Helper()
	date, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("unexpected error parsing date string: %s", err)
	}
	return &date
}

func CommitsEqual(a, b *gitdomain.Commit) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a.Author.Date != b.Author.Date {
		return false
	}
	a.Author.Date = b.Author.Date
	if ac, bc := a.Committer, b.Committer; ac != nil && bc != nil {
		if ac.Date != bc.Date {
			return false
		}
		ac.Date = bc.Date
	} else if !(ac == nil && bc == nil) {
		return false
	}
	return reflect.DeepEqual(a, b)
}

func TestRepository_ListBranches(t *testing.T) {
	ClientMocks.LocalGitserver = true
	t.Cleanup(func() {
		ResetClientMocks()
	})

	gitCommands := []string{
		"git commit --allow-empty -m foo",
		"git checkout -b b0",
		"git checkout -b b1",
	}

	wantBranches := []*gitdomain.Branch{{Name: "b0", Head: "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"}, {Name: "b1", Head: "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"}, {Name: "master", Head: "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"}}

	testBranches(t, gitCommands, wantBranches)
}

func testBranches(t *testing.T, gitCommands []string, wantBranches []*gitdomain.Branch) {
	t.Helper()

	repo := MakeGitRepository(t, gitCommands...)
	gotBranches, err := NewClient("test").ListBranches(context.Background(), repo)
	require.Nil(t, err)

	sort.Sort(branches(wantBranches))
	sort.Sort(branches(gotBranches))

	if diff := cmp.Diff(wantBranches, gotBranches); diff != "" {
		t.Fatalf("Branch mismatch (-want +got):\n%s", diff)
	}
}

// branches is a sortable slice of type Branch
type branches []*gitdomain.Branch

func (p branches) Len() int           { return len(p) }
func (p branches) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p branches) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func usePermissionsForFilePermissionsFunc(m *authz.MockSubRepoPermissionChecker) {
	m.FilePermissionsFuncFunc.SetDefaultHook(func(ctx context.Context, userID int32, repo api.RepoName) (authz.FilePermissionFunc, error) {
		return func(path string) (authz.Perms, error) {
			return m.Permissions(ctx, userID, authz.RepoContent{Repo: repo, Path: path})
		}, nil
	})
}

func TestClient_StreamBlameFile(t *testing.T) {
	t.Run("firstChunk memoization", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				bc := NewMockGitserverService_BlameClient()
				bc.RecvFunc.PushReturn(&proto.BlameResponse{Hunk: &proto.BlameHunk{Commit: "deadbeef"}}, nil)
				bc.RecvFunc.PushReturn(&proto.BlameResponse{Hunk: &proto.BlameHunk{Commit: "deadbeef2"}}, nil)
				bc.RecvFunc.PushReturn(nil, io.EOF)
				c.BlameFunc.SetDefaultReturn(bc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		hr, err := c.StreamBlameFile(context.Background(), "repo", "file", &BlameOptions{})
		require.NoError(t, err)

		// This chunk comes from the memoized first message.
		h, err := hr.Read()
		require.NoError(t, err)
		require.Equal(t, h.CommitID, api.CommitID("deadbeef"))

		// This chunk is returned from Recv inside the hunk reader.
		h, err = hr.Read()
		require.NoError(t, err)
		require.Equal(t, h.CommitID, api.CommitID("deadbeef2"))

		// Done.
		_, err = hr.Read()
		require.Error(t, err)
		require.Equal(t, io.EOF, err)

		require.NoError(t, hr.Close())
	})
	t.Run("permission errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				bc := NewMockGitserverService_BlameClient()
				bc.RecvFunc.PushReturn(nil, status.New(codes.PermissionDenied, "bad actor").Err())
				c.BlameFunc.SetDefaultReturn(bc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.StreamBlameFile(context.Background(), "repo", "file", &BlameOptions{})
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("revision not found errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				bc := NewMockGitserverService_BlameClient()
				s, err := status.New(codes.NotFound, "commit not found").WithDetails(&proto.RevisionNotFoundPayload{Repo: "repo", Spec: "deadbeef"})
				require.NoError(t, err)
				bc.RecvFunc.PushReturn(nil, s.Err())
				c.BlameFunc.SetDefaultReturn(bc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.StreamBlameFile(context.Background(), "repo", "file", &BlameOptions{})
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})
	t.Run("file not found errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				bc := NewMockGitserverService_BlameClient()
				s, err := status.New(codes.NotFound, "file not found").WithDetails(&proto.FileNotFoundPayload{Repo: "repo", Commit: "deadbeef", Path: "file"})
				require.NoError(t, err)
				bc.RecvFunc.PushReturn(nil, s.Err())
				c.BlameFunc.SetDefaultReturn(bc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.StreamBlameFile(context.Background(), "repo", "file", &BlameOptions{})
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("empty blame doesn't fail", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				bc := NewMockGitserverService_BlameClient()
				bc.RecvFunc.PushReturn(nil, io.EOF)
				c.BlameFunc.SetDefaultReturn(bc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		r, err := c.StreamBlameFile(context.Background(), "repo", "file", &BlameOptions{})
		require.NoError(t, err)
		h, err := r.Read()
		require.Equal(t, io.EOF, err)
		require.Nil(t, h)
		require.NoError(t, r.Close())
	})
}

func TestClient_GetDefaultBranch(t *testing.T) {
	t.Run("correctly returns server response", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				c.DefaultBranchFunc.SetDefaultReturn(&proto.DefaultBranchResponse{RefName: "refs/heads/master", Commit: "deadbeef"}, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		refName, sha, err := c.GetDefaultBranch(context.Background(), "repo", false)
		require.NoError(t, err)
		require.Equal(t, "refs/heads/master", refName)
		require.Equal(t, api.CommitID("deadbeef"), sha)
	})
	t.Run("returns empty for common errors", func(t *testing.T) {
		calls := 0
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				if calls == 0 {
					s, err := status.New(codes.NotFound, "bad revision").WithDetails(&proto.RevisionNotFoundPayload{Repo: "repo", Spec: "deadbeef"})
					require.NoError(t, err)
					c.DefaultBranchFunc.PushReturn(nil, s.Err())
					calls++
					return c
				}
				s, err := status.New(codes.NotFound, "repo cloning").WithDetails(&proto.RepoNotFoundPayload{Repo: "repo", CloneInProgress: true})
				require.NoError(t, err)
				c.DefaultBranchFunc.PushReturn(nil, s.Err())
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		// First request fails with clone error
		refName, sha, err := c.GetDefaultBranch(context.Background(), "repo", false)
		require.NoError(t, err)
		require.Equal(t, "", refName)
		require.Equal(t, api.CommitID(""), sha)
		// First request fails with bad rev error
		refName, sha, err = c.GetDefaultBranch(context.Background(), "repo", false)
		require.NoError(t, err)
		require.Equal(t, "", refName)
		require.Equal(t, api.CommitID(""), sha)
	})
}

func TestClient_MergeBase(t *testing.T) {
	t.Run("correctly returns server response", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				c.MergeBaseFunc.SetDefaultReturn(&proto.MergeBaseResponse{MergeBaseCommitSha: "deadbeef"}, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		sha, err := c.MergeBase(context.Background(), "repo", "master", "b2")
		require.NoError(t, err)
		require.Equal(t, api.CommitID("deadbeef"), sha)
	})
	t.Run("returns empty for empty merge base", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				c.MergeBaseFunc.SetDefaultReturn(&proto.MergeBaseResponse{MergeBaseCommitSha: ""}, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		sha, err := c.MergeBase(context.Background(), "repo", "master", "b2")
		require.NoError(t, err)
		require.Equal(t, api.CommitID(""), sha)
	})
	t.Run("revision not found", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				s, err := status.New(codes.NotFound, "bad revision").WithDetails(&proto.RevisionNotFoundPayload{Repo: "repo", Spec: "deadbeef"})
				require.NoError(t, err)
				c.MergeBaseFunc.SetDefaultReturn(nil, s.Err())
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.MergeBase(context.Background(), "repo", "master", "b2")
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})
}

func TestClient_NewFileReader(t *testing.T) {
	t.Run("firstChunk memoization", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ReadFileClient()
				rfc.RecvFunc.PushReturn(&proto.ReadFileResponse{Data: []byte("part1\n")}, nil)
				rfc.RecvFunc.PushReturn(&proto.ReadFileResponse{Data: []byte("part2\n")}, nil)
				rfc.RecvFunc.PushReturn(nil, io.EOF)
				c.ReadFileFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		r, err := c.NewFileReader(context.Background(), "repo", "deadbeef", "file")
		require.NoError(t, err)

		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		require.Equal(t, "part1\npart2\n", string(content))
	})
	t.Run("firstChunk error memoization", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ReadFileClient()
				rfc.RecvFunc.PushReturn(nil, io.EOF)
				c.ReadFileFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		r, err := c.NewFileReader(context.Background(), "repo", "deadbeef", "file")
		require.NoError(t, err)

		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		require.Equal(t, "", string(content))
	})
	t.Run("permission errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ReadFileClient()
				rfc.RecvFunc.PushReturn(nil, status.New(codes.PermissionDenied, "bad actor").Err())
				c.ReadFileFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.NewFileReader(context.Background(), "repo", "deadbeef", "file")
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("file not found errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ReadFileClient()
				s, err := status.New(codes.NotFound, "bad actor").WithDetails(&proto.FileNotFoundPayload{})
				require.NoError(t, err)
				rfc.RecvFunc.PushReturn(nil, s.Err())
				c.ReadFileFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.NewFileReader(context.Background(), "repo", "deadbeef", "file")
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("revision not found errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ReadFileClient()
				s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{})
				require.NoError(t, err)
				rfc.RecvFunc.PushReturn(nil, s.Err())
				c.ReadFileFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.NewFileReader(context.Background(), "repo", "deadbeef", "file")
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})
	t.Run("empty file", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ReadFileClient()
				rfc.RecvFunc.PushReturn(nil, io.EOF)
				c.ReadFileFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		r, err := c.NewFileReader(context.Background(), "repo", "deadbeef", "file")
		require.NoError(t, err)
		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Empty(t, content)
		require.NoError(t, r.Close())
	})
}

func TestClient_GetCommit(t *testing.T) {
	t.Run("correctly returns server response", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				c.GetCommitFunc.SetDefaultReturn(&proto.GetCommitResponse{Commit: &proto.GitCommit{Oid: "deadbeef"}}, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		commit, err := c.GetCommit(context.Background(), "repo", "deadbeef")
		require.NoError(t, err)
		require.Equal(t, api.CommitID("deadbeef"), commit.ID)
	})
	t.Run("returns correct error for not found", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				s, err := status.New(codes.NotFound, "bad revision").WithDetails(&proto.RevisionNotFoundPayload{Repo: "repo", Spec: "deadbeef"})
				require.NoError(t, err)
				c.GetCommitFunc.PushReturn(nil, s.Err())
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.GetCommit(context.Background(), "repo", "deadbeef")
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})
}

func Test_CommitLog(t *testing.T) {
	ClientMocks.LocalGitserver = true
	defer ResetClientMocks()

	tests := map[string]struct {
		extraGitCommands []string
		wantFiles        [][]string // put these in log reverse order
		wantCommits      int
		wantErr          string
	}{
		"commit changes files": {
			extraGitCommands: getGitCommandsWithFileLists([]string{"file1.txt", "file2.txt"}, []string{"file3.txt"}),
			wantFiles:        [][]string{{"file3.txt"}, {"file1.txt", "file2.txt"}},
			wantCommits:      2,
		},
		"no commits": {
			wantErr: "gitCommand fatal: your current branch 'master' does not have any commits yet: exit status 128",
		},
		"one file two commits": {
			extraGitCommands: getGitCommandsWithFileLists([]string{"file1.txt"}, []string{"file1.txt"}),
			wantFiles:        [][]string{{"file1.txt"}, {"file1.txt"}},
			wantCommits:      2,
		},
		"one commit": {
			extraGitCommands: getGitCommandsWithFileLists([]string{"file1.txt"}),
			wantFiles:        [][]string{{"file1.txt"}},
			wantCommits:      1,
		},
	}

	for label, test := range tests {
		t.Run(label, func(t *testing.T) {
			repo := MakeGitRepository(t, test.extraGitCommands...)
			logResults, err := NewClient("test").CommitLog(context.Background(), repo, time.Time{})
			if err != nil {
				require.ErrorContains(t, err, test.wantErr)
			}

			t.Log(test)
			for i, result := range logResults {
				t.Log(result)
				assert.Equal(t, "a@a.com", result.AuthorEmail)
				assert.Equal(t, "a", result.AuthorName)
				assert.Equal(t, 40, len(result.SHA))
				assert.ElementsMatch(t, test.wantFiles[i], result.ChangedFiles)
			}
			assert.Equal(t, test.wantCommits, len(logResults))
		})
	}
}

func TestErrorMessageTruncateOutput(t *testing.T) {
	cmd := []string{"git", "ls-files"}

	t.Run("short output", func(t *testing.T) {
		shortOutput := "aaaaaaaaaab"
		message := errorMessageTruncatedOutput(cmd, []byte(shortOutput))
		want := fmt.Sprintf("git command [git ls-files] failed (output: %q)", shortOutput)

		if diff := cmp.Diff(want, message); diff != "" {
			t.Fatalf("wrong message. diff: %s", diff)
		}
	})

	t.Run("truncating output", func(t *testing.T) {
		longOutput := strings.Repeat("a", 5000) + "b"
		message := errorMessageTruncatedOutput(cmd, []byte(longOutput))
		want := fmt.Sprintf("git command [git ls-files] failed (truncated output: %q, 1 more)", longOutput[:5000])

		if diff := cmp.Diff(want, message); diff != "" {
			t.Fatalf("wrong message. diff: %s", diff)
		}
	})
}

func TestClient_ArchiveReader(t *testing.T) {
	t.Run("firstChunk memoization", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ArchiveClient()
				rfc.RecvFunc.PushReturn(&proto.ArchiveResponse{Data: []byte("part1\n")}, nil)
				rfc.RecvFunc.PushReturn(&proto.ArchiveResponse{Data: []byte("part2\n")}, nil)
				rfc.RecvFunc.PushReturn(nil, io.EOF)
				c.ArchiveFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		r, err := c.ArchiveReader(context.Background(), "repo", ArchiveOptions{Treeish: "deadbeef", Format: ArchiveFormatTar, Paths: []string{"file"}})
		require.NoError(t, err)

		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		require.Equal(t, "part1\npart2\n", string(content))
	})
	t.Run("firstChunk error memoization", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ArchiveClient()
				rfc.RecvFunc.PushReturn(nil, io.EOF)
				c.ArchiveFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		r, err := c.ArchiveReader(context.Background(), "repo", ArchiveOptions{Treeish: "deadbeef", Format: ArchiveFormatTar, Paths: []string{"file"}})
		require.NoError(t, err)

		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		require.Equal(t, "", string(content))
	})
	t.Run("file not found errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ArchiveClient()
				s, err := status.New(codes.NotFound, "not found").WithDetails(&proto.FileNotFoundPayload{})
				require.NoError(t, err)
				rfc.RecvFunc.PushReturn(nil, s.Err())
				c.ArchiveFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.ArchiveReader(context.Background(), "repo", ArchiveOptions{Treeish: "deadbeef", Format: ArchiveFormatTar, Paths: []string{"file"}})
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("revision not found errors are returned early", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ArchiveClient()
				s, err := status.New(codes.NotFound, "revision not found").WithDetails(&proto.RevisionNotFoundPayload{})
				require.NoError(t, err)
				rfc.RecvFunc.PushReturn(nil, s.Err())
				c.ArchiveFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		_, err := c.ArchiveReader(context.Background(), "repo", ArchiveOptions{Treeish: "deadbeef", Format: ArchiveFormatTar, Paths: []string{"file"}})
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
	})
	t.Run("empty archive", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				rfc := NewMockGitserverService_ArchiveClient()
				rfc.RecvFunc.PushReturn(nil, io.EOF)
				c.ArchiveFunc.SetDefaultReturn(rfc, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		r, err := c.ArchiveReader(context.Background(), "repo", ArchiveOptions{Treeish: "deadbeef", Format: ArchiveFormatTar, Paths: []string{"file"}})
		require.NoError(t, err)
		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Empty(t, content)
		require.NoError(t, r.Close())
	})
}

func TestClient_ResolveRevision(t *testing.T) {
	t.Run("correctly returns server response", func(t *testing.T) {
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				c.ResolveRevisionFunc.SetDefaultReturn(&proto.ResolveRevisionResponse{CommitSha: "deadbeef"}, nil)
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		sha, err := c.ResolveRevision(context.Background(), "repo", "HEAD", ResolveRevisionOptions{})
		require.NoError(t, err)
		require.Equal(t, api.CommitID("deadbeef"), sha)
	})
	t.Run("returns common errors correctly", func(t *testing.T) {
		calls := 0
		source := NewTestClientSource(t, []string{"gitserver"}, func(o *TestClientSourceOptions) {
			o.ClientFunc = func(cc *grpc.ClientConn) proto.GitserverServiceClient {
				c := NewMockGitserverServiceClient()
				if calls == 0 {
					s, err := status.New(codes.NotFound, "bad revision").WithDetails(&proto.RevisionNotFoundPayload{Repo: "repo", Spec: "deadbeef"})
					require.NoError(t, err)
					c.ResolveRevisionFunc.PushReturn(nil, s.Err())
					calls++
					return c
				}
				s, err := status.New(codes.NotFound, "repo cloning").WithDetails(&proto.RepoNotFoundPayload{Repo: "repo", CloneInProgress: true})
				require.NoError(t, err)
				c.ResolveRevisionFunc.PushReturn(nil, s.Err())
				return c
			}
		})

		c := NewTestClient(t).WithClientSource(source)

		// First request fails with revision error
		_, err := c.ResolveRevision(context.Background(), "repo", "HEAD", ResolveRevisionOptions{})
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RevisionNotFoundError{}))
		// First request fails with clone error
		_, err = c.ResolveRevision(context.Background(), "repo", "HEAD", ResolveRevisionOptions{})
		require.Error(t, err)
		require.True(t, errors.HasType(err, &gitdomain.RepoNotExistError{}))
	})
}
