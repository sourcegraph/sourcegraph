package graphqlbackend

import (
	"context"
	"fmt"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/pkg/search/query/syntax"
	"sort"
)

type didYouMeanQuotedResolver struct {
	query string
	err   error
}

func (r *didYouMeanQuotedResolver) Results(context.Context) (*searchResultsResolver, error) {
	q := syntax.ParseAllowingErrors(r.query)
	// Make a map from various quotings of the query to their descriptions.
	// This should take care of deduplicating them.
	qq2d := make(map[string]string)
	qq2d[q.WithNonFieldPartsQuoted().String()] = "query with parts quoted, except for fields"
	qq2d[q.WithNonFieldsQuoted().String()] = "query quoted, except for fields"
	qq2d[q.WithPartsQuoted().String()] = "query with parts quoted"
	qq2d[fmt.Sprintf("%q", r.query)] = "query quoted entirely"
	var sqds []*searchQueryDescription
	for qq, desc := range qq2d {
		sqds = append(sqds, &searchQueryDescription{
			description: desc,
			query:       qq,
		})
	}
	sort.Slice(sqds, func(i, j int) bool { return sqds[i].description < sqds[j].description })
	srr := &searchResultsResolver{
		alert: &searchAlert{
			title:           "Try quoted",
			description:     r.err.Error(),
			proposedQueries: sqds,
		},
	}
	return srr, nil
}

func (r *didYouMeanQuotedResolver) Suggestions(context.Context, *searchSuggestionsArgs) ([]*searchSuggestionResolver, error) {
	return nil, nil
}

func (r *didYouMeanQuotedResolver) Stats(context.Context) (*searchResultsStats, error) {
	srs := &searchResultsStats{}
	return srs, nil
}

