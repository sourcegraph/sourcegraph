package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestSchemaResolver_CreateExecutorSecret(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := &schemaResolver{logger: logger, db: db}
	ctx := context.Background()

	user, err := db.Users().Create(ctx, database.NewUser{Username: "test-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}

	tts := []struct {
		name    string
		args    CreateExecutorSecretArgs
		actor   *actor.Actor
		wantErr error
	}{
		{
			name: "Empty key",
			args: CreateExecutorSecretArgs{
				// Empty key
				Key:   "",
				Scope: ExecutorSecretScopeBatches,
			},
			actor:   actor.FromUser(user.ID),
			wantErr: errors.New("key cannot be empty string"),
		},
		{
			name: "Invalid key",
			args: CreateExecutorSecretArgs{
				Key:   "1GitH-UbT0ken",
				Scope: ExecutorSecretScopeBatches,
			},
			actor:   actor.FromUser(user.ID),
			wantErr: errors.New("invalid key format, should be a valid env var name"),
		},
		{
			name: "Empty value",
			args: CreateExecutorSecretArgs{
				Key: "GITHUB_TOKEN",
				// Empty value
				Value: "",
				Scope: ExecutorSecretScopeBatches,
			},
			actor:   actor.FromUser(user.ID),
			wantErr: errors.New("value cannot be empty string"),
		},
		{
			name: "Create global secret",
			args: CreateExecutorSecretArgs{
				Key:   "GITHUB_TOKEN",
				Value: "1234",
				Scope: ExecutorSecretScopeBatches,
			},
			actor: actor.FromUser(user.ID),
		},
		{
			name: "Create user secret",
			args: CreateExecutorSecretArgs{
				Key:       "GITHUB_TOKEN",
				Value:     "1234",
				Scope:     ExecutorSecretScopeBatches,
				Namespace: pointers.Ptr(MarshalUserID(user.ID)),
			},
			actor: actor.FromUser(user.ID),
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.actor != nil {
				ctx = actor.WithActor(ctx, tt.actor)
			}
			_, err := r.CreateExecutorSecret(ctx, tt.args)
			if (err != nil) != (tt.wantErr != nil) {
				t.Fatalf("invalid error returned: have=%v want=%v", err, tt.wantErr)
			}
			if err != nil {
				if have, want := err.Error(), tt.wantErr.Error(); have != want {
					t.Fatalf("invalid error returned: have=%v want=%v", have, want)
				}
			}
		})
	}
}

func TestSchemaResolver_UpdateExecutorSecret(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := &schemaResolver{logger: logger, db: db}
	ctx := context.Background()
	internalCtx := actor.WithInternalActor(ctx)

	user, err := db.Users().Create(ctx, database.NewUser{Username: "test-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}

	globalSecret := &database.ExecutorSecret{
		Key:       "ASDF",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, globalSecret, "1234"); err != nil {
		t.Fatal(err)
	}

	userSecret := &database.ExecutorSecret{
		Key:             "ASDF",
		Scope:           database.ExecutorSecretScopeBatches,
		CreatorID:       user.ID,
		NamespaceUserID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, userSecret, "1234"); err != nil {
		t.Fatal(err)
	}

	tts := []struct {
		name    string
		args    UpdateExecutorSecretArgs
		actor   *actor.Actor
		wantErr error
	}{
		{
			name: "Empty value",
			args: UpdateExecutorSecretArgs{
				ID: marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(globalSecret.Scope))), globalSecret.ID),
				// Empty value
				Value: "",
				Scope: ExecutorSecretScopeBatches,
			},
			actor:   actor.FromUser(user.ID),
			wantErr: errors.New("value cannot be empty string"),
		},
		{
			name: "Update global secret",
			args: UpdateExecutorSecretArgs{
				ID:    marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(globalSecret.Scope))), globalSecret.ID),
				Value: "1234",
				Scope: ExecutorSecretScopeBatches,
			},
			actor: actor.FromUser(user.ID),
		},
		{
			name: "Update user secret",
			args: UpdateExecutorSecretArgs{
				ID:    marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(userSecret.Scope))), userSecret.ID),
				Value: "1234",
				Scope: ExecutorSecretScopeBatches,
			},
			actor: actor.FromUser(user.ID),
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.actor != nil {
				ctx = actor.WithActor(ctx, tt.actor)
			}
			_, err := r.UpdateExecutorSecret(ctx, tt.args)
			if (err != nil) != (tt.wantErr != nil) {
				t.Fatalf("invalid error returned: have=%v want=%v", err, tt.wantErr)
			}
			if err != nil {
				if have, want := err.Error(), tt.wantErr.Error(); have != want {
					t.Fatalf("invalid error returned: have=%v want=%v", have, want)
				}
			}
		})
	}
}

