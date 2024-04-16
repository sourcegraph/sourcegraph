package webhooks

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	bstore "github.com/sourcegraph/sourcegraph/internal/batches/store"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	et "github.com/sourcegraph/sourcegraph/internal/encryption/testing"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

var testExternalURL, _ = url.Parse("https://example.com/")

func testGitLabWebhook(db *sql.DB) func(*testing.T) {
	return func(t *testing.T) {
		logger := logtest.Scoped(t)
		ctx := context.Background()
		gsClient := gitserver.NewMockClient()
		gitLabURL, err := extsvc.NewCodeHostBaseURL("https://gitlab.com/")
		require.NoError(t, err)

		t.Run("ServeHTTP", func(t *testing.T) {
			t.Run("missing external service", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, 12345, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, nil)
				if err != nil {
					t.Fatal(err)
				}

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				if have, want := rec.Result().StatusCode, http.StatusUnauthorized; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
			})

			t.Run("invalid external service", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, 12345, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				u = strings.ReplaceAll(u, "12345", "foo")
				req, err := http.NewRequest("POST", u, nil)
				if err != nil {
					t.Fatal(err)
				}

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusInternalServerError; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
				assertBodyIncludes(t, resp.Body, "getting external service")
			})

			t.Run("missing secret", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				es := createGitLabExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, nil)
				if err != nil {
					t.Fatal(err)
				}

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusUnauthorized; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
				assertBodyIncludes(t, resp.Body, "shared secret is incorrect")
			})

			t.Run("incorrect secret", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				es := createGitLabExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, nil)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Add(webhooks.TokenHeaderName, "not a valid secret")

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusUnauthorized; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
				assertBodyIncludes(t, resp.Body, "shared secret is incorrect")
			})

			t.Run("missing body", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				es := createGitLabExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, nil)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Add(webhooks.TokenHeaderName, "secret")

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusBadRequest; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
				assertBodyIncludes(t, resp.Body, "missing request body")
			})

			t.Run("unreadable body", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				es := createGitLabExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, nil)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Add(webhooks.TokenHeaderName, "secret")
				req.Body = &brokenReader{errors.New("foo")}

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusInternalServerError; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
				assertBodyIncludes(t, resp.Body, "reading payload")
			})

			t.Run("malformed body", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				es := createGitLabExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, bytes.NewBufferString("invalid JSON"))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Add(webhooks.TokenHeaderName, "secret")

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusInternalServerError; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
				assertBodyIncludes(t, resp.Body, "unmarshalling payload")
			})

			t.Run("invalid body", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				es := createGitLabExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				body := bt.MarshalJSON(t, &webhooks.EventCommon{
					ObjectKind: "unknown",
				})
				req, err := http.NewRequest("POST", u, bytes.NewBufferString(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Add(webhooks.TokenHeaderName, "secret")

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusNoContent; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
			})

			t.Run("error from handleEvent", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				repoStore := database.ReposWith(logger, store)

				h := NewGitLabWebhook(store, gsClient, logger)
				// Force a failure
				h.failHandleEvent = errors.New("oops")

				es := createGitLabExternalService(t, ctx, store.ExternalServices())
				repo := createGitLabRepo(t, ctx, repoStore, es)
				changeset := createGitLabChangeset(t, ctx, store, repo)
				body := createMergeRequestPayload(t, repo, changeset, "close")

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, bytes.NewBufferString(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Add(webhooks.TokenHeaderName, "secret")

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusInternalServerError; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}
				assertBodyIncludes(t, resp.Body, "oops")
			})

			// The valid tests below are pretty "happy path": specific unit
			// tests for the utility methods on GitLabWebhook are below. We're
			// mostly just testing the routing here, since these are ServeHTTP
			// tests; however, these also act as integration tests. (Which,
			// considering they're ultimately invoked from TestIntegration,
			// seems fair.)

			t.Run("valid merge request approval events", func(t *testing.T) {
				for _, action := range []string{"approved", "unapproved"} {
					t.Run(action, func(t *testing.T) {
						store := gitLabTestSetup(t, db)
						repoStore := database.ReposWith(logger, store)
						h := NewGitLabWebhook(store, gsClient, logger)
						es := createGitLabExternalService(t, ctx, store.ExternalServices())
						repo := createGitLabRepo(t, ctx, repoStore, es)
						changeset := createGitLabChangeset(t, ctx, store, repo)
						body := createMergeRequestPayload(t, repo, changeset, "approved")

						u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
						if err != nil {
							t.Fatal(err)
						}

						req, err := http.NewRequest("POST", u, bytes.NewBufferString(body))
						if err != nil {
							t.Fatal(err)
						}
						req.Header.Add(webhooks.TokenHeaderName, "secret")

						changesetEnqueued := false
						repoupdater.MockEnqueueChangesetSync = func(ctx context.Context, ids []int64) error {
							changesetEnqueued = true
							if diff := cmp.Diff(ids, []int64{changeset.ID}); diff != "" {
								t.Errorf("unexpected changeset ID: %s", diff)
							}
							return nil
						}
						defer func() { repoupdater.MockEnqueueChangesetSync = nil }()

						rec := httptest.NewRecorder()
						h.ServeHTTP(rec, req)

						resp := rec.Result()
						if have, want := resp.StatusCode, http.StatusNoContent; have != want {
							t.Errorf("unexpected status code: have %d; want %d", have, want)
						}
						if !changesetEnqueued {
							t.Error("changeset was not enqueued")
						}
					})
				}
			})

			t.Run("valid merge request state change events", func(t *testing.T) {
				for action, want := range map[string]btypes.ChangesetEventKind{
					"close":  btypes.ChangesetEventKindGitLabClosed,
					"merge":  btypes.ChangesetEventKindGitLabMerged,
					"reopen": btypes.ChangesetEventKindGitLabReopened,
				} {
					t.Run(action, func(t *testing.T) {
						store := gitLabTestSetup(t, db)
						repoStore := database.ReposWith(logger, store)
						h := NewGitLabWebhook(store, gsClient, logger)
						es := createGitLabExternalService(t, ctx, store.ExternalServices())
						repo := createGitLabRepo(t, ctx, repoStore, es)
						changeset := createGitLabChangeset(t, ctx, store, repo)
						body := createMergeRequestPayload(t, repo, changeset, action)

						u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
						if err != nil {
							t.Fatal(err)
						}

						req, err := http.NewRequest("POST", u, bytes.NewBufferString(body))
						if err != nil {
							t.Fatal(err)
						}
						req.Header.Add(webhooks.TokenHeaderName, "secret")

						rec := httptest.NewRecorder()
						h.ServeHTTP(rec, req)

						resp := rec.Result()
						if have, want := resp.StatusCode, http.StatusNoContent; have != want {
							t.Errorf("unexpected status code: have %d; want %d", have, want)
						}

						// Verify that the changeset event was upserted.
						assertChangesetEventForChangeset(t, ctx, store, changeset, want)
					})
				}
			})

			t.Run("valid pipeline events", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				repoStore := database.ReposWith(logger, store)
				h := NewGitLabWebhook(store, gsClient, logger)
				es := createGitLabExternalService(t, ctx, store.ExternalServices())
				repo := createGitLabRepo(t, ctx, repoStore, es)
				changeset := createGitLabChangeset(t, ctx, store, repo)
				body := createPipelinePayload(t, repo, changeset, gitlab.Pipeline{
					ID:     123,
					Status: gitlab.PipelineStatusSuccess,
				})

				u, err := extsvc.WebhookURL(extsvc.TypeGitLab, es.ID, nil, testExternalURL)
				if err != nil {
					t.Fatal(err)
				}

				req, err := http.NewRequest("POST", u, bytes.NewBufferString(body))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Add(webhooks.TokenHeaderName, "secret")

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				resp := rec.Result()
				if have, want := resp.StatusCode, http.StatusNoContent; have != want {
					t.Errorf("unexpected status code: have %d; want %d", have, want)
				}

				assertChangesetEventForChangeset(t, ctx, store, changeset, btypes.ChangesetEventKindGitLabPipeline)
			})
		})

		t.Run("getExternalServiceFromRawID", func(t *testing.T) {
			// Since these tests don't write to the database, we can just share
			// the same database setup.
			store := gitLabTestSetup(t, db)
			h := NewGitLabWebhook(store, gsClient, logger)

			// Set up two GitLab external services.
			a := createGitLabExternalService(t, ctx, store.ExternalServices())
			b := createGitLabExternalService(t, ctx, store.ExternalServices())

			// Set up a GitHub external service.
			github := createGitHubExternalService(t, ctx, store.ExternalServices())
			github.Kind = extsvc.KindGitHub
			if err := store.ExternalServices().Upsert(ctx, github); err != nil {
				t.Fatal(err)
			}

			t.Run("invalid ID", func(t *testing.T) {
				for _, id := range []string{"", "foo"} {
					t.Run(id, func(t *testing.T) {
						es, err := h.getExternalServiceFromRawID(ctx, "foo")
						if es != nil {
							t.Errorf("unexpected non-nil external service: %+v", es)
						}
						if err == nil {
							t.Error("unexpected nil error")
						}
					})
				}
			})

			t.Run("missing ID", func(t *testing.T) {
				for name, id := range map[string]string{
					"not found":  "12345",
					"wrong kind": strconv.FormatInt(github.ID, 10),
				} {
					t.Run(name, func(t *testing.T) {
						es, err := h.getExternalServiceFromRawID(ctx, id)
						if es != nil {
							t.Errorf("unexpected non-nil external service: %+v", es)
						}
						if want := errExternalServiceNotFound; err != want {
							t.Errorf("unexpected error: have %+v; want %+v", err, want)
						}
					})
				}
			})

			t.Run("valid ID", func(t *testing.T) {
				for id, want := range map[int64]*types.ExternalService{
					a.ID: a,
					b.ID: b,
				} {
					sid := strconv.FormatInt(id, 10)
					t.Run(sid, func(t *testing.T) {
						have, err := h.getExternalServiceFromRawID(ctx, sid)
						if err != nil {
							t.Errorf("unexpected non-nil error: %+v", err)
						}
						if diff := cmp.Diff(have, want, et.CompareEncryptable); diff != "" {
							t.Errorf("unexpected external service: %s", diff)
						}
					})
				}
			})
		})

		t.Run("broken external services store", func(t *testing.T) {
			// This test is separate from the other unit tests for this
			// function above because it needs to set up a bad database
			// connection on the repo store.
			externalServices := dbmocks.NewMockExternalServiceStore()
			externalServices.ListFunc.SetDefaultReturn(nil, errors.New("foo"))
			mockDB := dbmocks.NewMockDBFrom(database.NewDB(logger, db))
			mockDB.ExternalServicesFunc.SetDefaultReturn(externalServices)

			store := gitLabTestSetup(t, db).With(mockDB)
			h := NewGitLabWebhook(store, gsClient, logger)

			_, err := h.getExternalServiceFromRawID(ctx, "12345")
			if err == nil {
				t.Error("unexpected nil error")
			}
		})

		t.Run("broken batches store", func(t *testing.T) {
			// We can induce an error with a broken database connection.
			s := gitLabTestSetup(t, db)
			h := NewGitLabWebhook(s, gsClient, logger)
			db := database.NewDBWith(logger, basestore.NewWithHandle(&brokenDB{errors.New("foo")}))
			h.Store = bstore.NewWithClock(db, observation.TestContextTB(t), nil, s.Clock())

			es, err := h.getExternalServiceFromRawID(ctx, "12345")
			if es != nil {
				t.Errorf("unexpected non-nil external service: %+v", es)
			}
			if err == nil {
				t.Error("unexpected nil error")
			}
		})

		t.Run("handleEvent", func(t *testing.T) {
			// There aren't a lot of these tests, as most of the viable error
			// paths are covered by the ServeHTTP tests above, but these fill
			// in the gaps as best we can.

			t.Run("unknown event type", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				createGitLabExternalService(t, ctx, store.ExternalServices())

				err := h.handleEvent(ctx, store.DatabaseDB(), gitLabURL, nil)
				if err != nil {
					t.Errorf("unexpected non-nil error: %+v", err)
				}
			})

			t.Run("error from enqueueChangesetSyncFromEvent", func(t *testing.T) {
				store := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(store, gsClient, logger)
				createGitLabExternalService(t, ctx, store.ExternalServices())

				// We can induce an error with an incomplete merge request
				// event that's missing a project.
				event := &webhooks.MergeRequestApprovedEvent{
					MergeRequestEventCommon: webhooks.MergeRequestEventCommon{
						MergeRequest: &gitlab.MergeRequest{IID: 42},
					},
				}

				err := h.handleEvent(ctx, store.DatabaseDB(), gitLabURL, event)
				require.Error(t, err)
			})

			t.Run("error from handleMergeRequestStateEvent", func(t *testing.T) {
				s := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(s, gsClient, logger)
				createGitLabExternalService(t, ctx, s.ExternalServices())

				event := &webhooks.MergeRequestCloseEvent{
					MergeRequestEventCommon: webhooks.MergeRequestEventCommon{
						MergeRequest: &gitlab.MergeRequest{IID: 42},
					},
				}

				// We can induce an error with a broken database connection.
				db := database.NewDBWith(logger, basestore.NewWithHandle(&brokenDB{errors.New("foo")}))
				h.Store = bstore.NewWithClock(db, observation.TestContextTB(t), nil, s.Clock())

				err := h.handleEvent(ctx, db, gitLabURL, event)
				require.Error(t, err)
			})

			t.Run("error from handlePipelineEvent", func(t *testing.T) {
				s := gitLabTestSetup(t, db)
				h := NewGitLabWebhook(s, gsClient, logger)
				createGitLabExternalService(t, ctx, s.ExternalServices())

				event := &webhooks.PipelineEvent{
					MergeRequest: &gitlab.MergeRequest{IID: 42},
				}

				// We can induce an error with a broken database connection.
				db := database.NewDBWith(logger, basestore.NewWithHandle(&brokenDB{errors.New("foo")}))
				h.Store = bstore.NewWithClock(db, observation.TestContextTB(t), nil, s.Clock())

				err := h.handleEvent(ctx, db, gitLabURL, event)
				require.Error(t, err)
			})
		})

		t.Run("enqueueChangesetSyncFromEvent", func(t *testing.T) {
			// Since these tests don't write to the database, we can just share
			// the same database setup.
			store := gitLabTestSetup(t, db)
			repoStore := database.ReposWith(logger, store)
			h := NewGitLabWebhook(store, gsClient, logger)
			es := createGitLabExternalService(t, ctx, store.ExternalServices())
			repo := createGitLabRepo(t, ctx, repoStore, es)
			changeset := createGitLabChangeset(t, ctx, store, repo)

			// Extract IDs we'll need to build events.
			cid, err := strconv.Atoi(changeset.ExternalID)
			if err != nil {
				t.Fatal(err)
			}

			pid, err := strconv.Atoi(repo.ExternalRepo.ID)
			if err != nil {
				t.Fatal(err)
			}

			esid, err := extractExternalServiceID(ctx, es)
			if err != nil {
				t.Fatal(err)
			}

			t.Run("missing repo", func(t *testing.T) {
				event := &webhooks.MergeRequestEventCommon{
					EventCommon: webhooks.EventCommon{
						Project: gitlab.ProjectCommon{ID: 12345},
					},
					MergeRequest: &gitlab.MergeRequest{IID: gitlab.ID(cid)},
				}

				if err := h.enqueueChangesetSyncFromEvent(ctx, esid, event); err == nil {
					t.Error("unexpected nil error")
				}
			})

			t.Run("missing changeset", func(t *testing.T) {
				event := &webhooks.MergeRequestEventCommon{
					EventCommon: webhooks.EventCommon{
						Project: gitlab.ProjectCommon{ID: pid},
					},
					MergeRequest: &gitlab.MergeRequest{IID: 12345},
				}

				if err := h.enqueueChangesetSyncFromEvent(ctx, esid, event); err == nil {
					t.Error("unexpected nil error")
				}
			})

			t.Run("repo updater error", func(t *testing.T) {
				event := &webhooks.MergeRequestEventCommon{
					EventCommon: webhooks.EventCommon{
						Project: gitlab.ProjectCommon{ID: pid},
					},
					MergeRequest: &gitlab.MergeRequest{IID: gitlab.ID(cid)},
				}

				want := errors.New("foo")
				repoupdater.MockEnqueueChangesetSync = func(ctx context.Context, ids []int64) error {
					return want
				}
				defer func() { repoupdater.MockEnqueueChangesetSync = nil }()

				if have := h.enqueueChangesetSyncFromEvent(ctx, esid, event); !errors.Is(have, want) {
					t.Errorf("unexpected error: have %+v; want %+v", have, want)
				}
			})

			t.Run("success", func(t *testing.T) {
				event := &webhooks.MergeRequestEventCommon{
					EventCommon: webhooks.EventCommon{
						Project: gitlab.ProjectCommon{ID: pid},
					},
					MergeRequest: &gitlab.MergeRequest{IID: gitlab.ID(cid)},
				}

				repoupdater.MockEnqueueChangesetSync = func(ctx context.Context, ids []int64) error {
					return nil
				}
				defer func() { repoupdater.MockEnqueueChangesetSync = nil }()

				if err := h.enqueueChangesetSyncFromEvent(ctx, esid, event); err != nil {
					t.Errorf("unexpected non-nil error: %+v", err)
				}
			})
		})

		t.Run("handlePipelineEvent", func(t *testing.T) {
			// As with the handleMergeRequestStateEvent test above, we don't
			// really need to test the success path here. However, there's one
			// extra error path, so we'll use two sub-tests to ensure we hit
			// them both.
			//
			// Again, we're going to set up a poisoned store database that will
			// error if a transaction is started.
			s := gitLabTestSetup(t, db)
			store := bstore.NewWithClock(database.NewDBWith(logger, basestore.NewWithHandle(&noNestingTx{s.Handle()})), observation.TestContextTB(t), nil, s.Clock())
			h := NewGitLabWebhook(store, gsClient, logger)

			t.Run("missing merge request", func(t *testing.T) {
				event := &webhooks.PipelineEvent{}

				if have := h.handlePipelineEvent(ctx, extsvc.CodeHostBaseURL{}, event); have != errPipelineMissingMergeRequest {
					t.Errorf("unexpected error: have %+v; want %+v", have, errPipelineMissingMergeRequest)
				}
			})

			t.Run("changeset upsert error", func(t *testing.T) {
				event := &webhooks.PipelineEvent{
					MergeRequest: &gitlab.MergeRequest{},
				}

				if err := h.handlePipelineEvent(ctx, extsvc.CodeHostBaseURL{}, event); err == nil || err == errPipelineMissingMergeRequest {
					t.Errorf("unexpected error: %+v", err)
				}
			})
		})
	}
}

