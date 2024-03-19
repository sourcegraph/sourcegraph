package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	"github.com/graph-gophers/graphql-go/errors"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestSchemaResolver_CodeHosts(t *testing.T) {
	t.Parallel()

	testCodeHosts := []*types.CodeHost{
		newCodeHost(1, "github.com", extsvc.KindGitHub, 1),
		newCodeHost(2, "gitlab.com", extsvc.KindGitLab, 2),
		newCodeHost(3, "bitbucket.server.com", extsvc.KindBitbucketServer, 0),
		newCodeHost(4, "bitbucket-cloud.com", extsvc.KindBitbucketCloud, 4),
	}

	tests := []struct {
		first int
		after int32
	}{
		{
			first: 1,
			after: 0,
		},
		{
			first: 1,
			after: 1,
		},
		{
			first: 1,
			after: 2,
		},
		{
			first: 1,
			after: 3,
		},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("first=%d after=%d", tc.first, tc.after), func(t *testing.T) {
			store := dbmocks.NewMockCodeHostStore()
			store.CountFunc.SetDefaultReturn(4, nil)
			testCodeHost := testCodeHosts[tc.after]

			store.ListFunc.SetDefaultHook(func(ctx context.Context, opts database.ListCodeHostsOpts) ([]*types.CodeHost, int32, error) {
				assert.Equal(t, tc.first, opts.Limit)
				assert.Equal(t, tc.after, opts.Cursor)
				next := tc.after + int32(tc.first)
				if int(next) >= len(testCodeHosts) {
					next = 0
				}

				return testCodeHosts[tc.after : tc.after+int32(tc.first)], next, nil
			})
			users := dbmocks.NewMockUserStore()
			users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

			eSvcs := []*types.ExternalService{
				{ID: 1, DisplayName: "GITLAB #1"},
				{ID: 2, DisplayName: "GITLAB #2"},
			}
			externalServices := dbmocks.NewMockExternalServiceStore()
			externalServices.ListFunc.SetDefaultHook(func(ctx context.Context, options database.ExternalServicesListOptions) ([]*types.ExternalService, error) {
				assert.Equal(t, options.CodeHostID, testCodeHost.ID)
				assert.Equal(t, options.Limit, tc.first)
				assert.Equal(t, options.Offset, 0)
				return eSvcs, nil
			})

			ctx := context.Background()
			db := dbmocks.NewMockDB()
			db.CodeHostsFunc.SetDefaultReturn(store)
			db.UsersFunc.SetDefaultReturn(users)
			db.ExternalServicesFunc.SetDefaultReturn(externalServices)
			variables := map[string]any{
				"first": tc.first,
			}

			gqlAfterID := MarshalCodeHostID(tc.after)
			if tc.after != 0 {
				variables["after"] = gqlAfterID
			}
			var wantEndCursor *string
			wantHasNext := false
			if int(tc.after+1) < len(testCodeHosts) {
				wantEndCursorValue := string(MarshalCodeHostID(tc.after + 1))
				wantEndCursor = &wantEndCursorValue
				wantHasNext = true
			}

			wantResult := codeHostsResult{
				CodeHosts: codeHosts{
					Nodes: []codeHostNode{
						{
							ID:                          string(MarshalCodeHostID(testCodeHost.ID)),
							Kind:                        testCodeHost.Kind,
							URL:                         testCodeHost.URL,
							ApiRateLimitQuota:           testCodeHost.APIRateLimitQuota,
							ApiRateLimitIntervalSeconds: testCodeHost.APIRateLimitIntervalSeconds,
							GitRateLimitQuota:           testCodeHost.GitRateLimitQuota,
							GitRateLimitIntervalSeconds: testCodeHost.GitRateLimitIntervalSeconds,
							ExternalServices: extSvcs{
								Nodes: []extSvcsNode{
									{
										ID:          "RXh0ZXJuYWxTZXJ2aWNlOjE=",
										DisplayName: "GITLAB #1",
									},
									{
										ID:          "RXh0ZXJuYWxTZXJ2aWNlOjI=",
										DisplayName: "GITLAB #2",
									},
								},
							},
						},
					},
					TotalCount: 4,
					PageInfo: pageInfo{
						HasNextPage: wantHasNext,
						EndCursor:   wantEndCursor,
					},
				},
			}
			wantResultResponse, err := json.Marshal(wantResult)
			assert.NoError(t, err)

			RunTest(t, &Test{
				Context:   ctx,
				Schema:    mustParseGraphQLSchema(t, db),
				Variables: variables,
				Query: `query CodeHosts($first: Int, $after: String) {
					codeHosts(first: $first, after: $after) {
						pageInfo {
							endCursor
							hasNextPage
						}
						totalCount
						nodes {
							id
							kind
							url
							apiRateLimitQuota
							apiRateLimitIntervalSeconds
							gitRateLimitQuota
							gitRateLimitIntervalSeconds
							externalServices(first: 1) {
								nodes {
									id
									displayName
								}
							}
						}
					}
				}`,
				ExpectedResult: string(wantResultResponse),
			})

			mockassert.CalledOnce(t, store.CountFunc)
			mockassert.CalledOnce(t, store.ListFunc)
			mockassert.CalledOnce(t, externalServices.ListFunc)
		})
	}
}

