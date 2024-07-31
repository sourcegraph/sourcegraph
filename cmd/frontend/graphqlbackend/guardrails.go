package graphqlbackend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
)

type GuardrailsResolver interface {
	SnippetAttribution(ctx context.Context, args *SnippetAttributionArgs) (SnippetAttributionConnectionResolver, error)
}

type SnippetAttributionArgs struct {
	graphqlutil.ConnectionArgs
	Snippet   string
	TimeoutMs *int32
}

type SnippetAttributionConnectionResolver interface {
	TotalCount() int32
	LimitHit() bool
	SnippetThreshold() AttributionSnippetThresholdResolver
	PageInfo() *graphqlutil.PageInfo
	Nodes() []SnippetAttributionResolver
}

type AttributionSnippetThresholdResolver interface {
	SearchPerformed() bool
	LinesLowerBound() int32
}

type SnippetAttributionResolver interface {
	RepositoryName() string
}
