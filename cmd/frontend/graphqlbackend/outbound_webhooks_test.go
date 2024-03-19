package graphqlbackend

import (
	"context"
	"sync/atomic"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/webhooks/outbound"
)

func TestSchemaResolver_OutboundWebhooks(t *testing.T) {
	t.Parallel()

	t.Run("not site admin", func(t *testing.T) {
		t.Parallel()

		db := dbmocks.NewMockDB()
		ctx, _, _ := fakeUser(t, context.Background(), db, false)

		runMustBeSiteAdminTest(t, []any{"outboundWebhooks"}, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				{
					outboundWebhooks {
						nodes {
							url
						}
						totalCount
						pageInfo {
							hasNextPage
						}
					}
				}
			`,
		})
	})

	t.Run("site admin", func(t *testing.T) {
		t.Parallel()

		for name, tc := range map[string]struct {
			params    string
			first     int32
			after     int32
			eventType string
		}{
			"first only": {
				params: `first: 2`,
				first:  2,
			},
			"first and after": {
				params: `first: 2, after: "1"`,
				first:  2,
				after:  1,
			},
			"event type": {
				params:    `first:2, eventType: "foo"`,
				first:     2,
				eventType: "foo",
			},
		} {
			t.Run(name, func(t *testing.T) {
				assertEventTypes := func(t *testing.T, have []database.FilterEventType) {
					t.Helper()

					if tc.eventType == "" {
						assert.Empty(t, have)
					} else {
						assert.Equal(t, []database.FilterEventType{{EventType: tc.eventType}}, have)
					}
				}

				store := dbmocks.NewMockOutboundWebhookStore()
				store.CountFunc.SetDefaultHook(func(ctx context.Context, opts database.OutboundWebhookCountOpts) (int64, error) {
					assertEventTypes(t, opts.EventTypes)
					return 4, nil
				})
				store.ListFunc.SetDefaultHook(func(ctx context.Context, opts database.OutboundWebhookListOpts) ([]*types.OutboundWebhook, error) {
					// The limit is +1 because the resolver adds an extra item
					// for pagination purposes.
					assert.EqualValues(t, tc.first+1, opts.Limit)
					assert.EqualValues(t, tc.after, opts.Offset)
					assertEventTypes(t, opts.EventTypes)

					return []*types.OutboundWebhook{
						{URL: encryption.NewUnencrypted("http://example.com/1")},
						{URL: encryption.NewUnencrypted("http://example.com/2")},
						{URL: encryption.NewUnencrypted("http://example.com/3")},
					}, nil
				})

				db := dbmocks.NewMockDB()
				db.OutboundWebhooksFunc.SetDefaultReturn(store)
				ctx, _, _ := fakeUser(t, context.Background(), db, true)

				RunTest(t, &Test{
					Context: ctx,
					Schema:  mustParseGraphQLSchema(t, db),
					Query: `
						{
							outboundWebhooks(` + tc.params + `) {
								nodes {
									url
								}
								totalCount
								pageInfo {
									hasNextPage
								}
							}
						}
					`,
					ExpectedResult: `
						{
							"outboundWebhooks": {
								"nodes": [
									{
										"url": "http://example.com/1"
									},
									{
										"url": "http://example.com/2"
									}
								],
								"totalCount": 4,
								"pageInfo": {
									"hasNextPage": true
								}
							}
						}
					`,
				})

				mockassert.CalledOnce(t, store.CountFunc)
				mockassert.CalledOnce(t, store.ListFunc)
			})
		}
	})
}

func TestSchemaResolver_OutboundWebhookEventTypes(t *testing.T) {
	t.Run("not site admin", func(t *testing.T) {
		db := dbmocks.NewMockDB()
		ctx, _, _ := fakeUser(t, context.Background(), db, false)

		runMustBeSiteAdminTest(t, []any{"outboundWebhookEventTypes"}, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				{
					outboundWebhookEventTypes {
						key
						description
					}
				}
			`,
		})
	})

	t.Run("site admin", func(t *testing.T) {
		for name, tc := range map[string]struct {
			eventTypes []outbound.EventType
			want       string
		}{
			"empty": {
				eventTypes: []outbound.EventType{},
				want:       `{"outboundWebhookEventTypes":[]}`,
			},
			"not empty": {
				eventTypes: []outbound.EventType{
					{Key: "test:a", Description: "a test"},
					{Key: "test:b", Description: "b test"},
				},
				want: `
					{
						"outboundWebhookEventTypes": [
							{
								"key": "test:a",
								"description": "a test"
							},
							{
								"key": "test:b",
								"description": "b test"
							}
						]
					}
				`,
			},
		} {
			t.Run(name, func(t *testing.T) {
				outbound.MockGetRegisteredEventTypes = func() []outbound.EventType {
					return tc.eventTypes
				}
				t.Cleanup(func() {
					outbound.MockGetRegisteredEventTypes = nil
				})

				db := dbmocks.NewMockDB()
				ctx, _, _ := fakeUser(t, context.Background(), db, true)

				RunTest(t, &Test{
					Context: ctx,
					Schema:  mustParseGraphQLSchema(t, db),
					Query: `
						{
							outboundWebhookEventTypes {
								key
								description
							}
						}
					`,
					ExpectedResult: tc.want,
				})
			})
		}
	})
}

