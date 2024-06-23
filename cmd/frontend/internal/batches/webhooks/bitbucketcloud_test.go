package webhooks

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	bbcs "github.com/sourcegraph/sourcegraph/internal/batches/sources/bitbucketcloud"
	bstore "github.com/sourcegraph/sourcegraph/internal/batches/store"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

const (
	bitbucketCloudExternalServiceURL = "https://bitbucket.org/"
	bitbucketCloudRepoUUID           = "{uuid}"
	bitbucketCloudSourceHash         = "0123456789012345678901234567890123456789"
	bitbucketCloudDestinationHash    = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
)

func testBitbucketCloudWebhook(db *sql.DB) func(*testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		logger := logtest.Scoped(t)

		cfg := &schema.BitbucketCloudConnection{
			WebhookSecret: "secretsecret",
		}
		esURL := bitbucketCloudExternalServiceURL

		gsClient := gitserver.NewMockClient()

		t.Run("ServeHTTP", func(t *testing.T) {
			t.Run("missing external service", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, 12345, cfg, esURL)
				assert.Nil(t, err)

				req, err := http.NewRequest("POST", u, bytes.NewBufferString("{}"))
				assert.Nil(t, err)

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusUnauthorized, rec.Result().StatusCode)
			})

			t.Run("invalid external service", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, 12345, cfg, esURL)
				assert.Nil(t, err)

				u = strings.ReplaceAll(u, "12345", "foo")
				req, err := http.NewRequest("POST", u, bytes.NewBufferString("{}"))
				assert.Nil(t, err)

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusBadRequest, rec.Result().StatusCode)
			})

			t.Run("missing secret", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)
				es := createBitbucketCloudExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, es.ID, cfg, esURL)
				assert.Nil(t, err)

				u = strings.ReplaceAll(u, "&secret=secretsecret", "")
				req, err := http.NewRequest("POST", u, bytes.NewBufferString("{}"))
				assert.Nil(t, err)

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusUnauthorized, rec.Result().StatusCode)
			})

			t.Run("incorrect secret", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)
				es := createBitbucketCloudExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, es.ID, cfg, esURL)
				assert.Nil(t, err)

				u = strings.ReplaceAll(u, "&secret=secretsecret", "&secret=not+correct")
				req, err := http.NewRequest("POST", u, bytes.NewBufferString("{}"))
				assert.Nil(t, err)

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusUnauthorized, rec.Result().StatusCode)
			})

			t.Run("missing body", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, 12345, cfg, "https://bitbucket.org/")
				assert.Nil(t, err)

				req, err := http.NewRequest("POST", u, nil)
				assert.Nil(t, err)

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusBadRequest, rec.Result().StatusCode)
			})

			t.Run("unreadable body", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)
				es := createBitbucketCloudExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, es.ID, cfg, esURL)
				assert.Nil(t, err)

				req, err := http.NewRequest("POST", u, &brokenReader{errors.New("foo")})
				assert.Nil(t, err)

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusInternalServerError, rec.Result().StatusCode)
			})

			t.Run("malformed body", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)
				es := createBitbucketCloudExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, es.ID, cfg, esURL)
				assert.Nil(t, err)

				req, err := http.NewRequest("POST", u, bytes.NewBufferString("invalid JSON"))
				assert.Nil(t, err)

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusBadRequest, rec.Result().StatusCode)
			})

			t.Run("invalid event key", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)
				es := createBitbucketCloudExternalService(t, ctx, store.ExternalServices())

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, es.ID, cfg, esURL)
				assert.Nil(t, err)

				req, err := http.NewRequest("POST", u, bytes.NewBufferString("{}"))
				assert.Nil(t, err)
				req.Header.Add("X-Event-Key", "not a real key")

				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				assert.EqualValues(t, http.StatusBadRequest, rec.Result().StatusCode)
			})

			// Very happy path integration tests follow;
			// bitbucketcloud.ParseWebhookEvent has unit tests that are more
			// robust for that function. This is mostly ensuring that things
			// that look valid end up in the right table.

			t.Run("valid events", func(t *testing.T) {
				store := bitbucketCloudTestSetup(t, db)
				h := NewBitbucketCloudWebhook(store, gsClient, logger)
				es := createBitbucketCloudExternalService(t, ctx, store.ExternalServices())
				repo := createBitbucketCloudRepo(t, ctx, store.Repos(), es)

				// Set up mocks to prevent the diffstat computation from trying to
				// use a real gitserver, and so we can control what diff is used to
				// create the diffstat.
				state := bt.MockChangesetSyncState(&protocol.RepoInfo{
					Name: "repo",
				})
				defer state.Unmock()

				u, err := extsvc.WebhookURL(extsvc.KindBitbucketCloud, es.ID, cfg, esURL)
				assert.Nil(t, err)

				for _, tc := range []struct {
					buildEvent func(pullRequestEvent bitbucketcloud.PullRequestEvent) any
					key        string
					want       btypes.ChangesetEventKind
				}{
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestApprovalEvent{
								PullRequestEvent: pullRequestEvent,
							}
						},
						key:  "pullrequest:approved",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestApproved,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestChangesRequestCreatedEvent{
								PullRequestChangesRequestEvent: bitbucketcloud.PullRequestChangesRequestEvent{
									PullRequestEvent: pullRequestEvent,
								},
							}
						},
						key:  "pullrequest:changes_request_created",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestChangesRequestCreated,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestChangesRequestRemovedEvent{
								PullRequestChangesRequestEvent: bitbucketcloud.PullRequestChangesRequestEvent{
									PullRequestEvent: pullRequestEvent,
								},
							}
						},
						key:  "pullrequest:changes_request_removed",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestChangesRequestRemoved,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestCommentCreatedEvent{
								PullRequestCommentEvent: bitbucketcloud.PullRequestCommentEvent{
									PullRequestEvent: pullRequestEvent,
								},
							}
						},
						key:  "pullrequest:comment_created",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestCommentCreated,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestCommentDeletedEvent{
								PullRequestCommentEvent: bitbucketcloud.PullRequestCommentEvent{
									PullRequestEvent: pullRequestEvent,
								},
							}
						},
						key:  "pullrequest:comment_deleted",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestCommentDeleted,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestCommentUpdatedEvent{
								PullRequestCommentEvent: bitbucketcloud.PullRequestCommentEvent{
									PullRequestEvent: pullRequestEvent,
								},
							}
						},
						key:  "pullrequest:comment_updated",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestCommentUpdated,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestFulfilledEvent{
								PullRequestEvent: pullRequestEvent,
							}
						},
						key:  "pullrequest:fulfilled",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestFulfilled,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestRejectedEvent{
								PullRequestEvent: pullRequestEvent,
							}
						},
						key:  "pullrequest:rejected",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestRejected,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestUnapprovedEvent{
								PullRequestApprovalEvent: bitbucketcloud.PullRequestApprovalEvent{
									PullRequestEvent: pullRequestEvent,
								},
							}
						},
						key:  "pullrequest:unapproved",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestUnapproved,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.PullRequestUpdatedEvent{
								PullRequestEvent: pullRequestEvent,
							}
						},
						key:  "pullrequest:updated",
						want: btypes.ChangesetEventKindBitbucketCloudPullRequestUpdated,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.RepoCommitStatusCreatedEvent{
								RepoCommitStatusEvent: bitbucketcloud.RepoCommitStatusEvent{
									RepoEvent: pullRequestEvent.RepoEvent,
									CommitStatus: bitbucketcloud.CommitStatus{
										Commit: bitbucketcloud.Commit{Hash: bitbucketCloudSourceHash[0:12]},
										State:  bitbucketcloud.PullRequestStatusStateSuccessful,
									},
								},
							}
						},
						key:  "repo:commit_status_created",
						want: btypes.ChangesetEventKindBitbucketCloudRepoCommitStatusCreated,
					},
					{
						buildEvent: func(pullRequestEvent bitbucketcloud.PullRequestEvent) any {
							return &bitbucketcloud.RepoCommitStatusUpdatedEvent{
								RepoCommitStatusEvent: bitbucketcloud.RepoCommitStatusEvent{
									RepoEvent: pullRequestEvent.RepoEvent,
									CommitStatus: bitbucketcloud.CommitStatus{
										Commit: bitbucketcloud.Commit{Hash: bitbucketCloudSourceHash},
										State:  bitbucketcloud.PullRequestStatusStateSuccessful,
									},
								},
							}
						},
						key:  "repo:commit_status_updated",
						want: btypes.ChangesetEventKindBitbucketCloudRepoCommitStatusUpdated,
					},
				} {
					t.Run(tc.key, func(t *testing.T) {
						changeset := createBitbucketCloudChangeset(t, ctx, store, repo)
						cid, err := strconv.ParseInt(changeset.ExternalID, 10, 64)
						assert.Nil(t, err)

						// All the events ultimately need a pullRequestEvent, so
						// let's create one for the mocked changeset.
						pullRequestEvent := bitbucketcloud.PullRequestEvent{
							RepoEvent: bitbucketcloud.RepoEvent{
								Repository: bitbucketcloud.Repo{
									UUID: bitbucketCloudRepoUUID,
								},
							},
							PullRequest: bitbucketcloud.PullRequest{
								ID: cid,
							},
						}

						data := bt.MarshalJSON(t, tc.buildEvent(pullRequestEvent))
						req, err := http.NewRequest("POST", u, bytes.NewBufferString(data))
						assert.Nil(t, err)
						req.Header.Add("X-Event-Key", tc.key)

						rec := httptest.NewRecorder()
						h.ServeHTTP(rec, req)

						assert.EqualValues(t, http.StatusNoContent, rec.Result().StatusCode)
						assertChangesetEventForChangeset(t, ctx, store, changeset, tc.want)
					})
				}
			})
		})
	}
}

