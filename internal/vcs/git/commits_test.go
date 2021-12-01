package git

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
)

func TestRepository_GetCommit(t *testing.T) {
	ctx := context.Background()

	gitCommands := []string{
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit --allow-empty -m foo --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		"GIT_COMMITTER_NAME=c GIT_COMMITTER_EMAIL=c@c.com GIT_COMMITTER_DATE=2006-01-02T15:04:07Z git commit --allow-empty -m bar --author='a <a@a.com>' --date 2006-01-02T15:04:06Z",
	}
	wantGitCommit := &gitdomain.Commit{
		ID:        "b266c7e3ca00b1a17ad0b1449825d0854225c007",
		Author:    gitdomain.Signature{Name: "a", Email: "a@a.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:06Z")},
		Committer: &gitdomain.Signature{Name: "c", Email: "c@c.com", Date: MustParseTime(time.RFC3339, "2006-01-02T15:04:07Z")},
		Message:   "bar",
		Parents:   []api.CommitID{"ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"},
	}
	tests := map[string]struct {
		repo             api.RepoName
		id               api.CommitID
		wantCommit       *gitdomain.Commit
		noEnsureRevision bool
	}{
		"git cmd with NoEnsureRevision false": {
			repo:             MakeGitRepository(t, gitCommands...),
			id:               "b266c7e3ca00b1a17ad0b1449825d0854225c007",
			wantCommit:       wantGitCommit,
			noEnsureRevision: false,
		},
		"git cmd with NoEnsureRevision true": {
			repo:             MakeGitRepository(t, gitCommands...),
			id:               "b266c7e3ca00b1a17ad0b1449825d0854225c007",
			wantCommit:       wantGitCommit,
			noEnsureRevision: true,
		},
	}

	oldRunCommitLog := runCommitLog

	for label, test := range tests {
		var noEnsureRevision bool
		t.Cleanup(func() {
			runCommitLog = oldRunCommitLog
		})
		runCommitLog = func(ctx context.Context, cmd *gitserver.Cmd, opt CommitsOptions) ([]*gitdomain.Commit, error) {
			// Track the value of NoEnsureRevision we pass to gitserver
			noEnsureRevision = opt.NoEnsureRevision
			return oldRunCommitLog(ctx, cmd, opt)
		}

		resolveRevisionOptions := ResolveRevisionOptions{
			NoEnsureRevision: test.noEnsureRevision,
		}
		commit, err := GetCommit(ctx, test.repo, test.id, resolveRevisionOptions)
		if err != nil {
			t.Errorf("%s: GetCommit: %s", label, err)
			continue
		}

		if !CommitsEqual(commit, test.wantCommit) {
			t.Errorf("%s: got commit == %+v, want %+v", label, commit, test.wantCommit)
		}

		// Test that trying to get a nonexistent commit returns RevisionNotFoundError.
		if _, err := GetCommit(ctx, test.repo, NonExistentCommitID, resolveRevisionOptions); !errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
			t.Errorf("%s: for nonexistent commit: got err %v, want RevisionNotFoundError", label, err)
		}

		if noEnsureRevision != test.noEnsureRevision {
			t.Fatalf("Expected %t, got %t", test.noEnsureRevision, noEnsureRevision)
		}
	}
}

func TestRepository_HasCommitAfter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testCases := []struct {
		commitDates []string
		after       string
		revspec     string
		want        bool
	}{
		{
			commitDates: []string{
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:05Z",
				"2008-01-02T15:04:05Z",
			},
			after:   "2006-01-02T15:04:05Z",
			revspec: "master",
			want:    true,
		},
		{
			commitDates: []string{
				"2016-01-02T15:04:05Z",
				"2017-01-02T15:04:05Z",
				"2017-01-02T15:04:06Z",
			},
			after:   "1 year ago",
			revspec: "master",
			want:    false,
		},
		{
			commitDates: []string{
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:05Z",
				"2008-01-02T15:04:05Z",
			},
			after:   "2010-01-02T15:04:05Z",
			revspec: "HEAD",
			want:    false,
		},
		{
			commitDates: []string{
				"2006-01-02T15:04:05Z",
				"2007-01-02T15:04:05Z",
				"2007-01-02T15:04:06Z",
			},
			after:   "2007-01-02T15:04:05Z",
			revspec: "HEAD",
			want:    true,
		},
		{
			commitDates: []string{
				"2016-01-02T15:04:05Z",
				"2017-01-02T15:04:05Z",
				"2017-01-02T15:04:06Z",
			},
			after:   "10 years ago",
			revspec: "HEAD",
			want:    true,
		},
	}

	for _, tc := range testCases {
		gitCommands := make([]string, len(tc.commitDates))
		for i, date := range tc.commitDates {
			gitCommands[i] = fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=%s git commit --allow-empty -m foo --author='a <a@a.com>'", date)
		}

		repo := MakeGitRepository(t, gitCommands...)
		got, err := HasCommitAfter(ctx, repo, tc.after, tc.revspec)
		if err != nil || got != tc.want {
			t.Errorf("got %t hascommitafter, want %t", got, tc.want)
		}
	}
}

