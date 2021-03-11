package resolvers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
)

func TestCodeHostConnectionResolver(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := backend.WithAuthzBypass(context.Background())
	db := dbtesting.GetDB(t)

	pruneUserCredentials(t, db)

	userID := ct.CreateTestUser(t, db, false).ID

	cstore := store.New(db)

	ghRepos, _ := ct.CreateTestRepos(t, ctx, db, 1)
	ghRepo := ghRepos[0]
	glRepos, _ := ct.CreateGitlabTestRepos(t, ctx, db, 1)
	glRepo := glRepos[0]
	bbsRepos, _ := ct.CreateBbsTestRepos(t, ctx, db, 1)
	bbsRepo := bbsRepos[0]

	cred, err := cstore.UserCredentials().Create(ctx, database.UserCredentialScope{
		Domain:              database.UserCredentialDomainBatches,
		ExternalServiceID:   ghRepo.ExternalRepo.ServiceID,
		ExternalServiceType: ghRepo.ExternalRepo.ServiceType,
		UserID:              userID,
	}, &auth.OAuthBearerToken{Token: "SOSECRET"})
	if err != nil {
		t.Fatal(err)
	}

	s, err := graphqlbackend.NewSchema(db, &Resolver{store: cstore}, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	userAPIID := string(graphqlbackend.MarshalUserID(userID))
	nodes := []apitest.BatchChangesCodeHost{
		{
			ExternalServiceURL:  bbsRepo.ExternalRepo.ServiceID,
			ExternalServiceKind: extsvc.TypeToKind(bbsRepo.ExternalRepo.ServiceType),
		},
		{
			ExternalServiceURL:  ghRepo.ExternalRepo.ServiceID,
			ExternalServiceKind: extsvc.TypeToKind(ghRepo.ExternalRepo.ServiceType),
			Credential: apitest.BatchChangesCredential{
				ID:                  string(marshalBatchChangesCredentialID(cred.ID)),
				ExternalServiceKind: extsvc.TypeToKind(cred.ExternalServiceType),
				ExternalServiceURL:  cred.ExternalServiceID,
				CreatedAt:           cred.CreatedAt.Format(time.RFC3339),
			},
		},
		{
			ExternalServiceURL:  glRepo.ExternalRepo.ServiceID,
			ExternalServiceKind: extsvc.TypeToKind(glRepo.ExternalRepo.ServiceType),
		},
	}

	tests := []struct {
		firstParam      int
		wantHasNextPage bool
		wantEndCursor   string
		wantTotalCount  int
		wantNodes       []apitest.BatchChangesCodeHost
	}{
		{firstParam: 1, wantHasNextPage: true, wantEndCursor: "1", wantTotalCount: 3, wantNodes: nodes[:1]},
		{firstParam: 2, wantHasNextPage: true, wantEndCursor: "2", wantTotalCount: 3, wantNodes: nodes[:2]},
		{firstParam: 3, wantHasNextPage: false, wantTotalCount: 3, wantNodes: nodes[:3]},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("First %d", tc.firstParam), func(t *testing.T) {
			input := map[string]interface{}{"user": userAPIID, "first": int64(tc.firstParam)}
			var response struct{ Node apitest.User }
			apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryCodeHostConnection)

			var wantEndCursor *string
			if tc.wantEndCursor != "" {
				wantEndCursor = &tc.wantEndCursor
			}

			wantChangesets := apitest.BatchChangesCodeHostsConnection{
				TotalCount: tc.wantTotalCount,
				PageInfo: apitest.PageInfo{
					EndCursor:   wantEndCursor,
					HasNextPage: tc.wantHasNextPage,
				},
				Nodes: tc.wantNodes,
			}

			if diff := cmp.Diff(wantChangesets, response.Node.BatchChangesCodeHosts); diff != "" {
				t.Fatalf("wrong changesets response (-want +got):\n%s", diff)
			}
		})
	}

	var endCursor *string
	for i := range nodes {
		input := map[string]interface{}{"user": userAPIID, "first": 1}
		if endCursor != nil {
			input["after"] = *endCursor
		}
		wantHasNextPage := i != len(nodes)-1

		var response struct{ Node apitest.User }
		apitest.MustExec(actor.WithActor(context.Background(), actor.FromUser(userID)), t, s, input, &response, queryCodeHostConnection)

		hosts := response.Node.BatchChangesCodeHosts
		if diff := cmp.Diff(1, len(hosts.Nodes)); diff != "" {
			t.Fatalf("unexpected number of nodes (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(len(nodes), hosts.TotalCount); diff != "" {
			t.Fatalf("unexpected total count (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(wantHasNextPage, hosts.PageInfo.HasNextPage); diff != "" {
			t.Fatalf("unexpected hasNextPage (-want +got):\n%s", diff)
		}

		endCursor = hosts.PageInfo.EndCursor
		if want, have := wantHasNextPage, endCursor != nil; have != want {
			t.Fatalf("unexpected endCursor existence. want=%t, have=%t", want, have)
		}
	}
}

const queryCodeHostConnection = `
query($user: ID!, $first: Int, $after: String){
  node(id: $user) {
    ... on User {
      batchChangesCodeHosts(first: $first, after: $after) {
        totalCount
        nodes {
		  externalServiceKind
		  externalServiceURL
		  credential {
			  id
			  externalServiceKind
			  externalServiceURL
			  createdAt
		  }
        }
        pageInfo {
          endCursor
          hasNextPage
        }
      }
    }
  }
}
`