func TestValidateGitLabSecret(t *testing.T) {
	t.Parallel()

	t.Run("empty secret", func(t *testing.T) {
		ok, err := validateGitLabSecret(context.Background(), nil, "")
		if ok {
			t.Errorf("unexpected ok: %v", ok)
		}
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
	})

	t.Run("invalid configuration", func(t *testing.T) {
		es := &types.ExternalService{
			Config: extsvc.NewEmptyConfig(),
		}
		ok, err := validateGitLabSecret(context.Background(), es, "secret")
		if ok {
			t.Errorf("unexpected ok: %v", ok)
		}
		if err == nil {
			t.Error("unexpected nil error")
		}
	})

	t.Run("not a GitLab connection", func(t *testing.T) {
		es := &types.ExternalService{
			Kind:   extsvc.KindGitHub,
			Config: extsvc.NewEmptyConfig(),
		}
		ok, err := validateGitLabSecret(context.Background(), es, "secret")
		if ok {
			t.Errorf("unexpected ok: %v", ok)
		}
		if err != errExternalServiceWrongKind {
			t.Errorf("unexpected error: have %+v; want %+v", err, errExternalServiceWrongKind)
		}
	})

	t.Run("no webhooks", func(t *testing.T) {
		es := &types.ExternalService{
			Kind: extsvc.KindGitLab,
			Config: extsvc.NewUnencryptedConfig(bt.MarshalJSON(t, &schema.GitLabConnection{
				Webhooks: []*schema.GitLabWebhook{},
			})),
		}

		ok, err := validateGitLabSecret(context.Background(), es, "secret")
		if ok {
			t.Errorf("unexpected ok: %v", ok)
		}
		if err != nil {
			t.Errorf("unexpected non-nil error: %+v", err)
		}
	})

	t.Run("valid webhooks", func(t *testing.T) {
		for secret, want := range map[string]bool{
			"not secret": false,
			"secret":     true,
			"super":      true,
		} {
			t.Run(secret, func(t *testing.T) {
				es := &types.ExternalService{
					Kind: extsvc.KindGitLab,
					Config: extsvc.NewUnencryptedConfig(bt.MarshalJSON(t, &schema.GitLabConnection{
						Webhooks: []*schema.GitLabWebhook{
							{Secret: "super"},
							{Secret: "secret"},
						},
					})),
				}

				ok, err := validateGitLabSecret(context.Background(), es, secret)
				if ok != want {
					t.Errorf("unexpected ok: have %v; want %v", ok, want)
				}
				if err != nil {
					t.Errorf("unexpected non-nil error: %+v", err)
				}
			})
		}
	})
}

