package ui

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/mux"
	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	uirouter "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui/router"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/routevar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/randstring"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	routeHome                    = "home"
	routeSearch                  = "search"
	routeSearchBadge             = "search-badge"
	routeRepo                    = "repo"
	routeRepoSettings            = "repo-settings"
	routeRepoCodeIntelligence    = "repo-code-intelligence"
	routeRepoCommit              = "repo-commit"
	routeRepoBranches            = "repo-branches"
	routeRepoBatchChanges        = "repo-batch-changes"
	routeRepoCommits             = "repo-commits"
	routeRepoTags                = "repo-tags"
	routeRepoCompare             = "repo-compare"
	routeRepoStats               = "repo-stats"
	routeInsights                = "insights"
	routeSetup                   = "setup"
	routeBatchChanges            = "batch-changes"
	routeWelcome                 = "welcome"
	routeCodeMonitoring          = "code-monitoring"
	routeContexts                = "contexts"
	routeThreads                 = "threads"
	routeTree                    = "tree"
	routeBlob                    = "blob"
	routeRaw                     = "raw"
	routeOrganizations           = "org"
	routeTeams                   = "team"
	routeSettings                = "settings"
	routeSiteAdmin               = "site-admin"
	routeAPIConsole              = "api-console"
	routeUser                    = "user"
	routeUserSettings            = "user-settings"
	routeUserRedirect            = "user-redirect"
	routeAboutSubdomain          = "about-subdomain"
	aboutRedirectScheme          = "https"
	aboutRedirectHost            = "about.sourcegraph.com"
	routeSurvey                  = "survey"
	routeSurveyScore             = "survey-score"
	routeHelp                    = "help"
	routeCommunitySearchContexts = "community-search-contexts"
	routeCncf                    = "community-search-contexts.cncf"
	routeSnippets                = "snippets"
	routeSubscriptions           = "subscriptions"
	routeViews                   = "views"
	routeDevToolTime             = "devtooltime"
	routeEmbed                   = "embed"
	routeCody                    = "cody"

	routeSearchStream  = "search.stream"
	routeSearchConsole = "search.console"
	routeNotebooks     = "search.notebook"

	// Legacy redirects
	routeLegacyLogin                   = "login"
	routeLegacyCareers                 = "careers"
	routeLegacyDefLanding              = "page.def.landing"
	routeLegacyOldRouteDefLanding      = "page.def.landing.old"
	routeLegacyRepoLanding             = "page.repo.landing"
	routeLegacyDefRedirectToDefLanding = "page.def.redirect"
	routeLegacyExtensions              = "extensions"
)

// aboutRedirects contains map entries, each of which indicates that
// sourcegraph.com/$KEY should redirect to about.sourcegraph.com/$VALUE.
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

// Router returns the router that serves pages for our web app.
func Router() *mux.Router {
	return uirouter.Router
}

// InitRouter create the router that serves pages for our web app
// and assigns it to uirouter.Router.
// The router can be accessed by calling Router().
func InitRouter(db database.DB, enterpriseJobs jobutil.EnterpriseJobs) {
	router := newRouter()
	initRouter(db, enterpriseJobs, router)
}

var mockServeRepo func(w http.ResponseWriter, r *http.Request)

