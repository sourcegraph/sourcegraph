package app

import (
	"net/http"

	"github.com/NYTimes/gziphandler"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/userpasswd"
	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/errorutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/router"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/session"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
)

// NewHandler returns a new app handler that uses the app router.
//
// 🚨 SECURITY: The caller MUST wrap the returned handler in middleware that checks authentication
// and sets the actor in the request context.
func NewHandler(db dbutil.DB) http.Handler {
	session.SetSessionStore(session.NewRedisStore(func() bool {
		return globals.ExternalURL().Scheme == "https"
	}))

	r := router.Router()

	m := http.NewServeMux()

	m.Handle("/", r)

	r.Get(router.RobotsTxt).Handler(trace.Route(http.HandlerFunc(robotsTxt)))
	r.Get(router.Favicon).Handler(trace.Route(http.HandlerFunc(favicon)))
	r.Get(router.OpenSearch).Handler(trace.Route(http.HandlerFunc(openSearch)))

	r.Get(router.RepoBadge).Handler(trace.Route(errorutil.Handler(serveRepoBadge)))

	// Redirects
	r.Get(router.OldToolsRedirect).Handler(trace.Route(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/beta", http.StatusMovedPermanently)
	})))

	r.Get(router.GopherconLiveBlog).Handler(trace.Route(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://about.sourcegraph.com/go", http.StatusFound)
	})))

	if envvar.SourcegraphDotComMode() {
		r.Get(router.GoSymbolURL).Handler(trace.Route(errorutil.Handler(serveGoSymbolURL)))
	}

	r.Get(router.UI).Handler(ui.Router())

	r.Get(router.SignUp).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleSignUp)))
	r.Get(router.SiteInit).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleSiteInit)))
	r.Get(router.SignIn).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleSignIn)))
	r.Get(router.SignOut).Handler(trace.Route(http.HandlerFunc(serveSignOut)))
	r.Get(router.VerifyEmail).Handler(trace.Route(http.HandlerFunc(serveVerifyEmail)))
	r.Get(router.ResetPasswordInit).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleResetPasswordInit)))
	r.Get(router.ResetPasswordCode).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleResetPasswordCode)))

	r.Get(router.CheckUsernameTaken).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleCheckUsernameTaken(db))))

	r.Get(router.RegistryExtensionBundle).Handler(trace.Route(gziphandler.GzipHandler(http.HandlerFunc(registry.HandleRegistryExtensionBundle))))

	// Usage statistics ZIP download
	r.Get(router.UsageStatsDownload).Handler(trace.Route(http.HandlerFunc(usageStatsArchiveHandler(db))))

	// Ping retrieval
	r.Get(router.LatestPing).Handler(trace.Route(http.HandlerFunc(latestPingHandler(db))))

	r.Get(router.GDDORefs).Handler(trace.Route(errorutil.Handler(serveGDDORefs)))
	r.Get(router.Editor).Handler(trace.Route(errorutil.Handler(serveEditor)))

	r.Get(router.DebugHeaders).Handler(trace.Route(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("Cookie")
		_ = r.Header.Write(w)
	})))
	addDebugHandlers(r.Get(router.Debug).Subrouter())

	rickRoll := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
	})
	for _, p := range []string{"/.env", "/admin.php", "/wp-login.php", "/wp-admin"} {
		m.Handle(p, rickRoll)
	}

	return m
}