// brokenDB provides a dbutil.DB that always fails: for methods that return an
// error, the err field will be returned; otherwise nil will be returned.
type brokenDB struct{ err error }

func (db *brokenDB) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return nil, db.err
}

func (db *brokenDB) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return nil, db.err
}

func (db *brokenDB) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	return nil
}

func (db *brokenDB) Transact(context.Context) (basestore.TransactableHandle, error) {
	return nil, db.err
}

func (db *brokenDB) Done(err error) error {
	return err
}

func (db *brokenDB) InTransaction() bool {
	return false
}

var _ basestore.TransactableHandle = (*brokenDB)(nil)

// brokenReader implements an io.ReadCloser that always returns an error when
// read.
type brokenReader struct{ err error }

func (br *brokenReader) Close() error { return nil }

func (br *brokenReader) Read(p []byte) (int, error) {
	return 0, br.err
}

// nestedTx wraps an existing transaction and overrides its transaction methods
// to be no-ops. This allows us to have a master transaction used in tests that
// test functions that attempt to create and commit transactions: since
// PostgreSQL doesn't support nested transactions, we can still use the master
// transaction to manage the test database state without rollback/commit
// already performed errors.
//
// It would be theoretically possible to use savepoints to implement something
// resembling the semantics of a true nested transaction, but that's
// unnecessary for these tests.
type nestedTx struct{ basestore.TransactableHandle }

