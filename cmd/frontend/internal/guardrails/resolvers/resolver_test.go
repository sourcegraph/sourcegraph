package resolvers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/guardrails/attribution"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
)

type fakeAttributionService struct {
	searchResult []string
	err          error
}

func (s *fakeAttributionService) SnippetAttribution(ctx context.Context, snippet string, limit int) (result *attribution.SnippetAttributions, err error) {
	return &attribution.SnippetAttributions{
		RepositoryNames: s.searchResult,
		TotalCount:      len(s.searchResult),
		LimitHit:        len(s.searchResult) == limit,
	}, s.err
}

func TestSuccessfulAttribution(t *testing.T) {
	db := dbmocks.NewMockDB()
	attributionService := &fakeAttributionService{}
	schema, err := graphqlbackend.NewSchema(db, nil, []graphqlbackend.OptionalResolver{
		{GuardrailsResolver: NewGuardrailsResolver(attributionService)}})
	require.NoError(t, err)
	t.Run("search performed", func(t *testing.T) {
		attributionService.searchResult = nil
		graphqlbackend.RunTest(t, &graphqlbackend.Test{
			Schema:  schema,
			Context: context.Background(),
			Query: `
				query SnippetAttribution($snippet: String!) {
					snippetAttribution(snippet: $snippet) {
						limitHit
						nodes {
							repositoryName
						}
						snippetThreshold {
							searchPerformed
							linesLowerBound
						}
					}
				}`,
			ExpectedResult: `{
				"snippetAttribution": {
					"limitHit": false,
					"nodes": [],
					"snippetThreshold": {
						"searchPerformed": true,
						"linesLowerBound": 10
					}
				}
			}`,
			Variables: map[string]any{
				"snippet": `1st line
				2nd line
				3rd line
				4th line
				5th line
				6th line
				7th line
				8th line
				9th line
				10th line`,
			},
		})
	})

	t.Run("below search lower bound", func(t *testing.T) {
		// even if there would have been search results for short snippet.
		attributionService.searchResult = []string{"repo1", "repo2"}
		graphqlbackend.RunTest(t, &graphqlbackend.Test{
			Schema:  schema,
			Context: context.Background(),
			Query: `
				query SnippetAttribution($snippet: String!) {
					snippetAttribution(snippet: $snippet) {
						limitHit
						nodes {
							repositoryName
						}
						snippetThreshold {
							searchPerformed
							linesLowerBound
						}
					}
				}`,
			ExpectedResult: `{
				"snippetAttribution": {
					"limitHit": false,
					"nodes": [],
					"snippetThreshold": {
						"searchPerformed": false,
						"linesLowerBound": 10
					}
				}
			}`,
			Variables: map[string]any{
				"snippet": `1st line
				2nd line
				3rd line
				4th line
				5th line
				6th line
				7th line`,
			},
		})
	})

	t.Run("search bounds are zero on dotcom", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		// even if there would have been search results for short snippet.
		attributionService.searchResult = []string{"repo1", "repo2"}
		graphqlbackend.RunTest(t, &graphqlbackend.Test{
			Schema:  schema,
			Context: context.Background(),
			Query: `
				query SnippetAttribution($snippet: String!) {
					snippetAttribution(snippet: $snippet) {
						limitHit
						nodes {
							repositoryName
						}
						snippetThreshold {
							searchPerformed
							linesLowerBound
						}
					}
				}`,
			ExpectedResult: `{
				"snippetAttribution": {
					"limitHit": false,
					"nodes": [
						{"repositoryName": "repo1"},
						{"repositoryName": "repo2"}
					],
					"snippetThreshold": {
						"searchPerformed": true,
						"linesLowerBound": 0
					}
				}
			}`,
			Variables: map[string]any{
				"snippet": `1st line
				2nd line
				3rd line
				4th line
				5th line
				6th line
				7th line`,
			},
		})
	})
}
