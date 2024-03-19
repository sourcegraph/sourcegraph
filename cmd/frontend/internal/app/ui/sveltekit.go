package ui

import (
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/ui/assets"
)

// Keep in sync with 'client/web-sveltekit/src/lib/navigation.ts' and 'client/web/src/sveltekite/util.ts'
var sveltekitEnabledRoutes = map[string]struct{}{
	routeSearch:       {},
	routeTree:         {},
	routeBlob:         {},
	routeRepo:         {},
	routeRepoCommit:   {},
	routeRepoBranches: {},
	routeRepoCommits:  {},
	routeRepoTags:     {},
	routeRepoStats:    {},
}

// rolledOutRoutes is a set of routes that are enabled via a different feature flag.
// This allows us to have a two-stage rollout of SvelteKit, where we can enable it for
// a subset of routes before enabling it for all routes.
// Should be a subset of sveltekitEnabledRoutes.
// Keep in sync with 'client/web-sveltekit/src/lib/navigation.ts' and 'client/web/src/sveltekite/util.ts'
var rolledOutRoutes = map[string]struct{}{
	routeSearch: {},
}

// useSvelteKit returns true if the route is configured to be supported by useSvelteKit
// (see svelteKitEnabledRoutes) and the 'web-next' or 'web-next-rollout' feature flags are set
func useSvelteKit(r *http.Request) bool {
	route := mux.CurrentRoute(r)
	if route == nil {
		return false
	}
	routeName := route.GetName()

	if _, ok := sveltekitEnabledRoutes[routeName]; !ok {
		return false
	}

	ff := featureflag.FromContext(r.Context())

	if ff.GetBoolOr("web-next", false) {
		return true
	}

	if _, ok := rolledOutRoutes[routeName]; ok {
		return ff.GetBoolOr("web-next-rollout", false)
	}

	return false
}

// renderSvelteKit writes SvelteKit's fallback page to the provided writer
func renderSvelteKit(dst io.Writer) error {
	file, err := assets.Provider.Assets().Open("_sk/index.html")
	if err != nil {
		return err
	}
	defer file.Close()
	io.Copy(dst, file)
	return nil
}
