package attribution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Service is for the attribution service which searches for matches on
// snippets of code.
type Service interface {
	SnippetAttribution(ctx context.Context, snippet string, limit int, timeout time.Duration) (result *SnippetAttributions, err error)
}

// SnippetAttributions is holds the collection of attributions for a snippet.
type SnippetAttributions struct {
	// RepositoryNames is the list of repository names. We intend on mixing
	// names from both the local instance as well as from sourcegraph.com. So
	// we intentionally use a string since the name may not represent a
	// repository available on this instance.
	//
	// Note: for now this is a simple slice, we likely will expand what is
	// represented here and it will change into a struct capturing more
	// information.
	RepositoryNames []string

	// TotalCount is the total number of repository attributions we found
	// before stopping the search.
	//
	// Note: if we didn't finish searching the full corpus then LimitHit will
	// be true. For filtering use case this means if LimitHit is true you need
	// to be conservative with TotalCount and assume it could be higher.
	TotalCount int

	// LimitHit is true if we stopped searching before looking into the full
	// corpus. If LimitHit is true then it is possible there are more than
	// TotalCount attributions.
	LimitHit bool
}

type Uninitialized struct{}

func (_ Uninitialized) SnippetAttribution(context.Context, string, int, time.Duration) (result *SnippetAttributions, err error) {
	return nil, errors.New("Attribution is not initialized. Please update site config.")
}

// gatewayProxy is a Service that proxies requests to cody gateway.
type gatewayProxy struct {
	client     codygateway.Client
	operations *operations
}

// NewGatewayProxy returns an attribution service that proxies to a gateway request.
//
// Note: this registers metrics so should only be called once with the same
// observationCtx.
func NewGatewayProxy(observationCtx *observation.Context, client codygateway.Client) Service {
	return &gatewayProxy{
		operations: newOperations(observationCtx),
		client:     client,
	}
}

// SnippetAttribution will search the instances indexed code for code matching
// snippet and return the attribution results.
func (c *gatewayProxy) SnippetAttribution(ctx context.Context, snippet string, limit int, timeout time.Duration) (result *SnippetAttributions, err error) {
	ctx, traceLogger, endObservation := c.operations.snippetAttribution.With(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.Int("snippet.len", len(snippet)),
			attribute.Int("limit", limit),
			attribute.Stringer("timeout", timeout),
		},
	})
	defer endObservationWithResult(traceLogger, endObservation, &result)()
	// TODO check if there is another repo that needs updating, or if I can
	// just update this request?
	attribution, err := c.client.Attribution(ctx, snippet, limit)
	if err != nil {
		return nil, err
	}
	return &SnippetAttributions{
		RepositoryNames: attribution.Repositories,
		TotalCount:      len(attribution.Repositories), // TODO: Remove total count.
		LimitHit:        attribution.LimitHit,
	}, nil
}

type localSearch struct {
	client     client.SearchClient
	operations *operations
}

// NewLocalSearch returns an attribution service that searches this instance.
//
// Note: this registers metrics so should only be called once with the same
// observationCtx.
func NewLocalSearch(observationCtx *observation.Context, client client.SearchClient) Service {
	return &localSearch{
		operations: newOperations(observationCtx),
		client:     client,
	}
}

func (c *localSearch) SnippetAttribution(ctx context.Context, snippet string, limit int, timeout time.Duration) (result *SnippetAttributions, err error) {
	ctx, traceLogger, endObservation := c.operations.snippetAttributionLocal.With(ctx, &err, observation.Args{})
	defer endObservationWithResult(traceLogger, endObservation, &result)()

	const (
		version    = "V3"
		searchMode = search.Precise
		protocol   = search.Streaming
	)

	timeoutPart := ""
	if timeout != 0 {
		timeoutPart = fmt.Sprintf("timeout:%s", timeout)
	}

	patternType := "literal"
	searchQuery := fmt.Sprintf("type:file select:repo index:only case:yes %s count:%d content:%q", timeoutPart, limit, snippet)

	inputs, err := c.client.Plan(
		ctx,
		version,
		&patternType,
		searchQuery,
		searchMode,
		protocol,
		pointers.Ptr(int32(0)),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create search plan")
	}

	// TODO(keegancsmith) Reading the SearchClient code it seems to miss out
	// on some of the observability that we instead add in at a later stage.
	// For example the search dataset in honeycomb will be missing. Will have
	// to follow-up with observability and maybe solve it for all users.
	//
	// Note: In our current API we could just store repo names in seen. But it
	// is safer to rely on searches ranking for result stability than doing
	// something like sorting by name from the map.
	var (
		mu        sync.Mutex
		seen      = map[api.RepoID]struct{}{}
		repoNames []string
		limitHit  bool
	)
	_, err = c.client.Execute(ctx, streaming.StreamFunc(func(ev streaming.SearchEvent) {
		mu.Lock()
		defer mu.Unlock()

		limitHit = limitHit || ev.Stats.IsLimitHit

		for _, m := range ev.Results {
			repo := m.RepoName()
			if _, ok := seen[repo.ID]; ok {
				continue
			}
			seen[repo.ID] = struct{}{}
			repoNames = append(repoNames, string(repo.Name))
		}
	}), inputs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute search")
	}

	// Note: Our search API is missing total count internally, but Zoekt does
	// expose this. For now we just count what we found.
	totalCount := len(repoNames)
	if len(repoNames) > limit {
		repoNames = repoNames[:limit]
	}

	return &SnippetAttributions{
		RepositoryNames: repoNames,
		TotalCount:      totalCount,
		LimitHit:        limitHit,
	}, nil
}