func TestSchemaResolver_DeleteExecutorSecret(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := &schemaResolver{logger: logger, db: db}
	ctx := context.Background()
	internalCtx := actor.WithInternalActor(ctx)

	user, err := db.Users().Create(ctx, database.NewUser{Username: "test-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}

	globalSecret := &database.ExecutorSecret{
		Key:       "ASDF",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, globalSecret, "1234"); err != nil {
		t.Fatal(err)
	}

	userSecret := &database.ExecutorSecret{
		Key:             "ASDF",
		Scope:           database.ExecutorSecretScopeBatches,
		CreatorID:       user.ID,
		NamespaceUserID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, userSecret, "1234"); err != nil {
		t.Fatal(err)
	}

	tts := []struct {
		name    string
		args    DeleteExecutorSecretArgs
		actor   *actor.Actor
		wantErr error
	}{
		{
			name: "Delete global secret",
			args: DeleteExecutorSecretArgs{
				ID:    marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(globalSecret.Scope))), globalSecret.ID),
				Scope: ExecutorSecretScopeBatches,
			},
			actor: actor.FromUser(user.ID),
		},
		{
			name: "Delete user secret",
			args: DeleteExecutorSecretArgs{
				ID:    marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(userSecret.Scope))), userSecret.ID),
				Scope: ExecutorSecretScopeBatches,
			},
			actor: actor.FromUser(user.ID),
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.actor != nil {
				ctx = actor.WithActor(ctx, tt.actor)
			}
			_, err := r.DeleteExecutorSecret(ctx, tt.args)
			if (err != nil) != (tt.wantErr != nil) {
				t.Fatalf("invalid error returned: have=%v want=%v", err, tt.wantErr)
			}
			if err != nil {
				if have, want := err.Error(), tt.wantErr.Error(); have != want {
					t.Fatalf("invalid error returned: have=%v want=%v", have, want)
				}
			}
		})
	}
}

