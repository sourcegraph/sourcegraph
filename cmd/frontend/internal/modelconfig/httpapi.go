package modelconfig

import (
	"encoding/json"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	modelconfigSDK "github.com/sourcegraph/sourcegraph/internal/modelconfig"
)

// HTTP handlers for interacting with this Sourcegraph instance's
// LLM Model configuration. These handlers perform auth checks.
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

// GetSupportedModelsHandler returns the current Sourcegraph instance's LLM model configuration
// data as JSON. Requires that the calling user is an authenticated.
func (h *HTTPHandlers) GetSupportedModelsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.logger.Info("fetching supported LLMs")

	// Auth check.
	callingActor := actor.FromContext(ctx)
	if callingActor == nil || !callingActor.IsAuthenticated() {
		h.logger.Warn("unauthenticated user requesting model config")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	modelConfigSvc := Get()
	currentConfig, err := modelConfigSvc.Get()
	if err != nil {
		h.logger.Error("fetching current model configuration", log.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// SECURITY: It's critical that we do not serve any server-side configuration data
	// to the client, as it contains sensitive data like access tokens.
	modelconfigSDK.RedactServerSideConfig(currentConfig)

	rawJSON, err := json.MarshalIndent(currentConfig, "", "    ")
	if err != nil {
		h.logger.Error("marshalling configuration", log.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	http.Error(w, string(rawJSON), http.StatusOK)
}
