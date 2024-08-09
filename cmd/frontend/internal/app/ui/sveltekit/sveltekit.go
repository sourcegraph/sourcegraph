package sveltekit

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/grafana/regexp"

	"github.com/gorilla/mux"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui/sveltekit/tags"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/ui/assets"
)

var namedCaptureGroup = regexp.MustCompile(`\(\?P<[^>]+>`)

// JSContext is the metadata sent to the client apps
type JSContext struct {
	// EnabledRoutes is a list of routes for which the SvelteKit app should be rendered.
	// On the client this list is used to determine whether to perform client side navigation
	// or to perform a full page reload.
	EnabledRoutes []int `json:"enabledRoutes"`

	// Whether or not to allow the user to toggle the SvelteKit app on or off.
	ShowToggle bool `json:"showToggle"`

	// KnownRoutes is a list of all routes that are known to the server. The client app
	// needs this list to determine whether a pathname is a repsitory route or not, because
	// the repository route is a catch-all route.
	KnownRoutes []string `json:"knownRoutes"`
}

type contextKey struct{}

type contextValue struct {
	enabledRoutes []int
	knownRoutes   []string
	enabled       bool
}

func (r *svelteKitRoute) isRepoRoot() bool {
	return r.Tag&tags.RepoRoot != 0
}

func (r *svelteKitRoute) matches(url *url.URL) bool {
	return r.Pattern.MatchString(url.Path)
}

// RegisterSvelteKit registers a middleware that determines which routes are enabled for SvelteKit.
// It also extends the request context with information that is sent to the client apps via JSContext.
func RegisterSvelteKit(r *mux.Router, repoRootRoute *mux.Route) {
	isDotComMode := dotcom.SourcegraphDotComMode()
	var knownRoutes []string

	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		regexp, err := route.GetPathRegexp()
		if err != nil {
			return nil
		}

		// Ignore the repository root route, as it is a catch all route and would
		// defeat the purpose of tracking all *other* routes.
		if route == repoRootRoute {
			return nil
		}

		// routes can contain named capture groups which don't work in JavaScript.
		// We replace them with non-capturing groups.
		knownRoutes = append(knownRoutes, namedCaptureGroup.ReplaceAllString(regexp, "(?:"))
		return nil
	})

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			ff := featureflag.FromContext(ctx)
			route := mux.CurrentRoute(req)

			enabledRoutes := make([]int, 0, len(svelteKitRoutes))
			enabled := false

			inclusionMask := tags.EnableAlways
			if ff.GetBoolOr("web-next", false) {
				inclusionMask |= tags.EnableOptIn
			}
			if ff.GetBoolOr("web-next-rollout", false) {
				inclusionMask |= tags.EnableRollout
			}

			var exclusionMask tags.Tag
			if !isDotComMode {
				exclusionMask |= tags.Dotcom
			}

			for i, skr := range svelteKitRoutes {
				if skr.Tag&inclusionMask != 0 && skr.Tag&exclusionMask == 0{
					enabledRoutes = append(enabledRoutes, i)

					if !enabled {
						// The repo root is essentially a catch-all route. If it matches we need
						// to make sure that the server also interprets it as the repo root route.
						if skr.matches(req.URL) && (!skr.isRepoRoot() || route == repoRootRoute) {
							enabled = true
						}
					}
				}
			}

			value := &contextValue{enabledRoutes: enabledRoutes, knownRoutes: knownRoutes, enabled: enabled}
			existingValue := fromContext(ctx)
			if existingValue != nil {
				value.knownRoutes = append(existingValue.knownRoutes, knownRoutes...)
			}
			next.ServeHTTP(w, req.WithContext(context.WithValue(ctx, contextKey{}, value)))
		})
	})
}

func fromContext(ctx context.Context) *contextValue {
	v := ctx.Value(contextKey{})
	if v == nil {
		return nil
	}
	return v.(*contextValue)
}

// Enabled returns true if the route is configured to be supported by useSvelteKit
func Enabled(ctx context.Context) bool {
	skctx := fromContext(ctx)
	if skctx == nil {
		return false
	}
	return skctx.enabled
}

// SvelteKitJSContext is the context object that is passed to the client apps
// to determine which routes are enabled for SvelteKit.
func GetJSContext(ctx context.Context) JSContext {
	skctx := fromContext(ctx)

	if skctx == nil {
		return JSContext{}
	}

	ff := featureflag.FromContext(ctx)

	return JSContext{
		ShowToggle:    ff.GetBoolOr("web-next-toggle", true),
		KnownRoutes:   skctx.knownRoutes,
		EnabledRoutes: skctx.enabledRoutes,
	}
}

func loadSvelteKitInjections() (head, body string, _ error) {
	fileName := "_sk/index.html"
	file, err := assets.Provider.Assets().Open(fileName)
	if err != nil {
		return "", "", errors.Errorf("failed to open %s: %w", fileName, err)
	}
	defer file.Close()

	contents, err := io.ReadAll(file)
	if err != nil {
		return "", "", err
	}

	splits := strings.Split(string(contents), "<!-- SPLIT -->")
	if len(splits) != 3 {
		return "", "", errors.New("failed to parse svelte injections")
	}
	return splits[1], splits[2], nil
}

type loadResult struct {
	head, body string
	err        error
}

var (
	injectOnce sync.Once
	injections loadResult
)

func LoadCachedSvelteKitInjections() (head, body string, err error) {
	if env.InsecureDev {
		return loadSvelteKitInjections()
	}

	injectOnce.Do(func() {
		head, body, err := loadSvelteKitInjections()
		injections = loadResult{head, body, err}
	})
	return injections.head, injections.body, injections.err
}
