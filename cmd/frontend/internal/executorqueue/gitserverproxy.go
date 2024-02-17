package executorqueue

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"runtime"

	"github.com/gorilla/mux"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

type GitserverClient interface {
	// AddrForRepo returns the gitserver address to use for the given repo name.
	AddrForRepo(context.Context, api.RepoName) string
}

// gitserverProxy creates an HTTP handler that will proxy requests to the correct
// gitserver at the given gitPath.
func gitserverProxy(logger log.Logger, gitserverClient GitserverClient, gitPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := getRepoName(r)

		addrForRepo := gitserverClient.AddrForRepo(r.Context(), api.RepoName(repo))

		cli, err := httpcli.NewInternalClientFactory("gitserverproxy").Client()
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create http client: %s", err.Error()), http.StatusInternalServerError)
			return
		}

		p := httputil.ReverseProxy{
			Director: func(r *http.Request) {
				u := &url.URL{
					Scheme:   "http",
					Host:     addrForRepo,
					Path:     path.Join("/git", repo, gitPath),
					RawQuery: r.URL.RawQuery,
				}
				r.URL = u
			},
			Transport: cli.Transport,
		}
		defer func() {
			e := recover()
			if e != nil {
				if e == http.ErrAbortHandler {
					logger.Warn("failed to read gitserver response")
				} else {
					const size = 64 << 10
					buf := make([]byte, size)
					buf = buf[:runtime.Stack(buf, false)]
					logger.Error("reverseproxy: panic reading response", log.String("stack", string(buf)))
				}
			}
		}()
		p.ServeHTTP(w, r)
	})
}

// getRepoName returns the "RepoName" segment of the request's URL. This is a function variable so
// we can swap it out easily during testing. The gorilla/mux does have a testing function to
// set variables on a request context, but the context gets lost somewhere between construction
// of the request and the default client's handling of the request.
var getRepoName = func(r *http.Request) string {
	return mux.Vars(r)["RepoName"]
}
