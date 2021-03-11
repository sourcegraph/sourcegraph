package webhooks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

type GitLabWebhook struct{ *Webhook }

func NewGitLabWebhook(store *store.Store) *GitLabWebhook {
	return &GitLabWebhook{&Webhook{store, store.ExternalServices(), store.Clock(), extsvc.TypeGitLab}}
}

// ServeHTTP implements the http.Handler interface.
func (h *GitLabWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Look up the external service.
	extSvc, err := h.getExternalServiceFromRawID(r.Context(), r.FormValue(extsvc.IDParam))
	if err == errExternalServiceNotFound {
		respond(w, http.StatusUnauthorized, err)
		return
	} else if err != nil {
		respond(w, http.StatusInternalServerError, errors.Wrap(err, "getting external service"))
		return
	}

	// 🚨 SECURITY: Verify the shared secret against the GitLab external service
	// configuration. If there isn't a webhook defined in the service with this
	// secret, or the header is empty, then we return a 401 to the client.
	if ok, err := validateGitLabSecret(extSvc, r.Header.Get(webhooks.TokenHeaderName)); err != nil {
		respond(w, http.StatusInternalServerError, errors.Wrap(err, "validating the shared secret"))
		return
	} else if !ok {
		respond(w, http.StatusUnauthorized, "shared secret is incorrect")
		return
	}

	// Parse the event proper.
	if r.Body == nil {
		respond(w, http.StatusBadRequest, "missing request body")
		return
	}
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		respond(w, http.StatusInternalServerError, errors.Wrap(err, "reading payload"))
		return
	}

	event, err := webhooks.UnmarshalEvent(payload)
	if err != nil {
		if errors.Is(err, webhooks.ErrObjectKindUnknown) {
			// We don't want to return a non-2XX status code and have GitLab
			// retry the webhook, so we'll log that we don't know what to do
			// and return 204.
			log15.Debug("unknown object kind", "err", err)

			// We don't use respond() here so that we don't log an error, since
			// this really isn't one.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNoContent)
			fmt.Fprintf(w, "%v", err)
		} else {
			respond(w, http.StatusInternalServerError, errors.Wrap(err, "unmarshalling payload"))
		}
		return
	}

	// Route the request based on the event type.
	if err := h.handleEvent(r.Context(), extSvc, event); err != nil {
		respond(w, err.code, err)
	} else {
		respond(w, http.StatusNoContent, nil)
	}
}

var (
	errExternalServiceNotFound     = errors.New("external service not found")
	errExternalServiceWrongKind    = errors.New("external service is not of the expected kind")
	errPipelineMissingMergeRequest = errors.New("pipeline event does not include a merge request")
)

// getExternalServiceFromRawID retrieves the external service matching the
// given raw ID, which is usually going to be the string in the
// externalServiceID URL parameter.
//
// On failure, errExternalServiceNotFound is returned if the ID doesn't match
// any GitLab service.
func (h *GitLabWebhook) getExternalServiceFromRawID(ctx context.Context, raw string) (*types.ExternalService, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "parsing the raw external service ID")
	}

	es, err := h.ExternalServices.List(ctx, database.ExternalServicesListOptions{
		IDs:   []int64{id},
		Kinds: []string{extsvc.KindGitLab},
	})
	if err != nil {
		return nil, errors.Wrap(err, "listing external services")
	}

	if len(es) == 0 {
		return nil, errExternalServiceNotFound
	} else if len(es) > 1 {
		// This _really_ shouldn't happen, since we provided only one ID above.
		return nil, errors.New("too many external services found")
	}

	return es[0], nil
}

