package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	uirouter "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui/router"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func init() {
	// Enable SourcegraphDotComMode for all tests in this package.
	dotcom.MockSourcegraphDotComMode(fakeTB{}, true)
}

type fakeTB struct{}

func (fakeTB) Cleanup(func()) {}

func TestRouter(t *testing.T) {
	InitRouter(dbmocks.NewMockDB())
	router := Router()
	tests := []struct {
		path      string
		wantRoute string
		wantVars  map[string]string
	}{
		// home
		{
			path:      "/",
			wantRoute: routeHome,
			wantVars:  map[string]string{},
		},

		// search
		{
			path:      "/search",
			wantRoute: routeSearch,
			wantVars:  map[string]string{},
		},

		// search badge
		{
			path:      "/search/badge",
			wantRoute: routeSearchBadge,
			wantVars:  map[string]string{},
		},

		// repo
		{
			path:      "/r",
			wantRoute: routeRepo,
			wantVars:  map[string]string{"Repo": "r", "Rev": ""},
		},
		{
			path:      "/r/r",
			wantRoute: routeRepo,
			wantVars:  map[string]string{"Repo": "r/r", "Rev": ""},
		},
		{
			path:      "/r/r@v",
			wantRoute: routeRepo,
			wantVars:  map[string]string{"Repo": "r/r", "Rev": "@v"},
		},
		{
			path:      "/r/r@v/v",
			wantRoute: routeRepo,
			wantVars:  map[string]string{"Repo": "r/r", "Rev": "@v/v"},
		},

		// tree
		{
			path:      "/r@v/-/tree",
			wantRoute: routeTree,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": ""},
		},
		{
			path:      "/r@v/-/tree/d",
			wantRoute: routeTree,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": "/d"},
		},
		{
			path:      "/r@v/-/tree/d/d",
			wantRoute: routeTree,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": "/d/d"},
		},

		// blob
		{
			path:      "/r@v/-/blob/f",
			wantRoute: routeBlob,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": "/f"},
		},
		{
			path:      "/r@v/-/blob/d/f",
			wantRoute: routeBlob,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": "/d/f"},
		},

		// raw
		{
			path:      "/r@v/-/raw",
			wantRoute: routeRaw,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": ""},
		},
		{
			path:      "/r@v/-/raw/f",
			wantRoute: routeRaw,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": "/f"},
		},
		{
			path:      "/r@v/-/raw/d/f",
			wantRoute: routeRaw,
			wantVars:  map[string]string{"Repo": "r", "Rev": "@v", "Path": "/d/f"},
		},

		// sourcegraph.com redirects
		{
			path:      "/about",
			wantRoute: routeAboutSubdomain,
			wantVars:  map[string]string{"Path": "about"},
		},
		{
			path:      "/privacy",
			wantRoute: routeAboutSubdomain,
			wantVars:  map[string]string{"Path": "privacy"},
		},
		{
			path:      "/help/terms",
			wantRoute: routeAboutSubdomain,
			wantVars:  map[string]string{"Path": "help/terms"},
		},

		// sign-in
		{
			path:      "/sign-in",
			wantRoute: uirouter.RouteSignIn,
			wantVars:  map[string]string{},
		},

		// request-access
		{
			path:      "/request-access",
			wantRoute: uirouter.RouteRequestAccess,
			wantVars:  map[string]string{},
		},

		// settings
		{
			path:      "/settings",
			wantRoute: routeSettings,
			wantVars:  map[string]string{},
		},
		{
			path:      "/settings/profile",
			wantRoute: routeSettings,
			wantVars:  map[string]string{},
		},

		// password invite
		{
			path:      "/password-reset",
			wantRoute: uirouter.RoutePasswordReset,
			wantVars:  map[string]string{},
		},

		{
			path:      "/site-admin",
			wantRoute: routeSiteAdmin,
			wantVars:  map[string]string{},
		},
		{
			path:      "/site-admin/config",
			wantRoute: routeSiteAdmin,
			wantVars:  map[string]string{},
		},

		// legacy login
		{
			path:      "/login",
			wantRoute: routeLegacyLogin,
			wantVars:  map[string]string{},
		},

		// legacy careers
		{
			path:      "/careers",
			wantRoute: routeLegacyCareers,
			wantVars:  map[string]string{},
		},
	}
	for _, tst := range tests {
		t.Run(tst.wantRoute+"/"+tst.path, func(t *testing.T) {
			var (
				routeMatch mux.RouteMatch
				routeName  string
			)
			match := router.Match(&http.Request{Method: "GET", URL: &url.URL{Path: tst.path}}, &routeMatch)
			if match {
				routeName = routeMatch.Route.GetName()
			}
			if routeName != tst.wantRoute {
				t.Fatalf("path %q got route %q want %q", tst.path, routeName, tst.wantRoute)
			}
			if !reflect.DeepEqual(routeMatch.Vars, tst.wantVars) {
				t.Fatalf("path %q got vars %v want %v", tst.path, routeMatch.Vars, tst.wantVars)
			}
		})
	}
}

func TestRouter_RootPath(t *testing.T) {
	InitRouter(dbmocks.NewMockDB())
	router := Router()

	tests := []struct {
		repo   api.RepoName
		exists bool
	}{
		{
			repo:   "about",
			exists: false,
		},
		{
			repo:   "pricing",
			exists: false,
		},
		{
			repo:   "foo/bar/baz",
			exists: true,
		},
	}
	for _, tst := range tests {
		t.Run(fmt.Sprintf("%s_%v", tst.repo, tst.exists), func(t *testing.T) {
			mockServeRepo = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}

			// Mock GetByName to return the proper repo not found error type.
			backend.Mocks.Repos.GetByName = func(ctx context.Context, name api.RepoName) (*types.Repo, error) {
				if name != tst.repo {
					panic("unexpected")
				}
				if tst.exists {
					return &types.Repo{Name: name}, nil
				}
				return nil, &errcode.Mock{Message: "repo not found", IsNotFound: true}
			}
			// Perform a request that we expect to redirect to the about subdomain.
			rec := httptest.NewRecorder()
			req := &http.Request{Method: "GET", URL: &url.URL{Path: "/" + string(tst.repo)}}
			router.ServeHTTP(rec, req)
			if !tst.exists {
				// expecting redirect
				if rec.Code != http.StatusTemporaryRedirect {
					t.Fatalf("got code %v want %v", rec.Code, http.StatusTemporaryRedirect)
				}
				wantLoc := "https://sourcegraph.com/" + string(tst.repo)
				if got := rec.Header().Get("Location"); got != wantLoc {
					t.Fatalf("got location %q want location %q", got, wantLoc)
				}
			} else {
				// expecting repo served
				if rec.Code != http.StatusOK {
					t.Fatalf("got code %v want %v", rec.Code, http.StatusOK)
				}
			}
		})
	}
}