func (ntx *nestedTx) Done(error) error                                               { return nil }
func (ntx *nestedTx) Transact(context.Context) (basestore.TransactableHandle, error) { return ntx, nil }

// noNestingTx is another transaction wrapper that always returns an error when
// a transaction is attempted.
type noNestingTx struct{ basestore.TransactableHandle }

func (ntx *noNestingTx) Transact(context.Context) (basestore.TransactableHandle, error) {
	return nil, errors.New("foo")
}

// gitLabTestSetup instantiates the stores and a clock for use within tests.
// Any changes made to the stores will be rolled back after the test is
// complete.
func gitLabTestSetup(t *testing.T, sqlDB *sql.DB) *bstore.Store {
	logger := logtest.Scoped(t)
	c := &bt.TestClock{Time: timeutil.Now()}
	tx := dbtest.NewTx(t, sqlDB)

	// Note that tx is wrapped in nestedTx to effectively neuter further use of
	// transactions within the test.
	db := database.NewDBWith(logger, basestore.NewWithHandle(&nestedTx{basestore.NewHandleWithTx(tx, sql.TxOptions{})}))

	// Note that tx is wrapped in nestedTx to effectively neuter further use of
	// transactions within the test.
	return bstore.NewWithClock(db, observation.TestContextTB(t), nil, c.Now)
}

