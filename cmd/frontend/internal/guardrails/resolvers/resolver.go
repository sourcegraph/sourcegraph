package resolvers

import (
	"context"
	"sync"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/guardrails/attribution"
	"github.com/sourcegraph/sourcegraph/internal/guardrails"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

var _ graphqlbackend.GuardrailsResolver = &GuardrailsResolver{}

type GuardrailsResolver struct {
	mu                 sync.Mutex
	attributionService attribution.Service
}

func NewGuardrailsResolver(s attribution.Service) *GuardrailsResolver {
	return &GuardrailsResolver{
		attributionService: s,
	}
}

func (c *GuardrailsResolver) UpdateService(s attribution.Service) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attributionService = s
}

func (c *GuardrailsResolver) service() attribution.Service {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.attributionService
}

func (c *GuardrailsResolver) SnippetAttribution(ctx context.Context, args *graphqlbackend.SnippetAttributionArgs) (graphqlbackend.SnippetAttributionConnectionResolver, error) {
	limit := 5
	if args.First != nil {
		limit = int(*args.First)
	}
	if !guardrails.NewThreshold().ShouldSearch(args.Snippet) {
		// Below search threshold, no search is performed and nil result is rendered.
		// snippetThreshold.searchPerformed field within the resolver indicates this case.
		return snippetAttributionConnectionResolver{}, nil
	}
	timeout := time.Duration(pointers.DerefZero(args.TimeoutMs)) * time.Millisecond
	result, err := c.service().SnippetAttribution(ctx, args.Snippet, limit, timeout)
	if err != nil {
		return nil, err
	}
	return snippetAttributionConnectionResolver{
		result: result,
	}, nil
}

type snippetAttributionConnectionResolver struct {
	// result is nil if snippet was below search threshold and search did not run.
	result *attribution.SnippetAttributions
}

func (c snippetAttributionConnectionResolver) TotalCount() int32 {
	if c.result == nil {
		return 0
	}
	return int32(c.result.TotalCount)
}
func (c snippetAttributionConnectionResolver) LimitHit() bool {
	if c.result == nil {
		return false
	}
	return c.result.LimitHit
}
func (c snippetAttributionConnectionResolver) PageInfo() *graphqlutil.PageInfo {
	return graphqlutil.HasNextPage(false)
}
func (c snippetAttributionConnectionResolver) SnippetThreshold() graphqlbackend.AttributionSnippetThresholdResolver {
	return &attributionSnippetThresholdResolver{
		searchPerformed: c.result != nil,
	}
}
func (c snippetAttributionConnectionResolver) Nodes() []graphqlbackend.SnippetAttributionResolver {
	if c.result == nil {
		return nil
	}
	var nodes []graphqlbackend.SnippetAttributionResolver
	for _, name := range c.result.RepositoryNames {
		nodes = append(nodes, snippetAttributionResolver(name))
	}
	return nodes
}

type snippetAttributionResolver string

func (c snippetAttributionResolver) RepositoryName() string {
	return string(c)
}

type attributionSnippetThresholdResolver struct {
	searchPerformed bool
}

func (t attributionSnippetThresholdResolver) SearchPerformed() bool {
	return t.searchPerformed
}
func (t attributionSnippetThresholdResolver) LinesLowerBound() int32 {
	return int32(guardrails.NewThreshold().LinesLowerBound())
}
