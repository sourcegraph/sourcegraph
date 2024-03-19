package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	gh "github.com/google/go-github/v55/github"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches/sources"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/batches/syncer"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
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

// Run from webhooks_integration_test.go
func testGitHubWebhook(db database.DB, userID int32) func(*testing.T) {
	return func(t *testing.T) {
		ratelimit.SetupForTest(t)

		// @BolajiOlajide hardcoded the time here because we use the time to generate the key
		// for events in the `fixtures`. This key needs to be static else the tests fails, and it's not
		// advisable to ignore the `key` field because if the logic changes, our tests won't catch it.
		clock := func() time.Time { return time.Date(2023, time.May, 16, 12, 0, 0, 0, time.UTC) }

		ctx := context.Background()

		rcache.SetupForTest(t)

		bt.TruncateTables(t, db, "changeset_events", "changesets")

		cf, save := httptestutil.NewGitHubRecorderFactory(t, *update, "github-webhooks")
		defer save()

		secret := "secret"
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = "no-GITHUB_TOKEN-set"
		}
		repoStore := db.Repos()
		esStore := db.ExternalServices()
		extSvc := &types.ExternalService{
			Kind:        extsvc.KindGitHub,
			DisplayName: "GitHub",
			Config: extsvc.NewUnencryptedConfig(bt.MarshalJSON(t, &schema.GitHubConnection{
				Url:      "https://github.com",
				Repos:    []string{"sourcegraph/sourcegraph"},
				Webhooks: []*schema.GitHubWebhook{{Org: "sourcegraph", Secret: secret}},
				Token:    "abc",
			})),
		}

		err := esStore.Upsert(ctx, extSvc)
		if err != nil {
			t.Fatal(err)
		}

		githubSrc, err := repos.NewGitHubSource(ctx, logtest.Scoped(t), db, extSvc, cf)
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

		s := store.NewWithClock(db, &observation.TestContext, nil, clock)
		if err := s.CreateSiteCredential(ctx, &btypes.SiteCredential{
			ExternalServiceType: githubRepo.ExternalRepo.ServiceType,
			ExternalServiceID:   githubRepo.ExternalRepo.ServiceID,
		},
			&auth.OAuthBearerTokenWithSSH{
				OAuthBearerToken: auth.OAuthBearerToken{Token: token},
			},
		); err != nil {
			t.Fatal(err)
		}
		sourcer := sources.NewSourcer(cf)

		spec := &btypes.BatchSpec{
			NamespaceUserID: userID,
			UserID:          userID,
		}
		if err := s.CreateBatchSpec(ctx, spec); err != nil {
			t.Fatal(err)
		}

		batchChange := &btypes.BatchChange{
			Name:            "Test-batch-changes",
			Description:     "Testing THE WEBHOOKS",
			CreatorID:       userID,
			NamespaceUserID: userID,
			LastApplierID:   userID,
			LastAppliedAt:   clock(),
			BatchSpecID:     spec.ID,
		}

		err = s.CreateBatchChange(ctx, batchChange)
		if err != nil {
			t.Fatal(err)
		}

		// NOTE: Your sample payload should apply to a PR with the number matching below
		changeset := &btypes.Changeset{
			RepoID:              githubRepo.ID,
			ExternalID:          "10156",
			ExternalServiceType: githubRepo.ExternalRepo.ServiceType,
			BatchChanges:        []btypes.BatchChangeAssoc{{BatchChangeID: batchChange.ID}},
		}

		err = s.CreateChangeset(ctx, changeset)
		if err != nil {
			t.Fatal(err)
		}

		// Set up mocks to prevent the diffstat computation from trying to
		// use a real gitserver, and so we can control what diff is used to
		// create the diffstat.
		state := bt.MockChangesetSyncState(&protocol.RepoInfo{
			Name: "repo",
			VCS:  protocol.VCSInfo{URL: "https://example.com/repo/"},
		})
		defer state.Unmock()

		src, err := sourcer.ForChangeset(ctx, s, changeset, sources.AuthenticationStrategyUserCredential, githubRepo)
		if err != nil {
			t.Fatal(err)
		}

		gsClient := gitserver.NewMockClient()
		gsClient.ResolveRevisionFunc.SetDefaultHook(func(context.Context, api.RepoName, string, gitserver.ResolveRevisionOptions) (api.CommitID, error) {
			return "", nil
		})

		err = syncer.SyncChangeset(ctx, s, gsClient, src, githubRepo, changeset)
		if err != nil {
			t.Fatal(err)
		}

		hook := NewGitHubWebhook(s, gsClient, logtest.Scoped(t))

		fixtureFiles, err := filepath.Glob("testdata/fixtures/webhooks/github/*.json")
		if err != nil {
			t.Fatal(err)
		}

		for _, fixtureFile := range fixtureFiles {
			_, name := path.Split(fixtureFile)
			name = strings.TrimSuffix(name, ".json")
			t.Run(name, func(t *testing.T) {
				bt.TruncateTables(t, db, "changeset_events")

				tc := loadWebhookTestCase(t, fixtureFile)

				// Send all events twice to ensure we are idempotent
				for range 2 {
					for _, event := range tc.Payloads {
						handler := webhooks.GitHubWebhook{
							Router: &webhooks.Router{
								DB: db,
							},
						}
						hook.Register(handler.Router)

						u, err := extsvc.WebhookURL(extsvc.TypeGitHub, extSvc.ID, nil, "https://example.com/")
						if err != nil {
							t.Fatal(err)
						}

						req, err := http.NewRequest("POST", u, bytes.NewReader(event.Data))
						if err != nil {
							t.Fatal(err)
						}
						req.Header.Set("X-Github-Event", event.PayloadType)
						req.Header.Set("X-Hub-Signature", sign(t, event.Data, []byte(secret)))

						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, req)
						resp := rec.Result()

						if resp.StatusCode != http.StatusOK {
							t.Fatalf("Non 200 code: %v", resp.StatusCode)
						}
					}
				}

				have, _, err := s.ListChangesetEvents(ctx, store.ListChangesetEventsOpts{})
				if err != nil {
					t.Fatal(err)
				}

				// Overwrite and format test case
				if *update {
					tc.ChangesetEvents = have
					data, err := json.MarshalIndent(tc, "  ", "  ")
					if err != nil {
						t.Fatal(err)
					}
					err = os.WriteFile(fixtureFile, data, 0o666)
					if err != nil {
						t.Fatal(err)
					}
				}

				opts := []cmp.Option{
					cmpopts.IgnoreFields(btypes.ChangesetEvent{}, "CreatedAt"),
					cmpopts.IgnoreFields(btypes.ChangesetEvent{}, "UpdatedAt"),
				}

				if diff := cmp.Diff(tc.ChangesetEvents, have, opts...); diff != "" {
					t.Error(diff)
				}
			})
		}

		t.Run("unexpected payload", func(t *testing.T) {
			// GitHub pull request events are processed based on the action
			// embedded within them, but that action is just a string that could
			// be anything. We need to ensure that this is hardened against
			// unexpected input.
			n := 10156
			action := "this is a bad action"
			u, err := extsvc.NewCodeHostBaseURL("github.com")
			require.NoError(t, err)
			if err := hook.handleGitHubWebhook(ctx, db, u, &gh.PullRequestEvent{
				Number: &n,
				Repo: &gh.Repository{
					NodeID: &githubRepo.ExternalRepo.ID,
				},
				Action: &action,
			}); err != nil {
				t.Errorf("unexpected non-nil error: %v", err)
			}
		})
	}
}
