package webhooks

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/google/go-github/v55/github"
	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type GitHubWebhook struct {
	*Router
}

func (h *GitHubWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := log.Scoped("ServeGitHubWebhook")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log15.Error("Error parsing github webhook event", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// get external service and validate webhook payload signature
	extSvc, err := h.getExternalService(r, body)
	if err != nil {
		log15.Error("Could not find valid external service for webhook", "error", err)

		if errcode.IsNotFound(err) {
			http.Error(w, "External service not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Error validating payload", http.StatusBadRequest)
		return
	}

	SetExternalServiceID(r.Context(), extSvc.ID)

	c, err := extSvc.Configuration(r.Context())
	if err != nil {
		log15.Error("Could not decode external service config", "error", err)
		http.Error(w, "Invalid external service config", http.StatusInternalServerError)
		return
	}

	config, ok := c.(*schema.GitHubConnection)
	if !ok {
		log15.Error("External service config is not a GitHub config")
		http.Error(w, "Invalid external service config", http.StatusInternalServerError)
		return
	}

	codeHostURN, err := extsvc.NewCodeHostBaseURL(config.Url)
	if err != nil {
		log15.Error("Could not parse code host URL from config", "error", err)
		http.Error(w, "Invalid code host URL", http.StatusInternalServerError)
		return
	}

	h.HandleWebhook(logger, w, r, codeHostURN, body)
}

func (h *GitHubWebhook) HandleWebhook(logger log.Logger, w http.ResponseWriter, r *http.Request, codeHostURN extsvc.CodeHostBaseURL, requestBody []byte) {
	// 🚨 SECURITY: now that the payload and shared secret have been validated,
	// we can use an internal actor on the context.
	ctx := actor.WithInternalActor(r.Context())

	// parse event
	eventType := github.WebHookType(r)
	e, err := github.ParseWebHook(eventType, requestBody)
	if err != nil {
		logger.Error("Error parsing github webhook event", log.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// match event handlers
	err = h.Dispatch(ctx, eventType, extsvc.KindGitHub, codeHostURN, e)
	if err != nil {
		logger.Error("Error handling github webhook event", log.Error(err))
		if errcode.IsNotFound(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *GitHubWebhook) getExternalService(r *http.Request, body []byte) (*types.ExternalService, error) {
	var (
		sig   = r.Header.Get("X-Hub-Signature")
		rawID = r.FormValue(extsvc.IDParam)
		err   error
	)

	// this should only happen on old legacy webhook configurations
	// TODO: delete this path once legacy webhooks are deprecated
	if rawID == "" {
		return h.findAndValidateExternalService(r.Context(), sig, body)
	}

	externalServiceID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return nil, err
	}
	e, err := h.DB.ExternalServices().GetByID(r.Context(), externalServiceID)
	if err != nil {
		return nil, err
	}
	c, err := e.Configuration(r.Context())
	if err != nil {
		return nil, err
	}
	gc, ok := c.(*schema.GitHubConnection)
	if !ok {
		return nil, errors.Errorf("invalid configuration, received github webhook for non-github external service: %v", externalServiceID)
	}

	if err := validateAnyConfiguredSecret(gc, sig, body); err != nil {
		return nil, errors.Wrap(err, "validating webhook payload")
	}
	return e, nil
}

// findAndValidateExternalService is the slow path for validating an incoming webhook against a configured
// external service, it iterates over all configured external services and attempts to match
// the signature to the configured secret
// TODO: delete this once old style webhooks are deprecated
func (h *GitHubWebhook) findAndValidateExternalService(ctx context.Context, sig string, body []byte) (*types.ExternalService, error) {
	// 🚨 SECURITY: Try to authenticate the request with any of the stored secrets
	// in GitHub external services config.
	// If there are no secrets or no secret managed to authenticate the request,
	// we return an error to the client.
	args := database.ExternalServicesListOptions{Kinds: []string{extsvc.KindGitHub}}
	es, err := h.DB.ExternalServices().List(ctx, args)
	if err != nil {
		return nil, err
	}

	for _, e := range es {
		var c any
		c, err = e.Configuration(ctx)
		if err != nil {
			return nil, err
		}
		gc, ok := c.(*schema.GitHubConnection)
		if !ok {
			continue
		}

		if err := validateAnyConfiguredSecret(gc, sig, body); err == nil {
			return e, nil
		}
	}
	return nil, errors.Errorf("couldn't find any external service for webhook")
}

func validateAnyConfiguredSecret(c *schema.GitHubConnection, sig string, body []byte) error {
	if sig == "" {
		// No signature, this implies no secret was configured
		return nil
	}

	// 🚨 SECURITY: Try to authenticate the request with any of the stored secrets
	// If there are no secrets or no secret managed to authenticate the request,
	// we return an error to the client.
	if len(c.Webhooks) == 0 {
		return errors.Errorf("no webhooks defined")
	}

	for _, hook := range c.Webhooks {
		if hook.Secret == "" {
			continue
		}

		if err := github.ValidateSignature(sig, body, []byte(hook.Secret)); err == nil {
			return nil
		}
	}

	// If we make it here then none of our webhook secrets were valid
	return errors.Errorf("unable to validate webhook signature")
}

func handleGitHubWebHook(logger log.Logger, w http.ResponseWriter, r *http.Request, urn extsvc.CodeHostBaseURL, secret string, gh *GitHubWebhook) {
	if secret == "" {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error while reading request body.", http.StatusInternalServerError)
			return
		}

		gh.HandleWebhook(logger, w, r, urn, payload)
		return
	}

	payload, err := github.ValidatePayload(r, []byte(secret))
	if err != nil {
		http.Error(w, "Could not validate payload with secret.", http.StatusBadRequest)
		return
	}

	gh.HandleWebhook(logger, w, r, urn, payload)
}
