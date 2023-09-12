package shared

import (
	"net/http"

	"github.com/sourcegraph/sourcegraph/internal/debugserver"
)

func createDebugServerEndpoints(ready chan struct{}, debugserverEndpoints *LazyDebugserverEndpoint) []debugserver.Endpoint {
	return []debugserver.Endpoint{
		{
			Name: "Repo Updater State",
			Path: "/repo-updater-state",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// wait until we're healthy to respond
				<-ready
				// repoUpdaterStateEndpoint is guaranteed to be assigned now
				debugserverEndpoints.repoUpdaterStateEndpoint(w, r)
			}),
		},
		{
			Name: "List Authz Providers",
			Path: "/list-authz-providers",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// wait until we're healthy to respond
				<-ready
				// listAuthzProvidersEndpoint is guaranteed to be assigned now
				debugserverEndpoints.listAuthzProvidersEndpoint(w, r)
			}),
		},
		{
			Name: "Gitserver Repo Status",
			Path: "/gitserver-repo-status",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-ready
				debugserverEndpoints.gitserverReposStatusEndpoint(w, r)
			}),
		},
	}
}
