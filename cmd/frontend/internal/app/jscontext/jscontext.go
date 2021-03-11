// Package jscontext contains functionality for information we pass down into
// the JS webapp.
package jscontext

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/csrf"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/assetsutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/siteid"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/globalstatedb"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/schema"
)

// BillingPublishableKey is the publishable (non-secret) API key for the billing system, if any.
var BillingPublishableKey string

type authProviderInfo struct {
	IsBuiltin         bool   `json:"isBuiltin"`
	DisplayName       string `json:"displayName"`
	ServiceType       string `json:"serviceType"`
	AuthenticationURL string `json:"authenticationURL"`
}

// JSContext is made available to JavaScript code via the
// "sourcegraph/app/context" module.
//
// 🚨 SECURITY: This struct is sent to all users regardless of whether or
// not they are logged in, for example on an auth.public=false private
// server. Including secret fields here is OK if it is based on the user's
// authentication above, but do not include e.g. hard-coded secrets about
// the server instance here as they would be sent to anonymous users.
type JSContext struct {
	AppRoot        string            `json:"appRoot,omitempty"`
	ExternalURL    string            `json:"externalURL,omitempty"`
	XHRHeaders     map[string]string `json:"xhrHeaders"`
	CSRFToken      string            `json:"csrfToken"`
	UserAgentIsBot bool              `json:"userAgentIsBot"`
	AssetsRoot     string            `json:"assetsRoot"`
	Version        string            `json:"version"`

	IsAuthenticatedUser bool `json:"isAuthenticatedUser"`

	SentryDSN     *string `json:"sentryDSN"`
	SiteID        string  `json:"siteID"`
	SiteGQLID     string  `json:"siteGQLID"`
	Debug         bool    `json:"debug"`
	NeedsSiteInit bool    `json:"needsSiteInit"`
	EmailEnabled  bool    `json:"emailEnabled"`

	Site              schema.SiteConfiguration `json:"site"` // public subset of site configuration
	LikelyDockerOnMac bool                     `json:"likelyDockerOnMac"`
	NeedServerRestart bool                     `json:"needServerRestart"`
	DeployType        string                   `json:"deployType"`

	SourcegraphDotComMode bool `json:"sourcegraphDotComMode"`

	BillingPublishableKey string `json:"billingPublishableKey,omitempty"`

	AccessTokensAllow conf.AccessTokAllow `json:"accessTokensAllow"`

	AllowSignup bool `json:"allowSignup"`

	ResetPasswordEnabled bool `json:"resetPasswordEnabled"`

	ExternalServicesUserModeEnabled bool `json:"externalServicesUserModeEnabled"`

	AuthProviders []authProviderInfo `json:"authProviders"`

	Branding *schema.Branding `json:"branding"`

	BatchChangesEnabled bool `json:"batchChangesEnabled"`

	CodeIntelAutoIndexingEnabled bool `json:"codeIntelAutoIndexingEnabled"`

	ProductResearchPageEnabled bool `json:"productResearchPageEnabled"`

	ExperimentalFeatures schema.ExperimentalFeatures `json:"experimentalFeatures"`
}

