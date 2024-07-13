package ui

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/mux"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	uirouter "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui/router"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui/sveltekit"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/githubapp"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/randstring"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	routeHome             = "home"
	routeSearch           = "search"
	routeSearchBadge      = "search-badge"
	routeRepo             = "repo"
	routeRepoSettings     = "repo-settings"
	routeRepoCodeGraph    = "repo-code-intelligence"
	routeRepoCommit       = "repo-commit"
	routeRepoBranches     = "repo-branches"
	routeRepoBatchChanges = "repo-batch-changes"
	routeRepoCommits      = "repo-commits"
	routeRepoTags         = "repo-tags"
	routeRepoCompare      = "repo-compare"
	routeRepoStats        = "repo-stats"
	routeRepoOwn          = "repo-own"
	routeTree             = "tree"
	routeBlob             = "blob"
	routeRaw              = "raw"
	routeSettings         = "settings"
	routeSiteAdmin        = "site-admin"

	routeAboutSubdomain = "about-subdomain"
	aboutRedirectScheme = "https"
	aboutRedirectHost   = "sourcegraph.com"

	// Legacy redirects
	routeLegacyLogin      = "login"
	routeLegacyCareers    = "careers"
	routeLegacyDefLanding = "page.def.landing"
)

// aboutRedirects contains map entries, each of which indicates that
// sourcegraph.com/$KEY should redirect to sourcegraph.com/$VALUE.
var aboutRedirects = map[string]string{
	"about":      "about",
	"blog":       "blog",
	"customers":  "customers",
	"docs":       "docs",
	"handbook":   "handbook",
	"news":       "news",
	"plan":       "plan",
	"contact":    "contact",
	"pricing":    "pricing",
	"privacy":    "privacy",
	"security":   "security",
	"terms":      "terms",
	"jobs":       "jobs",
	"help/terms": "terms",
}

type staticPageInfo struct {
	// Specify either path OR pathPrefix.
	path, pathPrefix string
	name, title      string
	index            bool
}

// Router returns the router that serves pages for our web app.
func Router() *mux.Router {
	return uirouter.Router
}

