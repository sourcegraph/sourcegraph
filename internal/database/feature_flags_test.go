package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	ff "github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestFeatureFlagStore(t *testing.T) {
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))

	t.Run("NewFeatureFlag", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testNewFeatureFlagRoundtrip(t, db)
	})
	t.Run("ListFeatureFlags", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testListFeatureFlags(t, db)
	})
	t.Run("Overrides", func(t *testing.T) {
		t.Run("NewOverride", func(t *testing.T) {
			t.Cleanup(cleanup(t, db))
			testNewOverrideRoundtrip(t, db)
		})
		t.Run("ListUserOverrides", func(t *testing.T) {
			t.Cleanup(cleanup(t, db))
			testListUserOverrides(t, db)
		})
		t.Run("ListOrgOverrides", func(t *testing.T) {
			t.Cleanup(cleanup(t, db))
			testListOrgOverrides(t, db)
		})
	})
	t.Run("UserFlags", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testUserFlags(t, db)
	})
	t.Run("AnonymousUserFlags", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testAnonymousUserFlags(t, db)
	})
	t.Run("UserlessFeatureFlags", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testUserlessFeatureFlags(t, db)
	})
	t.Run("OrganizationFeatureFlag", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testOrgFeatureFlag(t, db)
	})
	t.Run("GetFeatureFlag", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testGetFeatureFlag(t, db)
	})
	t.Run("UpdateFeatureFlag", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		testUpdateFeatureFlag(t, db)
	})
}

func errorContains(s string) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, msg ...any) {
		require.Error(t, err)
		require.Contains(t, err.Error(), s, msg)
	}
}

func cleanup(t *testing.T, db DB) func() {
	return func() {
		if t.Failed() {
			// Retain content on failed tests
			return
		}
		_, err := db.ExecContext(
			context.Background(),
			`truncate feature_flag_overrides, feature_flags, users, orgs, org_members, names cascade;`,
		)
		require.NoError(t, err)
	}
}

func setupClearRedisCacheTest(t *testing.T, expectedFlagName string) *bool {
	clearRedisCacheCalled := false
	oldClearRedisCache := clearRedisCache
	clearRedisCache = func(flagName string) {
		if flagName == expectedFlagName {
			clearRedisCacheCalled = true
		}
	}
	t.Cleanup(func() { clearRedisCache = oldClearRedisCache })
	return &clearRedisCacheCalled
}