// bitbucketCloudTestSetup instantiates the stores and a clock for use within
// tests. Any changes made to the stores will be rolled back after the test is
// complete.
func bitbucketCloudTestSetup(t *testing.T, sqlDB *sql.DB) *bstore.Store {
	logger := logtest.Scoped(t)
	clock := &bt.TestClock{Time: timeutil.Now()}
	tx := dbtest.NewTx(t, sqlDB)

	// Note that tx is wrapped in nestedTx to effectively neuter further use of
	// transactions within the test.
	db := database.NewDBWith(logger, basestore.NewWithHandle(&nestedTx{basestore.NewHandleWithTx(tx, sql.TxOptions{})}))

	return bstore.NewWithClock(db, observation.TestContextTB(t), nil, clock.Now)
}

// createBitbucketCloudExternalService creates a mock Bitbucket Cloud service
// with a valid configuration, including the webhook secret "secret".
func createBitbucketCloudExternalService(t *testing.T, ctx context.Context, esStore database.ExternalServiceStore) *types.ExternalService {
	es := &types.ExternalService{
		Kind:        extsvc.KindBitbucketCloud,
		DisplayName: "bitbucketcloud",
		Config: extsvc.NewUnencryptedConfig(bt.MarshalJSON(t, &schema.BitbucketCloudConnection{
			Url:           bitbucketCloudExternalServiceURL,
			Username:      "user",
			AppPassword:   "password",
			WebhookSecret: "secretsecret",
		})),
	}
	if err := esStore.Upsert(ctx, es); err != nil {
		t.Fatal(err)
	}

	return es
}

