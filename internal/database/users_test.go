package database

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	et "github.com/sourcegraph/sourcegraph/internal/encryption/testing"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// usernamesForTests is a list of test cases containing valid and invalid usernames.
var usernamesForTests = []struct {
	name      string
	wantValid bool
}{
	{"nick", true},
	{"n1ck", true},
	{"Nick2", true},
	{"N-S", true},
	{"nick-s", true},
	{"renfred-xh", true},
	{"renfred-x-h", true},
	{"deadmau5", true},
	{"deadmau-5", true},
	{"3blindmice", true},
	{"nick.com", true},
	{"nick.com.uk", true},
	{"nick.com-put-er", true},
	{"nick-", true},
	{"777", true},
	{"7-7", true},
	{"long-butnotquitelongenoughtoreachlimit", true},
	{"7_7", true},
	{"a_b", true},
	{"nick__bob", true},
	{"bob_", true},
	{"nick__", true},
	{"__nick", true},
	{"__-nick", true},

	{".nick", false},
	{"-nick", false},
	{"nick.", false},
	{"nick--s", false},
	{"nick--sny", false},
	{"nick..sny", false},
	{"nick.-sny", false},
	{"ke$ha", false},
	{"ni%k", false},
	{"#nick", false},
	{"@nick", false},
	{"", false},
	{"nick s", false},
	{" ", false},
	{"-", false},
	{"--", false},
	{"-s", false},
	{"レンフレッド", false},
	{"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", false},
}

func TestUsers_ValidUsernames(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	for _, test := range usernamesForTests {
		t.Run(test.name, func(t *testing.T) {
			valid := true
			if _, err := db.Users().Create(ctx, NewUser{Username: test.name}); err != nil {
				var e ErrCannotCreateUser
				if errors.As(err, &e) && (e.Code() == "users_username_max_length" || e.Code() == "users_username_valid_chars") {
					valid = false
				} else {
					t.Fatal(err)
				}
			}
			if valid != test.wantValid {
				t.Errorf("%q: got valid %v, want %v", test.name, valid, test.wantValid)
			}
		})
	}
}

