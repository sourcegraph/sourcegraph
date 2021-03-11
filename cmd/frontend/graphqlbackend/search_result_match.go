package graphqlbackend

import "github.com/sourcegraph/sourcegraph/internal/search/result"

// A resolver for the GraphQL type GenericSearchMatch
type searchResultMatchResolver struct {
	url        string
	body       string
	highlights []result.HighlightedRange
}

func (m *searchResultMatchResolver) URL() string {
	return m.url
}

func (m *searchResultMatchResolver) Body() Markdown {
	return Markdown(m.body)
}

func (m *searchResultMatchResolver) Highlights() []highlightedRangeResolver {
	res := make([]highlightedRangeResolver, len(m.highlights))
	for i, hl := range m.highlights {
		res[i] = highlightedRangeResolver{hl}
	}
	return res
}
