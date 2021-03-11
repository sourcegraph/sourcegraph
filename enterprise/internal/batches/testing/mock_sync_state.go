package testing

import (
	"io"
	"io/ioutil"
	"strings"

	"github.com/pkg/errors"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

type MockedChangesetSyncState struct {
	// DiffStat is the diff.Stat of the mocked "git diff" call to gitserver.
	DiffStat *diff.Stat

	execReader      func([]string) (io.ReadCloser, error)
	mockRepoLookup  func(protocol.RepoLookupArgs) (*protocol.RepoLookupResult, error)
	resolveRevision func(string, git.ResolveRevisionOptions) (api.CommitID, error)
}

// MockChangesetSyncState sets up mocks such that invoking SetDerivedState() with
// a Changeset will use the same diff (+1, ~1, -3) when setting the SyncState
// on a Changeset.
//
// state.Unmock() must called to clean up, usually via defer.
func MockChangesetSyncState(repo *protocol.RepoInfo) *MockedChangesetSyncState {
	state := &MockedChangesetSyncState{
		// This diff.Stat matches the testGitHubDiff below
		DiffStat: &diff.Stat{Added: 1, Changed: 1, Deleted: 3},

		execReader:      git.Mocks.ExecReader,
		mockRepoLookup:  repoupdater.MockRepoLookup,
		resolveRevision: git.Mocks.ResolveRevision,
	}

	repoupdater.MockRepoLookup = func(args protocol.RepoLookupArgs) (*protocol.RepoLookupResult, error) {
		return &protocol.RepoLookupResult{
			Repo: repo,
		}, nil
	}

	git.Mocks.ExecReader = func(args []string) (io.ReadCloser, error) {
		// This provides a diff that will resolve to 1 added line, 1 changed
		// line, and 3 deleted lines.
		const testGitHubDiff = `
diff --git a/test.py b/test.py
index 884601b..c4886d5 100644
--- a/test.py
+++ b/test.py
@@ -1,6 +1,4 @@
+# square makes a value squarer.
 def square(a):
-    """
-    square makes a value squarer.
-    """

-    return a * a
+    return pow(a, 2)

`

		if len(args) < 1 && args[0] != "diff" {
			if state.execReader != nil {
				return state.execReader(args)
			}
			return nil, errors.New("cannot handle non-diff command in mock ExecReader")
		}
		return ioutil.NopCloser(strings.NewReader(testGitHubDiff)), nil
	}

	git.Mocks.ResolveRevision = func(spec string, opt git.ResolveRevisionOptions) (api.CommitID, error) {
		return "mockcommitid", nil
	}

	return state
}

// Unmock resets the mocks set up by MockGitHubChangesetSync.
func (state *MockedChangesetSyncState) Unmock() {
	git.Mocks.ExecReader = state.execReader
	git.Mocks.ResolveRevision = state.resolveRevision
	repoupdater.MockRepoLookup = state.mockRepoLookup
}
