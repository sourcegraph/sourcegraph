// Package router contains the URL router for the frontend app.
//
// It is in a separate package from app so that other packages may use it to generate URLs without resulting in Go
// import cycles.
package router

import (
	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
)

const (
	RobotsTxt = "robots-txt"
	Favicon   = "favicon"

	OpenSearch = "opensearch"

	RepoBadge = "repo.badge"

	Logout = "logout"

	SignIn             = "sign-in"
	SignOut            = "sign-out"
	SignUp             = "sign-up"
	SiteInit           = "site-init"
	VerifyEmail        = "verify-email"
	ResetPasswordInit  = "reset-password.init"
	ResetPasswordCode  = "reset-password.code"
	CheckUsernameTaken = "check-username-taken"

	RegistryExtensionBundle = "registry.extension.bundle"

	UsageStatsDownload = "usage-stats.download"

	LatestPing = "pings.latest"

	OldToolsRedirect = "old-tools-redirect"
	OldTreeRedirect  = "old-tree-redirect"

	GDDORefs = "gddo.refs"
	Editor   = "editor"

	Debug        = "debug"
	DebugHeaders = "debug.headers"

	GopherconLiveBlog = "gophercon.live.blog"

	GoSymbolURL = "go-symbol-url"

	UI = "ui"
)

// Router returns the frontend app router.
func Router() *mux.Router { return router }

var router = newRouter()

func newRouter() *mux.Router {
	base := mux.NewRouter()

	base.StrictSlash(true)

	base.Path("/robots.txt").Methods("GET").Name(RobotsTxt)
	base.Path("/favicon.ico").Methods("GET").Name(Favicon)
	base.Path("/opensearch.xml").Methods("GET").Name(OpenSearch)

	base.Path("/-/logout").Methods("GET").Name(Logout)

	base.Path("/-/sign-up").Methods("POST").Name(SignUp)
	base.Path("/-/site-init").Methods("POST").Name(SiteInit)
	base.Path("/-/verify-email").Methods("GET").Name(VerifyEmail)
	base.Path("/-/sign-in").Methods("POST").Name(SignIn)
	base.Path("/-/sign-out").Methods("GET").Name(SignOut)
	base.Path("/-/reset-password-init").Methods("POST").Name(ResetPasswordInit)
	base.Path("/-/reset-password-code").Methods("POST").Name(ResetPasswordCode)

	base.Path("/-/check-username-taken/{username}").Methods("GET").Name(CheckUsernameTaken)

	base.Path("/-/static/extension/{RegistryExtensionReleaseFilename}").Methods("GET").Name(RegistryExtensionBundle)

	base.Path("/-/godoc/refs").Methods("GET").Name(GDDORefs)
	base.Path("/-/editor").Methods("GET").Name(Editor)

	base.Path("/-/debug/headers").Methods("GET").Name(DebugHeaders)
	base.PathPrefix("/-/debug").Name(Debug)

	base.Path("/gophercon").Methods("GET").Name(GopherconLiveBlog)

	addOldTreeRedirectRoute(base)
	base.Path("/tools").Methods("GET").Name(OldToolsRedirect)

	base.Path("/site-admin/usage-statistics/archive").Methods("GET").Name(UsageStatsDownload)

	base.Path("/site-admin/pings/latest").Methods("GET").Name(LatestPing)

	if envvar.SourcegraphDotComMode() {
		base.PathPrefix("/go/").Methods("GET").Name(GoSymbolURL)
	}

	repoPath := `/` + routevar.Repo
	repo := base.PathPrefix(repoPath + "/" + routevar.RepoPathDelim + "/").Subrouter()
	repo.Path("/badge.svg").Methods("GET").Name(RepoBadge)

	// Must come last
	base.PathPrefix("/").Name(UI)

	return base
}