func newRouter() *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	homeRouteMethods := []string{"GET"}
	if envvar.SourcegraphDotComMode() {
		homeRouteMethods = append(homeRouteMethods, "HEAD")
	}

	// Top-level routes.
	r.Path("/").Methods(homeRouteMethods...).Name(routeHome)
	r.PathPrefix("/threads").Methods("GET").Name(routeThreads)
	r.Path("/search").Methods("GET").Name(routeSearch)
	r.Path("/search/badge").Methods("GET").Name(routeSearchBadge)
	r.Path("/search/stream").Methods("GET").Name(routeSearchStream)
	r.Path("/search/console").Methods("GET").Name(routeSearchConsole)
	r.Path("/sign-in").Methods("GET").Name(uirouter.RouteSignIn)
	r.Path("/sign-up").Methods("GET").Name(uirouter.RouteSignUp)
	r.PathPrefix("/request-access").Methods("GET").Name(uirouter.RouteRequestAccess)
	r.Path("/unlock-account/{token}").Methods("GET").Name(uirouter.RouteUnlockAccount)
	r.Path("/welcome").Methods("GET").Name(routeWelcome)
	r.PathPrefix("/insights").Methods("GET").Name(routeInsights)
	r.PathPrefix("/setup").Methods("GET").Name(routeSetup)
	r.PathPrefix("/batch-changes").Methods("GET").Name(routeBatchChanges)
	r.PathPrefix("/code-monitoring").Methods("GET").Name(routeCodeMonitoring)
	r.PathPrefix("/contexts").Methods("GET").Name(routeContexts)
	r.PathPrefix("/notebooks").Methods("GET").Name(routeNotebooks)
	r.PathPrefix("/organizations").Methods("GET").Name(routeOrganizations)
	r.PathPrefix("/teams").Methods("GET").Name(routeTeams)
	r.PathPrefix("/settings").Methods("GET").Name(routeSettings)
	r.PathPrefix("/site-admin").Methods("GET").Name(routeSiteAdmin)
	r.Path("/password-reset").Methods("GET").Name(uirouter.RoutePasswordReset)
	r.Path("/api/console").Methods("GET").Name(routeAPIConsole)
	r.Path("/{Path:(?:" + strings.Join(mapKeys(aboutRedirects), "|") + ")}").Methods("GET").Name(routeAboutSubdomain)
	r.PathPrefix("/users/{username}/settings").Methods("GET").Name(routeUserSettings)
	r.PathPrefix("/users/{username}").Methods("GET").Name(routeUser)
	r.PathPrefix("/user").Methods("GET").Name(routeUserRedirect)
	r.Path("/survey").Methods("GET").Name(routeSurvey)
	r.Path("/survey/{score}").Methods("GET").Name(routeSurveyScore)
	r.PathPrefix("/help").Methods("GET").Name(routeHelp)
	r.PathPrefix("/snippets").Methods("GET").Name(routeSnippets)
	r.PathPrefix("/subscriptions").Methods("GET").Name(routeSubscriptions)
	r.PathPrefix("/views").Methods("GET").Name(routeViews)
	r.PathPrefix("/devtooltime").Methods("GET").Name(routeDevToolTime)
	r.PathPrefix("/cody").Methods("GET").Name(routeCody)
	r.Path("/ping-from-self-hosted").Methods("GET", "OPTIONS").Name(uirouter.RoutePingFromSelfHosted)

	// 🚨 SECURITY: The embed route is used to serve embeddable content (via an iframe) to 3rd party sites.
	// Any changes to the embedding route could have security implications. Please consult the security team
	// before making changes. See the `serveEmbed` function for further details.
	r.PathPrefix("/embed").Methods("GET").Name(routeEmbed)

	// Community search contexts pages. Must mirror client/web/src/communitySearchContexts/routes.tsx
	if envvar.SourcegraphDotComMode() {
		communitySearchContexts := []string{"kubernetes", "stanford", "stackstorm", "temporal", "o3de", "chakraui", "julia", "backstage"}
		r.Path("/{Path:(?:" + strings.Join(communitySearchContexts, "|") + ")}").Methods("GET").Name(routeCommunitySearchContexts)
		r.Path("/cncf").Methods("GET").Name(routeCncf)
	}

	// Legacy redirects
	r.Path("/login").Methods("GET").Name(routeLegacyLogin)
	r.Path("/careers").Methods("GET").Name(routeLegacyCareers)
	if envvar.SourcegraphDotComMode() {
		r.PathPrefix("/extensions").Methods("GET").Name(routeLegacyExtensions)
	}

	// repo
	repoRevPath := "/" + routevar.Repo + routevar.RepoRevSuffix
	r.Path(repoRevPath).Methods("GET").Name(routeRepo)

	// tree
	repoRev := r.PathPrefix(repoRevPath + "/" + routevar.RepoPathDelim).Subrouter()
	repoRev.Path("/tree{Path:.*}").Methods("GET").Name(routeTree)

	repoRev.PathPrefix("/commits").Methods("GET").Name(routeRepoCommits)

	// blob
	repoRev.Path("/blob{Path:.*}").Methods("GET").Name(routeBlob)

	// raw
	repoRev.Path("/raw{Path:.*}").Methods("GET", "HEAD").Name(routeRaw)

	repo := r.PathPrefix(repoRevPath + "/" + routevar.RepoPathDelim).Subrouter()
	repo.PathPrefix("/settings").Methods("GET").Name(routeRepoSettings)
	repo.PathPrefix("/code-intelligence").Methods("GET").Name(routeRepoCodeIntelligence)
	repo.PathPrefix("/commit").Methods("GET").Name(routeRepoCommit)
	repo.PathPrefix("/branches").Methods("GET").Name(routeRepoBranches)
	repo.PathPrefix("/batch-changes").Methods("GET").Name(routeRepoBatchChanges)
	repo.PathPrefix("/tags").Methods("GET").Name(routeRepoTags)
	repo.PathPrefix("/compare").Methods("GET").Name(routeRepoCompare)
	repo.PathPrefix("/stats").Methods("GET").Name(routeRepoStats)

	// legacy redirects
	repo.Path("/info").Methods("GET").Name(routeLegacyRepoLanding)
	repoRev.Path("/{dummy:def|refs}/" + routevar.Def).Methods("GET").Name(routeLegacyDefRedirectToDefLanding)
	repoRev.Path("/info/" + routevar.Def).Methods("GET").Name(routeLegacyDefLanding)
	repoRev.Path("/land/" + routevar.Def).Methods("GET").Name(routeLegacyOldRouteDefLanding)
	return r
}

