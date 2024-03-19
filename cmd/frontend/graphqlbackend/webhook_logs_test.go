package graphqlbackend

import (
	"context"
	"testing"
	"time"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/lib/pointers"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestWebhookLogsArgs(t *testing.T) {
	// Create two times for easier reuse in test cases.
	var (
		now       = time.Date(2021, 11, 1, 18, 25, 10, 0, time.UTC)
		later     = now.Add(1 * time.Hour)
		webhookID = marshalWebhookID(123)
	)

	t.Run("success", func(t *testing.T) {
		for name, tc := range map[string]struct {
			id    webhookLogsExternalServiceID
			input WebhookLogsArgs
			want  database.WebhookLogListOpts
		}{
			"no arguments": {
				id:    WebhookLogsAllExternalServices,
				input: WebhookLogsArgs{},
				want: database.WebhookLogListOpts{
					Limit: 50,
				},
			},
			"OnlyErrors false": {
				id: WebhookLogsUnmatchedExternalService,
				input: WebhookLogsArgs{
					OnlyErrors: pointers.Ptr(false),
				},
				want: database.WebhookLogListOpts{
					Limit:             50,
					ExternalServiceID: pointers.Ptr(int64(0)),
					OnlyErrors:        false,
				},
			},
			"all arguments": {
				id: webhookLogsExternalServiceID(1),
				input: WebhookLogsArgs{
					ConnectionArgs: graphqlutil.ConnectionArgs{
						First: pointers.Ptr(int32(25)),
					},
					After:      pointers.Ptr("40"),
					OnlyErrors: pointers.Ptr(true),
					Since:      pointers.Ptr(now),
					Until:      pointers.Ptr(later),
					WebhookID:  pointers.Ptr(webhookID),
				},
				want: database.WebhookLogListOpts{
					Limit:             25,
					Cursor:            40,
					ExternalServiceID: pointers.Ptr(int64(1)),
					OnlyErrors:        true,
					Since:             pointers.Ptr(now),
					Until:             pointers.Ptr(later),
					WebhookID:         pointers.Ptr(int32(123)),
				},
			},
		} {
			t.Run(name, func(t *testing.T) {
				have, err := tc.input.toListOpts(tc.id)
				assert.Nil(t, err)
				assert.Equal(t, tc.want, have)
			})
		}
	})

	t.Run("errors", func(t *testing.T) {
		for _, input := range []string{
			"",
			"-",
			"0.0",
			"foo",
		} {
			t.Run(input, func(t *testing.T) {
				_, err := (&WebhookLogsArgs{After: &input}).toListOpts(WebhookLogsUnmatchedExternalService)
				assert.NotNil(t, err)
			})
		}
	})
}

func TestNewWebhookLogConnectionResolver(t *testing.T) {
	// We'll test everything else below, but let's just make sure the admin
	// check occurs.
	t.Run("unauthenticated user", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(nil, nil)

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		_, err := NewWebhookLogConnectionResolver(context.Background(), db, nil, WebhookLogsUnmatchedExternalService)
		assert.ErrorIs(t, err, auth.ErrNotAuthenticated)
	})

	t.Run("regular user", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{}, nil)

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		_, err := NewWebhookLogConnectionResolver(context.Background(), db, nil, WebhookLogsUnmatchedExternalService)
		assert.ErrorIs(t, err, auth.ErrMustBeSiteAdmin)
	})

	t.Run("admin user", func(t *testing.T) {
		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

		db := dbmocks.NewMockDB()
		db.UsersFunc.SetDefaultReturn(users)

		_, err := NewWebhookLogConnectionResolver(context.Background(), db, nil, WebhookLogsAllExternalServices)
		assert.Nil(t, err)
	})
}