func TestRepository_FirstEverCommit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
	for _, tc := range testCases {
		gitCommands := make([]string, len(tc.commitDates))
		for i, date := range tc.commitDates {
			gitCommands[i] = fmt.Sprintf("GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=%s git commit --allow-empty -m foo --author='a <a@a.com>'", date)
		}

		repo := MakeGitRepository(t, gitCommands...)
		gotCommit, err := FirstEverCommit(ctx, repo)
		if err != nil {
			t.Fatal(err)
		}
		got := gotCommit.Committer.Date.Format(time.RFC3339)
		if got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestHead(t *testing.T) {
	t.Parallel()

	gitCommands := []string{
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit --allow-empty -m foo --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
	}
	repo := MakeGitRepository(t, gitCommands...)
	ctx := context.Background()

	head, exists, err := Head(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	wantHead := "ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8"
	if head != wantHead {
		t.Fatalf("Want %q, got %q", wantHead, head)
	}
	if !exists {
		t.Fatal("Should exist")
	}
}

func TestCommitExists(t *testing.T) {
	t.Parallel()

	gitCommands := []string{
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit --allow-empty -m foo --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
	}
	repo := MakeGitRepository(t, gitCommands...)
	ctx := context.Background()

	wantCommit := api.CommitID("ea167fe3d76b1e5fd3ed8ca44cbd2fe3897684f8")
	exists, err := CommitExists(ctx, repo, wantCommit)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("Should exist")
	}

	exists, err = CommitExists(ctx, repo, NonExistentCommitID)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("Should not exist")
	}
}

func TestRepository_Commits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// TODO(sqs): test CommitsOptions.Base

	gitCommands := []string{
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit --allow-empty -m foo --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
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

	for label, test := range tests {
		commits, err := Commits(ctx, test.repo, CommitsOptions{Range: string(test.id)})
		if err != nil {
			t.Errorf("%s: Commits: %s", label, err)
			continue
		}

		total, err := CommitCount(ctx, test.repo, CommitsOptions{Range: string(test.id)})
		if err != nil {
			t.Errorf("%s: CommitCount: %s", label, err)
			continue
		}

		if total != test.wantTotal {
			t.Errorf("%s: got %d total commits, want %d", label, total, test.wantTotal)
		}

		if len(commits) != len(test.wantCommits) {
			t.Errorf("%s: got %d commits, want %d", label, len(commits), len(test.wantCommits))
		}

		for i := 0; i < len(commits) || i < len(test.wantCommits); i++ {
			var gotC, wantC *gitdomain.Commit
			if i < len(commits) {
				gotC = commits[i]
			}
			if i < len(test.wantCommits) {
				wantC = test.wantCommits[i]
			}
			if !CommitsEqual(gotC, wantC) {
				t.Errorf("%s: got commit %d == %+v, want %+v", label, i, gotC, wantC)
			}
		}

		// Test that trying to get a nonexistent commit returns RevisionNotFoundError.
		if _, err := Commits(ctx, test.repo, CommitsOptions{Range: string(NonExistentCommitID)}); !errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
			t.Errorf("%s: for nonexistent commit: got err %v, want RevisionNotFoundError", label, err)
		}
	}
}

