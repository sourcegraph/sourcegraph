package oauth

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func NewMiddleware(db database.DB, serviceType, authPrefix string, isAPIHandler bool, next http.Handler) http.Handler {
	oauthFlowHandler := http.StripPrefix(authPrefix, newOAuthFlowHandler(serviceType))
	traceFamily := fmt.Sprintf("oauth.%s", serviceType)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This span should be manually finished before delegating to the next handler or
		// redirecting.
		span, ctx := trace.New(r.Context(), traceFamily+".middleware")
		span.SetAttributes(attribute.Bool("isAPIHandler", isAPIHandler))

		// Delegate to the auth flow handler
		if !isAPIHandler && strings.HasPrefix(r.URL.Path, authPrefix+"/") {
			span.AddEvent("delegate to auth flow handler")
			r = withOAuthExternalClient(r)
			span.End()
			oauthFlowHandler.ServeHTTP(w, r)
			return
		}

		// If the actor is authenticated and not performing an OAuth flow, then proceed to
		// next.
		if actor.FromContext(ctx).IsAuthenticated() {
			span.AddEvent("authenticated, proceeding to next")
			span.End()
			next.ServeHTTP(w, r)
			return
		}

		// If there is only one auth provider configured, the single auth provider is a OAuth
		// instance, it's an app request, and the sign-out cookie is not present, redirect to sign-in immediately.
		//
		// For sign-out requests (signout cookie is  present), the user will be redirected to the SG login page.
		pc := getExactlyOneOAuthProvider()
		if pc != nil && !isAPIHandler && pc.AuthPrefix == authPrefix && !auth.HasSignOutCookie(r) && isHuman(r) {
			span.AddEvent("redirect to signin")
			v := make(url.Values)
			v.Set("redirect", auth.SafeRedirectURL(r.URL.String()))
			v.Set("pc", pc.ConfigID().ID)
			span.End()
			http.Redirect(w, r, authPrefix+"/login?"+v.Encode(), http.StatusFound)

			return
		}

		span.AddEvent("proceeding to next")
		span.End()
		next.ServeHTTP(w, r)
	})
}

func newOAuthFlowHandler(serviceType string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("pc")
		p := GetProvider(serviceType, id)
		if p == nil {
			// NOTE: Within the Sourcegraph application, we have been using both the
			// "redirect" and "returnTo" query parameters inconsistently, and some of the
			// usages are also on the client side (Cody clients). If we ever settle on one
			// and updated all usages on both server and client side, we need to make sure
			// to have a grace period (e.g. 3 months) for the client side because we have no
			// control over when users will actually upgrade their clients.
			redirect := r.URL.Query().Get("redirect")
			if redirect == "" {
				redirect = r.URL.Query().Get("returnTo")
			}

			log15.Warn("no OAuth provider found with ID and service type", "id", id, "serviceType", serviceType)
			http.Redirect(w, r, "/sign-in?returnTo="+redirect, http.StatusFound)
			return
		}
		p.Login(p.OAuth2Config()).ServeHTTP(w, r)
	}))
	mux.Handle("/callback", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state, err := DecodeState(r.URL.Query().Get("state"))
		if err != nil {
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not decode OAuth state from URL parameter.", http.StatusBadRequest)
			return
		}

		p := GetProvider(serviceType, state.ProviderID)
		if p == nil {
			log15.Error("OAuth failed: in callback, no auth provider found with ID and service type", "id", state.ProviderID, "serviceType", serviceType)
			http.Error(w, "Authentication failed. Try signing in again (and clearing cookies for the current site). The error was: could not find provider that matches the OAuth state parameter.", http.StatusBadRequest)
			return
		}
		p.Callback(p.OAuth2Config()).ServeHTTP(w, r)
	}))
	return mux
}

// withOAuthExternalClient updates client such that the
// golang.org/x/oauth2 package will use our http client which is configured
// with proxy and TLS settings/etc.
func withOAuthExternalClient(r *http.Request) *http.Request {
	client := httpcli.ExternalClient
	if traceLogEnabled {
		loggingClient := *client
		loggingClient.Transport = &loggingRoundTripper{
			log:        log.Scoped("oauth_external.transport"),
			underlying: client.Transport,
		}
		client = &loggingClient
	}
	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, client)
	return r.WithContext(ctx)
}

var traceLogEnabled, _ = strconv.ParseBool(env.Get("INSECURE_OAUTH2_LOG_TRACES", "false", "Log all OAuth2-related HTTP requests and responses. Only use during testing because the log messages will contain sensitive data."))

type loggingRoundTripper struct {
	log        log.Logger
	underlying http.RoundTripper
}

func previewAndDuplicateReader(reader io.ReadCloser) (preview string, freshReader io.ReadCloser, err error) {
	if reader == nil {
		return "", reader, nil
	}
	defer reader.Close()
	b, err := io.ReadAll(reader)
	if err != nil {
		return "", nil, err
	}
	preview = string(b)
	if len(preview) > 1000 {
		preview = preview[:1000]
	}
	return preview, io.NopCloser(bytes.NewReader(b)), nil
}

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	{
		var err error
		var preview string
		preview, req.Body, err = previewAndDuplicateReader(req.Body)
		if err != nil {
			l.log.Error("Unexpected error in OAuth2 debug log",
				log.String("operation", "reading request body"),
				log.Error(err))
			return nil, errors.Wrap(err, "Unexpected error in OAuth2 debug log, reading request body")
		}

		headerFields := make([]log.Field, 0, len(req.Header))
		for k, v := range req.Header {
			headerFields = append(headerFields, log.Strings(k, v))
		}
		l.log.Info("HTTP request",
			log.String("method", req.Method),
			log.String("url", req.URL.String()),
			log.Object("header", headerFields...),
			log.String("body", preview))
	}

	resp, err := l.underlying.RoundTrip(req)
	if err != nil {
		l.log.Error("Error getting HTTP response", log.Error(err))
		return resp, err
	}

	{
		var err error
		var preview string
		preview, resp.Body, err = previewAndDuplicateReader(resp.Body)
		if err != nil {
			l.log.Error("Unexpected error in OAuth2 debug log", log.String("operation", "reading response body"), log.Error(err))
			return nil, errors.Wrap(err, "Unexpected error in OAuth2 debug log, reading response body")
		}

		headerFields := make([]log.Field, 0, len(resp.Header))
		for k, v := range resp.Header {
			headerFields = append(headerFields, log.Strings(k, v))
		}
		l.log.Info("HTTP response",
			log.String("method", req.Method),
			log.String("url", req.URL.String()),
			log.Object("header", headerFields...),
			log.String("body", preview))

		return resp, err
	}
}

func getExactlyOneOAuthProvider() *Provider {
	ps := providers.SignInProviders()
	if len(ps) != 1 {
		return nil
	}
	p, ok := ps[0].(*Provider)
	if !ok {
		return nil
	}
	if !isOAuth(p.Config()) {
		return nil
	}
	return p
}

var isOAuths []func(p schema.AuthProviders) bool

func AddIsOAuth(f func(p schema.AuthProviders) bool) {
	isOAuths = append(isOAuths, f)
}

func isOAuth(p schema.AuthProviders) bool {
	for _, f := range isOAuths {
		if f(p) {
			return true
		}
	}
	return false
}

// isHuman returns true if the request probably came from a human, rather than a bot. Used to
// prevent unfurling the wrong URL preview.
func isHuman(req *http.Request) bool {
	return strings.Contains(strings.ToLower(req.UserAgent()), "mozilla")
}
