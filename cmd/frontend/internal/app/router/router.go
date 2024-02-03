// Package router contains the URL router for the frontend app.
//
// It is in a separate package from app so that other packages may use it to generate URLs without resulting in Go
// import cycles.
package router

import (
	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
)

const (
	RobotsTxt    = "robots-txt"
	SitemapXmlGz = "sitemap-xml-gz"
	Favicon      = "favicon"

	OpenSearch = "opensearch"

	RepoBadge = "repo.badge"

	Logout = "logout"

	SignIn             = "sign-in"
	SignOut            = "sign-out"
	SignUp             = "sign-up"
	RequestAccess      = "request-access"
	UnlockAccount      = "unlock-account"
	UnlockUserAccount  = "unlock-user-account"
	Welcome            = "welcome"
	SiteInit           = "site-init"
	VerifyEmail        = "verify-email"
	ResetPasswordInit  = "reset-password.init"
	ResetPasswordCode  = "reset-password.code"
	CheckUsernameTaken = "check-username-taken"

	UsageStatsDownload = "usage-stats.download"
	AllEventsDownload  = "all-events.download"

	OneClickExportArchive = "one-click-export.archive"

	LatestPing = "pings.latest"

	OldToolsRedirect = "old-tools-redirect"
	OldTreeRedirect  = "old-tree-redirect"

	Editor = "editor"

	Debug        = "debug"
	DebugHeaders = "debug.headers"

	GopherconLiveBlog = "gophercon.live.blog"

	UI = "ui"
)

// Router returns the frontend app router.
func Router() *mux.Router { return router }

var router = newRouter()

func newRouter() *mux.Router {
	base := mux.NewRouter()

	base.StrictSlash(true)

	base.Path("/robots.txt").Methods("GET").Name(RobotsTxt)
	base.Path("/sitemap{number:(?:_(?:[0-9]+))?}.xml.gz").Methods("GET").Name(SitemapXmlGz)
	base.Path("/favicon.ico").Methods("GET").Name(Favicon)
	base.Path("/opensearch.xml").Methods("GET").Name(OpenSearch)

	base.Path("/-/logout").Methods("GET").Name(Logout)

	base.Path("/-/sign-up").Methods("POST").Name(SignUp)
	base.Path("/-/request-access").Methods("POST").Name(RequestAccess)
	base.Path("/-/welcome").Methods("GET").Name(Welcome)
	base.Path("/-/site-init").Methods("POST").Name(SiteInit)
	base.Path("/-/verify-email").Methods("GET").Name(VerifyEmail)
	base.Path("/-/sign-in").Methods("POST").Name(SignIn)
	base.Path("/-/sign-out").Methods("GET").Name(SignOut)
	base.Path("/-/unlock-account").Methods("POST").Name(UnlockAccount)
	base.Path("/-/unlock-user-account").Methods("POST").Name(UnlockUserAccount)
	base.Path("/-/reset-password-init").Methods("POST").Name(ResetPasswordInit)
	base.Path("/-/reset-password-code").Methods("POST").Name(ResetPasswordCode)

	base.Path("/-/check-username-taken/{username}").Methods("GET").Name(CheckUsernameTaken)

	base.Path("/-/editor").Methods("GET").Name(Editor)

	base.Path("/-/debug/headers").Methods("GET").Name(DebugHeaders)
	base.PathPrefix("/-/debug").Name(Debug)

	base.Path("/gophercon").Methods("GET").Name(GopherconLiveBlog)

	addOldTreeRedirectRoute(base)
	base.Path("/tools").Methods("GET").Name(OldToolsRedirect)

	base.Path("/site-admin/usage-statistics/archive").Methods("GET").Name(UsageStatsDownload)
	base.Path("/site-admin/admin-analytics-all-events/archive").Methods("GET").Name(AllEventsDownload)

	base.Path("/site-admin/data-export/archive").Methods("POST").Name(OneClickExportArchive)

	base.Path("/site-admin/pings/latest").Methods("GET").Name(LatestPing)

	repoPath := `/` + routevar.Repo
	repo := base.PathPrefix(repoPath + "/" + routevar.RepoPathDelim + "/").Subrouter()
	repo.Path("/badge.svg").Methods("GET").Name(RepoBadge)

	// Must come last
	base.PathPrefix("/").Name(UI)

	return base
}