func TestSchemaResolver_ExecutorSecrets(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	r := &schemaResolver{logger: logger, db: db}
	ctx := context.Background()
	internalCtx := actor.WithInternalActor(ctx)

	user, err := db.Users().Create(ctx, database.NewUser{Username: "test-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}

	userCtx := actor.WithActor(ctx, actor.FromUser(user.ID))

	secret1 := &database.ExecutorSecret{
		Key:       "ASDF",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret1, "1234"); err != nil {
		t.Fatal(err)
	}
	secret2 := &database.ExecutorSecret{
		Key:             "ASDF",
		Scope:           database.ExecutorSecretScopeBatches,
		CreatorID:       user.ID,
		NamespaceUserID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret2, "1234"); err != nil {
		t.Fatal(err)
	}
	secret3 := &database.ExecutorSecret{
		Key:       "FOOBAR",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret3, "1234"); err != nil {
		t.Fatal(err)
	}

	ls, err := r.ExecutorSecrets(userCtx, ExecutorSecretsListArgs{
		Scope: ExecutorSecretScopeBatches,
		First: 50,
	})
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := ls.Nodes(userCtx)
	if err != nil {
		t.Fatal(err)
	}

	// Expect only global secrets to be returned.
	if len(nodes) != 2 {
		t.Fatalf("invalid count of nodes returned: %d", len(nodes))
	}

	tc, err := ls.TotalCount(userCtx)
	if err != nil {
		t.Fatal(err)
	}
	if tc != 2 {
		t.Fatalf("invalid totalcount returned: %d", len(nodes))
	}
}

func TestUserResolver_ExecutorSecrets(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	internalCtx := actor.WithInternalActor(ctx)

	user, err := db.Users().Create(ctx, database.NewUser{Username: "test-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}

	r, err := UserByIDInt32(ctx, db, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	userCtx := actor.WithActor(ctx, actor.FromUser(user.ID))

	secret1 := &database.ExecutorSecret{
		Key:       "ASDF",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret1, "1234"); err != nil {
		t.Fatal(err)
	}
	secret2 := &database.ExecutorSecret{
		Key:             "ASDF",
		Scope:           database.ExecutorSecretScopeBatches,
		CreatorID:       user.ID,
		NamespaceUserID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret2, "1234"); err != nil {
		t.Fatal(err)
	}
	secret3 := &database.ExecutorSecret{
		Key:       "FOOBAR",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret3, "1234"); err != nil {
		t.Fatal(err)
	}

	ls, err := r.ExecutorSecrets(userCtx, ExecutorSecretsListArgs{
		Scope: ExecutorSecretScopeBatches,
		First: 50,
	})
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := ls.Nodes(userCtx)
	if err != nil {
		t.Fatal(err)
	}

	// Expect user and global secrets, but ASDF is overwritten by user, so only 2 here.
	if len(nodes) != 2 {
		t.Fatalf("invalid count of nodes returned: %d", len(nodes))
	}

	tc, err := ls.TotalCount(userCtx)
	if err != nil {
		t.Fatal(err)
	}
	if tc != 2 {
		t.Fatalf("invalid totalcount returned: %d", len(nodes))
	}
}

func TestOrgResolver_ExecutorSecrets(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	internalCtx := actor.WithInternalActor(ctx)

	user, err := db.Users().Create(ctx, database.NewUser{Username: "test-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}

	org, err := db.Orgs().Create(ctx, "super-org", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.OrgMembers().Create(ctx, org.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	r, err := OrgByIDInt32(ctx, db, org.ID)
	if err != nil {
		t.Fatal(err)
	}

	userCtx := actor.WithActor(ctx, actor.FromUser(user.ID))

	secret1 := &database.ExecutorSecret{
		Key:       "ASDF",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret1, "1234"); err != nil {
		t.Fatal(err)
	}
	secret2 := &database.ExecutorSecret{
		Key:             "ASDF",
		Scope:           database.ExecutorSecretScopeBatches,
		CreatorID:       user.ID,
		NamespaceUserID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret2, "1234"); err != nil {
		t.Fatal(err)
	}
	secret3 := &database.ExecutorSecret{
		Key:            "FOOBAR",
		Scope:          database.ExecutorSecretScopeBatches,
		CreatorID:      user.ID,
		NamespaceOrgID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(internalCtx, database.ExecutorSecretScopeBatches, secret3, "1234"); err != nil {
		t.Fatal(err)
	}

	ls, err := r.ExecutorSecrets(userCtx, ExecutorSecretsListArgs{
		Scope: ExecutorSecretScopeBatches,
		First: 50,
	})
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := ls.Nodes(userCtx)
	if err != nil {
		t.Fatal(err)
	}

	// Expect org and global secrets.
	if len(nodes) != 2 {
		t.Fatalf("invalid count of nodes returned: %d", len(nodes))
	}

	tc, err := ls.TotalCount(userCtx)
	if err != nil {
		t.Fatal(err)
	}
	if tc != 2 {
		t.Fatalf("invalid totalcount returned: %d", len(nodes))
	}
}

func TestExecutorSecretsIntegration(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, database.NewUser{Username: "test-1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, true); err != nil {
		t.Fatal(err)
	}

	org, err := db.Orgs().Create(ctx, "super-org", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.OrgMembers().Create(ctx, org.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	userCtx := actor.WithActor(ctx, actor.FromUser(user.ID))

	secret1 := &database.ExecutorSecret{
		Key:       "ASDF",
		Scope:     database.ExecutorSecretScopeBatches,
		CreatorID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(userCtx, database.ExecutorSecretScopeBatches, secret1, "1234"); err != nil {
		t.Fatal(err)
	}
	secret2 := &database.ExecutorSecret{
		Key:             "ASDF",
		Scope:           database.ExecutorSecretScopeBatches,
		CreatorID:       user.ID,
		NamespaceUserID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(userCtx, database.ExecutorSecretScopeBatches, secret2, "1234"); err != nil {
		t.Fatal(err)
	}
	secret3 := &database.ExecutorSecret{
		Key:             "FOOBAR",
		Scope:           database.ExecutorSecretScopeBatches,
		NamespaceUserID: user.ID,
	}
	if err := db.ExecutorSecrets(&encryption.NoopKey{}).Create(userCtx, database.ExecutorSecretScopeBatches, secret3, "1234"); err != nil {
		t.Fatal(err)
	}

	// Read secret2 twice.
	for range 2 {
		_, err := secret2.Value(userCtx, db.ExecutorSecretAccessLogs())
		if err != nil {
			t.Fatal(err)
		}
	}

	als, _, err := db.ExecutorSecretAccessLogs().List(ctx, database.ExecutorSecretAccessLogsListOpts{ExecutorSecretID: secret2.ID})
	if err != nil {
		t.Fatal(err)
	}

	if len(als) != 2 {
		t.Fatal("invalid number of access logs found in DB")
	}

	s, err := NewSchemaWithoutResolvers(db)
	if err != nil {
		t.Fatal(err)
	}

	resp := s.Exec(userCtx, fmt.Sprintf(`
query ExecutorSecretsIntegrationTest {
	node(id: %q) {
		__typename
		... on User {
			executorSecrets(scope: BATCHES, first: 10) {
				totalCount
				pageInfo { hasNextPage endCursor }
				nodes {
					id
					key
					scope
					overwritesGlobalSecret
					namespace {
						id
					}
					creator {
						id
					}
					createdAt
					updatedAt
					accessLogs(first: 2) {
						totalCount
						pageInfo { hasNextPage endCursor }
						nodes {
							id
							executorSecret {
								id
							}
							user {
								id
							}
							createdAt
						}
					}
				}
			}
		}
	}
}
	`, MarshalUserID(user.ID)), "ExecutorSecretsIntegrationTest", nil)
	if len(resp.Errors) > 0 {
		t.Fatal(resp.Errors)
	}
	data := &executorSecretsIntegrationTestResponse{}
	if err := json.Unmarshal(resp.Data, data); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(data, &executorSecretsIntegrationTestResponse{
		Node: executorSecretsIntegrationTestResponseNode{
			Typename: "User",
			ExecutorSecrets: executorSecretsIntegrationTestResponseExecutorSecrets{
				TotalCount: 2,
				PageInfo: executorSecretsIntegrationTestResponsePageInfo{
					HasNextPage: false,
					EndCursor:   "",
				},
				Nodes: []executorSecretsIntegrationTestResponseSecretNode{
					{
						ID:    marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(secret2.Scope))), secret2.ID),
						Key:   "ASDF",
						Scope: string(ExecutorSecretScopeBatches),
						Namespace: executorSecretsIntegrationTestResponseNamespace{
							ID: MarshalUserID(user.ID),
						},
						Creator: executorSecretsIntegrationTestResponseCreator{
							ID: MarshalUserID(user.ID),
						},
						OverwritesGlobalSecret: true,
						CreatedAt:              secret2.CreatedAt.Format(time.RFC3339),
						UpdatedAt:              secret2.UpdatedAt.Format(time.RFC3339),
						AccessLogs: executorSecretsIntegrationTestResponseAccessLogs{
							TotalCount: 2,
							PageInfo: executorSecretsIntegrationTestResponsePageInfo{
								HasNextPage: false,
								EndCursor:   "",
							},
							Nodes: []executorSecretsIntegrationTestResponseAccessLogNode{
								{
									ID: marshalExecutorSecretAccessLogID(als[0].ID),
									ExecutorSecret: executorSecretsIntegrationTestResponseAccessLogExecutorSecret{
										ID: marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(secret2.Scope))), secret2.ID),
									},
									User: executorSecretsIntegrationTestResponseAccessLogUser{
										ID: MarshalUserID(user.ID),
									},
									CreatedAt: als[0].CreatedAt.Format(time.RFC3339),
								},
								{
									ID: marshalExecutorSecretAccessLogID(als[1].ID),
									ExecutorSecret: executorSecretsIntegrationTestResponseAccessLogExecutorSecret{
										ID: marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(secret2.Scope))), secret2.ID),
									},
									User: executorSecretsIntegrationTestResponseAccessLogUser{
										ID: MarshalUserID(user.ID),
									},
									CreatedAt: als[1].CreatedAt.Format(time.RFC3339),
								},
							},
						},
					},
					{
						ID:    marshalExecutorSecretID(ExecutorSecretScope(strings.ToUpper(string(secret3.Scope))), secret3.ID),
						Key:   "FOOBAR",
						Scope: string(ExecutorSecretScopeBatches),
						Namespace: executorSecretsIntegrationTestResponseNamespace{
							ID: MarshalUserID(user.ID),
						},
						Creator: executorSecretsIntegrationTestResponseCreator{
							ID: MarshalUserID(user.ID),
						},
						OverwritesGlobalSecret: false,
						CreatedAt:              secret3.CreatedAt.Format(time.RFC3339),
						UpdatedAt:              secret3.UpdatedAt.Format(time.RFC3339),
						AccessLogs: executorSecretsIntegrationTestResponseAccessLogs{
							TotalCount: 0,
							PageInfo: executorSecretsIntegrationTestResponsePageInfo{
								HasNextPage: false,
								EndCursor:   "",
							},
							Nodes: []executorSecretsIntegrationTestResponseAccessLogNode{},
						},
					},
				},
			},
		},
	}); diff != "" {
		t.Fatalf("invalid response: %s", diff)
	}
}

type executorSecretsIntegrationTestResponse struct {
	Node executorSecretsIntegrationTestResponseNode `json:"node"`
}

type executorSecretsIntegrationTestResponseNode struct {
	Typename        string                                                `json:"__typename"`
	ExecutorSecrets executorSecretsIntegrationTestResponseExecutorSecrets `json:"executorSecrets"`
}

type executorSecretsIntegrationTestResponseExecutorSecrets struct {
	TotalCount int32                                              `json:"totalCount"`
	PageInfo   executorSecretsIntegrationTestResponsePageInfo     `json:"pageInfo"`
	Nodes      []executorSecretsIntegrationTestResponseSecretNode `json:"nodes"`
}

type executorSecretsIntegrationTestResponsePageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type executorSecretsIntegrationTestResponseSecretNode struct {
	ID                     graphql.ID                                       `json:"id"`
	Key                    string                                           `json:"key"`
	Scope                  string                                           `json:"scope"`
	OverwritesGlobalSecret bool                                             `json:"overwritesGlobalSecret"`
	Namespace              executorSecretsIntegrationTestResponseNamespace  `json:"namespace"`
	Creator                executorSecretsIntegrationTestResponseCreator    `json:"creator"`
	CreatedAt              string                                           `json:"createdAt"`
	UpdatedAt              string                                           `json:"updatedAt"`
	AccessLogs             executorSecretsIntegrationTestResponseAccessLogs `json:"accessLogs"`
}

type executorSecretsIntegrationTestResponseNamespace struct {
	ID graphql.ID `json:"id"`
}

type executorSecretsIntegrationTestResponseCreator struct {
	ID graphql.ID `json:"id"`
}

type executorSecretsIntegrationTestResponseAccessLogs struct {
	TotalCount int32                                                 `json:"totalCount"`
	PageInfo   executorSecretsIntegrationTestResponsePageInfo        `json:"pageInfo"`
	Nodes      []executorSecretsIntegrationTestResponseAccessLogNode `json:"nodes"`
}

type executorSecretsIntegrationTestResponseAccessLogNode struct {
	ID             graphql.ID                                                    `json:"id"`
	ExecutorSecret executorSecretsIntegrationTestResponseAccessLogExecutorSecret `json:"executorSecret"`
	User           executorSecretsIntegrationTestResponseAccessLogUser           `json:"user"`
	CreatedAt      string                                                        `json:"createdAt"`
}

type executorSecretsIntegrationTestResponseAccessLogExecutorSecret struct {
	ID graphql.ID `json:"id"`
}

type executorSecretsIntegrationTestResponseAccessLogUser struct {
	ID graphql.ID `json:"id"`
}

func TestValidateExecutorSecret(t *testing.T) {
	tts := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{
			name:    "empty value",
			value:   "",
			wantErr: "value cannot be empty string",
		},
		{
			name:    "valid secret",
			value:   "set",
			key:     "ANY",
			wantErr: "",
		},
		{
			name:    "unparseable docker auth config",
			key:     "DOCKER_AUTH_CONFIG",
			value:   "notjson",
			wantErr: "failed to unmarshal docker auth config for validation: invalid character 'o' in literal null (expecting 'u')",
		},
		{
			name:    "docker auth config with cred helper",
			key:     "DOCKER_AUTH_CONFIG",
			value:   `{"credHelpers": { "hub.docker.com": "sg-login" }}`,
			wantErr: "cannot use credential helpers in docker auth config set via secrets",
		},
		{
			name:    "docker auth config with cred helper",
			key:     "DOCKER_AUTH_CONFIG",
			value:   `{"credsStore": "desktop"}`,
			wantErr: "cannot use credential stores in docker auth config set via secrets",
		},
		{
			name:    "docker auth config with additional property",
			key:     "DOCKER_AUTH_CONFIG",
			value:   `{"additionalProperty": true}`,
			wantErr: "failed to unmarshal docker auth config for validation: json: unknown field \"additionalProperty\"",
		},
		{
			name:    "docker auth config with invalid auth value",
			key:     "DOCKER_AUTH_CONFIG",
			value:   `{"auths": { "hub.docker.com": { "auth": "bm90d2l0aGNvbG9u" }}}`, // content: base64(notwithcolon)
			wantErr: "invalid credential in auths section for \"hub.docker.com\" format has to be base64(username:password)",
		},
		{
			name:    "docker auth config with valid auth value",
			key:     "DOCKER_AUTH_CONFIG",
			value:   `{"auths": { "hub.docker.com": { "auth": "dXNlcm5hbWU6cGFzc3dvcmQ=" }}}`, // content: base64(username:password)
			wantErr: "",
		},
	}
	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			have := validateExecutorSecret(&database.ExecutorSecret{Key: tt.key}, tt.value)
			if have == nil && tt.wantErr == "" {
				return
			}
			if have != nil && tt.wantErr == "" {
				t.Fatalf("invalid non-nil error returned %s", have)
			}
			if have == nil && tt.wantErr != "" {
				t.Fatalf("invalid nil error returned")
			}
			if have.Error() != tt.wantErr {
				t.Fatalf("invalid error, want=%q have =%q", tt.wantErr, have.Error())
			}
		})
	}
}