func testNewFeatureFlagRoundtrip(t *testing.T, db DB) {
	ctx := actor.WithInternalActor(context.Background())
	flagStore := db.FeatureFlags()

	cases := []struct {
		flag      *ff.FeatureFlag
		assertErr require.ErrorAssertionFunc
	}{
		{
			flag: &ff.FeatureFlag{Name: "bool_true", Bool: &ff.FeatureFlagBool{Value: true}},
		},
		{
			flag: &ff.FeatureFlag{Name: "bool_false", Bool: &ff.FeatureFlagBool{Value: false}},
		},
		{
			flag: &ff.FeatureFlag{Name: "min_rollout", Rollout: &ff.FeatureFlagRollout{Rollout: 0}},
		},
		{
			flag: &ff.FeatureFlag{Name: "mid_rollout", Rollout: &ff.FeatureFlagRollout{Rollout: 3124}},
		},
		{
			flag: &ff.FeatureFlag{Name: "max_rollout", Rollout: &ff.FeatureFlagRollout{Rollout: 10000}},
		},
		{
			flag:      &ff.FeatureFlag{Name: "err_too_high_rollout", Rollout: &ff.FeatureFlagRollout{Rollout: 10001}},
			assertErr: errorContains(`violates check constraint "feature_flags_rollout_check"`),
		},
		{
			flag:      &ff.FeatureFlag{Name: "err_too_low_rollout", Rollout: &ff.FeatureFlagRollout{Rollout: -1}},
			assertErr: errorContains(`violates check constraint "feature_flags_rollout_check"`),
		},
		{
			flag:      &ff.FeatureFlag{Name: "err_no_types"},
			assertErr: errorContains(`feature flag must have exactly one type`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.flag.Name, func(t *testing.T) {
			res, err := flagStore.CreateFeatureFlag(ctx, tc.flag)
			if tc.assertErr != nil {
				tc.assertErr(t, err)
				return
			}
			require.NoError(t, err)

			// Only assert that the values it is created with are equal.
			// Don't bother with the timestamps
			require.Equal(t, tc.flag.Name, res.Name)
			require.Equal(t, tc.flag.Bool, res.Bool)
			require.Equal(t, tc.flag.Rollout, res.Rollout)
		})
	}
}

func testListFeatureFlags(t *testing.T, db DB) {
	ctx := actor.WithInternalActor(context.Background())
	flagStore := db.FeatureFlags()

	flag1 := &ff.FeatureFlag{Name: "bool_true", Bool: &ff.FeatureFlagBool{Value: true}}
	flag2 := &ff.FeatureFlag{Name: "bool_false", Bool: &ff.FeatureFlagBool{Value: false}}
	flag3 := &ff.FeatureFlag{Name: "mid_rollout", Rollout: &ff.FeatureFlagRollout{Rollout: 3124}}
	flag4 := &ff.FeatureFlag{Name: "deletable", Rollout: &ff.FeatureFlagRollout{Rollout: 3125}}
	flags := []*ff.FeatureFlag{flag1, flag2, flag3, flag4}

	for _, flag := range flags {
		_, err := flagStore.CreateFeatureFlag(ctx, flag)
		require.NoError(t, err)
	}

	// Deleted flag4
	_, err := db.ExecContext(ctx, "DELETE FROM feature_flags WHERE flag_name = 'deletable'")
	require.NoError(t, err)

	expected := []*ff.FeatureFlag{flag1, flag2, flag3}

	res, err := flagStore.GetFeatureFlags(ctx)
	require.NoError(t, err)
	for _, flag := range res {
		// Unset any timestamps
		flag.CreatedAt = time.Time{}
		flag.UpdatedAt = time.Time{}
		flag.DeletedAt = nil
	}

	require.EqualValues(t, res, expected)
}

func testNewOverrideRoundtrip(t *testing.T, db DB) {
	users := db.Users()
	ctx := actor.WithInternalActor(context.Background())

	flagStore := db.FeatureFlags()

	ff1, err := flagStore.CreateBool(ctx, "t", true)
	require.NoError(t, err)

	u1, err := users.Create(ctx, NewUser{Username: "u", Password: "p"})
	require.NoError(t, err)

	invalidUserID := int32(38535)

	cases := []struct {
		override  *ff.Override
		assertErr require.ErrorAssertionFunc
	}{
		{
			override: &ff.Override{UserID: &u1.ID, FlagName: ff1.Name, Value: false},
		},
		{
			override:  &ff.Override{UserID: &invalidUserID, FlagName: ff1.Name, Value: false},
			assertErr: errorContains(`violates foreign key constraint "feature_flag_overrides_namespace_user_id_fkey"`),
		},
		{
			override: &ff.Override{UserID: &u1.ID, FlagName: "invalid-flag-name", Value: false},
		},
		{
			override:  &ff.Override{FlagName: ff1.Name, Value: false},
			assertErr: errorContains(`violates check constraint "feature_flag_overrides_has_org_or_user_id"`),
		},
	}

	for _, tc := range cases {
		t.Run("case", func(t *testing.T) {
			res, err := flagStore.CreateOverride(ctx, tc.override)
			if tc.assertErr != nil {
				tc.assertErr(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.override, res)
		})
	}
}

func testListUserOverrides(t *testing.T, db DB) {
	flagStore := db.FeatureFlags()
	users := db.Users()
	ctx := actor.WithInternalActor(context.Background())

	mkUser := func(name string) *types.User {
		u, err := users.Create(ctx, NewUser{Username: name, Password: "p"})
		require.NoError(t, err)
		return u
	}

	mkFFBool := func(name string, val bool) *ff.FeatureFlag {
		res, err := flagStore.CreateBool(ctx, name, val)
		require.NoError(t, err)
		return res
	}

	mkOverride := func(user int32, flag string, val bool) *ff.Override {
		ffo, err := flagStore.CreateOverride(ctx, &ff.Override{UserID: &user, FlagName: flag, Value: val})
		require.NoError(t, err)
		return ffo
	}

	t.Run("no overrides", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u")
		mkFFBool("f", true)
		got, err := flagStore.GetUserOverrides(ctx, u1.ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("some overrides", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u")
		f1 := mkFFBool("f", true)
		o1 := mkOverride(u1.ID, f1.Name, false)
		got, err := flagStore.GetUserOverrides(ctx, u1.ID)
		require.NoError(t, err)
		require.Equal(t, got, []*ff.Override{o1})
	})

	t.Run("overrides for other users", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u1")
		u2 := mkUser("u2")
		f1 := mkFFBool("f", true)
		o1 := mkOverride(u1.ID, f1.Name, false)
		mkOverride(u2.ID, f1.Name, true)
		got, err := flagStore.GetUserOverrides(ctx, u1.ID)
		require.NoError(t, err)
		require.Equal(t, got, []*ff.Override{o1})
	})

	t.Run("deleted override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u1")
		f1 := mkFFBool("f", true)
		mkOverride(u1.ID, f1.Name, false)
		_, err := db.ExecContext(ctx, "UPDATE feature_flag_overrides SET deleted_at = now()")
		require.NoError(t, err)
		got, err := flagStore.GetUserOverrides(ctx, u1.ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("non-unique override upserts", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u1")
		f1 := mkFFBool("f", true)
		_, err := flagStore.CreateOverride(ctx, &ff.Override{UserID: &u1.ID, FlagName: f1.Name, Value: true})
		require.NoError(t, err)
		_, err = flagStore.CreateOverride(ctx, &ff.Override{UserID: &u1.ID, FlagName: f1.Name, Value: false})
		require.NoError(t, err)
		flags, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		require.Equal(t, flags[f1.Name], false)
	})
}

func testListOrgOverrides(t *testing.T, db DB) {

	flagStore := db.FeatureFlags()
	users := db.Users()
	orgs := db.Orgs()
	orgMembers := db.OrgMembers()
	ctx := actor.WithInternalActor(context.Background())

	mkUser := func(name string, orgIDs ...int32) *types.User {
		u, err := users.Create(ctx, NewUser{Username: name, Password: "p"})
		require.NoError(t, err)
		for _, id := range orgIDs {
			_, err := orgMembers.Create(ctx, id, u.ID)
			require.NoError(t, err)
		}
		return u
	}

	mkFFBool := func(name string, val bool) *ff.FeatureFlag {
		res, err := flagStore.CreateBool(ctx, name, val)
		require.NoError(t, err)
		return res
	}

	mkOverride := func(org int32, flag string, val bool) *ff.Override {
		ffo, err := flagStore.CreateOverride(ctx, &ff.Override{OrgID: &org, FlagName: flag, Value: val})
		require.NoError(t, err)
		return ffo
	}

	mkOrg := func(name string) *types.Org {
		o, err := orgs.Create(ctx, name, nil)
		require.NoError(t, err)
		return o
	}

	t.Run("no overrides", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u")
		mkFFBool("f", true)

		got, err := flagStore.GetUserOverrides(ctx, u1.ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("some overrides", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		org1 := mkOrg("org1")
		u1 := mkUser("u", org1.ID)
		f1 := mkFFBool("f", true)
		o1 := mkOverride(org1.ID, f1.Name, false)

		got, err := flagStore.GetOrgOverridesForUser(ctx, u1.ID)
		require.NoError(t, err)
		require.Equal(t, got, []*ff.Override{o1})
	})

	t.Run("deleted overrides", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		org1 := mkOrg("org1")
		u1 := mkUser("u", org1.ID)
		f1 := mkFFBool("f", true)
		mkOverride(org1.ID, f1.Name, false)
		_, err := db.ExecContext(ctx, "UPDATE feature_flag_overrides SET deleted_at = now()")
		require.NoError(t, err)

		got, err := flagStore.GetOrgOverridesForUser(ctx, u1.ID)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("non-unique override errors", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		org1 := mkOrg("org1")
		f1 := mkFFBool("f", true)

		_, err := flagStore.CreateOverride(ctx, &ff.Override{OrgID: &org1.ID, FlagName: f1.Name, Value: true})
		require.NoError(t, err)
		_, err = flagStore.CreateOverride(ctx, &ff.Override{OrgID: &org1.ID, FlagName: f1.Name, Value: false})
		require.Error(t, err)
	})
}

func testUserFlags(t *testing.T, db DB) {
	flagStore := db.FeatureFlags()
	users := db.Users()
	orgs := db.Orgs()
	orgMembers := db.OrgMembers()
	ctx := actor.WithInternalActor(context.Background())

	mkUser := func(name string, orgIDs ...int32) *types.User {
		u, err := users.Create(ctx, NewUser{Username: name, Password: "p"})
		require.NoError(t, err)
		for _, id := range orgIDs {
			_, err := orgMembers.Create(ctx, id, u.ID)
			require.NoError(t, err)
		}
		return u
	}

	mkFFBool := func(name string, val bool) *ff.FeatureFlag {
		res, err := flagStore.CreateBool(ctx, name, val)
		require.NoError(t, err)
		return res
	}

	mkFFBoolVar := func(name string, rollout int32) *ff.FeatureFlag {
		res, err := flagStore.CreateRollout(ctx, name, rollout)
		require.NoError(t, err)
		return res
	}

	mkUserOverride := func(user int32, flag string, val bool) *ff.Override {
		ffo, err := flagStore.CreateOverride(ctx, &ff.Override{UserID: &user, FlagName: flag, Value: val})
		require.NoError(t, err)
		return ffo
	}

	mkOrgOverride := func(org int32, flag string, val bool) *ff.Override {
		ffo, err := flagStore.CreateOverride(ctx, &ff.Override{OrgID: &org, FlagName: flag, Value: val})
		require.NoError(t, err)
		return ffo
	}

	mkOrg := func(name string) *types.Org {
		o, err := orgs.Create(ctx, name, nil)
		require.NoError(t, err)
		return o
	}

	t.Run("bool vals", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u")
		mkFFBool("f1", true)
		mkFFBool("f2", false)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": false}
		require.Equal(t, expected, got)
	})

	t.Run("bool vars", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u")
		mkFFBoolVar("f1", 10000)
		mkFFBoolVar("f2", 0)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": false}
		require.Equal(t, expected, got)
	})

	t.Run("bool vals with user override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u")
		mkFFBool("f1", true)
		mkFFBool("f2", false)
		mkUserOverride(u1.ID, "f2", true)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": true}
		require.Equal(t, expected, got)
	})

	t.Run("bool vars with user override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		u1 := mkUser("u")
		mkFFBoolVar("f1", 10000)
		mkFFBoolVar("f2", 0)
		mkUserOverride(u1.ID, "f2", true)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": true}
		require.Equal(t, expected, got)
	})

	t.Run("bool vals with org override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		o1 := mkOrg("o1")
		u1 := mkUser("u", o1.ID)
		mkFFBool("f1", true)
		mkFFBool("f2", false)
		mkOrgOverride(o1.ID, "f2", true)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": true}
		require.Equal(t, expected, got)
	})

	t.Run("bool vars with org override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		o1 := mkOrg("o1")
		u1 := mkUser("u", o1.ID)
		mkFFBoolVar("f1", 10000)
		mkFFBoolVar("f2", 0)
		mkOrgOverride(o1.ID, "f2", true)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": true}
		require.Equal(t, expected, got)
	})

	t.Run("user override beats org override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		o1 := mkOrg("o1")
		u1 := mkUser("u", o1.ID)
		mkFFBoolVar("f1", 10000)
		mkFFBoolVar("f2", 0)
		mkOrgOverride(o1.ID, "f2", true)
		mkUserOverride(u1.ID, "f2", false)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": false}
		require.Equal(t, expected, got)
	})

	t.Run("newer org override beats older org override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		o1 := mkOrg("o1")
		o2 := mkOrg("o2")
		u1 := mkUser("u", o1.ID, o2.ID)
		mkFFBoolVar("f1", 10000)
		mkFFBoolVar("f2", 0)
		mkOrgOverride(o1.ID, "f2", true)
		mkOrgOverride(o2.ID, "f2", false)

		got, err := flagStore.GetUserFlags(ctx, u1.ID)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": false}
		require.Equal(t, expected, got)
	})

	t.Run("delete flag with override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		o1 := mkOrg("o1")
		u1 := mkUser("u", o1.ID)
		f1 := mkFFBool("f1", true)
		mkUserOverride(u1.ID, "f1", false)
		clearRedisCacheCalled := setupClearRedisCacheTest(t, f1.Name)

		err := flagStore.DeleteFeatureFlag(ctx, f1.Name)
		require.NoError(t, err)
		require.True(t, *clearRedisCacheCalled)

		flags, err := flagStore.GetFeatureFlags(ctx)
		require.NoError(t, err)
		require.Len(t, flags, 0)
	})
}

