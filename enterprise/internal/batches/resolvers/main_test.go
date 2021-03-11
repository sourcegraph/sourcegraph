package resolvers

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

func init() {
	dbtesting.DBNameSuffix = "batchchangesresolversdb"
}

var update = flag.Bool("update", false, "update testdata")

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		log15.Root().SetHandler(log15.DiscardHandler())
	}
	os.Exit(m.Run())
}

const testDiff = `diff README.md README.md
index 671e50a..851b23a 100644
--- README.md
+++ README.md
@@ -1,2 +1,2 @@
 # README
-This file is hosted at example.com and is a test file.
+This file is hosted at sourcegraph.com and is a test file.
diff --git urls.txt urls.txt
index 6f8b5d9..17400bc 100644
--- urls.txt
+++ urls.txt
@@ -1,3 +1,3 @@
 another-url.com
-example.com
+sourcegraph.com
 never-touch-the-mouse.com
`

// testDiffGraphQL is the parsed representation of testDiff.
var testDiffGraphQL = apitest.FileDiffs{
	TotalCount: 2,
	RawDiff:    testDiff,
	DiffStat:   apitest.DiffStat{Changed: 2},
	PageInfo:   apitest.PageInfo{},
	Nodes: []apitest.FileDiff{
		{
			OldPath: "README.md",
			NewPath: "README.md",
			OldFile: apitest.File{Name: "README.md"},
			Hunks: []apitest.FileDiffHunk{
				{
					Body:     " # README\n-This file is hosted at example.com and is a test file.\n+This file is hosted at sourcegraph.com and is a test file.\n",
					OldRange: apitest.DiffRange{StartLine: 1, Lines: 2},
					NewRange: apitest.DiffRange{StartLine: 1, Lines: 2},
				},
			},
			Stat: apitest.DiffStat{Changed: 1},
		},
		{
			OldPath: "urls.txt",
			NewPath: "urls.txt",
			OldFile: apitest.File{Name: "urls.txt"},
			Hunks: []apitest.FileDiffHunk{
				{
					Body:     " another-url.com\n-example.com\n+sourcegraph.com\n never-touch-the-mouse.com\n",
					OldRange: apitest.DiffRange{StartLine: 1, Lines: 3},
					NewRange: apitest.DiffRange{StartLine: 1, Lines: 3},
				},
			},
			Stat: apitest.DiffStat{Changed: 1},
		},
	},
}

func marshalDateTime(t testing.TB, ts time.Time) string {
	t.Helper()

	dt := graphqlbackend.DateTime{Time: ts}

	bs, err := dt.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// Unquote the date time.
	return strings.ReplaceAll(string(bs), "\"", "")
}

func parseJSONTime(t testing.TB, ts string) time.Time {
	t.Helper()

	timestamp, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatal(err)
	}

	return timestamp
}

func newGitHubExternalService(t *testing.T, store *database.ExternalServiceStore) *types.ExternalService {
	t.Helper()

	clock := timeutil.NewFakeClock(time.Now(), 0)
	now := clock.Now()

	svc := types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "Github - Test",
		// The authorization field is needed to enforce permissions
		Config:    `{"url": "https://github.com", "authorization": {}}`,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := store.Upsert(context.Background(), &svc); err != nil {
		t.Fatalf("failed to insert external services: %v", err)
	}

	return &svc
}

func newGitHubTestRepo(name string, externalService *types.ExternalService) *types.Repo {
	return &types.Repo{
		Name:    api.RepoName(name),
		Private: true,
		ExternalRepo: api.ExternalRepoSpec{
			ID:          fmt.Sprintf("external-id-%d", externalService.ID),
			ServiceType: "github",
			ServiceID:   "https://github.com/",
		},
		Sources: map[string]*types.SourceInfo{
			externalService.URN(): {
				ID:       externalService.URN(),
				CloneURL: fmt.Sprintf("https://secrettoken@%s", name),
			},
		},
	}
}

func mockBackendCommits(t *testing.T, revs ...api.CommitID) {
	t.Helper()

	byRev := map[api.CommitID]struct{}{}
	for _, r := range revs {
		byRev[r] = struct{}{}
	}

	backend.Mocks.Repos.ResolveRev = func(_ context.Context, _ *types.Repo, rev string) (api.CommitID, error) {
		if _, ok := byRev[api.CommitID(rev)]; !ok {
			t.Fatalf("ResolveRev received unexpected rev: %q", rev)
		}
		return api.CommitID(rev), nil
	}
	t.Cleanup(func() { backend.Mocks.Repos.ResolveRev = nil })

	backend.Mocks.Repos.GetCommit = func(_ context.Context, _ *types.Repo, id api.CommitID) (*git.Commit, error) {
		if _, ok := byRev[id]; !ok {
			t.Fatalf("GetCommit received unexpected ID: %s", id)
		}
		return &git.Commit{ID: id}, nil
	}
	t.Cleanup(func() { backend.Mocks.Repos.GetCommit = nil })
}

func mockRepoComparison(t *testing.T, baseRev, headRev, diff string) {
	t.Helper()

	spec := fmt.Sprintf("%s...%s", baseRev, headRev)

	git.Mocks.ResolveRevision = func(spec string, opt git.ResolveRevisionOptions) (api.CommitID, error) {
		if spec != baseRev && spec != headRev {
			t.Fatalf("git.Mocks.ResolveRevision received unknown spec: %s", spec)
		}
		return api.CommitID(spec), nil
	}
	t.Cleanup(func() { git.Mocks.GetCommit = nil })

	git.Mocks.ExecReader = func(args []string) (io.ReadCloser, error) {
		if len(args) < 1 && args[0] != "diff" {
			t.Fatalf("gitserver.ExecReader received wrong args: %v", args)
		}

		if have, want := args[len(args)-2], spec; have != want {
			t.Fatalf("gitserver.ExecReader received wrong spec: %q, want %q", have, want)
		}
		return ioutil.NopCloser(strings.NewReader(diff)), nil
	}
	t.Cleanup(func() { git.Mocks.ExecReader = nil })

	git.Mocks.MergeBase = func(repo api.RepoName, a, b api.CommitID) (api.CommitID, error) {
		if string(a) != baseRev && string(b) != headRev {
			t.Fatalf("git.Mocks.MergeBase received unknown commit ids: %s %s", a, b)
		}
		return a, nil
	}
	t.Cleanup(func() { git.Mocks.MergeBase = nil })
}

func addChangeset(t *testing.T, ctx context.Context, s *store.Store, c *batches.Changeset, batchChange int64) {
	t.Helper()

	c.BatchChanges = append(c.BatchChanges, batches.BatchChangeAssoc{BatchChangeID: batchChange})
	if err := s.UpdateChangeset(ctx, c); err != nil {
		t.Fatal(err)
	}
}

func pruneUserCredentials(t *testing.T, db dbutil.DB) {
	t.Helper()
	creds, _, err := database.UserCredentials(db).List(context.Background(), database.UserCredentialsListOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range creds {
		if err := database.UserCredentials(db).Delete(context.Background(), c.ID); err != nil {
			t.Fatal(err)
		}
	}
}