// InitRouter create the router that serves pages for our web app
// and assigns it to uirouter.Router.
// The router can be accessed by calling Router().
func InitRouter(db database.DB) {
	logger := log.Scoped("router")

	brandedIndex := func(titles string) http.Handler {
		return handler(db, serveBrandedPageString(db, titles, nil, index))
	}

	brandedNoIndex := func(titles string) http.Handler {
		return handler(db, serveBrandedPageString(db, titles, nil, noIndex))
	}

	r := mux.NewRouter()
	r.StrictSlash(true)

	// Top-level routes.
	r.Path("/").Methods(http.MethodGet, http.MethodHead).Name(routeHome).Handler(handler(db, serveHome(db)))

	r.Path("/sign-in").Methods(http.MethodGet, http.MethodHead).Name(uirouter.RouteSignIn).Handler(handler(db, serveSignIn(db)))
	r.Path("/ping-from-self-hosted").Methods("GET", "OPTIONS").Name(uirouter.RoutePingFromSelfHosted).Handler(handler(db, servePingFromSelfHosted))

	ghAppRouter := r.PathPrefix("/githubapp/").Subrouter()
	githubapp.SetupGitHubAppRoutes(ghAppRouter, db)

	// Basic pages with static titles.
	staticPages := []staticPageInfo{
		// with index:
		{pathPrefix: "/insights", name: "insights", title: "Insights", index: true},
		{pathPrefix: "/search-jobs", name: "search-jobs", title: "Search Jobs", index: true},
		{pathPrefix: "/setup", name: "setup", title: "Setup", index: true},
		{pathPrefix: "/batch-changes", name: "batch-changes", title: "Batch Changes", index: true},
		{pathPrefix: "/code-monitoring", name: "code-monitoring", title: "Code Monitoring", index: true},
		{pathPrefix: "/notebooks", name: "search.notebook", title: "Notebooks", index: true},
		{pathPrefix: "/request-access", name: uirouter.RouteRequestAccess, title: "Request access", index: true},
		{path: "/search/console", name: "search.console", title: "Search console", index: true},
		{path: "/api/console", name: "api-console", title: "API console", index: true},
		{path: "/sign-up", name: uirouter.RouteSignUp, title: "Sign up", index: true},

		// without index:
		{pathPrefix: "/organizations", name: "org", title: "Organization", index: false},
		{pathPrefix: "/teams", name: "team", title: "Team", index: false},
		{pathPrefix: "/settings", name: routeSettings, title: "Settings", index: false},
		{pathPrefix: "/site-admin", name: routeSiteAdmin, title: "Admin", index: false},
		{pathPrefix: "/contexts", name: "contexts", title: "Search Contexts", index: false},
		{path: "/cody/manage", name: "cody", title: "Cody Manage", index: false},
		{path: "/cody/subscription", name: "cody", title: "Cody Pricing", index: false},
		{path: "/cody/chat", name: "cody", title: "Cody", index: false},
		{path: "/cody/chat/{chatID}", name: "cody-chat", title: "Cody", index: false},
		// TODO: [TEMPORARY] remove this redirect route when the marketing page is added.
		{path: "/cody", name: "cody", title: "Cody", index: false},
		// TODO: [TEMPORARY] remove this redirect route when the marketing page is added.
		{path: "/cody/{chatID}", name: "cody-chat", title: "Cody", index: false},
		{path: "/unlock-account/{token}", name: uirouter.RouteUnlockAccount, title: "Unlock Your Account", index: false},
		{path: "/password-reset", name: uirouter.RoutePasswordReset, title: "Reset password", index: false},
		{path: "/survey", name: "survey", title: "Survey", index: false},
		{path: "/survey/{score}", name: "survey-score", title: "Survey", index: false},
	}

	config := conf.Get()
	// Register Sourcegraph.com-specific pages as applicable.
	if config.Dotcom != nil && config.Dotcom.CodyProConfig != nil {
		staticPages = append(staticPages, staticPageInfo{
			path: "/cody/manage/subscription/new", name: "cody",
			title: "New Cody Pro Subscription", index: false,
		})
	}

	for _, p := range staticPages {
		var handler http.Handler
		if p.index {
			handler = brandedIndex(p.title)
		} else {
			handler = brandedNoIndex(p.title)
		}

		if p.pathPrefix != "" {
			r.Methods("GET").PathPrefix(p.pathPrefix).Name(p.name).Handler(handler)
		} else {
			r.Methods("GET").Path(p.path).Name(p.name).Handler(handler)
		}
	}

	// 🚨 SECURITY: The embed route is used to serve embeddable content (via an iframe) to 3rd party sites.
	// Any changes to the embedding route could have security implications. Please consult the security team
	// before making changes. See the `serveEmbed` function for further details.
	r.PathPrefix("/embed").Methods("GET").Name("embed").Handler(handler(db, serveEmbed(db)))

	// users
	r.PathPrefix("/users/{username}/settings").Methods("GET").Name("user-settings").Handler(brandedNoIndex("User settings"))
	r.PathPrefix("/user").Methods("GET").Name("user-redirect").Handler(brandedNoIndex("User"))
	r.PathPrefix("/users/{username}").Methods("GET").
		Name("user").
		Handler(handler(db, serveBasicPage(db, func(c *Common, r *http.Request) string {
			return brandNameSubtitle(mux.Vars(r)["username"])
		}, nil, noIndex)))

	// search
	r.Path("/search").Methods("GET").Name(routeSearch).
		Handler(handler(db, serveBasicPage(db, func(_ *Common, r *http.Request) string {
			shortQuery := limitString(r.URL.Query().Get("q"), 25, true)
			if shortQuery == "" {
				return conf.Branding().BrandName
			}
			// e.g. "myquery - Sourcegraph"
			return brandNameSubtitle(shortQuery)
		}, nil, index)))
	// streaming search
	r.Path("/search/stream").Methods("GET").Name("search.stream").Handler(search.StreamHandler(db))
	// search badge
	r.Path("/search/badge").Methods("GET").Name(routeSearchBadge).Handler(searchBadgeHandler())

	if dotcom.SourcegraphDotComMode() {
		// sourcegraph.com subdomain
		r.Path("/{Path:(?:" + strings.Join(mapKeys(aboutRedirects), "|") + ")}").Methods("GET").
			Name(routeAboutSubdomain).
			Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.URL.Scheme = aboutRedirectScheme
				r.URL.User = nil
				r.URL.Host = aboutRedirectHost
				r.URL.Path = "/" + aboutRedirects[mux.Vars(r)["Path"]]
				http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
			}))

		// Community search contexts pages. Must mirror client/web/src/communitySearchContexts/routes.tsx
		communitySearchContexts := []string{"kubernetes", "stanford", "stackstorm", "temporal", "o3de", "chakraui", "julia", "backstage"}
		r.Path("/{Path:(?:" + strings.Join(communitySearchContexts, "|") + ")}").Methods("GET").Name("community-search-contexts").Handler(brandedNoIndex("Community search context"))

		cncfDescription := "Search all repositories in the Cloud Native Computing Foundation (CNCF)."
		r.Path("/cncf").Methods("GET").Name("community-search-contexts.cncf").Handler(handler(db, serveBrandedPageString(db, "CNCF code search", &cncfDescription, index)))
		r.PathPrefix("/devtooltime").Methods("GET").Name("devtooltime").Handler(staticRedirectHandler("https://info.sourcegraph.com/dev-tool-time", http.StatusMovedPermanently))

		// legacy routes
		r.Path("/login").Methods("GET").Name(routeLegacyLogin).Handler(staticRedirectHandler("/sign-in", http.StatusMovedPermanently))
		r.Path("/careers").Methods("GET").Name(routeLegacyCareers).Handler(staticRedirectHandler("https://sourcegraph.com/jobs", http.StatusMovedPermanently))

		r.PathPrefix("/extensions").Methods("GET").Name("extensions").
			HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/", http.StatusMovedPermanently)
			})
	}

	// Help, has to be defined after about subdomain
	r.PathPrefix("/help").Methods("GET").Name("help").HandlerFunc(serveHelp)

	// repo, has to come last
	serveRepoHandler := handler(db, serveRepoOrBlob(db, routeRepo, func(c *Common, r *http.Request) string {
		// e.g. "gorilla/mux - Sourcegraph"
		return brandNameSubtitle(repoShortName(c.Repo.Name))
	}))
	repoRevPath := "/" + routevar.Repo + routevar.RepoRevSuffix
	repoRoot := r.Path(repoRevPath).Methods("GET").Name(routeRepo).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Debug mode: register the __errorTest handler.
		if env.InsecureDev && r.URL.Path == "/__errorTest" {
			handler(db, serveErrorTest(db)).ServeHTTP(w, r)
			return
		}

		if mockServeRepo != nil {
			mockServeRepo(w, r)
			return
		}
		serveRepoHandler.ServeHTTP(w, r)
	}))

	// We don't need to know about repo subroutes
	sveltekit.RegisterSvelteKit(r, repoRoot)

	repoRev := r.PathPrefix(repoRevPath + "/" + routevar.RepoPathDelim).Subrouter()
	// tree
	repoRev.Path("/tree{Path:.*}").Methods("GET").
		Name(routeTree).
		Handler(handler(db, serveTree(db, func(c *Common, r *http.Request) string {
			// e.g. "src - gorilla/mux - Sourcegraph"
			dirName := path.Base(mux.Vars(r)["Path"])
			return brandNameSubtitle(dirName, repoShortName(c.Repo.Name))
		})))

	// blob
	repoRev.Path("/blob{Path:.*}").Methods("GET").
		Name(routeBlob).
		Handler(handler(db, serveRepoOrBlob(db, routeBlob, func(c *Common, r *http.Request) string {
			// e.g. "mux.go - gorilla/mux - Sourcegraph"
			fileName := path.Base(mux.Vars(r)["Path"])
			return brandNameSubtitle(fileName, repoShortName(c.Repo.Name))
		})))

	// raw
	repoRev.Path("/raw{Path:.*}").Methods("GET", "HEAD").Name(routeRaw).Handler(handler(db, serveRaw(logger, db, gitserver.NewClient("http.raw"))))

	// batch changes - branded
	repoRev.PathPrefix("/batch-changes").Methods("GET").Name("repo-batch-changes").Handler(brandedIndex("Batch Changes"))

	for _, p := range []struct {
		pathPrefix, name, title string
	}{
		{pathPrefix: "/settings", name: "repo-settings", title: "Repository settings"},
		{pathPrefix: "/code-graph", name: "repo-code-intelligence", title: "Code graph"},
		{pathPrefix: "/commits", name: "repo-commits", title: "Commits"},
		{pathPrefix: "/commit", name: "repo-commit", title: "Commit"},
		{pathPrefix: "/branches", name: "repo-branches", title: "Branches"},
		{pathPrefix: "/tags", name: "repo-tags", title: "Tags"},
		{pathPrefix: "/compare", name: "repo-compare", title: "Compare"},
		{pathPrefix: "/stats", name: "repo-stats", title: "Stats"},
		{pathPrefix: "/own", name: "repo-own", title: "Ownership"},
	} {
		repoRev.PathPrefix(p.pathPrefix).Methods("GET").Name(p.name).Handler(brandedNoIndex(p.title))
	}

	// legacy redirects
	if dotcom.SourcegraphDotComMode() {
		repoRev.Path("/info").Methods("GET").Name("page.repo.landing").Handler(handler(db, serveRepoLanding(db)))
		repoRev.Path("/{dummy:def|refs}/" + routevar.Def).Methods("GET").Name("page.def.redirect").Handler(http.HandlerFunc(serveDefRedirectToDefLanding))
		repoRev.Path("/info/" + routevar.Def).Methods("GET").Name(routeLegacyDefLanding).Handler(handler(db, serveDefLanding))
		repoRev.Path("/land/" + routevar.Def).Methods("GET").Name("page.def.landing.old").Handler(http.HandlerFunc(serveOldRouteDefLanding))
	}

	// All other routes that are not found.
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveError(w, r, db, errors.New("route not found"), http.StatusNotFound)
	})

	uirouter.Router = r // make accessible to other packages
}