func testAnonymousUserFlags(t *testing.T, db DB) {

	flagStore := db.FeatureFlags()
	ctx := actor.WithInternalActor(context.Background())

	mkFFBool := func(name string, val bool) *ff.FeatureFlag {
		res, err := flagStore.CreateBool(ctx, name, val)
		require.NoError(t, err)
		return res
	}

	mkFFBoolVar := func(name string, rollout int32) *ff.FeatureFlag {
		res, err := flagStore.CreateRollout(ctx, name, rollout)
		require.NoError(t, err)
		return res
	}

	t.Run("bool vals", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		mkFFBool("f1", true)
		mkFFBool("f2", false)

		got, err := flagStore.GetAnonymousUserFlags(ctx, "testuser")
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": false}
		require.Equal(t, expected, got)
	})

	t.Run("bool vars", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		mkFFBoolVar("f1", 10000)
		mkFFBoolVar("f2", 0)

		got, err := flagStore.GetAnonymousUserFlags(ctx, "testuser")
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": false}
		require.Equal(t, expected, got)
	})

	// No override tests for AnonymousUserFlags because no override
	// can be defined for an anonymous user.
}

func testUserlessFeatureFlags(t *testing.T, db DB) {

	flagStore := db.FeatureFlags()
	ctx := actor.WithInternalActor(context.Background())

	mkFFBool := func(name string, val bool) *ff.FeatureFlag {
		res, err := flagStore.CreateBool(ctx, name, val)
		require.NoError(t, err)
		return res
	}

	mkFFBoolVar := func(name string, rollout int32) *ff.FeatureFlag {
		res, err := flagStore.CreateRollout(ctx, name, rollout)
		require.NoError(t, err)
		return res
	}

	t.Run("bool vals", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		mkFFBool("f1", true)
		mkFFBool("f2", false)

		got, err := flagStore.GetGlobalFeatureFlags(ctx)
		require.NoError(t, err)
		expected := map[string]bool{"f1": true, "f2": false}
		require.Equal(t, expected, got)
	})

	t.Run("bool vars", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		mkFFBoolVar("f1", 10000)
		mkFFBoolVar("f2", 0)

		got, err := flagStore.GetGlobalFeatureFlags(ctx)
		require.NoError(t, err)

		// Userless requests don't have a stable user to evaluate
		// bool variable flags, so none should be defined.
		//
		// TODO(camdencheek): consider evaluating rollout feature
		// flags with a static string so they are defined and stable,
		// but effectively statically random.
		expected := map[string]bool{}
		require.Equal(t, expected, got)
	})
}

