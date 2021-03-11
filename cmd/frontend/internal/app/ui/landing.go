package ui

import (
	"net/http"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/handlerutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

var goSymbolReg = lazyregexp.New("/info/GoPackage/(.+)$")

// serveRepoLanding simply redirects the old (sourcegraph.com/<repo>/-/info) repo landing page
// URLs directly to the repo itself (sourcegraph.com/<repo>).
func serveRepoLanding(w http.ResponseWriter, r *http.Request) error {
	legacyRepoLandingCounter.Inc()

	repo, commitID, err := handlerutil.GetRepoAndRev(r.Context(), mux.Vars(r))
	if err != nil {
		if errcode.IsHTTPErrorCode(err, http.StatusNotFound) {
			return &errcode.HTTPErr{Status: http.StatusNotFound, Err: err}
		}
		return errors.Wrap(err, "GetRepoAndRev")
	}
	http.Redirect(w, r, "/"+string(repo.Name)+"@"+string(commitID), http.StatusMovedPermanently)
	return nil
}

func serveDefLanding(w http.ResponseWriter, r *http.Request) (err error) {
	span, ctx := ot.StartSpanFromContext(r.Context(), "serveDefLanding")
	r = r.WithContext(ctx)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	legacyDefLandingCounter.Inc()

	match := goSymbolReg.FindStringSubmatch(r.URL.Path)
	if match == nil {
		return &errcode.HTTPErr{Status: http.StatusNotFound, Err: err}
	}
	http.Redirect(w, r, "/go/"+match[1], http.StatusMovedPermanently)
	return nil
}

var legacyDefLandingCounter = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "src",
	Name:      "legacy_def_landing_webapp",
	Help:      "Number of times a legacy def landing page has been served.",
})

var legacyRepoLandingCounter = promauto.NewCounter(prometheus.CounterOpts{
	Namespace: "src",
	Name:      "legacy_repo_landing_webapp",
	Help:      "Number of times a legacy repo landing page has been served.",
})

// serveDefRedirectToDefLanding redirects from /REPO/refs/... and
// /REPO/def/... URLs to the def landing page. Those URLs used to
// point to JavaScript-backed pages in the UI for a refs list and code
// view, respectively, but now def URLs are only for SEO (and thus
// those URLs are only handled by this package).
func serveDefRedirectToDefLanding(w http.ResponseWriter, r *http.Request) {
	routeVars := mux.Vars(r)
	pairs := make([]string, 0, len(routeVars)*2)
	for k, v := range routeVars {
		if k == "dummy" { // only used for matching string "def" or "refs"
			continue
		}
		pairs = append(pairs, k, v)
	}
	u, err := Router().Get(routeLegacyDefLanding).URL(pairs...)
	if err != nil {
		log15.Error("Def redirect URL construction failed.", "url", r.URL.String(), "routeVars", routeVars, "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
}

// Redirect from old /land/ def landing URLs to new /info/ URLs
func serveOldRouteDefLanding(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	infoURL, err := Router().Get(routeLegacyDefLanding).URL(
		"Repo", vars["Repo"], "Path", vars["Path"], "Rev", vars["Rev"], "UnitType", vars["UnitType"], "Unit", vars["Unit"])
	if err != nil {
		repoURL, err := Router().Get(routeRepo).URL("Repo", vars["Repo"], "Rev", vars["Rev"])
		if err != nil {
			// Last recourse is redirect to homepage
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		// Redirect to repo page if info page URL could not be constructed
		http.Redirect(w, r, repoURL.String(), http.StatusFound)
		return
	}
	// Redirect to /info/ page
	http.Redirect(w, r, infoURL.String(), http.StatusMovedPermanently)
}