func TestCodeHostByID(t *testing.T) {
	tests := []struct {
		siteAdmin      bool
		expectedResult string
		expectedErrors []*errors.QueryError
	}{{
		siteAdmin: true,
		expectedResult: `{
			"node": {
				"id": "Q29kZUhvc3Q6MQ==",
				"__typename": "CodeHost",
				"kind": "GITHUB",
				"url": "github.com"
			}
		}`,
	}, {
		siteAdmin:      false,
		expectedResult: `{"node": null}`,
		expectedErrors: []*errors.QueryError{{
			Path:    []any{string("node")},
			Message: "must be site admin",
		}},
	}}

	codeHost := newCodeHost(1, "github.com", extsvc.KindGitHub, 1)
	store := dbmocks.NewMockCodeHostStore()
	store.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int32) (*types.CodeHost, error) {
		assert.Equal(t, id, codeHost.ID)
		return codeHost, nil
	})

	for _, tc := range tests {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: tc.siteAdmin}, nil)

		ctx := context.Background()
		db := dbmocks.NewMockDB()
		db.CodeHostsFunc.SetDefaultReturn(store)
		db.UsersFunc.SetDefaultReturn(users)

		variables := map[string]any{}

		RunTest(t, &Test{
			Context:        ctx,
			Schema:         mustParseGraphQLSchema(t, db),
			Variables:      variables,
			ExpectedErrors: tc.expectedErrors,
			Query: `query CodeHostByID() {
			node(id: "Q29kZUhvc3Q6MQ==") {
				id
				__typename
				... on CodeHost {
					kind
					url
				}
			}
		}`,
			ExpectedResult: tc.expectedResult,
		})

		mockassert.CalledOnce(t, store.GetByIDFunc)
	}
}

func newCodeHost(id int32, url, kind string, quota int32) *types.CodeHost {
	var q *int32 = nil
	if quota != 0 {
		q = &quota
	}
	return &types.CodeHost{
		ID:                          id,
		URL:                         url,
		Kind:                        kind,
		APIRateLimitQuota:           q,
		APIRateLimitIntervalSeconds: q,
		GitRateLimitQuota:           q,
		GitRateLimitIntervalSeconds: q,
	}
}

type codeHostsResult struct {
	CodeHosts codeHosts `json:"codeHosts"`
}

type codeHosts struct {
	Nodes      []codeHostNode `json:"nodes"`
	TotalCount int            `json:"totalCount"`
	PageInfo   pageInfo       `json:"pageInfo"`
}

type codeHostNode struct {
	ID                          string  `json:"id"`
	Kind                        string  `json:"kind"`
	URL                         string  `json:"url"`
	ApiRateLimitIntervalSeconds *int32  `json:"apiRateLimitIntervalSeconds"`
	ApiRateLimitQuota           *int32  `json:"apiRateLimitQuota"`
	GitRateLimitIntervalSeconds *int32  `json:"gitRateLimitIntervalSeconds"`
	GitRateLimitQuota           *int32  `json:"gitRateLimitQuota"`
	ExternalServices            extSvcs `json:"externalServices"`
}

type extSvcs struct {
	Nodes []extSvcsNode `json:"nodes"`
}

type extSvcsNode struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

type pageInfo struct {
	HasNextPage bool    `json:"hasNextPage"`
	EndCursor   *string `json:"endCursor"`
}
