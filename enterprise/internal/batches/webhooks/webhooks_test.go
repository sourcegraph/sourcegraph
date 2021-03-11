package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/ioutil"
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

	"github.com/sourcegraph/sourcegraph/cmd/frontend/webhooks"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/syncer"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

var update = flag.Bool("update", false, "update testdata")

// Run from integration_test.go
func testGitHubWebhook(db *sql.DB, userID int32) func(*testing.T) {
	return func(t *testing.T) {
		now := timeutil.Now()
		clock := func() time.Time { return now }

		ctx := context.Background()

		rcache.SetupForTest(t)

		ct.TruncateTables(t, db, "changeset_events", "changesets")

		cf, save := httptestutil.NewGitHubRecorderFactory(t, *update, "github-webhooks")
		defer save()

		secret := "secret"
		repoStore := database.Repos(db)
		esStore := database.ExternalServices(db)
		extSvc := &types.ExternalService{
			Kind:        extsvc.KindGitHub,
			DisplayName: "GitHub",
			Config: ct.MarshalJSON(t, &schema.GitHubConnection{
				Url:      "https://github.com",
				Token:    os.Getenv("GITHUB_TOKEN"),
				Repos:    []string{"sourcegraph/sourcegraph"},
				Webhooks: []*schema.GitHubWebhook{{Org: "sourcegraph", Secret: secret}},
			}),
		}

		err := esStore.Upsert(ctx, extSvc)
		if err != nil {
			t.Fatal(t)
		}

		githubSrc, err := repos.NewGithubSource(extSvc, cf)
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

		s := store.NewWithClock(db, clock)

		spec := &batches.BatchSpec{
			NamespaceUserID: userID,
			UserID:          userID,
		}
		if err := s.CreateBatchSpec(ctx, spec); err != nil {
			t.Fatal(err)
		}

		batchChange := &batches.BatchChange{
			Name:             "Test batch changes",
			Description:      "Testing THE WEBHOOKS",
			InitialApplierID: userID,
			NamespaceUserID:  userID,
			LastApplierID:    userID,
			LastAppliedAt:    clock(),
			BatchSpecID:      spec.ID,
		}

		err = s.CreateBatchChange(ctx, batchChange)
		if err != nil {
			t.Fatal(err)
		}

		// NOTE: Your sample payload should apply to a PR with the number matching below
		changeset := &batches.Changeset{
			RepoID:              githubRepo.ID,
			ExternalID:          "10156",
			ExternalServiceType: githubRepo.ExternalRepo.ServiceType,
			BatchChanges:        []batches.BatchChangeAssoc{{BatchChangeID: batchChange.ID}},
		}

		err = s.CreateChangeset(ctx, changeset)
		if err != nil {
			t.Fatal(err)
		}

		// Set up mocks to prevent the diffstat computation from trying to
		// use a real gitserver, and so we can control what diff is used to
		// create the diffstat.
		state := ct.MockChangesetSyncState(&protocol.RepoInfo{
			Name: "repo",
			VCS:  protocol.VCSInfo{URL: "https://example.com/repo/"},
		})
		defer state.Unmock()

		err = syncer.SyncChangeset(ctx, s, githubSrc, githubRepo, changeset)
		if err != nil {
			t.Fatal(err)
		}

		hook := NewGitHubWebhook(s)

		fixtureFiles, err := filepath.Glob("testdata/fixtures/webhooks/github/*.json")
		if err != nil {
			t.Fatal(err)
		}

		for _, fixtureFile := range fixtureFiles {
			_, name := path.Split(fixtureFile)
			name = strings.TrimSuffix(name, ".json")
			t.Run(name, func(t *testing.T) {
				ct.TruncateTables(t, db, "changeset_events")

				tc := loadWebhookTestCase(t, fixtureFile)

				// Send all events twice to ensure we are idempotent
				for i := 0; i < 2; i++ {
					for _, event := range tc.Payloads {
						handler := webhooks.GitHubWebhook{
							ExternalServices: esStore,
						}
						hook.Register(&handler)

						u := extsvc.WebhookURL(extsvc.TypeGitHub, extSvc.ID, "https://example.com/")

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
					err = ioutil.WriteFile(fixtureFile, data, 0666)
					if err != nil {
						t.Fatal(err)
					}
				}

				opts := []cmp.Option{
					cmpopts.IgnoreFields(batches.ChangesetEvent{}, "CreatedAt"),
					cmpopts.IgnoreFields(batches.ChangesetEvent{}, "UpdatedAt"),
				}
				if diff := cmp.Diff(tc.ChangesetEvents, have, opts...); diff != "" {
					t.Error(diff)
				}
			})
		}
	}
}

// Run from integration_test.go
func testBitbucketWebhook(db *sql.DB, userID int32) func(*testing.T) {
	return func(t *testing.T) {
		now := timeutil.Now()
		clock := func() time.Time { return now }

		ctx := context.Background()

		rcache.SetupForTest(t)

		ct.TruncateTables(t, db, "changeset_events", "changesets")

		cf, save := httptestutil.NewGitHubRecorderFactory(t, *update, "bitbucket-webhooks")
		defer save()

		secret := "secret"
		repoStore := database.Repos(db)
		esStore := database.ExternalServices(db)
		extSvc := &types.ExternalService{
			Kind:        extsvc.KindBitbucketServer,
			DisplayName: "Bitbucket",
			Config: ct.MarshalJSON(t, &schema.BitbucketServerConnection{
				Url:   "https://bitbucket.sgdev.org",
				Token: os.Getenv("BITBUCKET_SERVER_TOKEN"),
				Repos: []string{"SOUR/automation-testing"},
				Webhooks: &schema.Webhooks{
					Secret: secret,
				},
			}),
		}

		err := esStore.Upsert(ctx, extSvc)
		if err != nil {
			t.Fatal(t)
		}

		bitbucketSource, err := repos.NewBitbucketServerSource(extSvc, cf)
		if err != nil {
			t.Fatal(t)
		}

		bitbucketRepo, err := getSingleRepo(ctx, bitbucketSource, "bitbucket.sgdev.org/SOUR/automation-testing")
		if err != nil {
			t.Fatal(err)
		}

		if bitbucketRepo == nil {
			t.Fatal("repo not found")
		}

		err = repoStore.Create(ctx, bitbucketRepo)
		if err != nil {
			t.Fatal(err)
		}

		s := store.NewWithClock(db, clock)

		spec := &batches.BatchSpec{
			NamespaceUserID: userID,
			UserID:          userID,
		}
		if err := s.CreateBatchSpec(ctx, spec); err != nil {
			t.Fatal(err)
		}

		batchChange := &batches.BatchChange{
			Name:             "Test batch change",
			Description:      "Testing THE WEBHOOKS",
			InitialApplierID: userID,
			NamespaceUserID:  userID,
			LastApplierID:    userID,
			LastAppliedAt:    clock(),
			BatchSpecID:      spec.ID,
		}

		err = s.CreateBatchChange(ctx, batchChange)
		if err != nil {
			t.Fatal(err)
		}

		changesets := []*batches.Changeset{
			{
				RepoID:              bitbucketRepo.ID,
				ExternalID:          "69",
				ExternalServiceType: bitbucketRepo.ExternalRepo.ServiceType,
				BatchChanges:        []batches.BatchChangeAssoc{{BatchChangeID: batchChange.ID}},
			},
			{
				RepoID:              bitbucketRepo.ID,
				ExternalID:          "19",
				ExternalServiceType: bitbucketRepo.ExternalRepo.ServiceType,
				BatchChanges:        []batches.BatchChangeAssoc{{BatchChangeID: batchChange.ID}},
			},
		}

		// Set up mocks to prevent the diffstat computation from trying to
		// use a real gitserver, and so we can control what diff is used to
		// create the diffstat.
		state := ct.MockChangesetSyncState(&protocol.RepoInfo{
			Name: "repo",
			VCS:  protocol.VCSInfo{URL: "https://example.com/repo/"},
		})
		defer state.Unmock()

		for _, ch := range changesets {
			if err := s.CreateChangeset(ctx, ch); err != nil {
				t.Fatal(err)
			}

			err = syncer.SyncChangeset(ctx, s, bitbucketSource, bitbucketRepo, ch)
			if err != nil {
				t.Fatal(err)
			}
		}

		hook := NewBitbucketServerWebhook(s, "testhook")

		fixtureFiles, err := filepath.Glob("testdata/fixtures/webhooks/bitbucketserver/*.json")
		if err != nil {
			t.Fatal(err)
		}

		for _, fixtureFile := range fixtureFiles {
			_, name := path.Split(fixtureFile)
			name = strings.TrimSuffix(name, ".json")
			t.Run(name, func(t *testing.T) {
				ct.TruncateTables(t, db, "changeset_events")

				tc := loadWebhookTestCase(t, fixtureFile)

				// Send all events twice to ensure we are idempotent
				for i := 0; i < 2; i++ {
					for _, event := range tc.Payloads {
						u := extsvc.WebhookURL(extsvc.TypeBitbucketServer, extSvc.ID, "https://example.com/")

						req, err := http.NewRequest("POST", u, bytes.NewReader(event.Data))
						if err != nil {
							t.Fatal(err)
						}
						req.Header.Set("X-Event-Key", event.PayloadType)
						req.Header.Set("X-Hub-Signature", sign(t, event.Data, []byte(secret)))

						rec := httptest.NewRecorder()
						hook.ServeHTTP(rec, req)
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
					err = ioutil.WriteFile(fixtureFile, data, 0666)
					if err != nil {
						t.Fatal(err)
					}
				}

				opts := []cmp.Option{
					cmpopts.IgnoreFields(batches.ChangesetEvent{}, "CreatedAt"),
					cmpopts.IgnoreFields(batches.ChangesetEvent{}, "UpdatedAt"),
				}
				if diff := cmp.Diff(tc.ChangesetEvents, have, opts...); diff != "" {
					t.Error(diff)
				}

			})
		}
	}
}

func getSingleRepo(ctx context.Context, bitbucketSource *repos.BitbucketServerSource, name string) (*types.Repo, error) {
	repoChan := make(chan repos.SourceResult)
	go func() {
		bitbucketSource.ListRepos(ctx, repoChan)
		close(repoChan)
	}()

	var bitbucketRepo *types.Repo
	for result := range repoChan {
		if result.Err != nil {
			return nil, result.Err
		}
		if result.Repo == nil {
			continue
		}
		if string(result.Repo.Name) == name {
			bitbucketRepo = result.Repo
		}
	}

	return bitbucketRepo, nil
}

type webhookTestCase struct {
	Payloads []struct {
		PayloadType string          `json:"payload_type"`
		Data        json.RawMessage `json:"data"`
	} `json:"payloads"`
	ChangesetEvents []*batches.ChangesetEvent `json:"changeset_events"`
}

func loadWebhookTestCase(t testing.TB, path string) webhookTestCase {
	t.Helper()

	bs, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var tc webhookTestCase
	if err := json.Unmarshal(bs, &tc); err != nil {
		t.Fatal(err)
	}
	for i, ev := range tc.ChangesetEvents {
		meta, err := batches.NewChangesetEventMetadata(ev.Kind)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(ev.Metadata)
		if err != nil {
			t.Fatal(err)
		}
		err = json.Unmarshal(raw, &meta)
		if err != nil {
			t.Fatal(err)
		}
		tc.ChangesetEvents[i].Metadata = meta
	}

	return tc
}

func sign(t *testing.T, message, secret []byte) string {
	t.Helper()

	mac := hmac.New(sha256.New, secret)

	_, err := mac.Write(message)
	if err != nil {
		t.Fatalf("writing hmac message failed: %s", err)
	}

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