func TestWebhookLogConnectionResolver(t *testing.T) {
	ctx := context.Background()

	// We'll set up a fake page of 20 logs.
	var logs []*types.WebhookLog
	for range 20 {
		logs = append(logs, &types.WebhookLog{})
	}

	createMockStore := func(logs []*types.WebhookLog, next int64, err error) *dbmocks.MockWebhookLogStore {
		store := dbmocks.NewMockWebhookLogStore()
		store.ListFunc.SetDefaultReturn(logs, next, err)
		store.HandleFunc.SetDefaultReturn(nil)

		return store
	}

	t.Run("empty and has no further pages", func(t *testing.T) {
		store := createMockStore([]*types.WebhookLog{}, 0, nil)

		r := &WebhookLogConnectionResolver{
			args: &WebhookLogsArgs{
				ConnectionArgs: graphqlutil.ConnectionArgs{
					First: pointers.Ptr(int32(20)),
				},
			},
			externalServiceID: webhookLogsExternalServiceID(1),
			store:             store,
		}

		nodes, err := r.Nodes(ctx)
		assert.Len(t, nodes, 0)
		assert.Nil(t, err)

		page, err := r.PageInfo(context.Background())
		assert.False(t, page.HasNextPage())
		assert.Nil(t, err)

		mockassert.CalledOnceWith(
			t, store.ListFunc,
			mockassert.Values(
				mockassert.Skip,
				database.WebhookLogListOpts{
					ExternalServiceID: pointers.Ptr(int64(1)),
					Limit:             20,
				},
			),
		)
	})

	t.Run("full and has next page", func(t *testing.T) {
		store := createMockStore(logs, 20, nil)

		r := &WebhookLogConnectionResolver{
			args: &WebhookLogsArgs{
				ConnectionArgs: graphqlutil.ConnectionArgs{
					First: pointers.Ptr(int32(20)),
				},
			},
			externalServiceID: webhookLogsExternalServiceID(1),
			store:             store,
		}

		nodes, err := r.Nodes(ctx)
		for i, node := range nodes {
			assert.Equal(t, logs[i], node.log)
		}
		assert.Nil(t, err)

		page, err := r.PageInfo(context.Background())
		assert.True(t, page.HasNextPage())
		assert.Nil(t, err)

		mockassert.CalledOnceWith(
			t, store.ListFunc,
			mockassert.Values(
				mockassert.Skip,
				database.WebhookLogListOpts{
					ExternalServiceID: pointers.Ptr(int64(1)),
					Limit:             20,
				},
			),
		)
	})

	t.Run("errors", func(t *testing.T) {
		want := errors.New("error")
		store := createMockStore(nil, 0, want)

		r := &WebhookLogConnectionResolver{
			args: &WebhookLogsArgs{
				ConnectionArgs: graphqlutil.ConnectionArgs{
					First: pointers.Ptr(int32(20)),
				},
			},
			externalServiceID: webhookLogsExternalServiceID(1),
			store:             store,
		}

		_, err := r.PageInfo(context.Background())
		assert.ErrorIs(t, err, want)
	})
}

func TestWebhookLogConnectionResolver_TotalCount(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := dbmocks.NewMockWebhookLogStore()
		store.CountFunc.SetDefaultReturn(40, nil)

		r := &WebhookLogConnectionResolver{
			args: &WebhookLogsArgs{
				OnlyErrors: pointers.Ptr(true),
			},
			externalServiceID: webhookLogsExternalServiceID(1),
			store:             store,
		}

		have, err := r.TotalCount(context.Background())
		assert.EqualValues(t, 40, have)
		assert.Nil(t, err)

		mockassert.CalledOnceWith(
			t, store.CountFunc,
			mockassert.Values(
				mockassert.Skip,
				database.WebhookLogListOpts{
					ExternalServiceID: pointers.Ptr(int64(1)),
					Limit:             50,
					OnlyErrors:        true,
				},
			),
		)
	})

	t.Run("errors", func(t *testing.T) {
		want := errors.New("error")
		store := dbmocks.NewMockWebhookLogStore()
		store.CountFunc.SetDefaultReturn(0, want)

		r := &WebhookLogConnectionResolver{
			args: &WebhookLogsArgs{
				OnlyErrors: pointers.Ptr(true),
			},
			externalServiceID: webhookLogsExternalServiceID(1),
			store:             store,
		}

		_, err := r.TotalCount(context.Background())
		assert.ErrorIs(t, err, want)
	})
}