// assertBodyIncludes checks for a specific substring within the given response
// body, and generates a test error if the substring is not found. This is
// mostly useful to look for wrapped errors in the output.
func assertBodyIncludes(t *testing.T, r io.Reader, want string) {
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(want)) {
		t.Errorf("cannot find expected string in output: want: %s; have:\n%s", want, string(body))
	}
}

// assertChangesetEventForChangeset checks that one (and only one) changeset
// event has been created on the given changeset, and that it is of the given
// kind.
func assertChangesetEventForChangeset(t *testing.T, ctx context.Context, tx *bstore.Store, changeset *btypes.Changeset, want btypes.ChangesetEventKind) {
	ces, _, err := tx.ListChangesetEvents(ctx, bstore.ListChangesetEventsOpts{
		ChangesetIDs: []int64{changeset.ID},
		LimitOpts:    bstore.LimitOpts{Limit: 100},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ces) == 1 {
		ce := ces[0]

		if ce.ChangesetID != changeset.ID {
			t.Errorf("unexpected changeset ID: have %d; want %d", ce.ChangesetID, changeset.ID)
		}
		if ce.Kind != want {
			t.Errorf(
				"unexpected changeset event kind: have %v; want %v", ce.Kind, want)
		}
	} else {
		t.Errorf("unexpected number of changeset events; got %+v", ces)
	}
}

// createGitLabExternalService creates a mock GitLab service with a valid
// configuration, including the secrets "super" and "secret".
func createGitLabExternalService(t *testing.T, ctx context.Context, esStore database.ExternalServiceStore) *types.ExternalService {
	es := &types.ExternalService{
		Kind:        extsvc.KindGitLab,
		DisplayName: "gitlab",
		Config: extsvc.NewUnencryptedConfig(bt.MarshalJSON(t, &schema.GitLabConnection{
			Url:   "https://gitlab.com/",
			Token: "secret-gitlab-token",
			Webhooks: []*schema.GitLabWebhook{
				{Secret: "super"},
				{Secret: "secret"},
			},
			ProjectQuery: []string{"none"},
		})),
	}
	if err := esStore.Upsert(ctx, es); err != nil {
		t.Fatal(err)
	}

	return es
}

// createGitLabExternalService creates a mock GitHub service with a valid
// configuration, including the secrets "super" and "secret".
func createGitHubExternalService(t *testing.T, ctx context.Context, esStore database.ExternalServiceStore) *types.ExternalService {
	es := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "github",
		Config: extsvc.NewUnencryptedConfig(bt.MarshalJSON(t, &schema.GitHubConnection{
			Url:   "https://github.com/",
			Token: "secret-github-token",
			Webhooks: []*schema.GitHubWebhook{
				{Org: "org1", Secret: "super"},
				{Org: "org2", Secret: "secret"},
			},
			Repos: []string{"owner/name"},
		})),
	}
	if err := esStore.Upsert(ctx, es); err != nil {
		t.Fatal(err)
	}

	return es
}

