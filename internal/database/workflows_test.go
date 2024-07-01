package database

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestWorkflowsCreate(t *testing.T) {
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

	input := types.Workflow{
		Name:         "n",
		Description:  "d",
		TemplateText: "q",
		Draft:        true,
		Owner:        types.NamespaceUser(user.ID),
	}
	got, err := db.Workflows().Create(ctx, &input, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := input
	want.ID = got.ID
	want.CreatedByUser = &user.ID
	want.UpdatedByUser = &user.ID
	normalizeWorkflows(got, &want)
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("got %+v, want %+v", *got, want)
	}
}

func TestWorkflowsUpdate(t *testing.T) {
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

	_, err = db.Workflows().Create(ctx, &types.Workflow{
		Name:         "n",
		TemplateText: "q",
		Owner:        types.NamespaceUser(user.ID),
	}, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	update := types.Workflow{
		ID:           1,
		Name:         "n2",
		TemplateText: "q2",
	}
	got, err := db.Workflows().Update(ctx, &update, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := update
	want.Owner = types.NamespaceUser(user.ID)
	want.CreatedByUser = &user.ID
	want.UpdatedByUser = &user.ID
	normalizeWorkflows(got, &want)
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("got %+v, want %+v", *got, want)
	}
}

func TestWorkflowsDelete(t *testing.T) {
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

	fixture1, err := db.Workflows().Create(ctx, &types.Workflow{
		Name:         "n",
		TemplateText: "q",
		Owner:        types.NamespaceUser(user.ID),
	}, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Workflows().Delete(ctx, fixture1.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Workflows().Count(ctx, WorkflowListArgs{}); err != nil {
		t.Fatal(err)
	} else if got != 0 {
		t.Error()
	}
}

func TestWorkflowsGetByID(t *testing.T) {
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

	input := types.Workflow{
		Name:         "n",
		TemplateText: "q",
		Owner:        types.NamespaceUser(user.ID),
	}
	fixture1, err := db.Workflows().Create(ctx, &input, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.Workflows().GetByID(ctx, fixture1.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := input
	want.ID = got.ID
	want.CreatedByUser = &user.ID
	want.UpdatedByUser = &user.ID
	normalizeWorkflows(got, &want)
	if diff := cmp.Diff(want, *got); diff != "" {
		t.Fatalf("Mismatch (-want +got):\n%s", diff)
	}
}

func TestWorkflows_ListCount(t *testing.T) {
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

	fixture1, err := db.Workflows().Create(ctx, &types.Workflow{
		Name:  "fixture1",
		Owner: types.NamespaceUser(user.ID),
	}, user.ID)
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
	fixture2, err := db.Workflows().Create(ctx, &types.Workflow{
		Name:  "fixture2",
		Owner: types.NamespaceOrg(org1.ID),
	}, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture3, err := db.Workflows().Create(ctx, &types.Workflow{
		Name:  "fixture3",
		Owner: types.NamespaceOrg(org2.ID),
	}, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = db.OrgMembers().Create(ctx, org1.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	fixture4, err := db.Workflows().Create(ctx, &types.Workflow{
		Name:  "fixture4",
		Draft: true,
		Owner: types.NamespaceUser(user.ID),
	}, user.ID)
	if err != nil {
		t.Fatal(err)
	}

	testListCount := func(t *testing.T, args WorkflowListArgs, want []*types.Workflow) {
		t.Helper()

		got, err := db.Workflows().List(ctx, args, &PaginationArgs{Ascending: true})
		if err != nil {
			t.Fatal(err)
		}
		normalizeWorkflows(got...)
		normalizeWorkflows(want...)
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}

		gotCount, err := db.Workflows().Count(ctx, args)
		if err != nil {
			t.Fatal(err)
		}
		if wantCount := len(want); gotCount != wantCount {
			t.Errorf("got count %d, want %d", gotCount, wantCount)
		}
	}

	t.Run("list all", func(t *testing.T) {
		testListCount(t, WorkflowListArgs{}, []*types.Workflow{fixture1, fixture2, fixture3, fixture4})
	})

	t.Run("query", func(t *testing.T) {
		testListCount(t, WorkflowListArgs{Query: "u/fiXTUre1"}, []*types.Workflow{fixture1})
	})

	t.Run("list owned by user", func(t *testing.T) {
		userNS := types.NamespaceUser(user.ID)
		testListCount(t, WorkflowListArgs{Owner: &userNS}, []*types.Workflow{fixture1, fixture4})
	})

	t.Run("list owned by nonexistent user", func(t *testing.T) {
		userNS := types.NamespaceUser(1234999 /* user doesn't exist */)
		testListCount(t, WorkflowListArgs{Owner: &userNS}, nil)
	})

	t.Run("list owned by org1", func(t *testing.T) {
		orgNS := types.NamespaceOrg(org1.ID)
		testListCount(t, WorkflowListArgs{Owner: &orgNS}, []*types.Workflow{fixture2})
	})

	t.Run("affiliated with user", func(t *testing.T) {
		testListCount(t, WorkflowListArgs{AffiliatedUser: &user.ID}, []*types.Workflow{fixture1, fixture2, fixture4})
	})

	t.Run("hide drafts", func(t *testing.T) {
		userNS := types.NamespaceUser(user.ID)
		testListCount(t, WorkflowListArgs{Owner: &userNS, HideDrafts: true}, []*types.Workflow{fixture1})
	})

	t.Run("order by", func(t *testing.T) {
		testListCount(t, WorkflowListArgs{
			OrderBy: WorkflowsOrderByUpdatedAt,
		}, []*types.Workflow{fixture4, fixture3, fixture2, fixture1})
	})
}

func normalizeWorkflows(ws ...*types.Workflow) {
	for _, w := range ws {
		w.CreatedAt = time.Time{}
		w.UpdatedAt = time.Time{}
	}
	return
}