func TestSchemaResolver_CreateOutboundWebhook(t *testing.T) {
	t.Parallel()

	var (
		url       = "http://example.com"
		secret    = "super secret"
		eventType = "test:event"
		inputVars = map[string]any{
			"input": map[string]any{
				"url":    url,
				"secret": secret,
				"eventTypes": []any{
					map[string]any{"eventType": eventType},
				},
			},
		}
	)

	t.Run("not site admin", func(t *testing.T) {
		t.Parallel()

		db := dbmocks.NewMockDB()
		ctx, _, _ := fakeUser(t, context.Background(), db, false)

		runMustBeSiteAdminTest(t, []any{"createOutboundWebhook"}, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation CreateOutboundWebhook($input: OutboundWebhookCreateInput!) {
					createOutboundWebhook(input: $input) {
						id
					}
				}
			`,
			Variables: inputVars,
		})
	})

	t.Run("site admin", func(t *testing.T) {
		t.Parallel()

		store := dbmocks.NewMockOutboundWebhookStore()
		store.CreateFunc.SetDefaultHook(func(ctx context.Context, webhook *types.OutboundWebhook) error {
			valueOf := func(e *encryption.Encryptable) string {
				t.Helper()

				value, err := e.Decrypt(ctx)
				require.NoError(t, err)

				return value
			}

			assert.Equal(t, url, valueOf(webhook.URL))
			assert.Equal(t, secret, valueOf(webhook.Secret))
			assert.Equal(t, []types.OutboundWebhookEventType{
				{EventType: eventType},
			}, webhook.EventTypes)

			webhook.ID = 1
			return nil
		})

		db := dbmocks.NewMockDB()
		db.OutboundWebhooksFunc.SetDefaultReturn(store)
		ctx, _, _ := fakeUser(t, context.Background(), db, true)

		RunTest(t, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation CreateOutboundWebhook($input: OutboundWebhookCreateInput!) {
					createOutboundWebhook(input: $input) {
						id
						url
						eventTypes {
							eventType
						}
					}
				}
			`,
			Variables: inputVars,
			ExpectedResult: `
				{
					"createOutboundWebhook": {
						"id": "T3V0Ym91bmRXZWJob29rOjE=",
						"url": "` + url + `",
						"eventTypes": [
							{
								"eventType": "` + eventType + `"
							}
						]
					}
				}
			`,
		})

		mockassert.CalledOnce(t, store.CreateFunc)
	})
}

func TestSchemaResolver_DeleteOutboundWebhook(t *testing.T) {
	t.Parallel()

	// Outbound webhook ID 1.
	id := "T3V0Ym91bmRXZWJob29rOjE="

	t.Run("not site admin", func(t *testing.T) {
		t.Parallel()

		db := dbmocks.NewMockDB()
		ctx, _, _ := fakeUser(t, context.Background(), db, false)

		runMustBeSiteAdminTest(t, []any{"deleteOutboundWebhook"}, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation DeleteOutboundWebhook($id: ID!) {
					deleteOutboundWebhook(id: $id) {
						alwaysNil
					}
				}
			`,
			Variables: map[string]any{"id": id},
		})
	})

	t.Run("site admin", func(t *testing.T) {
		t.Parallel()

		store := dbmocks.NewMockOutboundWebhookStore()
		store.DeleteFunc.SetDefaultHook(func(ctx context.Context, id int64) error {
			assert.EqualValues(t, 1, id)
			return nil
		})

		db := dbmocks.NewMockDB()
		db.OutboundWebhooksFunc.SetDefaultReturn(store)
		ctx, _, _ := fakeUser(t, context.Background(), db, true)

		RunTest(t, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation DeleteOutboundWebhook($id: ID!) {
					deleteOutboundWebhook(id: $id) {
						alwaysNil
					}
				}
			`,
			Variables: map[string]any{"id": id},
			ExpectedResult: `
				{
					"deleteOutboundWebhook": {
						"alwaysNil": null
					}
				}
			`,
		})

		mockassert.CalledOnce(t, store.DeleteFunc)
	})
}

