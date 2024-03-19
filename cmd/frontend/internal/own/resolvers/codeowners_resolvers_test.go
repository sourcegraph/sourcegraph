package resolvers

import (
	"context"
	"testing"

	"github.com/graph-gophers/graphql-go/errors"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/fakedb"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/own"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// userCtx returns a context where the given user ID identifies a logged-in user.
func userCtx(userID int32) context.Context {
	ctx := context.Background()
	a := actor.FromUser(userID)
	return actor.WithActor(ctx, a)
}

type fakeGitserver struct {
	gitserver.Client
}

func TestCodeownersIngestionGuarding(t *testing.T) {
	fs := fakedb.New()
	db := dbmocks.NewMockDB()
	fs.Wire(db)
	git := fakeGitserver{}
	svc := own.NewService(git, db)

	ctx := context.Background()
	adminUser := fs.AddUser(types.User{SiteAdmin: false})

	schema, err := graphqlbackend.NewSchema(db, git, []graphqlbackend.OptionalResolver{{OwnResolver: NewWithService(db, git, svc, logtest.NoOp(t))}})
	if err != nil {
		t.Fatal(err)
	}

	pathToQueries := map[string]string{
		"addCodeownersFile": `
		mutation add {
		  addCodeownersFile(input: {fileContents: "* @admin", repoName: "github.com/sourcegraph/sourcegraph"}) {
			id
		  }
		}`,
		"updateCodeownersFile": `
		mutation update {
		 updateCodeownersFile(input: {fileContents: "* @admin", repoName: "github.com/sourcegraph/sourcegraph"}) {
			id
		 }
		}`,
		"deleteCodeownersFiles": `
		mutation delete {
		 deleteCodeownersFiles(repositories:{repoName: "test"}) {
			alwaysNil
		 }
		}`,
		"codeownersIngestedFiles": `
		query files {
		 codeownersIngestedFiles(first:1) {
			nodes {
				id
			}
		 }
		}`,
	}
	for path, query := range pathToQueries {
		t.Run("dotcom guarding is respected for "+path, func(t *testing.T) {
			dotcom.MockSourcegraphDotComMode(t, true)
			graphqlbackend.RunTest(t, &graphqlbackend.Test{
				Schema:         schema,
				Context:        ctx,
				Query:          query,
				ExpectedResult: nullOrAlwaysNil(t, path),
				ExpectedErrors: []*errors.QueryError{
					{Message: "codeownership ingestion is not available on sourcegraph.com", Path: []any{path}},
				},
			})
		})
		t.Run("site admin guarding is respected for "+path, func(t *testing.T) {
			ctx := userCtx(adminUser)
			graphqlbackend.RunTest(t, &graphqlbackend.Test{
				Schema:         schema,
				Context:        ctx,
				Query:          query,
				ExpectedResult: nullOrAlwaysNil(t, path),
				ExpectedErrors: []*errors.QueryError{
					{Message: auth.ErrMustBeSiteAdmin.Error(), Path: []any{path}},
				},
			})
		})
	}
}

func nullOrAlwaysNil(t *testing.T, endpoint string) string {
	t.Helper()
	expectedResult := `null`
	if endpoint == "deleteCodeownersFiles" {
		expectedResult = `
					{
						"deleteCodeownersFiles": null
					}
				`
	}
	return expectedResult
}
