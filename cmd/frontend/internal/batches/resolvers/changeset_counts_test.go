package resolvers

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	bgql "github.com/sourcegraph/sourcegraph/internal/batches/graphql"
	"github.com/sourcegraph/sourcegraph/internal/batches/sources"
	"github.com/sourcegraph/sourcegraph/internal/batches/state"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/batches/syncer"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestChangesetCountsOverTimeResolver(t *testing.T) {
	counts := &state.ChangesetCounts{
		Time:                 time.Now(),
		Total:                10,
		Merged:               9,
		Closed:               8,
		Open:                 7,
		OpenApproved:         6,
		OpenChangesRequested: 5,
		OpenPending:          4,
	}

	resolver := changesetCountsResolver{counts: counts}

	tests := []struct {
		name   string
		method func() int32
		want   int32
	}{
		{name: "Total", method: resolver.Total, want: counts.Total},
		{name: "Merged", method: resolver.Merged, want: counts.Merged},
		{name: "Closed", method: resolver.Closed, want: counts.Closed},
		{name: "Open", method: resolver.Open, want: counts.Open},
		{name: "OpenApproved", method: resolver.OpenApproved, want: counts.OpenApproved},
		{name: "OpenChangesRequested", method: resolver.OpenChangesRequested, want: counts.OpenChangesRequested},
		{name: "OpenPending", method: resolver.OpenPending, want: counts.OpenPending},
	}

	for _, tc := range tests {
		if have := tc.method(); have != tc.want {
			t.Errorf("resolver.%s wrong. want=%d, have=%d", tc.name, tc.want, have)
		}
	}
}

func TestChangesetCountsOverTimeIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := actor.WithInternalActor(context.Background())
	db := database.NewDB(logger, dbtest.NewDB(t))
	rcache.SetupForTest(t)
	ratelimit.SetupForTest(t)

	cf, save := httptestutil.NewGitHubRecorderFactory(t, *update, "test-changeset-counts-over-time")
	defer save()

	userID := bt.CreateTestUser(t, db, false).ID

	repoStore := db.Repos()
	esStore := db.ExternalServices()

	gitHubToken := os.Getenv("GITHUB_TOKEN")
	if gitHubToken == "" {
		gitHubToken = "no-GITHUB_TOKEN-set"
	}
	githubExtSvc := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub",
		Config: extsvc.NewUnencryptedConfig(bt.MarshalJSON(t, &schema.GitHubConnection{
			Url:   "https://github.com",
			Token: "abc",
			Repos: []string{"sourcegraph/sourcegraph"},
		})),
	}

	err := esStore.Upsert(ctx, githubExtSvc)
	if err != nil {
		t.Fatalf("Failed to Upsert external service: %s", err)
	}

	githubSrc, err := repos.NewGitHubSource(ctx, logger, db, githubExtSvc, cf)
	if err != nil {
		t.Fatal(t)
	}

	githubRepo, err := githubSrc.GetRepo(ctx, "sourcegraph/sourcegraph")
	if err != nil {
		t.Fatal(err)
	}

	err = repoStore.Create(ctx, githubRepo)
	if err != nil {
		t.Fatal(err)
	}

	mockState := bt.MockChangesetSyncState(&protocol.RepoInfo{
		Name: githubRepo.Name,
	})
	defer mockState.Unmock()

	bstore := store.New(db, observation.TestContextTB(t), nil)

	if err := bstore.CreateSiteCredential(ctx,
		&btypes.SiteCredential{
			ExternalServiceType: githubRepo.ExternalRepo.ServiceType,
			ExternalServiceID:   githubRepo.ExternalRepo.ServiceID,
		},
		&auth.OAuthBearerTokenWithSSH{
			OAuthBearerToken: auth.OAuthBearerToken{Token: gitHubToken},
		},
	); err != nil {
		t.Fatal(err)
	}

	sourcer := sources.NewSourcer(cf)

	spec := &btypes.BatchSpec{
		NamespaceUserID: userID,
		UserID:          userID,
	}
	if err := bstore.CreateBatchSpec(ctx, spec); err != nil {
		t.Fatal(err)
	}

	batchChange := &btypes.BatchChange{
		Name:            "Test-batch-change",
		Description:     "Testing changeset counts",
		CreatorID:       userID,
		NamespaceUserID: userID,
		LastApplierID:   userID,
		LastAppliedAt:   time.Now(),
		BatchSpecID:     spec.ID,
	}

	err = bstore.CreateBatchChange(ctx, batchChange)
	if err != nil {
		t.Fatal(err)
	}

	changesets := []*btypes.Changeset{
		{
			RepoID:              githubRepo.ID,
			ExternalID:          "5834",
			ExternalServiceType: githubRepo.ExternalRepo.ServiceType,
			BatchChanges:        []btypes.BatchChangeAssoc{{BatchChangeID: batchChange.ID}},
			PublicationState:    btypes.ChangesetPublicationStatePublished,
		},
		{
			RepoID:              githubRepo.ID,
			ExternalID:          "5849",
			ExternalServiceType: githubRepo.ExternalRepo.ServiceType,
			BatchChanges:        []btypes.BatchChangeAssoc{{BatchChangeID: batchChange.ID}},
			PublicationState:    btypes.ChangesetPublicationStatePublished,
		},
	}

	for _, c := range changesets {
		if err = bstore.CreateChangeset(ctx, c); err != nil {
			t.Fatal(err)
		}

		src, err := sourcer.ForChangeset(ctx, bstore, c, sources.AuthenticationStrategyUserCredential, githubRepo)
		if err != nil {
			t.Fatalf("failed to build source for repo: %s", err)
		}
		if err := syncer.SyncChangeset(ctx, bstore, mockState.MockClient, src, githubRepo, c); err != nil {
			t.Fatal(err)
		}
	}

	s, err := newSchema(db, New(db, bstore, gitserver.NewMockClient(), logger))
	if err != nil {
		t.Fatal(err)
	}

	// We start exactly one day earlier than the first PR
	start := parseJSONTime(t, "2019-10-01T14:49:31Z")
	// Date when PR #5834 was created
	pr1Create := parseJSONTime(t, "2019-10-02T14:49:31Z")
	// Date when PR #5834 was closed
	pr1Close := parseJSONTime(t, "2019-10-03T14:02:51Z")
	// Date when PR #5834 was reopened
	pr1Reopen := parseJSONTime(t, "2019-10-03T14:02:54Z")
	// Date when PR #5834 was marked as ready for review
	pr1ReadyForReview := parseJSONTime(t, "2019-10-03T14:04:10Z")
	// Date when PR #5849 was created
	pr2Create := parseJSONTime(t, "2019-10-03T15:03:21Z")
	// Date when PR #5849 was approved
	pr2Approve := parseJSONTime(t, "2019-10-04T08:25:53Z")
	// Date when PR #5849 was merged
	pr2Merged := parseJSONTime(t, "2019-10-04T08:55:21Z")
	pr1Approved := parseJSONTime(t, "2019-10-07T12:45:49Z")
	// Date when PR #5834 was merged
	pr1Merged := parseJSONTime(t, "2019-10-07T13:13:45Z")
	// End time is when PR1 was merged
	end := parseJSONTime(t, "2019-10-07T13:13:45Z")

	input := map[string]any{
		"batchChange": string(bgql.MarshalBatchChangeID(batchChange.ID)),
		"from":        start,
		"to":          end,
	}

	var response struct{ Node apitest.BatchChange }

	apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryChangesetCountsConnection)

	wantEntries := []*state.ChangesetCounts{
		{Time: start},
		{Time: pr1Create, Total: 1, Draft: 1},
		{Time: pr1Close, Total: 1, Closed: 1},
		{Time: pr1Reopen, Total: 1, Draft: 1},
		{Time: pr1ReadyForReview, Total: 1, Open: 1, OpenPending: 1},
		{Time: pr2Create, Total: 2, Open: 2, OpenPending: 2},
		{Time: pr2Approve, Total: 2, Open: 2, OpenPending: 1, OpenApproved: 1},
		{Time: pr2Merged, Total: 2, Open: 1, OpenPending: 1, Merged: 1},
		{Time: pr1Approved, Total: 2, Open: 1, OpenApproved: 1, Merged: 1},
		{Time: pr1Merged, Total: 2, Merged: 2},
		{Time: end, Total: 2, Merged: 2},
	}
	tzs := state.GenerateTimestamps(start, end)
	wantCounts := make([]apitest.ChangesetCounts, 0, len(tzs))
	idx := 0
	for _, tz := range tzs {
		currentWant := wantEntries[idx]
		for len(wantEntries) > idx+1 && !tz.Before(wantEntries[idx+1].Time) {
			idx++
			currentWant = wantEntries[idx]
		}
		wantCounts = append(wantCounts, apitest.ChangesetCounts{
			Date:                 marshalDateTime(t, tz),
			Total:                currentWant.Total,
			Merged:               currentWant.Merged,
			Closed:               currentWant.Closed,
			Open:                 currentWant.Open,
			Draft:                currentWant.Draft,
			OpenApproved:         currentWant.OpenApproved,
			OpenChangesRequested: currentWant.OpenChangesRequested,
			OpenPending:          currentWant.OpenPending,
		})
	}

	if !reflect.DeepEqual(response.Node.ChangesetCountsOverTime, wantCounts) {
		t.Errorf("wrong counts listed. diff=%s", cmp.Diff(response.Node.ChangesetCountsOverTime, wantCounts))
	}
}

const queryChangesetCountsConnection = `
query($batchChange: ID!, $from: DateTime!, $to: DateTime!) {
  node(id: $batchChange) {
    ... on BatchChange {
	  changesetCountsOverTime(from: $from, to: $to) {
        date
        total
        merged
        draft
        closed
        open
        openApproved
        openChangesRequested
        openPending
      }
    }
  }
}
`
