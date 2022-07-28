package app

import (
	"net/http"
	"time"

	"github.com/NYTimes/gziphandler"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/errorutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/router"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/session"
	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/trace"
)

// NewHandler returns a new app handler that uses the app router.
//
// 🚨 SECURITY: The caller MUST wrap the returned handler in middleware that checks authentication
// and sets the actor in the request context.
func NewHandler(db database.DB, githubAppCloudSetupHandler http.Handler) http.Handler {
	session.SetSessionStore(session.NewRedisStore(func() bool {
		return globals.ExternalURL().Scheme == "https"
	}))

	r := router.Router()

	m := http.NewServeMux()

	m.Handle("/", r)

	r.Get(router.RobotsTxt).Handler(trace.Route(http.HandlerFunc(robotsTxt)))
	r.Get(router.SitemapXmlGz).Handler(trace.Route(http.HandlerFunc(sitemapXmlGz)))
	r.Get(router.Favicon).Handler(trace.Route(http.HandlerFunc(favicon)))
	r.Get(router.OpenSearch).Handler(trace.Route(http.HandlerFunc(openSearch)))

	r.Get(router.RepoBadge).Handler(trace.Route(errorutil.Handler(serveRepoBadge(db))))

	// Redirects
	r.Get(router.OldToolsRedirect).Handler(trace.Route(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/beta", http.StatusMovedPermanently)
	})))

	r.Get(router.GopherconLiveBlog).Handler(trace.Route(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://about.sourcegraph.com/go", http.StatusFound)
	})))

	r.Get(router.UI).Handler(ui.Router())

	lockoutOptions := conf.AuthLockout()
	lockoutStore := userpasswd.NewLockoutStore(
		lockoutOptions.FailedAttemptThreshold,
		time.Duration(lockoutOptions.LockoutPeriod)*time.Second,
		time.Duration(lockoutOptions.ConsecutivePeriod)*time.Second,
	)
	r.Get(router.SignUp).Handler(trace.Route(userpasswd.HandleSignUp(db)))
	r.Get(router.SiteInit).Handler(trace.Route(userpasswd.HandleSiteInit(db)))
	r.Get(router.SignIn).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleSignIn(db, lockoutStore))))
	r.Get(router.SignOut).Handler(trace.Route(http.HandlerFunc(serveSignOutHandler(db))))
	r.Get(router.UnlockAccount).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleUnlockAccount(db, lockoutStore))))
	r.Get(router.ResetPasswordInit).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleResetPasswordInit(db))))
	r.Get(router.ResetPasswordCode).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleResetPasswordCode(db))))
	r.Get(router.VerifyEmail).Handler(trace.Route(http.HandlerFunc(serveVerifyEmail(db))))

	r.Get(router.CheckUsernameTaken).Handler(trace.Route(http.HandlerFunc(userpasswd.HandleCheckUsernameTaken(db))))

	r.Get(router.RegistryExtensionBundle).Handler(trace.Route(gziphandler.GzipHandler(registry.HandleRegistryExtensionBundle(db))))

	// Usage statistics ZIP download
	r.Get(router.UsageStatsDownload).Handler(trace.Route(http.HandlerFunc(usageStatsArchiveHandler(db))))

	// Ping retrieval
	r.Get(router.LatestPing).Handler(trace.Route(http.HandlerFunc(latestPingHandler(db))))

	// Sourcegraph Cloud GitHub App setup
	r.Get(router.SetupGitHubAppCloud).Handler(trace.Route(githubAppCloudSetupHandler))

	r.Get(router.Editor).Handler(trace.Route(errorutil.Handler(serveEditor(db))))

	r.Get(router.DebugHeaders).Handler(trace.Route(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.Header.Write(w)
	})))
	addDebugHandlers(r.Get(router.Debug).Subrouter(), db)

	rickRoll := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://www.youtube.com/watch?v=dQw4w9WgXcQ", http.StatusFound)
	})
	for _, p := range []string{"/.env", "/admin.php", "/wp-login.php", "/wp-admin"} {
		m.Handle(p, rickRoll)
	}

	return m
}
