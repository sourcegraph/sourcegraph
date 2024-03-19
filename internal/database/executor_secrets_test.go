package database_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestEnsureActorHasNamespaceWriteAccess(t *testing.T) {
	userID := int32(1)
	adminID := int32(2)
	orgID := int32(1)

	db := dbmocks.NewMockDB()
	us := dbmocks.NewMockUserStore()
	us.GetByIDFunc.SetDefaultHook(func(ctx context.Context, i int32) (*types.User, error) {
		if i == userID {
			return &types.User{
				SiteAdmin: false,
			}, nil
		}
		if i == adminID {
			return &types.User{
				SiteAdmin: true,
			}, nil
		}
		return nil, errors.New("not found")
	})
	db.UsersFunc.SetDefaultReturn(us)
	om := dbmocks.NewMockOrgMemberStore()
	om.GetByOrgIDAndUserIDFunc.SetDefaultHook(func(ctx context.Context, oid, uid int32) (*types.OrgMembership, error) {
		if uid == userID && oid == orgID {
			// Is a member.
			return &types.OrgMembership{}, nil
		}
		return nil, nil
	})
	db.OrgMembersFunc.SetDefaultReturn(om)

	internalCtx := actor.WithInternalActor(context.Background())
	userCtx := actor.WithActor(context.Background(), actor.FromUser(userID))
	adminCtx := actor.WithActor(context.Background(), actor.FromUser(adminID))
	unauthedCtx := context.Background()

	tts := []struct {
		name            string
		namespaceOrgID  int32
		namespaceUserID int32
		ctx             context.Context
		wantErr         bool
	}{
		{
			name:    "unauthed actor accessing global secret",
			ctx:     unauthedCtx,
			wantErr: true,
		},
		{
			name:            "unauthed actor accessing user secret",
			namespaceUserID: userID,
			ctx:             unauthedCtx,
			wantErr:         true,
		},
		{
			name:           "unauthed actor accessing org secret",
			namespaceOrgID: orgID,
			ctx:            unauthedCtx,
			wantErr:        true,
		},
		{
			name:    "internal actor accessing global secret",
			ctx:     internalCtx,
			wantErr: false,
		},
		{
			name:            "internal actor accessing user secret",
			namespaceUserID: userID,
			ctx:             internalCtx,
			wantErr:         false,
		},
		{
			name:           "internal actor accessing org secret",
			namespaceOrgID: orgID,
			ctx:            internalCtx,
			wantErr:        false,
		},
		{
			name:    "site admin accessing global secret",
			ctx:     adminCtx,
			wantErr: false,
		},
		{
			name:            "site admin accessing user secret",
			namespaceUserID: userID,
			ctx:             adminCtx,
			wantErr:         false,
		},
		{
			name:           "site admin accessing org secret",
			namespaceOrgID: orgID,
			ctx:            adminCtx,
			wantErr:        false,
		},
		{
			name:    "user accessing global secret",
			ctx:     userCtx,
			wantErr: true,
		},
		{
			name:            "user accessing user secret",
			namespaceUserID: userID,
			ctx:             userCtx,
			wantErr:         false,
		},
		{
			name:            "user accessing user secret of other user",
			namespaceUserID: userID + 1,
			ctx:             userCtx,
			wantErr:         true,
		},
		{
			name:           "user accessing org secret",
			namespaceOrgID: orgID,
			ctx:            userCtx,
			wantErr:        false,
		},
		{
			name:           "user accessing org secret where not member",
			namespaceOrgID: orgID + 1,
			ctx:            userCtx,
			wantErr:        true,
		},
	}
	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			secret := &database.ExecutorSecret{}
			if tt.namespaceOrgID != 0 {
				secret.NamespaceOrgID = tt.namespaceOrgID
			}
			if tt.namespaceUserID != 0 {
				secret.NamespaceUserID = tt.namespaceUserID
			}
			err := database.EnsureActorHasNamespaceWriteAccess(tt.ctx, db, secret)
			if have, want := err != nil, tt.wantErr; have != want {
				t.Fatalf("unexpected err state: have=%t want=%t", have, want)
			}
		})
	}
}

