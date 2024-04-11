package shared

import (
	"net/http"

	"github.com/sourcegraph/sourcegraph/internal/debugserver"
)

func createDebugServerEndpoints(ready chan struct{}, addr string, debugserverEndpoints *LazyDebugserverEndpoint) []debugserver.Endpoint {
	return []debugserver.Endpoint{
		debugserver.NewGRPCWebUIEndpoint("gitserverproxy", addr),
		{
			Name: "Repository Locker State",
			Path: "/repository-locker-state",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// wait until we're healthy to respond
				<-ready
				// lockerStatusEndpoint is guaranteed to be assigned now
				debugserverEndpoints.lockerStatusEndpoint(w, r)
			}),
		},
	}
}