func TestListWebhookLogs(t *testing.T) {
	users := dbmocks.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})
	webhookLogsStore := dbmocks.NewMockWebhookLogStore()
	webhookLogs := []*types.WebhookLog{
		{ID: 1, WebhookID: pointers.Ptr(int32(1)), StatusCode: 200},
		{ID: 2, WebhookID: pointers.Ptr(int32(1)), StatusCode: 500},
		{ID: 3, WebhookID: pointers.Ptr(int32(1)), StatusCode: 200},
		{ID: 4, WebhookID: pointers.Ptr(int32(2)), StatusCode: 200},
		{ID: 5, WebhookID: pointers.Ptr(int32(2)), StatusCode: 200},
		{ID: 6, WebhookID: pointers.Ptr(int32(2)), StatusCode: 200},
		{ID: 7, WebhookID: pointers.Ptr(int32(3)), StatusCode: 500},
		{ID: 8, WebhookID: pointers.Ptr(int32(3)), StatusCode: 500},
	}
	webhookLogsStore.ListFunc.SetDefaultHook(func(_ context.Context, options database.WebhookLogListOpts) ([]*types.WebhookLog, int64, error) {
		var logs []*types.WebhookLog
		logs = append(logs, webhookLogs...)

		filter := func(items []*types.WebhookLog, predicate func(log *types.WebhookLog) bool) []*types.WebhookLog {
			var filtered []*types.WebhookLog
			for _, wl := range items {
				if predicate(wl) {
					filtered = append(filtered, wl)
				}
			}
			return filtered
		}

		if options.WebhookID != nil {
			logs = filter(
				logs,
				func(wl *types.WebhookLog) bool {
					return *wl.WebhookID == *options.WebhookID
				},
			)
		}

		if options.OnlyErrors {
			logs = filter(
				logs,
				func(wl *types.WebhookLog) bool {
					return wl.StatusCode < 100 || wl.StatusCode > 399
				},
			)
		}

		return logs, int64(len(logs)), nil
	})

	webhookLogsStore.CountFunc.SetDefaultHook(func(ctx context.Context, opts database.WebhookLogListOpts) (int64, error) {
		logs, _, err := webhookLogsStore.List(ctx, opts)
		return int64(len(logs)), err
	})

	db := dbmocks.NewMockDB()
	db.WebhookLogsFunc.SetDefaultReturn(webhookLogsStore)
	db.UsersFunc.SetDefaultReturn(users)
	schema := mustParseGraphQLSchema(t, db)
	RunTests(t, []*Test{
		{
			Label:   "only errors",
			Context: ctx,
			Schema:  schema,
			Query: `query WebhookLogs($onlyErrors: Boolean!) {
						webhookLogs(onlyErrors: $onlyErrors) {
							nodes { id }
							totalCount
						}
					}
			`,
			Variables: map[string]any{
				"onlyErrors": true,
			},
			ExpectedResult: `{"webhookLogs":
				{
					"nodes":[
						{"id":"V2ViaG9va0xvZzoy"},
						{"id":"V2ViaG9va0xvZzo3"},
						{"id":"V2ViaG9va0xvZzo4"}
					],
					"totalCount":3
				}}`,
		},
		{
			Label:   "specific webhook ID",
			Context: ctx,
			Schema:  schema,
			Query: `query WebhookLogs($webhookID: ID!) {
						webhookLogs(webhookID: $webhookID) {
							nodes { id }
							totalCount
						}
					}
			`,
			Variables: map[string]any{
				"webhookID": "V2ViaG9vazox",
			},
			ExpectedResult: `{"webhookLogs":
				{
					"nodes":[
						{"id":"V2ViaG9va0xvZzox"},
						{"id":"V2ViaG9va0xvZzoy"},
						{"id":"V2ViaG9va0xvZzoz"}
					],
					"totalCount":3
				}}`,
		},
		{
			Label:   "only errors for a specific webhook ID",
			Context: ctx,
			Schema:  schema,
			Query: `query WebhookLogs($webhookID: ID!, $onlyErrors: Boolean!) {
						webhookLogs(webhookID: $webhookID, onlyErrors: $onlyErrors) {
							nodes { id }
							totalCount
						}
					}
			`,
			Variables: map[string]any{
				"webhookID":  "V2ViaG9vazox",
				"onlyErrors": true,
			},
			ExpectedResult: `{"webhookLogs":
				{
					"nodes":[
						{"id":"V2ViaG9va0xvZzoy"}
					],
					"totalCount":1
				}}`,
		},
	})
}
