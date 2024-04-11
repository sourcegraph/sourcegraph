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
	}
}