var mockServeRepo func(w http.ResponseWriter, r *http.Request)

// brandNameSubtitle returns a string with the specified title sequence and the brand name as the
// last title component. This function indirectly calls conf.Get(), so should not be invoked from
// any function that is invoked by an init function.
func brandNameSubtitle(titles ...string) string {
	return strings.Join(append(titles, conf.Branding().BrandName), " - ")
}

// staticRedirectHandler returns an HTTP handler that redirects all requests to
// the specified url or relative path with the specified status code.
//
// The scheme, host, and path in the specified url override ones in the incoming
// request. For example:
//
//	staticRedirectHandler("http://google.com") serving "https://sourcegraph.com/foobar?q=foo" -> "http://google.com/foobar?q=foo"
//	staticRedirectHandler("/foo") serving "https://sourcegraph.com/bar?q=foo" -> "https://sourcegraph.com/foo?q=foo"
func staticRedirectHandler(u string, code int) http.Handler {
	target, err := url.Parse(u)
	if err != nil {
		// panic is OK here because staticRedirectHandler is called only inside
		// init / crash would be on server startup.
		panic(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if target.Scheme != "" {
			r.URL.Scheme = target.Scheme
		}
		if target.Host != "" {
			r.URL.Host = target.Host
		}
		if target.Path != "" {
			r.URL.Path = target.Path
		}
		http.Redirect(w, r, r.URL.String(), code)
	})
}

