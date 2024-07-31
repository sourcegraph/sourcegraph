package attribution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type fakeGateway struct {
	codygateway.Client
}

func (f fakeGateway) Attribution(ctx context.Context, snippet string, limit int) (codygateway.Attribution, error) {
	return codygateway.Attribution{
		Repositories: []string{"repo1", "repo2"},
		LimitHit:     false,
	}, nil
}

func TestProxySuccess(t *testing.T) {
	gateway := fakeGateway{}
	service := NewGatewayProxy(observation.TestContextTB(t), gateway)
	attribution, err := service.SnippetAttribution(context.Background(), "snippet", 3, 0)
	require.NoError(t, err)
	require.Equal(t, &SnippetAttributions{
		RepositoryNames: []string{"repo1", "repo2"},
		TotalCount:      2,
		LimitHit:        false,
	}, attribution)
}

type fakeSearch struct {
	client.SearchClient
	repos []string
}

func (s *fakeSearch) Plan(
	ctx context.Context,
	version string,
	patternType *string,
	searchQuery string,
	searchMode search.Mode,
	protocol search.Protocol,
	contextLines *int32,
) (*search.Inputs, error) {
	return &search.Inputs{}, nil
}

func (s *fakeSearch) Execute(
	ctx context.Context,
	stream streaming.Sender,
	inputs *search.Inputs,
) (_ *search.Alert, err error) {
	for i, repo := range s.repos {
		stream.Send(streaming.SearchEvent{
			Results: result.Matches{
				&result.FileMatch{
					File: result.File{
						Repo: types.MinimalRepo{
							// Repos are deduplicated in search results by ID.
							ID:   api.RepoID(i),
							Name: api.RepoName(repo),
						},
					},
				},
			},
		})
	}
	return nil, nil
}

func TestSearchSuccess(t *testing.T) {
	fs := &fakeSearch{
		repos: []string{"foo1", "bar2"},
	}
	service := NewLocalSearch(observation.TestContextTB(t), fs)
	attribution, err := service.SnippetAttribution(context.Background(), "snippet", 3, 0)
	require.NoError(t, err)
	require.Equal(t, &SnippetAttributions{
		RepositoryNames: []string{"foo1", "bar2"},
		TotalCount:      2,
		LimitHit:        false,
	}, attribution)
}