// createBitbucketCloudRepo creates a mock Bitbucket Cloud repo attached to the
// given external service.
func createBitbucketCloudRepo(t *testing.T, ctx context.Context, rstore database.RepoStore, es *types.ExternalService) *types.Repo {
	repo := (&types.Repo{
		Name: "bitbucket.org/sourcegraph/test",
		URI:  "bitbucket.org/sourcegraph/test",
		ExternalRepo: api.ExternalRepoSpec{
			ID:          bitbucketCloudRepoUUID,
			ServiceType: extsvc.TypeBitbucketCloud,
			ServiceID:   bitbucketCloudExternalServiceURL,
		},
	}).With(typestest.Opt.RepoSources(es.URN()))
	if err := rstore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	return repo
}

var bitbucketCloudChangesetID int64 = 0

// createBitbucketCloudChangeset creates a mock Bitbucket Cloud changeset.
func createBitbucketCloudChangeset(t *testing.T, ctx context.Context, store *bstore.Store, repo *types.Repo) *btypes.Changeset {
	id := bitbucketCloudChangesetID
	bitbucketCloudChangesetID++

	c := &btypes.Changeset{
		RepoID:              repo.ID,
		ExternalID:          strconv.FormatInt(id, 10),
		ExternalServiceType: extsvc.TypeBitbucketCloud,
		Metadata: &bbcs.AnnotatedPullRequest{
			PullRequest: &bitbucketcloud.PullRequest{
				ID:        id,
				CreatedOn: time.Now(),
				Source: bitbucketcloud.PullRequestEndpoint{
					Repo:   bitbucketcloud.Repo{UUID: bitbucketCloudRepoUUID},
					Branch: bitbucketcloud.PullRequestBranch{Name: "branch"},
					Commit: bitbucketcloud.PullRequestCommit{Hash: bitbucketCloudSourceHash[0:12]},
				},
				Destination: bitbucketcloud.PullRequestEndpoint{
					Repo:   bitbucketcloud.Repo{UUID: bitbucketCloudRepoUUID},
					Branch: bitbucketcloud.PullRequestBranch{Name: "main"},
					Commit: bitbucketcloud.PullRequestCommit{Hash: bitbucketCloudDestinationHash[0:12]},
				},
			},
		},
	}
	if err := store.CreateChangeset(ctx, c); err != nil {
		t.Fatal(err)
	}

	return c
}