// createGitLabRepo creates a mock GitLab repo attached to the given external
// service.
func createGitLabRepo(t *testing.T, ctx context.Context, rstore database.RepoStore, es *types.ExternalService) *types.Repo {
	repo := (&types.Repo{
		Name: "gitlab.com/sourcegraph/test",
		URI:  "gitlab.com/sourcegraph/test",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          "123",
			ServiceType: extsvc.TypeGitLab,
			ServiceID:   "https://gitlab.com/",
		},
	}).With(typestest.Opt.RepoSources(es.URN()))
	if err := rstore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	return repo
}

// createGitLabChangeset creates a mock GitLab changeset.
func createGitLabChangeset(t *testing.T, ctx context.Context, store *bstore.Store, repo *types.Repo) *btypes.Changeset {
	c := &btypes.Changeset{
		RepoID:              repo.ID,
		ExternalID:          "1",
		ExternalServiceType: extsvc.TypeGitLab,
	}
	if err := store.CreateChangeset(ctx, c); err != nil {
		t.Fatal(err)
	}

	return c
}

// createMergeRequestPayload creates a mock GitLab webhook payload of the merge
// request object kind.
func createMergeRequestPayload(t *testing.T, repo *types.Repo, changeset *btypes.Changeset, action string) string {
	cid, err := strconv.Atoi(changeset.ExternalID)
	if err != nil {
		t.Fatal(err)
	}

	pid, err := strconv.Atoi(repo.ExternalRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	// We use an untyped set of maps here because the webhooks package doesn't
	// export its internal mergeRequestEvent type that is used for
	// unmarshalling. (Which is fine; it's an implementation detail.)
	return bt.MarshalJSON(t, map[string]any{
		"object_kind": "merge_request",
		"project": map[string]any{
			"id": pid,
		},
		"object_attributes": map[string]any{
			"iid":    cid,
			"action": action,
		},
	})
}

// createPipelinePayload creates a mock GitLab webhook payload of the pipeline
// object kind.
func createPipelinePayload(t *testing.T, repo *types.Repo, changeset *btypes.Changeset, pipeline gitlab.Pipeline) string {
	pid, err := strconv.Atoi(repo.ExternalRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	payload := &webhooks.PipelineEvent{
		EventCommon: webhooks.EventCommon{
			ObjectKind: "pipeline",
			Project: gitlab.ProjectCommon{
				ID: pid,
			},
		},
		Pipeline: pipeline,
	}

	if changeset != nil {
		cid, err := strconv.Atoi(changeset.ExternalID)
		if err != nil {
			t.Fatal(err)
		}

		payload.MergeRequest = &gitlab.MergeRequest{
			IID: gitlab.ID(cid),
		}
	}

	return bt.MarshalJSON(t, payload)
}
