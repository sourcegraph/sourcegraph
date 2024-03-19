package graphqlbackend

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestSiteConfigurationDiff(t *testing.T) {
	stubs := setupSiteConfigStubs(t)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: stubs.users[0].ID})
	schemaResolver, err := newSchemaResolver(stubs.db, gitserver.NewTestClient(t)).Site().Configuration(ctx, &SiteConfigurationArgs{})
	if err != nil {
		t.Fatalf("failed to create schemaResolver: %v", err)
	}

	expectedDiffs := stubs.expectedDiffs

	expectedNodes := []struct {
		ID           int32
		AuthorUserID int32
		Diff         string
	}{
		{
			ID:           6,
			AuthorUserID: 3,
			Diff:         expectedDiffs[6],
		},
		{
			ID:           4,
			AuthorUserID: 1,
			Diff:         expectedDiffs[4],
		},
		{
			ID:           3,
			AuthorUserID: 2,
			Diff:         expectedDiffs[3],
		},
		{
			ID:           2,
			AuthorUserID: 0,
			Diff:         expectedDiffs[2],
		},
		{
			ID:           1,
			AuthorUserID: 0,
			Diff:         expectedDiffs[1],
		},
	}

	testCases := []struct {
		name string
		args *graphqlutil.ConnectionResolverArgs
	}{
		// We have tests for pagination so we can skip that here and just check for the diff for all
		// the nodes in both the directions.
		{
			name: "first: 10",
			args: &graphqlutil.ConnectionResolverArgs{First: pointers.Ptr(int32(10))},
		},
		{
			name: "last: 10",
			args: &graphqlutil.ConnectionResolverArgs{Last: pointers.Ptr(int32(10))},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			connectionResolver, err := schemaResolver.History(ctx, tc.args)
			if err != nil {
				t.Fatalf("failed to get history: %v", err)
			}

			nodes, err := connectionResolver.Nodes(ctx)
			if err != nil {
				t.Fatalf("failed to get nodes: %v", err)
			}

			totalNodes, totalExpectedNodes := len(nodes), len(expectedNodes)
			if totalNodes != totalExpectedNodes {
				t.Fatalf("mismatched number of nodes, expected %d, got: %d", totalExpectedNodes, totalNodes)
			}

			for i := range totalNodes {
				siteConfig, expectedNode := nodes[i].siteConfig, expectedNodes[i]

				if siteConfig.ID != expectedNode.ID {
					t.Errorf("mismatched node ID, expected: %d, but got: %d", siteConfig.ID, expectedNode.ID)
				}

				if siteConfig.AuthorUserID != expectedNode.AuthorUserID {
					t.Errorf("mismatched node AuthorUserID, expected: %d, but got: %d", siteConfig.ID, expectedNode.ID)
				}

				if diff := cmp.Diff(expectedNode.Diff, nodes[i].Diff()); diff != "" {
					t.Errorf("mismatched node diff (-want, +got):\n%s ", diff)
				}

				author, err := nodes[i].Author(ctx)
				if err != nil {
					t.Fatalf("failed to get author: %v", err)
				}
				if siteConfig.AuthorUserID == 3 || siteConfig.AuthorUserID == 0 { // User with ID 3 is not created in the test setup
					if author != nil {
						t.Fatalf("expected author to be nil for user ID %d", siteConfig.AuthorUserID)
					}
				} else {
					if author == nil {
						t.Fatalf("expected non-nil author resolver for user ID %d", siteConfig.AuthorUserID)
					}
				}
			}
		})
	}
}