func TestExecutorSecrets_CreateUpdateDelete(t *testing.T) {
	// Use an internal actor for most of these tests, namespace access is already properly
	// tested further down separately.
	ctx := actor.WithInternalActor(context.Background())
	logger := logtest.NoOp(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	user, err := db.Users().Create(ctx, database.NewUser{Username: "johndoe"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(ctx, user.ID, false); err != nil {
		t.Fatal(err)
	}
	org, err := db.Orgs().Create(ctx, "the-org", nil)
	if err != nil {
		t.Fatal(err)
	}
	userCtx := actor.WithActor(context.Background(), actor.FromUser(user.ID))
	store := db.ExecutorSecrets(&encryption.NoopKey{})
	secretVal := "sosecret"
	t.Run("global secret", func(t *testing.T) {
		secret := &database.ExecutorSecret{
			Key:       "GH_TOKEN",
			CreatorID: user.ID,
		}
		t.Run("non-admin user cannot create global secret", func(t *testing.T) {
			if err := store.Create(userCtx, database.ExecutorSecretScopeBatches, secret, secretVal); err == nil {
				t.Fatal("unexpected non-nil error")
			}
		})
		t.Run("empty secret is forbidden", func(t *testing.T) {
			if err := store.Create(ctx, database.ExecutorSecretScopeBatches, secret, ""); err == nil {
				t.Fatal("unexpected non-nil error")
			}
		})
		if err := store.Create(ctx, database.ExecutorSecretScopeBatches, secret, secretVal); err != nil {
			t.Fatal(err)
		}
		if val, err := secret.Value(ctx, dbmocks.NewMockExecutorSecretAccessLogStore()); err != nil {
			t.Fatal(err)
		} else if val != secretVal {
			t.Fatalf("stored value does not match passed secret have=%q want=%q", val, secretVal)
		}
		if have, want := secret.Scope, database.ExecutorSecretScopeBatches; have != want {
			t.Fatalf("invalid scope stored: have=%q want=%q", have, want)
		}
		if have, want := secret.CreatorID, user.ID; have != want {
			t.Fatalf("invalid creator ID stored: have=%q want=%q", have, want)
		}
		t.Run("duplicate keys are forbidden", func(t *testing.T) {
			secret := &database.ExecutorSecret{
				Key:       "GH_TOKEN",
				CreatorID: user.ID,
			}
			err := store.Create(ctx, database.ExecutorSecretScopeBatches, secret, secretVal)
			if err == nil {
				t.Fatal("no error for duplicate key")
			}
			if err != database.ErrDuplicateExecutorSecret {
				t.Fatal("incorrect error returned")
			}
		})
		t.Run("update", func(t *testing.T) {
			newSecretValue := "evenmoresecret"

			t.Run("non-admin user cannot update global secret", func(t *testing.T) {
				if err := store.Update(userCtx, database.ExecutorSecretScopeBatches, secret, newSecretValue); err == nil {
					t.Fatal("unexpected non-nil error")
				}
			})

			t.Run("empty secret is forbidden", func(t *testing.T) {
				if err := store.Update(ctx, database.ExecutorSecretScopeBatches, secret, ""); err == nil {
					t.Fatal("unexpected non-nil error")
				}
			})

			if err := store.Update(ctx, database.ExecutorSecretScopeBatches, secret, newSecretValue); err != nil {
				t.Fatal(err)
			}
			if val, err := secret.Value(ctx, dbmocks.NewMockExecutorSecretAccessLogStore()); err != nil {
				t.Fatal(err)
			} else if val != newSecretValue {
				t.Fatalf("stored value does not match passed secret have=%q want=%q", val, newSecretValue)
			}
		})
		t.Run("delete", func(t *testing.T) {
			t.Run("non-admin user cannot delete global secret", func(t *testing.T) {
				if err := store.Delete(userCtx, database.ExecutorSecretScopeBatches, secret.ID); err == nil {
					t.Fatal("unexpected non-nil error")
				}
			})

			if err := store.Delete(ctx, database.ExecutorSecretScopeBatches, secret.ID); err != nil {
				t.Fatal(err)
			}
			_, err = store.GetByID(ctx, database.ExecutorSecretScopeBatches, secret.ID)
			if err == nil {
				t.Fatal("secret not deleted")
			}
			esnfe := &database.ExecutorSecretNotFoundErr{}
			if !errors.As(err, esnfe) {
				t.Fatal("invalid error returned, expected not found")
			}
		})
	})
	t.Run("user secret", func(t *testing.T) {
		secret := &database.ExecutorSecret{
			Key:             "GH_TOKEN",
			NamespaceUserID: user.ID,
			CreatorID:       user.ID,
		}
		if err := store.Create(ctx, database.ExecutorSecretScopeBatches, secret, secretVal); err != nil {
			t.Fatal(err)
		}
		if val, err := secret.Value(ctx, dbmocks.NewMockExecutorSecretAccessLogStore()); err != nil {
			t.Fatal(err)
		} else if val != secretVal {
			t.Fatalf("stored value does not match passed secret have=%q want=%q", val, secretVal)
		}
		if have, want := secret.Scope, database.ExecutorSecretScopeBatches; have != want {
			t.Fatalf("invalid scope stored: have=%q want=%q", have, want)
		}
		if have, want := secret.CreatorID, user.ID; have != want {
			t.Fatalf("invalid creator ID stored: have=%q want=%q", have, want)
		}
		if have, want := secret.NamespaceUserID, user.ID; have != want {
			t.Fatalf("invalid namespace user ID stored: have=%q want=%q", have, want)
		}
		t.Run("duplicate keys are forbidden", func(t *testing.T) {
			secret := &database.ExecutorSecret{
				Key:             "GH_TOKEN",
				NamespaceUserID: user.ID,
				CreatorID:       user.ID,
			}
			err := store.Create(ctx, database.ExecutorSecretScopeBatches, secret, secretVal)
			if err == nil {
				t.Fatal("no error for duplicate key")
			}
		})
		t.Run("update", func(t *testing.T) {
			newSecretValue := "evenmoresecret"
			if err := store.Update(ctx, database.ExecutorSecretScopeBatches, secret, newSecretValue); err != nil {
				t.Fatal(err)
			}
			if val, err := secret.Value(ctx, dbmocks.NewMockExecutorSecretAccessLogStore()); err != nil {
				t.Fatal(err)
			} else if val != newSecretValue {
				t.Fatalf("stored value does not match passed secret have=%q want=%q", val, newSecretValue)
			}
		})
		t.Run("delete", func(t *testing.T) {
			if err := store.Delete(ctx, database.ExecutorSecretScopeBatches, secret.ID); err != nil {
				t.Fatal(err)
			}
			_, err = store.GetByID(ctx, database.ExecutorSecretScopeBatches, secret.ID)
			if err == nil {
				t.Fatal("secret not deleted")
			}
			esnfe := &database.ExecutorSecretNotFoundErr{}
			if !errors.As(err, esnfe) {
				t.Fatal("invalid error returned, expected not found")
			}
		})
	})
	t.Run("org secret", func(t *testing.T) {
		secret := &database.ExecutorSecret{
			Key:            "GH_TOKEN",
			NamespaceOrgID: org.ID,
			CreatorID:      user.ID,
		}
		if err := store.Create(ctx, database.ExecutorSecretScopeBatches, secret, secretVal); err != nil {
			t.Fatal(err)
		}
		if val, err := secret.Value(ctx, dbmocks.NewMockExecutorSecretAccessLogStore()); err != nil {
			t.Fatal(err)
		} else if val != secretVal {
			t.Fatalf("stored value does not match passed secret have=%q want=%q", val, secretVal)
		}
		if have, want := secret.Scope, database.ExecutorSecretScopeBatches; have != want {
			t.Fatalf("invalid scope stored: have=%q want=%q", have, want)
		}
		if have, want := secret.CreatorID, user.ID; have != want {
			t.Fatalf("invalid creator ID stored: have=%q want=%q", have, want)
		}
		if have, want := secret.NamespaceOrgID, org.ID; have != want {
			t.Fatalf("invalid namespace org ID stored: have=%q want=%q", have, want)
		}
		t.Run("duplicate keys are forbidden", func(t *testing.T) {
			secret := &database.ExecutorSecret{
				Key:            "GH_TOKEN",
				NamespaceOrgID: org.ID,
				CreatorID:      user.ID,
			}
			err := store.Create(ctx, database.ExecutorSecretScopeBatches, secret, secretVal)
			if err == nil {
				t.Fatal("no error for duplicate key")
			}
		})
		t.Run("update", func(t *testing.T) {
			newSecretValue := "evenmoresecret"
			if err := store.Update(ctx, database.ExecutorSecretScopeBatches, secret, newSecretValue); err != nil {
				t.Fatal(err)
			}
			if val, err := secret.Value(ctx, dbmocks.NewMockExecutorSecretAccessLogStore()); err != nil {
				t.Fatal(err)
			} else if val != newSecretValue {
				t.Fatalf("stored value does not match passed secret have=%q want=%q", val, newSecretValue)
			}
		})
		t.Run("delete", func(t *testing.T) {
			if err := store.Delete(ctx, database.ExecutorSecretScopeBatches, secret.ID); err != nil {
				t.Fatal(err)
			}
			_, err = store.GetByID(ctx, database.ExecutorSecretScopeBatches, secret.ID)
			if err == nil {
				t.Fatal("secret not deleted")
			}
			esnfe := &database.ExecutorSecretNotFoundErr{}
			if !errors.As(err, esnfe) {
				t.Fatal("invalid error returned, expected not found")
			}
		})
	})
}

func TestExecutorSecrets_GetListCount(t *testing.T) {
	internalCtx := actor.WithInternalActor(context.Background())
	logger := logtest.NoOp(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	user, err := db.Users().Create(internalCtx, database.NewUser{Username: "johndoe"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(internalCtx, user.ID, false); err != nil {
		t.Fatal(err)
	}
	otherUser, err := db.Users().Create(internalCtx, database.NewUser{Username: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Users().SetIsSiteAdmin(internalCtx, otherUser.ID, false); err != nil {
		t.Fatal(err)
	}
	org, err := db.Orgs().Create(internalCtx, "the-org", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.OrgMembers().Create(internalCtx, org.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	userCtx := actor.WithActor(context.Background(), actor.FromUser(user.ID))
	otherUserCtx := actor.WithActor(context.Background(), actor.FromUser(otherUser.ID))
	store := db.ExecutorSecrets(&encryption.NoopKey{})

	// We create a bunch of secrets to test overrides:
	// GH_TOKEN User:NULL Org:NULL
	// NPM_TOKEN User:NULL Org:NULL
	// GH_TOKEN User:Set Org:NULL
	// SG_TOKEN User:Set Org:NULL
	// NPM_TOKEN User:NULL Org:Set
	// DOCKER_TOKEN User:NULL Org:Set
	// Expected results:
	// Global: GH_TOKEN, NPM_TOKEN
	// User: GH_TOKEN (user-owned), NPM_TOKEN, SG_TOKEN (user-owned)
	// Org: GH_TOKEN, NPM_TOKEN (org-owned), DOCKER_TOKEN (org-owned)

	secretVal := "sosecret"
	createSecret := func(secret *database.ExecutorSecret) *database.ExecutorSecret {
		secret.CreatorID = user.ID
		if err := store.Create(internalCtx, database.ExecutorSecretScopeBatches, secret, secretVal); err != nil {
			t.Fatal(err)
		}
		otherSecret := *secret
		// We generate a second version of the secret in a separate scope (namespace)
		// to test that they're properly isolated.
		if err := store.Create(internalCtx, database.ExecutorSecretScopeCodeIntel, &otherSecret, secretVal); err != nil {
			t.Fatal(err)
		}
		return secret
	}
	globalGHToken := createSecret(&database.ExecutorSecret{Key: "GH_TOKEN"})
	globalNPMToken := createSecret(&database.ExecutorSecret{Key: "NPM_TOKEN"})
	userGHToken := createSecret(&database.ExecutorSecret{Key: "GH_TOKEN", NamespaceUserID: user.ID})
	userSGToken := createSecret(&database.ExecutorSecret{Key: "SG_TOKEN", NamespaceUserID: user.ID})
	orgNPMToken := createSecret(&database.ExecutorSecret{Key: "NPM_TOKEN", NamespaceOrgID: org.ID})
	orgDockerToken := createSecret(&database.ExecutorSecret{Key: "DOCKER_TOKEN", NamespaceOrgID: org.ID})

	t.Run("GetByID", func(t *testing.T) {
		t.Run("global secret as user", func(t *testing.T) {
			secret, err := store.GetByID(userCtx, database.ExecutorSecretScopeBatches, globalGHToken.ID)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(globalGHToken, secret, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
				t.Fatal(diff)
			}
		})
		t.Run("user secret as user", func(t *testing.T) {
			secret, err := store.GetByID(userCtx, database.ExecutorSecretScopeBatches, userGHToken.ID)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(userGHToken, secret, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
				t.Fatal(diff)
			}

			if !secret.OverwritesGlobalSecret {
				t.Fatal("not marked as overwriting global secret")
			}

			t.Run("accessing other users secret", func(t *testing.T) {
				if _, err := store.GetByID(otherUserCtx, database.ExecutorSecretScopeBatches, userGHToken.ID); err == nil {
					t.Fatal("unexpected non nil error")
				}
			})
		})
		t.Run("org secret as user", func(t *testing.T) {
			secret, err := store.GetByID(userCtx, database.ExecutorSecretScopeBatches, orgNPMToken.ID)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(orgNPMToken, secret, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
				t.Fatal(diff)
			}

			t.Run("accessing org secret as non-member", func(t *testing.T) {
				if _, err := store.GetByID(otherUserCtx, database.ExecutorSecretScopeBatches, orgNPMToken.ID); err == nil {
					t.Fatal("unexpected non nil error")
				}
			})
		})
	})

	t.Run("ListCount", func(t *testing.T) {
		t.Run("global secrets as user", func(t *testing.T) {
			opts := database.ExecutorSecretsListOpts{}
			secrets, _, err := store.List(userCtx, database.ExecutorSecretScopeBatches, opts)
			if err != nil {
				t.Fatal(err)
			}
			count, err := store.Count(userCtx, database.ExecutorSecretScopeBatches, opts)
			if err != nil {
				t.Fatal(err)
			}
			if have, want := count, len(secrets); have != want {
				t.Fatalf("invalid count returned: %d", have)
			}
			if diff := cmp.Diff([]*database.ExecutorSecret{globalGHToken, globalNPMToken}, secrets, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
				t.Fatal(diff)
			}
		})
		t.Run("user secrets as user", func(t *testing.T) {
			opts := database.ExecutorSecretsListOpts{NamespaceUserID: user.ID}
			secrets, _, err := store.List(userCtx, database.ExecutorSecretScopeBatches, opts)
			if err != nil {
				t.Fatal(err)
			}
			count, err := store.Count(userCtx, database.ExecutorSecretScopeBatches, opts)
			if err != nil {
				t.Fatal(err)
			}
			if have, want := count, len(secrets); have != want {
				t.Fatalf("invalid count returned: %d", have)
			}
			if diff := cmp.Diff([]*database.ExecutorSecret{userGHToken, globalNPMToken, userSGToken}, secrets, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
				t.Fatal(diff)
			}

			t.Run("by Keys", func(t *testing.T) {
				opts := database.ExecutorSecretsListOpts{NamespaceUserID: user.ID, Keys: []string{userGHToken.Key, globalNPMToken.Key}}
				secrets, _, err := store.List(userCtx, database.ExecutorSecretScopeBatches, opts)
				if err != nil {
					t.Fatal(err)
				}
				count, err := store.Count(userCtx, database.ExecutorSecretScopeBatches, opts)
				if err != nil {
					t.Fatal(err)
				}
				if have, want := count, len(secrets); have != want {
					t.Fatalf("invalid count returned: %d", have)
				}
				if diff := cmp.Diff([]*database.ExecutorSecret{userGHToken, globalNPMToken}, secrets, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
					t.Fatal(diff)
				}
			})

			t.Run("accessing other users secrets", func(t *testing.T) {
				secrets, _, err := store.List(otherUserCtx, database.ExecutorSecretScopeBatches, opts)
				if err != nil {
					t.Fatal(err)
				}
				// Only returns global tokens.
				if diff := cmp.Diff([]*database.ExecutorSecret{globalGHToken, globalNPMToken}, secrets, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
					t.Fatal(diff)
				}
			})
		})
		t.Run("org secrets as user", func(t *testing.T) {
			opts := database.ExecutorSecretsListOpts{NamespaceOrgID: org.ID}
			secrets, _, err := store.List(userCtx, database.ExecutorSecretScopeBatches, opts)
			if err != nil {
				t.Fatal(err)
			}
			count, err := store.Count(userCtx, database.ExecutorSecretScopeBatches, opts)
			if err != nil {
				t.Fatal(err)
			}
			if have, want := count, len(secrets); have != want {
				t.Fatalf("invalid count returned: %d", have)
			}
			if diff := cmp.Diff([]*database.ExecutorSecret{orgDockerToken, globalGHToken, orgNPMToken}, secrets, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
				t.Fatal(diff)
			}

			t.Run("accessing org secrets as non-member", func(t *testing.T) {
				secrets, _, err := store.List(otherUserCtx, database.ExecutorSecretScopeBatches, opts)
				if err != nil {
					t.Fatal(err)
				}
				// Only returns global tokens.
				if diff := cmp.Diff([]*database.ExecutorSecret{globalGHToken, globalNPMToken}, secrets, cmpopts.IgnoreUnexported(database.ExecutorSecret{})); diff != "" {
					t.Fatal(diff)
				}
			})
		})
	})
}

func TestExecutorSecretNotFoundError(t *testing.T) {
	err := database.ExecutorSecretNotFoundErr{}
	if have := errcode.IsNotFound(err); !have {
		t.Error("ExecutorSecretNotFoundErr does not say it represents a not found error")
	}
}
