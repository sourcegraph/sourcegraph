package resolvers

import (
	"context"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/background/repo"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

// TestDBPaginationWithRepoFilter exercises a bug filed in #58313 where
// a unscoped default ordering column from graphqlutil.ConnectionResolver ("id")
// makes into a query joining two tables (both having an id column),
// causing ambiguous SQL.
func TestDBPaginationWithRepoFilter(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	// Make a repo and an embedding job:
	err := db.Repos().Create(ctx, &types.Repo{
		Name: "testrepo",
	})
	require.NoError(t, err)
	r, err := db.Repos().GetByName(ctx, "testrepo")
	require.NoError(t, err)
	jobs := repo.NewRepoEmbeddingJobsStore(db)
	jobID, err := jobs.CreateRepoEmbeddingJob(ctx, r.ID, "commitID")
	require.NoError(t, err)

	// Enable embeddings, so that resolvers work:
	dotcom.MockSourcegraphDotComMode(t, true)

	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			CodyEnabled: pointers.Ptr(true),
			LicenseKey:  "foobar",
			Embeddings: &schema.Embeddings{
				Provider: "sourcegraph",
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })
	require.True(t, conf.EmbeddingsEnabled())

	// Authenticate with a site-admin.
	user, err := db.Users().Create(ctx, database.NewUser{Username: "admin"})
	require.NoError(t, err)
	require.NoError(t, db.Users().SetIsSiteAdmin(ctx, user.ID, true))
	a := actor.FromUser(user.ID)
	ctx = actor.WithActor(ctx, a)

	// Exercise pagination and filtering via graphQL:
	schema, err := graphqlbackend.NewSchema(db, nil, []graphqlbackend.OptionalResolver{{EmbeddingsResolver: NewResolver(db, logger, nil, nil, jobs)}})
	require.NoError(t, err)
	graphqlbackend.RunTest(t, &graphqlbackend.Test{
		Schema:  schema,
		Context: ctx,
		Query: `query RepoEmbeddingJobsList($first: Int, $after: String, $query: String) {
				repoEmbeddingJobs(first: $first, after: $after, query: $query) {
					nodes {
						id
					}
				}
			}`,
		// Want no error:
		ExpectedResult: `{
			"repoEmbeddingJobs": {
				"nodes": []
			}
		}`,
		Variables: map[string]any{
			"first": 1,
			"after": marshalRepoEmbeddingJobID(jobID),
			"query": r.Name,
		},
	})

}
