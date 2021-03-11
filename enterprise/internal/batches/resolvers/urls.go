package resolvers

import (
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
)

func batchChangesApplyURL(n graphqlbackend.Namespace, c graphqlbackend.BatchSpecResolver) string {
	return n.URL() + "/batch-changes/apply/" + string(c.ID())
}

func batchChangeURL(n graphqlbackend.Namespace, c graphqlbackend.BatchChangeResolver) string {
	// This needs to be kept consistent with batches.batchChangeURL().
	return n.URL() + "/batch-changes/" + c.Name()
}