// brandNameSubtitle returns a string with the specified title sequence and the brand name as the
// last title component. This function indirectly calls conf.Get(), so should not be invoked from
// any function that is invoked by an init function.
func brandNameSubtitle(titles ...string) string {
	return strings.Join(append(titles, globals.Branding().BrandName), " - ")
}

func initRouter(db database.DB, enterpriseJobs jobutil.EnterpriseJobs, router *mux.Router) {
	uirouter.Router = router // make accessible to other packages

	brandedIndex := func(titles string) http.Handler {
		return handler(db, serveBrandedPageString(db, titles, nil, index))
	}

	brandedNoIndex := func(titles string) http.Handler {
		return handler(db, serveBrandedPageString(db, titles, nil, noIndex))
	}

	// basic pages with static titles
	router.Get(routeHome).Handler(handler(db, serveHome(db)))
	router.Get(routeThreads).Handler(brandedNoIndex("Threads"))
	router.Get(routeInsights).Handler(brandedIndex("Insights"))
	router.Get(routeSetup).Handler(brandedIndex("Setup"))
	router.Get(routeBatchChanges).Handler(brandedIndex("Batch Changes"))
	router.Get(routeCodeMonitoring).Handler(brandedIndex("Code Monitoring"))
	router.Get(routeContexts).Handler(brandedNoIndex("Search Contexts"))
	router.Get(uirouter.RouteSignIn).Handler(handler(db, serveSignIn(db)))
	router.Get(uirouter.RouteRequestAccess).Handler(brandedIndex("Request access"))
	router.Get(uirouter.RouteSignUp).Handler(brandedIndex("Sign up"))
	router.Get(uirouter.RouteUnlockAccount).Handler(brandedNoIndex("Unlock Your Account"))
	router.Get(routeWelcome).Handler(brandedNoIndex("Welcome"))
	router.Get(routeOrganizations).Handler(brandedNoIndex("Organization"))
	router.Get(routeTeams).Handler(brandedNoIndex("Team"))
	router.Get(routeSettings).Handler(brandedNoIndex("Settings"))
	router.Get(routeSiteAdmin).Handler(brandedNoIndex("Admin"))
	router.Get(uirouter.RoutePasswordReset).Handler(brandedNoIndex("Reset password"))
	router.Get(routeAPIConsole).Handler(brandedIndex("API console"))
	router.Get(routeRepoSettings).Handler(brandedNoIndex("Repository settings"))
	router.Get(routeRepoCodeIntelligence).Handler(brandedNoIndex("Code intelligence"))
	router.Get(routeRepoCommit).Handler(brandedNoIndex("Commit"))
	router.Get(routeRepoBranches).Handler(brandedNoIndex("Branches"))
	router.Get(routeRepoBatchChanges).Handler(brandedIndex("Batch Changes"))
	router.Get(routeRepoCommits).Handler(brandedNoIndex("Commits"))
	router.Get(routeRepoTags).Handler(brandedNoIndex("Tags"))
	router.Get(routeRepoCompare).Handler(brandedNoIndex("Compare"))
	router.Get(routeRepoStats).Handler(brandedNoIndex("Stats"))
	router.Get(routeSurvey).Handler(brandedNoIndex("Survey"))
	router.Get(routeSurveyScore).Handler(brandedNoIndex("Survey"))
	router.Get(routeHelp).HandlerFunc(serveHelp)
	router.Get(routeSnippets).Handler(brandedNoIndex("Snippets"))
	router.Get(routeSubscriptions).Handler(brandedNoIndex("Subscriptions"))
	router.Get(routeViews).Handler(brandedNoIndex("View"))
	router.Get(routeCody).Handler(brandedNoIndex("Cody"))
	router.Get(uirouter.RoutePingFromSelfHosted).Handler(handler(db, servePingFromSelfHosted))

	// 🚨 SECURITY: The embed route is used to serve embeddable content (via an iframe) to 3rd party sites.
	// Any changes to the embedding route could have security implications. Please consult the security team
	// before making changes. See the `serveEmbed` function for further details.
	router.Get(routeEmbed).Handler(handler(db, serveEmbed(db)))

	router.Get(routeUserSettings).Handler(brandedNoIndex("User settings"))
	router.Get(routeUserRedirect).Handler(brandedNoIndex("User"))
	router.Get(routeUser).Handler(handler(db, serveBasicPage(db, func(c *Common, r *http.Request) string {
		return brandNameSubtitle(mux.Vars(r)["username"])
	}, nil, noIndex)))
	router.Get(routeSearchConsole).Handler(brandedIndex("Search console"))
	router.Get(routeNotebooks).Handler(brandedIndex("Notebooks"))

	// Legacy redirects
	if envvar.SourcegraphDotComMode() {
		router.Get(routeLegacyLogin).Handler(staticRedirectHandler("/sign-in", http.StatusMovedPermanently))
		router.Get(routeLegacyCareers).Handler(staticRedirectHandler("https://about.sourcegraph.com/jobs", http.StatusMovedPermanently))
		router.Get(routeLegacyOldRouteDefLanding).Handler(http.HandlerFunc(serveOldRouteDefLanding))
		router.Get(routeLegacyDefRedirectToDefLanding).Handler(http.HandlerFunc(serveDefRedirectToDefLanding))
		router.Get(routeLegacyDefLanding).Handler(handler(db, serveDefLanding))
		router.Get(routeLegacyRepoLanding).Handler(handler(db, serveRepoLanding(db)))
		router.Get(routeLegacyExtensions).Handler(http.NotFoundHandler())
	}

	// search
	router.Get(routeSearch).Handler(handler(db, serveBasicPage(db, func(c *Common, r *http.Request) string {
		shortQuery := limitString(r.URL.Query().Get("q"), 25, true)
		if shortQuery == "" {
			return globals.Branding().BrandName
		}
		// e.g. "myquery - Sourcegraph"
		return brandNameSubtitle(shortQuery)
	}, nil, index)))

	// streaming search
	router.Get(routeSearchStream).Handler(search.StreamHandler(db, enterpriseJobs))

	// search badge
	router.Get(routeSearchBadge).Handler(searchBadgeHandler())

	if envvar.SourcegraphDotComMode() {
		// about subdomain
		router.Get(routeAboutSubdomain).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Scheme = aboutRedirectScheme
			r.URL.User = nil
			r.URL.Host = aboutRedirectHost
			r.URL.Path = "/" + aboutRedirects[mux.Vars(r)["Path"]]
			http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
		}))
		router.Get(routeCommunitySearchContexts).Handler(brandedNoIndex("Community search context"))
		cncfDescription := "Search all repositories in the Cloud Native Computing Foundation (CNCF)."
		router.Get(routeCncf).Handler(handler(db, serveBrandedPageString(db, "CNCF code search", &cncfDescription, index)))
		router.Get(routeDevToolTime).Handler(staticRedirectHandler("https://info.sourcegraph.com/dev-tool-time", http.StatusMovedPermanently))
	}

	// repo
	serveRepoHandler := handler(db, serveRepoOrBlob(db, routeRepo, func(c *Common, r *http.Request) string {
		// e.g. "gorilla/mux - Sourcegraph"
		return brandNameSubtitle(repoShortName(c.Repo.Name))
	}))
	router.Get(routeRepo).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// tree
	router.Get(routeTree).Handler(handler(db, serveTree(db, func(c *Common, r *http.Request) string {
		// e.g. "src - gorilla/mux - Sourcegraph"
		dirName := path.Base(mux.Vars(r)["Path"])
		return brandNameSubtitle(dirName, repoShortName(c.Repo.Name))
	})))

	// blob
	router.Get(routeBlob).Handler(handler(db, serveRepoOrBlob(db, routeBlob, func(c *Common, r *http.Request) string {
		// e.g. "mux.go - gorilla/mux - Sourcegraph"
		fileName := path.Base(mux.Vars(r)["Path"])
		return brandNameSubtitle(fileName, repoShortName(c.Repo.Name))
	})))

	// raw
	router.Get(routeRaw).Handler(handler(db, serveRaw(db, gitserver.NewClient())))

	// All other routes that are not found.
	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveError(w, r, db, errors.New("route not found"), http.StatusNotFound)
	})
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

	// Determine trace URL and log the error.
	var traceURL string
	if span := opentracing.SpanFromContext(r.Context()); span != nil {
		ext.Error.Set(span, true)
		span.SetTag("err", err)
		span.SetTag("error-id", errorID)
		traceURL = trace.URL(trace.ID(r.Context()), conf.DefaultClient())
	}
	log15.Error("ui HTTP handler error response", "method", r.Method, "request_uri", r.URL.RequestURI(), "status_code", statusCode, "error", err, "error_id", errorID, "trace", traceURL)

	// In the case of recovering from a panic, we nicely include the stack
	// trace in the error that is shown on the page. Additionally, we log it
	// separately (since log15 prints the escaped sequence).
	var e recoverError
	if errors.As(err, &e) {
		err = errors.Errorf("ui: recovered from panic %v\n\n%s", e.recover, e.stack)
		log.Println(err)
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
			log15.Error("ui: error while serving fancy error template", "error", fancyErr)
			// continue onto fallback below..
		} else {
			return
		}
	}

	// Fallback to ugly / reliable error template.
	stdErr := renderTemplate(w, "error.html", pageErrorContext)
	if stdErr != nil {
		log15.Error("ui: error while serving final error template", "error", stdErr)
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
