package database

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestSavedSearchesCreate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{Username: "u"})
	if err != nil {
		t.Fatal(err)
	}

	input := &types.SavedSearch{
		Description: "d",
		Query:       "q",
		Owner:       types.NamespaceUser(user.ID),
	}
	created, err := db.SavedSearches().Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.ID = created.ID
	if want := input; !reflect.DeepEqual(created, want) {
		t.Errorf("got %+v, want %+v", created, want)
	}
}

func TestSavedSearchesUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{Username: "u"})
	if err != nil {
		t.Fatal(err)
	}

	fake := &types.SavedSearch{
		Description: "d",
		Query:       "q",
		Owner:       types.NamespaceUser(user.ID),
	}
	_, err = db.SavedSearches().Create(ctx, fake)
	if err != nil {
		t.Fatal(err)
	}

	updated := &types.SavedSearch{
		ID:          1,
		Description: "test2",
		Query:       "test2",
		Owner:       types.NamespaceUser(user.ID),
	}
	got, err := db.SavedSearches().Update(ctx, updated)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, updated) {
		t.Errorf("got %+v, want %+v", got, updated)
	}
}

func TestSavedSearchesDelete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{Username: "u"})
	if err != nil {
		t.Fatal(err)
	}

	fixture1, err := db.SavedSearches().Create(ctx, &types.SavedSearch{
		Description: "d",
		Query:       "q",
		Owner:       types.NamespaceUser(user.ID),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := db.SavedSearches().Delete(ctx, fixture1.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.SavedSearches().Count(ctx, SavedSearchListArgs{}); err != nil {
		t.Fatal(err)
	} else if got != 0 {
		t.Error()
	}
}

func TestSavedSearchesGetByID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{Username: "u"})
	if err != nil {
		t.Fatal(err)
	}

	input := &types.SavedSearch{
		Description: "d",
		Query:       "q",
		Owner:       types.NamespaceUser(user.ID),
	}
	fixture1, err := db.SavedSearches().Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.SavedSearches().GetByID(ctx, fixture1.ID)
	if err != nil {
		t.Fatal(err)
	}
	input.ID = got.ID
	want := input
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}
}

func TestSavedSearches_ListCount(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()
	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	user, err := db.Users().Create(ctx, NewUser{Username: "u"})
	if err != nil {
		t.Fatal(err)
	}

	fixture1, err := db.SavedSearches().Create(ctx, &types.SavedSearch{
		Description: "fixture1",
		Query:       "fixture1",
		Owner:       types.NamespaceUser(user.ID),
	})
	if err != nil {
		t.Fatal(err)
	}

	org1, err := db.Orgs().Create(ctx, "org1", nil)
	if err != nil {
		t.Fatal(err)
	}
	org2, err := db.Orgs().Create(ctx, "org2", nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture2, err := db.SavedSearches().Create(ctx, &types.SavedSearch{
		Description: "fixture2",
		Query:       "fixture2",
		Owner:       types.NamespaceOrg(org1.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture3, err := db.SavedSearches().Create(ctx, &types.SavedSearch{
		Description: "fixture3",
		Query:       "fixture3",
		Owner:       types.NamespaceOrg(org2.ID),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = db.OrgMembers().Create(ctx, org1.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	testListCount := func(t *testing.T, args SavedSearchListArgs, want []*types.SavedSearch) {
		t.Helper()

		got, err := db.SavedSearches().List(ctx, args, &PaginationArgs{Ascending: true})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}

		gotCount, err := db.SavedSearches().Count(ctx, args)
		if err != nil {
			t.Fatal(err)
		}
		if wantCount := len(want); gotCount != wantCount {
			t.Errorf("got count %d, want %d", gotCount, wantCount)
		}
	}

	t.Run("list all", func(t *testing.T) {
		testListCount(t, SavedSearchListArgs{}, []*types.SavedSearch{fixture1, fixture2, fixture3})
	})

	t.Run("list owned by user", func(t *testing.T) {
		userNS := types.NamespaceUser(user.ID)
		testListCount(t, SavedSearchListArgs{Owner: &userNS}, []*types.SavedSearch{fixture1})
	})

	t.Run("list owned by nonexistent user", func(t *testing.T) {
		userNS := types.NamespaceUser(1234999 /* user doesn't exist */)
		testListCount(t, SavedSearchListArgs{Owner: &userNS}, nil)
	})

	t.Run("list owned by org1", func(t *testing.T) {
		orgNS := types.NamespaceOrg(org1.ID)
		testListCount(t, SavedSearchListArgs{Owner: &orgNS}, []*types.SavedSearch{fixture2})
	})

	t.Run("affiliated with user", func(t *testing.T) {
		testListCount(t, SavedSearchListArgs{AffiliatedUser: &user.ID}, []*types.SavedSearch{fixture1, fixture2})
	})
}