// limitString limits the given string to at most N characters, optionally
// adding an ellipsis (…) at the end.
func limitString(s string, n int, ellipsis bool) string {
	if len(s) < n {
		return s
	}
	if ellipsis {
		return s[:n-1] + "…"
	}
	return s[:n-1]
}

// handler wraps an HTTP handler that returns potential errors. If any error is
// returned, serveError is called.
//
// Clients that wish to return their own HTTP status code should use this from
// their handler:
//
//	serveError(w, r, err, http.MyStatusCode)
//	return nil
func handler(db database.DB, f handlerFunc) http.Handler {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				serveError(w, r, db, recoverError{recover: rec, stack: debug.Stack()}, http.StatusInternalServerError)
			}
		}()
		if err := f(w, r); err != nil {
			serveError(w, r, db, err, http.StatusInternalServerError)
		}
	})
	return trace.Route(gziphandler.GzipHandler(h))
}

type recoverError struct {
	recover any
	stack   []byte
}

func (r recoverError) Error() string {
	return fmt.Sprintf("ui: recovered from panic: %v", r.recover)
}

// serveError serves the error template with the specified error message. It is
// assumed that the error message could accidentally contain sensitive data,
// and as such is only presented to the user in debug mode.
func serveError(w http.ResponseWriter, r *http.Request, db database.DB, err error, statusCode int) {
	serveErrorNoDebug(w, r, db, err, statusCode, false, false)
}