func TestSchemaResolver_UpdateOutboundWebhook(t *testing.T) {
	t.Parallel()

	var (
		id        = "T3V0Ym91bmRXZWJob29rOjE="
		url       = "http://example.com"
		eventType = "test:event"
		inputVars = map[string]any{
			"id": id,
			"input": map[string]any{
				"url": url,
				"eventTypes": []any{
					map[string]any{"eventType": eventType},
				},
			},
		}
	)

	t.Run("not site admin", func(t *testing.T) {
		t.Parallel()

		db := dbmocks.NewMockDB()
		ctx, _, _ := fakeUser(t, context.Background(), db, false)

		runMustBeSiteAdminTest(t, []any{"updateOutboundWebhook"}, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation UpdateOutboundWebhook($id: ID!, $input: OutboundWebhookUpdateInput!) {
					updateOutboundWebhook(id: $id, input: $input) {
						id
					}
				}
			`,
			Variables: inputVars,
		})
	})

	t.Run("site admin", func(t *testing.T) {
		t.Parallel()

		webhook := &types.OutboundWebhook{
			ID:  1,
			URL: encryption.NewUnencrypted(url),
			EventTypes: []types.OutboundWebhookEventType{
				{EventType: eventType},
			},
		}

		store := dbmocks.NewMockOutboundWebhookStore()

		// The update happens in a transaction, so we need to mock those methods
		// as well.
		store.DoneFunc.SetDefaultReturn(nil)
		store.TransactFunc.SetDefaultReturn(store, nil)

		store.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int64) (*types.OutboundWebhook, error) {
			assert.EqualValues(t, 1, webhook.ID)
			return webhook, nil
		})
		store.UpdateFunc.SetDefaultHook(func(ctx context.Context, have *types.OutboundWebhook) error {
			assert.Same(t, webhook, have)
			return nil
		})

		db := dbmocks.NewMockDB()
		db.OutboundWebhooksFunc.SetDefaultReturn(store)
		ctx, _, _ := fakeUser(t, context.Background(), db, true)

		RunTest(t, &Test{
			Context: ctx,
			Schema:  mustParseGraphQLSchema(t, db),
			Query: `
				mutation UpdateOutboundWebhook($id: ID!, $input: OutboundWebhookUpdateInput!) {
					updateOutboundWebhook(id: $id, input: $input) {
						id
						url
						eventTypes {
							eventType
						}
					}
				}
			`,
			Variables: inputVars,
			ExpectedResult: `
				{
					"updateOutboundWebhook": {
						"id": "T3V0Ym91bmRXZWJob29rOjE=",
						"url": "` + url + `",
						"eventTypes": [
							{
								"eventType": "` + eventType + `"
							}
						]
					}
				}
			`,
		})

		mockassert.CalledOnce(t, store.GetByIDFunc)
		mockassert.CalledOnce(t, store.UpdateFunc)
	})
}

var fakeUserID int32

func fakeUser(t *testing.T, inputCtx context.Context, db *dbmocks.MockDB, siteAdmin bool) (ctx context.Context, user *types.User, store *dbmocks.MockUserStore) {
	t.Helper()

	user = &types.User{
		ID:        atomic.AddInt32(&fakeUserID, 1),
		SiteAdmin: siteAdmin,
	}

	store = dbmocks.NewMockUserStore()
	store.GetByCurrentAuthUserFunc.SetDefaultReturn(user, nil)
	db.UsersFunc.SetDefaultReturn(store)

	ctx = actor.WithActor(inputCtx, &actor.Actor{UID: user.ID})

	return
}

func runMustBeSiteAdminTest(t *testing.T, path []any, test *Test) {
	t.Helper()

	// Check that the test doesn't already have expectations.
	require.Empty(t, test.ExpectedErrors)
	require.Empty(t, test.ExpectedResult)

	test.ExpectedErrors = []*gqlerrors.QueryError{
		{
			Message: "must be site admin",
			Path:    path,
		},
	}

	// Yes, the literal string "null".
	test.ExpectedResult = "null"

	RunTest(t, test)
}