// handleEvent is essentially a router: it dispatches based on the event type
// to perform whatever changeset action is appropriate for that event.
func (h *GitLabWebhook) handleEvent(ctx context.Context, extSvc *types.ExternalService, event interface{}) *httpError {
	log15.Debug("GitLab webhook received", "type", fmt.Sprintf("%T", event))

	esID, err := extractExternalServiceID(extSvc)
	if err != nil {
		return &httpError{
			code: http.StatusInternalServerError,
			err:  err,
		}
	}

	switch e := event.(type) {
	// Some merge request event types require us to do a full resync.
	//
	// For example, approvals and unapprovals manifest in normal syncs as
	// system notes, but we _don't_ get them as note events in webhooks.
	// Instead, we get a merge request webhook with an "approved" or
	// "unapproved" action field that magically appears in the object (merge
	// request) attributes and no further details on who changed the approval
	// status, the note ID, or anything else we can use to deduplicate later.
	//
	// Similarly, for update events, we don't get the full set of fields that
	// we get when we sync using the REST API (presumably because this reflects
	// the data types at the point webhooks were added to GitLab several years
	// ago, and not today): labels come in a different format outside of the
	// merge request, and we'd still have to go query for notes and pipelines.
	//
	// Therefore, the only realistic action we can take here is to re-sync the
	// changeset as a whole. The problem is that — since we only have the merge
	// request — this requires three requests to the REST API, and GitLab's
	// documentation is quite clear that webhooks should run as fast as possible
	// to avoid unexpected retries.
	//
	// To meet this goal, rather than synchronously synchronizing here, we'll
	// instead ask repo-updater to prioritize the sync of this changeset and let
	// the normal sync process take care of pulling the notes and pipelines and
	// putting things in the right places. The downside is that the updated
	// changeset state won't appear _quite_ as instantaneously to the user, but
	// this is the best compromise given the limited payload we get in the
	// webhook.
	case *webhooks.MergeRequestApprovedEvent,
		*webhooks.MergeRequestUnapprovedEvent,
		*webhooks.MergeRequestUpdateEvent:
		if err := h.enqueueChangesetSyncFromEvent(ctx, esID, e.(webhooks.MergeRequestEventCommonContainer).ToEventCommon()); err != nil {
			return &httpError{
				code: http.StatusInternalServerError,
				err:  err,
			}
		}
		return nil

	case webhooks.UpsertableWebhookEvent:
		eventCommon := e.ToEventCommon()
		event := e.ToEvent()
		pr := gitlabToPR(&eventCommon.Project, eventCommon.MergeRequest)
		if err := h.upsertChangesetEvent(ctx, esID, pr, event); err != nil {
			return &httpError{
				code: http.StatusInternalServerError,
				err:  errors.Wrap(err, "upserting changeset event"),
			}
		}
		return nil

	case *webhooks.PipelineEvent:
		if err := h.handlePipelineEvent(ctx, esID, e); err != nil && err != errPipelineMissingMergeRequest {
			return &httpError{
				code: http.StatusInternalServerError,
				err:  err,
			}
		}
		return nil
	}

	// We don't want to return a non-2XX status code and have GitLab retry the
	// webhook, so we'll log that we don't know what to do and return 204.
	log15.Debug("cannot handle GitLab webhook event of unknown type", "event", event, "type", fmt.Sprintf("%T", event))
	return nil
}

func (h *GitLabWebhook) enqueueChangesetSyncFromEvent(ctx context.Context, esID string, event *webhooks.MergeRequestEventCommon) error {
	// We need to get our changeset ID for this to work. To get _there_, we need
	// the repo ID, and then we can use the merge request IID to match the
	// external ID.
	pr := gitlabToPR(&event.Project, event.MergeRequest)
	repo, err := h.getRepoForPR(ctx, h.Store, pr, esID)
	if err != nil {
		return errors.Wrap(err, "getting repo")
	}

	c, err := h.getChangesetForPR(ctx, h.Store, &pr, repo)
	if err != nil {
		return errors.Wrap(err, "getting changeset")
	}

	if err := repoupdater.DefaultClient.EnqueueChangesetSync(ctx, []int64{c.ID}); err != nil {
		return errors.Wrap(err, "enqueuing changeset sync")
	}

	return nil
}

func (h *GitLabWebhook) handlePipelineEvent(ctx context.Context, esID string, event *webhooks.PipelineEvent) error {
	// Pipeline webhook payloads don't include the merge request very reliably:
	// for example, re-running a pipeline from the GitLab UI will result in no
	// merge request field, even when that pipeline was attached to a merge
	// request. So the very first thing we need to do is see if we even have the
	// merge request; if we don't, we can't do anything useful here, and we'll
	// just have to wait for the next scheduled sync.
	if event.MergeRequest == nil {
		log15.Debug("ignoring pipeline event without a merge request", "payload", event)
		return errPipelineMissingMergeRequest
	}

	pr := gitlabToPR(&event.Project, event.MergeRequest)
	if err := h.upsertChangesetEvent(ctx, esID, pr, &event.Pipeline); err != nil {
		return errors.Wrap(err, "upserting changeset event")
	}
	return nil
}

func (h *GitLabWebhook) getChangesetForPR(ctx context.Context, tx *store.Store, pr *PR, repo *types.Repo) (*batches.Changeset, error) {
	return tx.GetChangeset(ctx, store.GetChangesetOpts{
		RepoID:              repo.ID,
		ExternalID:          strconv.FormatInt(pr.ID, 10),
		ExternalServiceType: h.ServiceType,
	})
}

// gitlabToPR instantiates a new PR instance given fields that are commonly
// available in GitLab webhook payloads.
func gitlabToPR(project *gitlab.ProjectCommon, mr *gitlab.MergeRequest) PR {
	return PR{
		ID:             int64(mr.IID),
		RepoExternalID: strconv.Itoa(project.ID),
	}
}

// validateGitLabSecret validates that the given secret matches one of the
// webhooks in the external service.
func validateGitLabSecret(extSvc *types.ExternalService, secret string) (bool, error) {
	// An empty secret never succeeds.
	if secret == "" {
		return false, nil
	}

	// Get the typed configuration.
	c, err := extSvc.Configuration()
	if err != nil {
		return false, errors.Wrap(err, "getting external service configuration")
	}

	config, ok := c.(*schema.GitLabConnection)
	if !ok {
		return false, errExternalServiceWrongKind
	}

	// Iterate over the webhooks and look for one with the right secret. The
	// number of webhooks in an external service should be small enough that a
	// linear search like this is sufficient.
	for _, webhook := range config.Webhooks {
		if webhook.Secret == secret {
			return true, nil
		}
	}
	return false, nil
}