func testOrgFeatureFlag(t *testing.T, db DB) {

	flagStore := db.FeatureFlags()
	orgs := db.Orgs()
	ctx := actor.WithInternalActor(context.Background())

	mkFFBool := func(name string, val bool) *ff.FeatureFlag {
		res, err := flagStore.CreateBool(ctx, name, val)
		require.NoError(t, err)
		return res
	}

	mkOrgOverride := func(org int32, flag string, val bool) *ff.Override {
		ffo, err := flagStore.CreateOverride(ctx, &ff.Override{OrgID: &org, FlagName: flag, Value: val})
		require.NoError(t, err)
		return ffo
	}

	mkOrg := func(name string) *types.Org {
		o, err := orgs.Create(ctx, name, nil)
		require.NoError(t, err)
		return o
	}

	t.Run("bool vals", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		org := mkOrg("o")
		mkFFBool("f1", true)
		mkFFBool("f2", false)

		got1, err1 := flagStore.GetOrgFeatureFlag(ctx, org.ID, "f1")
		got2, err2 := flagStore.GetOrgFeatureFlag(ctx, org.ID, "f2")
		require.NoError(t, err1)
		require.NoError(t, err2)
		require.True(t, got1)
		require.False(t, got2)
	})

	t.Run("bool vals with org override", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		org1 := mkOrg("o1")
		org2 := mkOrg("o2")
		mkFFBool("f1", true)
		mkFFBool("f2", false)
		mkOrgOverride(org1.ID, "f1", false)
		mkOrgOverride(org1.ID, "f2", true)

		got, err := flagStore.GetOrgFeatureFlag(ctx, org1.ID, "f1")
		require.NoError(t, err)
		require.Equal(t, false, got)

		got, err = flagStore.GetOrgFeatureFlag(ctx, org1.ID, "f2")
		require.NoError(t, err)
		require.Equal(t, true, got)

		got, err = flagStore.GetOrgFeatureFlag(ctx, org2.ID, "f1")
		require.NoError(t, err)
		require.Equal(t, true, got)

		got, err = flagStore.GetOrgFeatureFlag(ctx, org2.ID, "f2")
		require.NoError(t, err)
		require.Equal(t, false, got)
	})

	t.Run("bool vals without flag defined", func(t *testing.T) {
		t.Cleanup(cleanup(t, db))
		org := mkOrg("o")

		got, err := flagStore.GetOrgFeatureFlag(ctx, org.ID, "f1")
		require.NoError(t, err)
		require.Equal(t, false, got)
	})
}

