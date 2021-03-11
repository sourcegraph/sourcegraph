package enterprise

import (
	"fmt"
	"net/http"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/webhooks"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
)

// Services is a bag of HTTP handlers and factory functions that are registered by the
// enterprise frontend setup hook.
type Services struct {
	GitHubWebhook             webhooks.Registerer
	GitLabWebhook             http.Handler
	BitbucketServerWebhook    http.Handler
	NewCodeIntelUploadHandler NewCodeIntelUploadHandler
	NewExecutorProxyHandler   NewExecutorProxyHandler
	AuthzResolver             graphqlbackend.AuthzResolver
	BatchChangesResolver      graphqlbackend.BatchChangesResolver
	CodeIntelResolver         graphqlbackend.CodeIntelResolver
	InsightsResolver          graphqlbackend.InsightsResolver
	CodeMonitorsResolver      graphqlbackend.CodeMonitorsResolver
	LicenseResolver           graphqlbackend.LicenseResolver
}

// NewCodeIntelUploadHandler creates a new handler for the LSIF upload endpoint. The
// resulting handler skips auth checks when the internal flag is true.
type NewCodeIntelUploadHandler func(internal bool) http.Handler

// NewExecutorProxyHandler creates a new proxy handler for routes accessible to the
// executor services deployed separately from the k8s cluster. This handler is protected
// via a shared username and password.
type NewExecutorProxyHandler func() http.Handler

// DefaultServices creates a new Services value that has default implementations for all services.
func DefaultServices() Services {
	return Services{
		GitHubWebhook:             registerFunc(func(webhook *webhooks.GitHubWebhook) {}),
		GitLabWebhook:             makeNotFoundHandler("gitlab webhook"),
		BitbucketServerWebhook:    makeNotFoundHandler("bitbucket server webhook"),
		NewCodeIntelUploadHandler: func(_ bool) http.Handler { return makeNotFoundHandler("code intel upload") },
		NewExecutorProxyHandler:   func() http.Handler { return makeNotFoundHandler("executor proxy") },
		AuthzResolver:             graphqlbackend.DefaultAuthzResolver,
		BatchChangesResolver:      graphqlbackend.DefaultBatchChangesResolver,
		InsightsResolver:          graphqlbackend.DefaultInsightsResolver,
		CodeMonitorsResolver:      graphqlbackend.DefaultCodeMonitorsResolver,
		LicenseResolver:           graphqlbackend.DefaultLicenseResolver,
	}
}

// makeNotFoundHandler returns an HTTP handler that respond 404 for all requests.
func makeNotFoundHandler(handlerName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(fmt.Sprintf("%s is only available in enterprise", handlerName)))
	})
}

type registerFunc func(webhook *webhooks.GitHubWebhook)

func (fn registerFunc) Register(w *webhooks.GitHubWebhook) {
	fn(w)
}
