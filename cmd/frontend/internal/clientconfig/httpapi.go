package clientconfig

import (
	"encoding/json"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

// HTTP handlers for interacting with this Sourcegraph instance's
// Cody client configuration. These handlers perform auth checks.
type HTTPHandlers struct {
	db     database.DB
	logger log.Logger
}

func NewHandlers(db database.DB, logger log.Logger) *HTTPHandlers {
	return &HTTPHandlers{
		db:     db,
		logger: logger,
	}
}

// GetClientConfigHandler returns the current Sourcegraph instance's Cody client configuration
// data as JSON. Requires that the calling user is an authenticated.
func (h *HTTPHandlers) GetClientConfigHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Info("fetching client config")

	// Auth check.
	callingActor := actor.FromContext(ctx)
	if callingActor == nil || !callingActor.IsAuthenticated() {
		h.logger.Warn("unauthenticated user requesting cody client config")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	clientConfigSvc := Get()
	currentConfig, err := clientConfigSvc.GetForActor(r.Context(), callingActor)
	if err != nil {
		h.logger.Error("fetching current cody client configuration", log.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	rawJSON, err := json.MarshalIndent(currentConfig, "", "    ")
	if err != nil {
		h.logger.Error("marshalling configuration", log.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	http.Error(w, string(rawJSON), http.StatusOK)
}