func testGetFeatureFlag(t *testing.T, db DB) {

	flagStore := db.FeatureFlags()
	ctx := context.Background()
	t.Run("no value", func(t *testing.T) {
		flag, err := flagStore.GetFeatureFlag(ctx, "does-not-exist")
		require.Equal(t, err, sql.ErrNoRows)
		require.Nil(t, flag)
	})
	t.Run("true value", func(t *testing.T) {
		_, err := flagStore.CreateBool(ctx, "is-true", true)
		require.NoError(t, err)
		flag, err := flagStore.GetFeatureFlag(ctx, "is-true")
		require.NoError(t, err)
		require.True(t, flag.Bool.Value)
	})
	t.Run("false value", func(t *testing.T) {
		_, err := flagStore.CreateBool(ctx, "is-false", true)
		require.NoError(t, err)
		flag, err := flagStore.GetFeatureFlag(ctx, "is-false")
		require.NoError(t, err)
		require.True(t, flag.Bool.Value)
	})
}

func testUpdateFeatureFlag(t *testing.T, db DB) {

	flagStore := db.FeatureFlags()
	ctx := context.Background()
	t.Run("invalid input", func(t *testing.T) {
		updatedFf, err := flagStore.UpdateFeatureFlag(ctx, &ff.FeatureFlag{Name: "invalid"})
		require.EqualError(t, err, "feature flag must have exactly one type")
		require.Nil(t, updatedFf)
	})
	t.Run("boolean flag successful update", func(t *testing.T) {
		boolFlag, err := flagStore.CreateBool(ctx, "update-test-true-flag", true)
		require.NoError(t, err)
		boolFlag.Bool.Value = false
		clearRedisCacheCalled := setupClearRedisCacheTest(t, boolFlag.Name)
		updatedFlag, err := flagStore.UpdateFeatureFlag(ctx, boolFlag)
		require.NoError(t, err)
		require.True(t, *clearRedisCacheCalled)
		assert.False(t, updatedFlag.Bool.Value)
		assert.Greater(t, updatedFlag.UpdatedAt, boolFlag.UpdatedAt)
	})
	t.Run("rollout flag successful update", func(t *testing.T) {
		rolloutFlag, err := flagStore.CreateRollout(ctx, "update-test-rollout-flag", 42)
		require.NoError(t, err)
		const expectedValue = int32(1337)
		rolloutFlag.Rollout.Rollout = expectedValue
		clearRedisCacheCalled := setupClearRedisCacheTest(t, rolloutFlag.Name)
		updatedFlag, err := flagStore.UpdateFeatureFlag(ctx, rolloutFlag)
		require.NoError(t, err)
		require.True(t, *clearRedisCacheCalled)
		assert.Equal(t, expectedValue, updatedFlag.Rollout.Rollout)
		assert.Greater(t, updatedFlag.UpdatedAt, rolloutFlag.UpdatedAt)
	})
}