func TestRepository_Commits_options(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gitCommands := []string{
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit --allow-empty -m foo --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
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
		repo        api.RepoName
		opt         CommitsOptions
		wantCommits []*gitdomain.Commit
		wantTotal   uint
	}{
		"git cmd": {
			repo:        MakeGitRepository(t, gitCommands...),
			opt:         CommitsOptions{Range: "ade564eba4cf904492fb56dcd287ac633e6e082c", N: 1, Skip: 1},
			wantCommits: wantGitCommits,
			wantTotal:   1,
		},
		"git cmd Head": {
			repo: MakeGitRepository(t, gitCommands...),
			opt: CommitsOptions{
				Range: "b266c7e3ca00b1a17ad0b1449825d0854225c007...ade564eba4cf904492fb56dcd287ac633e6e082c",
			},
			wantCommits: wantGitCommits2,
			wantTotal:   1,
		},
		"before": {
			repo: MakeGitRepository(t, gitCommands...),
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

	for label, test := range tests {
		commits, err := Commits(ctx, test.repo, test.opt)
		if err != nil {
			t.Errorf("%s: Commits(): %s", label, err)
			continue
		}

		total, err := CommitCount(ctx, test.repo, test.opt)
		if err != nil {
			t.Errorf("%s: CommitCount(): %s", label, err)
			continue
		}

		if total != test.wantTotal {
			t.Errorf("%s: got %d total commits, want %d", label, total, test.wantTotal)
		}

		if len(commits) != len(test.wantCommits) {
			t.Errorf("%s: got %d commits, want %d", label, len(commits), len(test.wantCommits))
		}

		for i := 0; i < len(commits) || i < len(test.wantCommits); i++ {
			var gotC, wantC *gitdomain.Commit
			if i < len(commits) {
				gotC = commits[i]
			}
			if i < len(test.wantCommits) {
				wantC = test.wantCommits[i]
			}
			if !CommitsEqual(gotC, wantC) {
				t.Errorf("%s: got commit %d == %+v, want %+v", label, i, gotC, wantC)
			}
		}
	}
}

func TestRepository_Commits_options_path(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gitCommands := []string{
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit --allow-empty -m commit1 --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		"touch file1",
		"touch --date=2006-01-02T15:04:05Z file1 || touch -t " + Times[0] + " file1",
		"git add file1",
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit -m commit2 --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
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
		repo        api.RepoName
		opt         CommitsOptions
		wantCommits []*gitdomain.Commit
		wantTotal   uint
	}{
		"git cmd Path 0": {
			repo: MakeGitRepository(t, gitCommands...),
			opt: CommitsOptions{
				Range: "master",
				Path:  "doesnt-exist",
			},
			wantCommits: nil,
			wantTotal:   0,
		},
		"git cmd Path 1": {
			repo: MakeGitRepository(t, gitCommands...),
			opt: CommitsOptions{
				Range: "master",
				Path:  "file1",
			},
			wantCommits: wantGitCommits,
			wantTotal:   1,
		},
	}

	for label, test := range tests {
		commits, err := Commits(ctx, test.repo, test.opt)
		if err != nil {
			t.Errorf("%s: Commits(): %s", label, err)
			continue
		}

		total, err := CommitCount(ctx, test.repo, test.opt)
		if err != nil {
			t.Errorf("%s: CommitCount: %s", label, err)
			continue
		}

		if total != test.wantTotal {
			t.Errorf("%s: got %d total commits, want %d", label, total, test.wantTotal)
		}

		if len(commits) != len(test.wantCommits) {
			t.Errorf("%s: got %d commits, want %d", label, len(commits), len(test.wantCommits))
		}

		for i := 0; i < len(commits) || i < len(test.wantCommits); i++ {
			var gotC, wantC *gitdomain.Commit
			if i < len(commits) {
				gotC = commits[i]
			}
			if i < len(test.wantCommits) {
				wantC = test.wantCommits[i]
			}
			if !CommitsEqual(gotC, wantC) {
				t.Errorf("%s: got commit %d == %+v, want %+v", label, i, gotC, wantC)
			}
		}
	}
}

func TestMessage(t *testing.T) {
	t.Run("Body", func(t *testing.T) {
		tests := map[gitdomain.Message]string{
			"hello":                 "",
			"hello\n":               "",
			"hello\n\n":             "",
			"hello\nworld":          "world",
			"hello\n\nworld":        "world",
			"hello\n\nworld\nfoo":   "world\nfoo",
			"hello\n\nworld\nfoo\n": "world\nfoo",
		}
		for input, want := range tests {
			got := input.Body()
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		}
	})
}

func TestParseCommitsUniqueToBranch(t *testing.T) {
	mustParseDate := func(s string) time.Time {
		date, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("unexpected error parsing date string: %s", err)
		}

		return date
	}

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
		"c165bfff52e9d4f87891bba497e3b70fea144d89": mustParseDate("2020-08-04T08:23:30-05:00"),
		"f73ee8ed601efea74f3b734eeb073307e1615606": mustParseDate("2020-04-16T16:06:21-04:00"),
		"6057f7ed8d331c82030c713b650fc8fd2c0c2347": mustParseDate("2020-04-16T16:20:26-04:00"),
		"7886287b8758d1baf19cf7b8253856128369a2a7": mustParseDate("2020-04-16T16:55:58-04:00"),
		"b69f89473bbcc04dc52cafaf6baa504e34791f5a": mustParseDate("2020-04-20T12:10:49-04:00"),
		"172b7fcf8b8c49b37b231693433586c2bfd1619e": mustParseDate("2020-04-20T12:37:36-04:00"),
		"5bc35c78fb5fb388891ca944cd12d85fd6dede95": mustParseDate("2020-05-05T12:53:18-05:00"),
	}
	if diff := cmp.Diff(expectedCommits, commits); diff != "" {
		t.Errorf("unexpected commits (-want +got):\n%s", diff)
	}
}

func TestParseBranchesContaining(t *testing.T) {
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
