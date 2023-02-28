package webhooks

import (
	"encoding/json"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab/webhooks"
	"io"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (wr *Router) HandleAzureDevOpsWebhook(logger log.Logger, w http.ResponseWriter, r *http.Request, codeHostURN extsvc.CodeHostBaseURL) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error while reading request body.", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	ctx := actor.WithInternalActor(r.Context())

	var event azuredevops.BaseEvent
	json.Unmarshal(payload, &event)
	e, err := azuredevops.ParseWebhookEvent(event.EventType, payload)
	if err != nil {
		if errors.Is(err, webhooks.ErrObjectKindUnknown) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Route the request based on the event type.
	err = wr.Dispatch(ctx, string(event.EventType), extsvc.KindAzureDevOps, codeHostURN, e)
	if err != nil {
		logger.Error("Error handling Azure DevOps webhook event", log.Error(err))
		if errcode.IsNotFound(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