// dangerouslyServeError is like serveError except it always shows the error to
// the user and as such, if it contains sensitive information, it can leak
// sensitive information.
//
// See https://github.com/sourcegraph/sourcegraph/issues/9453
func dangerouslyServeError(w http.ResponseWriter, r *http.Request, db database.DB, err error, statusCode int) {
	serveErrorNoDebug(w, r, db, err, statusCode, false, true)
}

type pageError struct {
	StatusCode int    `json:"statusCode"`
	StatusText string `json:"statusText"`
	Error      string `json:"error"`
	ErrorID    string `json:"errorID"`
}

// serveErrorNoDebug should not be called by anyone except serveErrorTest.
func serveErrorNoDebug(w http.ResponseWriter, r *http.Request, db database.DB, err error, statusCode int, nodebug, forceServeError bool) {
	w.WriteHeader(statusCode)
	errorID := randstring.NewLen(6)

	logger := log.Scoped("ui")

	// Determine trace URL and log the error.
	var traceURL string
	if tr := trace.FromContext(r.Context()); tr.IsRecording() {
		tr.SetError(err)
		tr.SetAttributes(attribute.String("error-id", errorID))
		traceURL = trace.URL(trace.ID(r.Context()))
	}
	logFields := []log.Field{
		log.String("method", r.Method),
		log.String("request_uri", r.URL.RequestURI()),
		log.Int("status_code", statusCode),
		log.Error(err),
		log.String("error_id", errorID),
		log.String("trace", traceURL),
	}
	if statusCode >= 400 && statusCode < 500 {
		logger.Warn(
			"ui HTTP handler error response",
			logFields...,
		)
	} else {
		logger.Error(
			"ui HTTP handler error response",
			logFields...,
		)
	}

	// In the case of recovering from a panic, we nicely include the stack
	// trace in the error that is shown on the page. Additionally, we log it
	// separately.
	var e recoverError
	if errors.As(err, &e) {
		err = errors.Errorf("%v\n\n%s", e.recover, e.stack)
		logger.Error(
			"recovered from panic",
			log.Error(err),
		)
	}

	var errorIfDebug string
	if forceServeError || (env.InsecureDev && !nodebug) {
		errorIfDebug = err.Error()
	}

	pageErrorContext := &pageError{
		StatusCode: statusCode,
		StatusText: http.StatusText(statusCode),
		Error:      errorIfDebug,
		ErrorID:    errorID,
	}

	// First try to render the error fancily: this relies on *Common
	// functionality that might always work (for example, if some services are
	// down rather than something that is primarily a user error).
	delete(mux.Vars(r), "Repo")
	var commonServeErr error
	title := brandNameSubtitle(fmt.Sprintf("%v %s", statusCode, http.StatusText(statusCode)))
	common, commonErr := newCommon(w, r, db, title, index, func(w http.ResponseWriter, r *http.Request, db database.DB, err error, statusCode int) {
		// Stub out serveError to newCommon so that it is not reentrant.
		commonServeErr = err
	})
	if commonErr == nil && commonServeErr == nil {
		if common == nil {
			return // request handled by newCommon
		}

		common.Error = pageErrorContext
		fancyErr := renderTemplate(w, "app.html", &struct {
			*Common
		}{
			Common: common,
		})
		if fancyErr != nil {
			logger.Error("ui: error while serving fancy error template", log.Error(fancyErr))
			// continue onto fallback below..
		} else {
			return
		}
	}

	// Fallback to ugly / reliable error template.
	stdErr := renderTemplate(w, "error.html", pageErrorContext)
	if stdErr != nil {
		logger.Error("error while serving final error template", log.Error(stdErr))
	}
}

// serveErrorTest makes it easy to test styling/layout of the error template by
// visiting:
//
//	http://localhost:3080/__errorTest?nodebug=true&error=theerror&status=500
//
// The `nodebug=true` parameter hides error messages (which is ALWAYS the case
// in production), `error` controls the error message text, and status controls
// the status code.
func serveErrorTest(db database.DB) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if !env.InsecureDev {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		q := r.URL.Query()
		nodebug := q.Get("nodebug") == "true"
		errorText := q.Get("error")
		statusCode, _ := strconv.Atoi(q.Get("status"))
		serveErrorNoDebug(w, r, db, errors.New(errorText), statusCode, nodebug, false)
		return nil
	}
}

func mapKeys(m map[string]string) (keys []string) {
	keys = make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