// NewJSContextFromRequest populates a JSContext struct from the HTTP
// request.
func NewJSContextFromRequest(req *http.Request) JSContext {
	actor := actor.FromContext(req.Context())

	headers := make(map[string]string)
	headers["x-sourcegraph-client"] = globals.ExternalURL().String()
	headers["X-Requested-With"] = "Sourcegraph" // required for httpapi to use cookie auth

	// Propagate Cache-Control no-cache and max-age=0 directives
	// to the requests made by our client-side JavaScript. This is
	// not a perfect parser, but it catches the important cases.
	if cc := req.Header.Get("cache-control"); strings.Contains(cc, "no-cache") || strings.Contains(cc, "max-age=0") {
		headers["Cache-Control"] = "no-cache"
	}

	csrfToken := csrf.Token(req)
	headers["X-Csrf-Token"] = csrfToken

	siteID := siteid.Get()

	// Show the site init screen?
	globalState, err := globalstatedb.Get(req.Context())
	needsSiteInit := err == nil && !globalState.Initialized

	// Auth providers
	var authProviders []authProviderInfo
	for _, p := range providers.Providers() {
		info := p.CachedInfo()
		if info != nil {
			authProviders = append(authProviders, authProviderInfo{
				IsBuiltin:         p.Config().Builtin != nil,
				DisplayName:       info.DisplayName,
				ServiceType:       p.ConfigID().Type,
				AuthenticationURL: info.AuthenticationURL,
			})
		}
	}

	var sentryDSN *string
	siteConfig := conf.Get().SiteConfiguration
	if siteConfig.Log != nil && siteConfig.Log.Sentry != nil && siteConfig.Log.Sentry.Dsn != "" {
		sentryDSN = &siteConfig.Log.Sentry.Dsn
	}

	// Check if batch changes are enabled for this user.
	batchChangesEnabled := conf.BatchChangesEnabled()
	if conf.BatchChangesRestrictedToAdmins() && backend.CheckCurrentUserIsSiteAdmin(req.Context()) != nil {
		batchChangesEnabled = false
	}

	// 🚨 SECURITY: This struct is sent to all users regardless of whether or
	// not they are logged in, for example on an auth.public=false private
	// server. Including secret fields here is OK if it is based on the user's
	// authentication above, but do not include e.g. hard-coded secrets about
	// the server instance here as they would be sent to anonymous users.
	return JSContext{
		ExternalURL:         globals.ExternalURL().String(),
		XHRHeaders:          headers,
		CSRFToken:           csrfToken,
		UserAgentIsBot:      isBot(req.UserAgent()),
		AssetsRoot:          assetsutil.URL("").String(),
		Version:             version.Version(),
		IsAuthenticatedUser: actor.IsAuthenticated(),
		SentryDSN:           sentryDSN,
		Debug:               env.InsecureDev,
		SiteID:              siteID,

		SiteGQLID: string(graphqlbackend.SiteGQLID()),

		NeedsSiteInit:     needsSiteInit,
		EmailEnabled:      conf.CanSendEmail(),
		Site:              publicSiteConfiguration(),
		LikelyDockerOnMac: likelyDockerOnMac(),
		NeedServerRestart: globals.ConfigurationServerFrontendOnly.NeedServerRestart(),
		DeployType:        conf.DeployType(),

		SourcegraphDotComMode: envvar.SourcegraphDotComMode(),

		BillingPublishableKey: BillingPublishableKey,

		// Experiments. We pass these through explicitly so we can
		// do the default behavior only in Go land.
		AccessTokensAllow: conf.AccessTokensAllow(),

		ResetPasswordEnabled: userpasswd.ResetPasswordEnabled(),

		ExternalServicesUserModeEnabled: conf.ExternalServiceUserMode() != conf.ExternalServiceModeDisabled,

		AllowSignup: conf.AuthAllowSignup(),

		AuthProviders: authProviders,

		Branding: globals.Branding(),

		BatchChangesEnabled: batchChangesEnabled,

		CodeIntelAutoIndexingEnabled: conf.CodeIntelAutoIndexingEnabled(),

		ProductResearchPageEnabled: conf.ProductResearchPageEnabled(),

		ExperimentalFeatures: conf.ExperimentalFeatures(),
	}
}

// publicSiteConfiguration is the subset of the site.schema.json site
// configuration that is necessary for the web app and is not sensitive/secret.
func publicSiteConfiguration() schema.SiteConfiguration {
	c := conf.Get()
	updateChannel := c.UpdateChannel
	if updateChannel == "" {
		updateChannel = "release"
	}
	return schema.SiteConfiguration{
		AuthPublic:    c.AuthPublic,
		UpdateChannel: updateChannel,
	}
}

var isBotPat = lazyregexp.New(`(?i:googlecloudmonitoring|pingdom.com|go .* package http|sourcegraph e2etest|bot|crawl|slurp|spider|feed|rss|camo asset proxy|http-client|sourcegraph-client)`)

func isBot(userAgent string) bool {
	return isBotPat.MatchString(userAgent)
}

func likelyDockerOnMac() bool {
	data, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return false // permission errors, or maybe not a Linux OS, etc. Assume we're not docker for mac.
	}
	return bytes.Contains(data, []byte("mac")) || bytes.Contains(data, []byte("osx"))
}
