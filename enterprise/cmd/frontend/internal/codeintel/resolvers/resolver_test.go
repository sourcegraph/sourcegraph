package resolvers

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	gql "github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel/autoindex/enqueuer"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestQueryResolver(t *testing.T) {
	mockDBStore := NewMockDBStore() // returns no dumps
	mockLSIFStore := NewMockLSIFStore()
	mockGitserverClient := NewMockGitserverClient()

	resolver := NewResolver(mockDBStore, mockLSIFStore, mockGitserverClient, nil, nil, &observation.TestContext)
	queryResolver, err := resolver.QueryResolver(context.Background(), &gql.GitBlobLSIFDataArgs{
		Repo:      &types.Repo{ID: 50},
		Commit:    api.CommitID("deadbeef"),
		Path:      "/foo/bar.go",
		ExactPath: true,
		ToolName:  "lsif-go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if queryResolver != nil {
		t.Errorf("expected nil-valued resolver")
	}
}

const expectedFallbackIndexConfiguration = `{
	"shared_steps": [],
	"index_jobs": [
		{
			"steps": [
				{
					"root": "",
					"image": "sourcegraph/lsif-go:latest",
					"commands": [
						"go mod download"
					]
				}
			],
			"local_steps": [],
			"root": "",
			"indexer": "sourcegraph/lsif-go:latest",
			"indexer_args": [
				"lsif-go",
				"--no-animation"
			],
			"outfile": ""
		}
	]
}`

func TestFallbackIndexConfiguration(t *testing.T) {
	mockDBStore := NewMockDBStore() // returns no dumps
	mockEnqueuerDBStore := enqueuer.NewMockDBStore()
	mockLSIFStore := NewMockLSIFStore()
	mockGitserverClient := NewMockGitserverClient()
	gitServerClient := enqueuer.NewMockGitserverClient()
	indexEnqueuer := enqueuer.NewIndexEnqueuer(mockEnqueuerDBStore, gitServerClient, &observation.TestContext)

	mockDBStore.GetIndexConfigurationByRepositoryIDFunc.SetDefaultReturn(dbstore.IndexConfiguration{}, false, nil)
	gitServerClient.HeadFunc.SetDefaultReturn("deadbeef", nil)
	gitServerClient.ListFilesFunc.SetDefaultReturn([]string{"go.mod"}, nil)

	resolver := NewResolver(mockDBStore, mockLSIFStore, mockGitserverClient, indexEnqueuer, nil, &observation.TestContext)
	json, err := resolver.IndexConfiguration(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if diff := cmp.Diff(string(json), expectedFallbackIndexConfiguration); diff != "" {
		t.Fatalf("Unexpected fallback index configuration:\n%s\n", diff)
	}
}