func TestUsers_Create_SiteAdmin(t *testing.T) {
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	if _, err := db.GlobalState().Get(ctx); err != nil {
		t.Fatal(err)
	}

	// Create site admin.
	user, err := db.Users().Create(ctx, NewUser{
		Email:                 "a@a.com",
		Username:              "u",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !user.SiteAdmin {
		t.Fatal("!user.SiteAdmin")
	}
	ur, err := getUserRoles(ctx, db, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ur) != 2 {
		t.Fatalf("expected user to be assigned two roles (USER and SITE_ADMINISTRATOR), got %d", len(ur))
	}

	// Creating a non-site-admin now that the site has already been initialized.
	u2, err := db.Users().Create(ctx, NewUser{
		Email:                 "a2@a2.com",
		Username:              "u2",
		Password:              "p2",
		EmailVerificationCode: "c2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if u2.SiteAdmin {
		t.Fatal("want u2 not site admin because site is already initialized")
	}
	ur, err = getUserRoles(ctx, db, u2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ur) != 1 {
		t.Fatalf("expected user to be assigned one role, got %d", len(ur))
	}

	// Similar to the above, but expect an error because we pass FailIfNotInitialUser: true.
	_, err = db.Users().Create(ctx, NewUser{
		Email:                 "a3@a3.com",
		Username:              "u3",
		Password:              "p3",
		EmailVerificationCode: "c3",
		FailIfNotInitialUser:  true,
	})
	if want := (ErrCannotCreateUser{"site_already_initialized"}); !errors.Is(err, want) {
		t.Fatalf("got error %v, want %v", err, want)
	}

	// Delete the site admin.
	if err := db.Users().Delete(ctx, user.ID); err != nil {
		t.Fatal(err)
	}

	// Disallow creating a site admin when a user already exists (even if the site is not yet initialized).
	if _, err := db.ExecContext(ctx, "UPDATE site_config SET initialized=false"); err != nil {
		t.Fatal(err)
	}
	u4, err := db.Users().Create(ctx, NewUser{
		Email:                 "a4@a4.com",
		Username:              "u4",
		Password:              "p4",
		EmailVerificationCode: "c4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if u4.SiteAdmin {
		t.Fatal("want u4 not site admin because site is already initialized")
	}
	ur, err = getUserRoles(ctx, db, u4.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ur) != 1 {
		t.Fatalf("expected user to be assigned one role, got %d", len(ur))
	}

	// Similar to the above, but expect an error because we pass FailIfNotInitialUser: true.
	if _, err := db.ExecContext(ctx, "UPDATE site_config SET initialized=false"); err != nil {
		t.Fatal(err)
	}
	_, err = db.Users().Create(ctx, NewUser{
		Email:                 "a5@a5.com",
		Username:              "u5",
		Password:              "p5",
		EmailVerificationCode: "c5",
		FailIfNotInitialUser:  true,
	})
	if want := (ErrCannotCreateUser{"initial_site_admin_must_be_first_user"}); !errors.Is(err, want) {
		t.Fatalf("got error %v, want %v", err, want)
	}
}

func TestUsers_CheckAndDecrementInviteQuota(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{
		Email:                 "a@a.com",
		Username:              "u",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check default invite quota.
	var inviteQuota int
	row := db.QueryRowContext(ctx, "SELECT invite_quota FROM users WHERE id=$1", user.ID)
	if err := row.Scan(&inviteQuota); err != nil {
		t.Fatal(err)
	}
	// Check that it's within some reasonable bounds. The upper bound number here can increased
	// if we increase the default.
	if lo, hi := 0, 100; inviteQuota <= lo || inviteQuota > hi {
		t.Fatalf("got default user invite quota %d, want in [%d,%d)", inviteQuota, lo, hi)
	}

	// Decrementing should succeed while we have remaining quota. Keep going until we exhaust it.
	// Since the quota is fairly low, this isn't too slow.
	for inviteQuota > 0 {
		if ok, err := db.Users().CheckAndDecrementInviteQuota(ctx, user.ID); !ok || err != nil {
			t.Fatal("initial CheckAndDecrementInviteQuota failed:", err)
		}
		inviteQuota--
	}

	// Now our quota is exhausted, and CheckAndDecrementInviteQuota should fail.
	if ok, err := db.Users().CheckAndDecrementInviteQuota(ctx, user.ID); ok || err != nil {
		t.Fatalf("over-limit CheckAndDecrementInviteQuota #1: got error %v", err)
	}

	// Check again that we're still over quota, just in case.
	if ok, err := db.Users().CheckAndDecrementInviteQuota(ctx, user.ID); ok || err != nil {
		t.Fatalf("over-limit CheckAndDecrementInviteQuota #2: got error %v", err)
	}
}

func TestUsers_ListCount(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{
		Email:                 "a@a.com",
		Username:              "u",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	if count, err := db.Users().Count(ctx, &UsersListOptions{}); err != nil {
		t.Fatal(err)
	} else if want := 1; count != want {
		t.Errorf("got %d, want %d", count, want)
	}
	if users, err := db.Users().List(ctx, &UsersListOptions{}); err != nil {
		t.Fatal(err)
	} else if users, want := normalizeUsers(users), normalizeUsers([]*types.User{user}); !reflect.DeepEqual(users, want) {
		t.Errorf("got %+v, want %+v", users, want)
	}

	if count, err := db.Users().Count(ctx, &UsersListOptions{UserIDs: []int32{}}); err != nil {
		t.Fatal(err)
	} else if want := 0; count != want {
		t.Errorf("got %d, want %d", count, want)
	}
	if users, err := db.Users().List(ctx, &UsersListOptions{UserIDs: []int32{}}); err != nil {
		t.Fatal(err)
	} else if len(users) > 0 {
		t.Errorf("got %d, want empty", len(users))
	}

	if users, err := db.Users().List(ctx, &UsersListOptions{}); err != nil {
		t.Fatal(err)
	} else if users, want := normalizeUsers(users), normalizeUsers([]*types.User{user}); !reflect.DeepEqual(users, want) {
		t.Errorf("got %+v, want %+v", users[0], user)
	}

	// By usernames.
	if users, err := db.Users().List(ctx, &UsersListOptions{Usernames: []string{user.Username}}); err != nil {
		t.Fatal(err)
	} else if users, want := normalizeUsers(users), normalizeUsers([]*types.User{user}); !reflect.DeepEqual(users, want) {
		t.Errorf("got %+v, want %+v", users[0], user)
	}

	if err := db.Users().Delete(ctx, user.ID); err != nil {
		t.Fatal(err)
	}

	if count, err := db.Users().Count(ctx, &UsersListOptions{}); err != nil {
		t.Fatal(err)
	} else if want := 0; count != want {
		t.Errorf("got %d, want %d", count, want)
	}

	// Create three users with common Sourcegraph admin username patterns.
	for _, admin := range []struct {
		username string
		email    string
	}{
		{"sourcegraph-admin", "admin@sourcegraph.com"},
		{"sourcegraph-management-abc", "support@sourcegraph.com"},
		{"managed-abc", "abc-support@sourcegraph.com"},
	} {
		user, err := db.Users().Create(ctx, NewUser{Username: admin.username})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UserEmails().Add(ctx, user.ID, admin.email, nil); err != nil {
			t.Fatal(err)
		}
	}

	if count, err := db.Users().Count(ctx, &UsersListOptions{ExcludeSourcegraphAdmins: false}); err != nil {
		t.Fatal(err)
	} else if want := 3; count != want {
		t.Errorf("got %d, want %d", count, want)
	}

	if count, err := db.Users().Count(ctx, &UsersListOptions{ExcludeSourcegraphAdmins: true}); err != nil {
		t.Fatal(err)
	} else if want := 0; count != want {
		t.Errorf("got %d, want %d", count, want)
	}
	if users, err := db.Users().List(ctx, &UsersListOptions{ExcludeSourcegraphAdmins: true}); err != nil {
		t.Fatal(err)
	} else if len(users) > 0 {
		t.Errorf("got %d, want empty", len(users))
	}

	// Create a Sourcegraph Operator user and should be excluded when desired
	_, err = db.Users().CreateWithExternalAccount(
		ctx,
		NewUser{
			Username: "sourcegraph-operator-logan",
		},
		&extsvc.Account{
			AccountSpec: extsvc.AccountSpec{
				ServiceType: "sourcegraph-operator",
			},
		})
	require.NoError(t, err)
	count, err := db.Users().Count(
		ctx,
		&UsersListOptions{
			ExcludeSourcegraphAdmins:    true,
			ExcludeSourcegraphOperators: true,
		},
	)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	users, err := db.Users().List(
		ctx,
		&UsersListOptions{
			ExcludeSourcegraphAdmins:    true,
			ExcludeSourcegraphOperators: true,
		},
	)
	require.NoError(t, err)
	assert.Len(t, users, 0)
}

func TestUsers_List_Query(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	users := map[string]int32{}
	for _, u := range []string{
		"foo",
		"bar",
		"baz",
	} {
		user, err := db.Users().Create(ctx, NewUser{
			Email:                 u + "@a.com",
			Username:              u,
			Password:              "p",
			EmailVerificationCode: "c",
		})
		if err != nil {
			t.Fatal(err)
		}
		users[u] = user.ID
	}

	cases := []struct {
		Name  string
		Query string
		Want  string
	}{{
		Name:  "all",
		Query: "",
		Want:  "foo bar baz",
	}, {
		Name:  "none",
		Query: "sdfsdf",
		Want:  "",
	}, {
		Name:  "some",
		Query: "a",
		Want:  "bar baz",
	}, {
		Name:  "id",
		Query: strconv.Itoa(int(users["foo"])),
		Want:  "foo",
	}, {
		Name:  "graphqlid",
		Query: string(relay.MarshalID("User", users["foo"])),
		Want:  "foo",
	}}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			us, err := db.Users().List(ctx, &UsersListOptions{
				Query: tc.Query,
			})
			if err != nil {
				t.Fatal(err)
			}

			want := strings.Fields(tc.Want)
			got := []string{}
			for _, u := range us {
				got = append(got, u.Username)
			}

			sort.Strings(want)
			sort.Strings(got)

			assert.Equal(t, want, got)
		})
	}
}

func TestUsers_ListForSCIM_Query(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	userToSoftDelete := NewUserForSCIM{NewUser: NewUser{Email: "notactive@example.com", Username: "notactive", EmailIsVerified: true}, SCIMExternalID: "notactive"}
	// Create users
	newUsers := []NewUserForSCIM{
		{NewUser: NewUser{Email: "alice@example.com", Username: "alice", EmailIsVerified: true}},
		{NewUser: NewUser{Email: "bob@example.com", Username: "bob", EmailVerificationCode: "bb"}, SCIMExternalID: "BOB"},
		{NewUser: NewUser{Email: "charlie@example.com", Username: "charlie", EmailIsVerified: true}, SCIMExternalID: "CHARLIE", AdditionalVerifiedEmails: []string{"charlie2@example.com"}},
		userToSoftDelete,
	}
	for _, newUser := range newUsers {
		user, err := db.Users().CreateWithExternalAccount(ctx, newUser.NewUser,
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{ServiceType: "scim", AccountID: newUser.SCIMExternalID},
			})
		for _, email := range newUser.AdditionalVerifiedEmails {
			verificationCode := "x"
			err := db.UserEmails().Add(ctx, user.ID, email, &verificationCode)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.UserEmails().Verify(ctx, user.ID, email, verificationCode)
			if err != nil {
				t.Fatal(err)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	inactiveUser, err := db.Users().GetByUsername(ctx, userToSoftDelete.Username)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Users().Delete(ctx, inactiveUser.ID)
	if err != nil {
		t.Fatal(err)
	}

	users, err := db.Users().ListForSCIM(ctx, &UsersListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, users, 4)
	assert.Equal(t, "alice", users[0].Username)
	assert.Equal(t, "", users[0].SCIMExternalID)
	assert.Equal(t, "BOB", users[1].SCIMExternalID)
	assert.Equal(t, "CHARLIE", users[2].SCIMExternalID)
	assert.Equal(t, "notactive", users[3].Username)
	assert.Len(t, users[0].Emails, 1)
	assert.Len(t, users[1].Emails, 0)
	assert.Len(t, users[2].Emails, 2)
}

func TestUsers_Update(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{
		Email:                 "a@a.com",
		Username:              "u",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Users().Update(ctx, user.ID, UserUpdate{
		Username:    "u1",
		DisplayName: pointers.Ptr("d1"),
		AvatarURL:   pointers.Ptr("a1"),
	}); err != nil {
		t.Fatal(err)
	}
	user, err = db.Users().GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := "u1"; user.Username != want {
		t.Errorf("got username %q, want %q", user.Username, want)
	}
	if want := "d1"; user.DisplayName != want {
		t.Errorf("got display name %q, want %q", user.DisplayName, want)
	}
	if want := "a1"; user.AvatarURL != want {
		t.Errorf("got avatar URL %q, want %q", user.AvatarURL, want)
	}

	if err := db.Users().Update(ctx, user.ID, UserUpdate{
		DisplayName: pointers.Ptr(""),
	}); err != nil {
		t.Fatal(err)
	}
	user, err = db.Users().GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := "u1"; user.Username != want {
		t.Errorf("got username %q, want %q", user.Username, want)
	}
	if user.DisplayName != "" {
		t.Errorf("got display name %q, want nil", user.DisplayName)
	}
	if want := "a1"; user.AvatarURL != want {
		t.Errorf("got avatar URL %q, want %q", user.AvatarURL, want)
	}

	// Can't update to duplicate username.
	user2, err := db.Users().Create(ctx, NewUser{
		Email:                 "a2@a.com",
		Username:              "u2",
		Password:              "p2",
		EmailVerificationCode: "c2",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.Users().Update(ctx, user2.ID, UserUpdate{Username: "u1"})
	if diff := cmp.Diff(err.Error(), "Username is already in use."); diff != "" {
		t.Fatal(diff)
	}

	// Can't update nonexistent user.
	if err := db.Users().Update(ctx, 12345, UserUpdate{Username: "u12345"}); err == nil {
		t.Fatal("want error when updating nonexistent user")
	}
}

func TestUsers_GetByVerifiedEmail(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{
		Email:                 "a@a.com",
		Username:              "u",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Users().GetByVerifiedEmail(ctx, "a@a.com"); !errcode.IsNotFound(err) {
		t.Errorf("for unverified email, got error %v, want IsNotFound", err)
	}

	if err := db.UserEmails().SetVerified(ctx, user.ID, "a@a.com", true); err != nil {
		t.Fatal(err)
	}

	gotUser, err := db.Users().GetByVerifiedEmail(ctx, "a@a.com")
	if err != nil {
		t.Fatal(err)
	}
	if gotUser.ID != user.ID {
		t.Errorf("got user %d, want %d", gotUser.ID, user.ID)
	}
}

func TestUsers_GetByUsername(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	newUsers := []NewUser{
		{
			Email:           "alice@example.com",
			Username:        "alice",
			EmailIsVerified: true,
		},
		{
			Email:           "bob@example.com",
			Username:        "bob",
			EmailIsVerified: true,
		},
	}

	for _, newUser := range newUsers {
		_, err := db.Users().Create(ctx, newUser)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{"alice", "bob", "cindy"} {
		have, err := db.Users().GetByUsername(ctx, want)
		if want == "cindy" {
			// Make sure the returned err fulfils the NotFounder interface.
			if !errcode.IsNotFound(err) {
				t.Fatalf("invalid error, expected not found got %v", err)
			}
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		if have.Username != want {
			t.Errorf("got %s, but want %s", have.Username, want)
		}
	}
}

func TestUsers_GetByUsernames(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	newUsers := []NewUser{
		{
			Email:           "alice@example.com",
			Username:        "alice",
			EmailIsVerified: true,
		},
		{
			Email:           "bob@example.com",
			Username:        "bob",
			EmailIsVerified: true,
		},
	}

	for _, newUser := range newUsers {
		_, err := db.Users().Create(ctx, newUser)
		if err != nil {
			t.Fatal(err)
		}
	}

	users, err := db.Users().GetByUsernames(ctx, "alice", "bob", "cindy")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, but want 2", len(users))
	}
	for i := range users {
		if users[i].Username != newUsers[i].Username {
			t.Errorf("got %s, but want %s", users[i].Username, newUsers[i].Username)
		}
	}
}

func TestUsers_Delete(t *testing.T) {
	t.Skip() // Flaky

	for name, hard := range map[string]bool{"soft": false, "hard": true} {
		hard := hard // fix for loop closure
		t.Run(name, func(t *testing.T) {
			if testing.Short() {
				t.Skip()
			}
			t.Parallel()
			logger := logtest.Scoped(t)
			db := NewDB(logger, dbtest.NewDB(t))
			ctx := context.Background()
			ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})

			otherUser, err := db.Users().Create(ctx, NewUser{Username: "other"})
			if err != nil {
				t.Fatal(err)
			}

			user, err := db.Users().Create(ctx, NewUser{
				Email:                 "a@a.com",
				Username:              "u",
				Password:              "p",
				EmailVerificationCode: "c",
			})
			if err != nil {
				t.Fatal(err)
			}

			// Create settings for the user, and for another user authored by this user.
			if _, err := db.Settings().CreateIfUpToDate(ctx, api.SettingsSubject{User: &user.ID}, nil, &user.ID, "{}"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Settings().CreateIfUpToDate(ctx, api.SettingsSubject{User: &otherUser.ID}, nil, &user.ID, "{}"); err != nil {
				t.Fatal(err)
			}

			// Create a repository to comply with the postgres repo constraint.
			if err := upsertRepo(ctx, db, InsertRepoOp{Name: "myrepo", Description: "", Fork: false}); err != nil {
				t.Fatal(err)
			}

			// Create a saved search owned by the user.
			if _, err := db.SavedSearches().Create(ctx, &types.SavedSearch{
				Description: "desc",
				Query:       "foo",
				Owner:       types.NamespaceUser(user.ID),
			}); err != nil {
				t.Fatal(err)
			}

			// Create an event log
			//lint:ignore SA1019 existing usage of deprecated functionality. Use EventRecorder from internal/telemetryrecorder instead.
			err = db.EventLogs().Insert(ctx, &Event{
				Name:            "something",
				URL:             "http://example.com",
				UserID:          uint32(user.ID),
				AnonymousUserID: "",
				Source:          "Test",
				Timestamp:       time.Now(),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Create and update a webhook
			webhook, err := db.Webhooks(nil).Create(ctx, "github webhook", extsvc.KindGitHub, testURN, user.ID, types.NewUnencryptedSecret("testSecret"))
			if err != nil {
				t.Fatal(err)
			}

			if hard {
				// Hard delete user.
				if err := db.Users().HardDelete(ctx, user.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				// Delete user.
				if err := db.Users().Delete(ctx, user.ID); err != nil {
					t.Fatal(err)
				}
			}

			// User no longer exists.
			_, err = db.Users().GetByID(ctx, user.ID)
			if !errcode.IsNotFound(err) {
				t.Errorf("got error %v, want ErrUserNotFound", err)
			}
			users, err := db.Users().List(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(users) > 1 {
				// The otherUser should still exist, which is why we check for 1 not 0.
				t.Errorf("got %d users, want 1", len(users))
			}

			// User's settings no longer exist.
			if settings, err := db.Settings().GetLatest(ctx, api.SettingsSubject{User: &user.ID}); err != nil {
				t.Error(err)
			} else if settings != nil {
				t.Errorf("got settings %+v, want nil", settings)
			}
			// Settings authored by user still exist but have nil author.
			if settings, err := db.Settings().GetLatest(ctx, api.SettingsSubject{User: &otherUser.ID}); err != nil {
				t.Fatal(err)
			} else if settings.AuthorUserID != nil {
				t.Errorf("got author %v, want nil", *settings.AuthorUserID)
			}

			// Can't delete already-deleted user.
			err = db.Users().Delete(ctx, user.ID)
			if !errcode.IsNotFound(err) {
				t.Errorf("got error %v, want ErrUserNotFound", err)
			}

			// Check event logs
			eventLogs, err := db.EventLogs().ListAll(ctx, EventLogsListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(eventLogs) != 1 {
				t.Fatal("Expected 1 event log")
			}
			eventLog := eventLogs[0]
			if hard {
				// Event logs should now be anonymous
				if eventLog.UserID != 0 {
					t.Error("After hard delete user id should be 0")
				}
				if len(eventLog.AnonymousUserID) == 0 {
					t.Error("After hard anonymous user id should not be blank")
				}
				// Webhooks `created_by_user_id` and `updated_by_user_id` should be NULL
				webhook, err = db.Webhooks(nil).GetByID(ctx, webhook.ID)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, int32(0), webhook.CreatedByUserID)
				assert.Equal(t, int32(0), webhook.UpdatedByUserID)
			} else {
				// Event logs are unchanged
				if int32(eventLog.UserID) != user.ID {
					t.Error("After soft delete user id should be non zero")
				}
				if len(eventLog.AnonymousUserID) != 0 {
					t.Error("After soft delete anonymous user id should be blank")
				}
				// Webhooks `created_by_user_id` and `updated_by_user_id` are unchanged
				webhook, err = db.Webhooks(nil).GetByID(ctx, webhook.ID)
				if err != nil {
					t.Fatal(err)
				}
				assert.Equal(t, user.ID, webhook.CreatedByUserID)
				assert.Equal(t, user.ID, webhook.UpdatedByUserID)
			}
		})
	}
}

func TestUsers_RecoverUsers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1, Internal: true})

	user, err := db.Users().Create(ctx, NewUser{
		Email:                 "a@a.com",
		Username:              "u",
		Password:              "p",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	otherUser, err := db.Users().Create(ctx, NewUser{Username: "other"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.UserExternalAccounts().Upsert(ctx,
		&extsvc.Account{
			UserID: otherUser.ID,
			AccountSpec: extsvc.AccountSpec{
				ServiceType: "github",
				ServiceID:   "https://github.com/",
				AccountID:   "alice_github",
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	// Test reviving a user that does not exist
	t.Run("fails on nonexistent user", func(t *testing.T) {
		ru, err := db.Users().RecoverUsersList(ctx, []int32{65})
		if err != nil {
			t.Errorf("got err %v, want nil", err)
		}
		if len(ru) != 0 {
			t.Errorf("got %d recovered users, want 0", len(ru))
		}
	})
	// Test reviving a user that does exist and hasn't not been deleted
	t.Run("fails on non-deleted user", func(t *testing.T) {
		ru, err := db.Users().RecoverUsersList(ctx, []int32{user.ID})
		if err == nil {
			t.Errorf("got err %v, want nil", err)
		}
		if len(ru) != 0 {
			t.Errorf("got %d users, want 0", len(ru))
		}
	})

	// Test reviving a user that does exist and does not have additional resources deleted in the same timeframe
	t.Run("revives user with no additional resources", func(t *testing.T) {
		err := db.Users().Delete(ctx, user.ID)
		if err != nil {
			t.Errorf("got err %v, want nil", err)
		}
		ru, err := db.Users().RecoverUsersList(ctx, []int32{user.ID})
		if err != nil {
			t.Fatalf("got err %v, want nil", err)
		}
		if len(ru) != 1 {
			t.Fatalf("got %d users, want 1", len(ru))
		}
		if ru[0] != user.ID {
			t.Errorf("got user %d, want %d", ru[0], user.ID)
		}

		users, err := db.Users().List(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(users) > 2 {
			// The otherUser should still exist, which is why we check for 1 not 0.
			t.Errorf("got %d users, want 1", len(users))
		}
	})
	// Test reviving a user that does exist and does have additional resources deleted in the same timeframe
	t.Run("revives user and additional resources", func(t *testing.T) {
		err := db.Users().Delete(ctx, otherUser.ID)
		if err != nil {
			t.Errorf("got err %v, want nil", err)
		}

		_, err = db.UserExternalAccounts().Get(ctx, otherUser.ID)
		if err == nil {
			t.Fatal("got err nil, want non-nil")
		}

		ru, err := db.Users().RecoverUsersList(ctx, []int32{otherUser.ID})
		if err != nil {
			t.Errorf("got err %v, want nil", err)
		}
		if len(ru) != 1 {
			t.Errorf("got %d users, want 1", len(ru))
		}
		if ru[0] != otherUser.ID {
			t.Errorf("got user %d, want %d", ru[0], otherUser.ID)
		}

		extAcc, err := db.UserExternalAccounts().Get(ctx, 1)
		if err != nil {
			t.Fatal("got err nil, want non-nil")
		}
		if extAcc.UserID != otherUser.ID {
			t.Errorf("got user %d, want %d", extAcc.UserID, otherUser.ID)
		}

		users, err := db.Users().List(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(users) > 2 {
			t.Errorf("got %d users, want 2", len(users))
		}
	})
}

func TestUsers_InvalidateSessions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	newUsers := []NewUser{
		{
			Email:           "alice@example.com",
			Username:        "alice",
			EmailIsVerified: true,
		},
		{
			Email:           "bob@example.com",
			Username:        "bob",
			EmailIsVerified: true,
		},
	}

	for _, newUser := range newUsers {
		_, err := db.Users().Create(ctx, newUser)
		if err != nil {
			t.Fatal(err)
		}
	}

	users, err := db.Users().GetByUsernames(ctx, "alice", "bob")
	if err != nil {
		t.Fatal(err)
	}

	if len(users) != 2 {
		t.Fatalf("got %d users, but want 2", len(users))
	}
	for i := range users {
		if err := db.Users().InvalidateSessionsByID(ctx, users[i].ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUsers_SetIsSiteAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	adminUser, err := db.Users().Create(ctx, NewUser{Username: "u"})
	if err != nil {
		t.Fatal(err)
	}
	// Create user. This user will have a `SiteAdmin` value of false because
	// Global state hasn't been initialized at this point, so technically this is the
	// first user.
	if !adminUser.SiteAdmin {
		t.Fatalf("expected site admin to be created")
	}

	regularUser, err := db.Users().Create(ctx, NewUser{Username: "u2"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("revoking site admin role for a site admin", func(t *testing.T) {
		// Confirm that the user has only two roles assigned to them.
		ur, err := db.UserRoles().GetByUserID(ctx, GetUserRoleOpts{UserID: adminUser.ID})
		require.NoError(t, err)
		require.Len(t, ur, 2)

		err = db.Users().SetIsSiteAdmin(ctx, adminUser.ID, false)
		require.NoError(t, err)

		// check that site admin role has been revoked for user
		ur, err = db.UserRoles().GetByUserID(ctx, GetUserRoleOpts{UserID: adminUser.ID})
		require.NoError(t, err)
		// Since we've revoked the SITE_ADMINISTRATOR role, the user should still have the
		// USER role assigned to them.
		require.Len(t, ur, 1)

		u, err := db.Users().GetByID(ctx, regularUser.ID)
		require.NoError(t, err)
		require.False(t, u.SiteAdmin)
	})

	t.Run("promoting a regular user to site admin", func(t *testing.T) {
		// Confirm that the user has only one role assigned to them.
		ur, err := db.UserRoles().GetByUserID(ctx, GetUserRoleOpts{UserID: regularUser.ID})
		require.NoError(t, err)
		require.Len(t, ur, 1)

		err = db.Users().SetIsSiteAdmin(ctx, regularUser.ID, true)
		require.NoError(t, err)

		// check that site admin role has been assigned to user
		ur, err = db.UserRoles().GetByUserID(ctx, GetUserRoleOpts{UserID: regularUser.ID})
		require.NoError(t, err)
		// The user should have both USER role and SITE_ADMINISTRATOR role assigned to them.
		require.Len(t, ur, 2)

		u, err := db.Users().GetByID(ctx, regularUser.ID)
		require.NoError(t, err)
		require.True(t, u.SiteAdmin)
	})
}

func TestUsers_GetSetChatCompletionsQuota(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{
		Email:           "alice@example.com",
		Username:        "alice",
		EmailIsVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initially, no quota should be set and nil should be returned.
	{
		quota, err := db.Users().GetChatCompletionsQuota(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, quota, "expected unconfigured quota to be nil")
	}

	// Set a quota. Expect it to be returned correctly.
	{
		wantQuota := 10
		err := db.Users().SetChatCompletionsQuota(ctx, user.ID, &wantQuota)
		if err != nil {
			t.Fatal(err)
		}

		quota, err := db.Users().GetChatCompletionsQuota(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		require.NotNil(t, quota, "expected quota to be non-nil after storing")
		require.Equal(t, wantQuota, *quota, "invalid quota returned")
	}

	// Now unset the quota.
	{
		err := db.Users().SetChatCompletionsQuota(ctx, user.ID, nil)
		if err != nil {
			t.Fatal(err)
		}

		quota, err := db.Users().GetChatCompletionsQuota(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, quota, "expected unconfigured quota to be nil")
	}
}

func TestUsers_GetSetCodeCompletionsQuota(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{
		Email:           "alice@example.com",
		Username:        "alice",
		EmailIsVerified: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initially, no quota should be set and nil should be returned.
	{
		quota, err := db.Users().GetCodeCompletionsQuota(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, quota, "expected unconfigured quota to be nil")
	}

	// Set a quota. Expect it to be returned correctly.
	{
		wantQuota := 10
		err := db.Users().SetCodeCompletionsQuota(ctx, user.ID, &wantQuota)
		if err != nil {
			t.Fatal(err)
		}

		quota, err := db.Users().GetCodeCompletionsQuota(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		require.NotNil(t, quota, "expected quota to be non-nil after storing")
		require.Equal(t, wantQuota, *quota, "invalid quota returned")
	}

	// Now unset the quota.
	{
		err := db.Users().SetCodeCompletionsQuota(ctx, user.ID, nil)
		if err != nil {
			t.Fatal(err)
		}

		quota, err := db.Users().GetCodeCompletionsQuota(ctx, user.ID)
		if err != nil {
			t.Fatal(err)
		}
		require.Nil(t, quota, "expected unconfigured quota to be nil")
	}
}

func TestUsers_CreateWithExternalAccount(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	spec := extsvc.AccountSpec{
		ServiceType: "xa",
		ServiceID:   "xb",
		ClientID:    "xc",
		AccountID:   "xd",
	}

	authData := json.RawMessage(`"authData"`)
	data := json.RawMessage(`"data"`)
	accountData := extsvc.AccountData{
		AuthData: extsvc.NewUnencryptedData(authData),
		Data:     extsvc.NewUnencryptedData(data),
	}
	user, err := db.Users().CreateWithExternalAccount(ctx, NewUser{Username: "u"}, &extsvc.Account{AccountSpec: spec, AccountData: accountData})
	if err != nil {
		t.Fatal(err)
	}
	if want := "u"; user.Username != want {
		t.Errorf("got %q, want %q", user.Username, want)
	}

	accounts, err := db.UserExternalAccounts().List(ctx, ExternalAccountsListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got len(accounts) == %d, want 1", len(accounts))
	}
	account := accounts[0]
	simplifyExternalAccount(account)
	account.ID = 0

	want := &extsvc.Account{
		UserID:      user.ID,
		AccountSpec: spec,
		AccountData: accountData,
	}
	if diff := cmp.Diff(want, account, et.CompareEncryptable); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}

	userRoles, err := db.UserRoles().GetByUserID(ctx, GetUserRoleOpts{
		UserID: user.ID,
	})
	require.NoError(t, err)
	// Both USER and SITE_ADMINISTRATOR role have been assigned.
	require.Len(t, userRoles, 2)
}

func TestUsers_CreateWithExternalAccount_NilData(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	spec := extsvc.AccountSpec{
		ServiceType: "xa",
		ServiceID:   "xb",
		ClientID:    "xc",
		AccountID:   "xd",
	}

	user, err := db.Users().CreateWithExternalAccount(ctx, NewUser{Username: "u"}, &extsvc.Account{AccountSpec: spec})
	if err != nil {
		t.Fatal(err)
	}
	if want := "u"; user.Username != want {
		t.Errorf("got %q, want %q", user.Username, want)
	}

	accounts, err := db.UserExternalAccounts().List(ctx, ExternalAccountsListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got len(accounts) == %d, want 1", len(accounts))
	}
	account := accounts[0]
	simplifyExternalAccount(account)
	account.ID = 0

	want := &extsvc.Account{
		UserID:      user.ID,
		AccountSpec: spec,
	}
	if diff := cmp.Diff(want, account, et.CompareEncryptable); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}
}

func TestUsers_CreateCancelAccessRequest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	usersStore := db.Users()
	accessRequestsStore := db.AccessRequests()

	_, err := accessRequestsStore.Create(ctx, &types.AccessRequest{
		Email:          "a123@email.com",
		Name:           "a123",
		AdditionalInfo: "info1",
	})
	require.NoError(t, err)

	_, err = usersStore.Create(ctx, NewUser{Username: "u1ted", Email: "a123@email.com", EmailVerificationCode: "e"})
	require.NoError(t, err)

	updated, _ := accessRequestsStore.GetByEmail(ctx, "a123@email.com")

	assert.Equal(t, updated.Status, types.AccessRequestStatusCanceled)
}

func TestUserNotFoundFulfillsNotFound(t *testing.T) {
	err := NewUserNotFoundError(123)
	require.True(t, errcode.IsNotFound(err))
}

func normalizeUsers(users []*types.User) []*types.User {
	for _, u := range users {
		u.CreatedAt = u.CreatedAt.Local().Round(time.Second)
		u.UpdatedAt = u.UpdatedAt.Local().Round(time.Second)
		u.InvalidatedSessionsAt = u.InvalidatedSessionsAt.Local().Round(time.Second)
	}
	return users
}

func getUserRoles(ctx context.Context, db DB, userID int32) ([]*types.UserRole, error) {
	return db.UserRoles().GetByUserID(ctx, GetUserRoleOpts{UserID: userID})
}
