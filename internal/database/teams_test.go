package database

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestTeams_CreateUpdateDelete(t *testing.T) {
	ctx := actor.WithInternalActor(context.Background())
	logger := logtest.NoOp(t)
	db := NewDB(logger, dbtest.NewDB(t))
	user, err := db.Users().Create(ctx, NewUser{Username: "johndoe"})
	if err != nil {
		t.Fatal(err)
	}

	store := db.Teams()

	team := &types.Team{
		Name:        "own",
		DisplayName: "Sourcegraph Own",
		ReadOnly:    true,
		CreatorID:   user.ID,
	}
	if _, err := store.CreateTeam(ctx, team); err != nil {
		t.Fatal(err)
	}

	member := &types.TeamMember{TeamID: team.ID, UserID: user.ID}

	t.Run("create/remove team member", func(t *testing.T) {
		if err := store.CreateTeamMember(ctx, member); err != nil {
			t.Fatal(err)
		}

		// Should not allow a second insert
		if err := store.CreateTeamMember(ctx, member); err != nil {
			t.Fatal("error for reinsert")
		}

		if err := store.DeleteTeamMember(ctx, member); err != nil {
			t.Fatal(err)
		}

		// Should allow a second delete without side-effects
		if err := store.DeleteTeamMember(ctx, member); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("duplicate team names are forbidden", func(t *testing.T) {
		_, err := store.CreateTeam(ctx, team)
		if err == nil {
			t.Fatal("got no error")
		}
		if !errors.Is(err, ErrTeamNameAlreadyExists) {
			t.Fatalf("invalid err returned %v", err)
		}
	})

	t.Run("duplicate names with users are forbidden", func(t *testing.T) {
		tm := &types.Team{
			Name:      user.Username,
			CreatorID: user.ID,
		}
		_, err := store.CreateTeam(ctx, tm)
		if err == nil {
			t.Fatal("got no error")
		}
		if !errors.Is(err, ErrTeamNameAlreadyExists) {
			t.Fatalf("invalid err returned %v", err)
		}
	})

	t.Run("duplicate names with orgs are forbidden", func(t *testing.T) {
		name := "theorg"
		_, err := db.Orgs().Create(ctx, name, nil)
		if err != nil {
			t.Fatal(err)
		}

		tm := &types.Team{
			Name:      name,
			CreatorID: user.ID,
		}
		_, err = store.CreateTeam(ctx, tm)
		if err == nil {
			t.Fatal("got no error")
		}
		if !errors.Is(err, ErrTeamNameAlreadyExists) {
			t.Fatalf("invalid err returned %v", err)
		}
	})

	t.Run("update", func(t *testing.T) {
		otherTeam := &types.Team{Name: "own2", CreatorID: user.ID}
		_, err := store.CreateTeam(ctx, otherTeam)
		if err != nil {
			t.Fatal(err)
		}
		team.DisplayName = ""
		team.ParentTeamID = otherTeam.ID
		if err := store.UpdateTeam(ctx, team); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, otherTeam.ID, team.ParentTeamID)
		// Should be properly unset in the DB.
		require.Equal(t, "", team.DisplayName)
	})

	t.Run("delete", func(t *testing.T) {
		if err := store.DeleteTeam(ctx, team.ID); err != nil {
			t.Fatal(err)
		}
		_, err = store.GetTeamByID(ctx, team.ID)
		if err == nil {
			t.Fatal("team not deleted")
		}
		var tnfe TeamNotFoundError
		if !errors.As(err, &tnfe) {
			t.Fatalf("invalid error returned, expected not found got %v", err)
		}

		// Check that we cannot delete the team a second time without error.
		err = store.DeleteTeam(ctx, team.ID)
		if err == nil {
			t.Fatal("team deleted twice")
		}
		if !errors.As(err, &tnfe) {
			t.Fatalf("invalid error returned, expected not found got %v", err)
		}

		// Check that we can create a new team with the same name now.
		_, err := store.CreateTeam(ctx, team)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestTeams_GetListCount(t *testing.T) {
	internalCtx := actor.WithInternalActor(context.Background())
	logger := logtest.NoOp(t)
	db := NewDB(logger, dbtest.NewDB(t))
	johndoe, err := db.Users().Create(internalCtx, NewUser{Username: "johndoe"})
	if err != nil {
		t.Fatal(err)
	}
	alice, err := db.Users().Create(internalCtx, NewUser{Username: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	alex, err := db.Users().Create(internalCtx, NewUser{Username: "alex"})
	if err != nil {
		t.Fatal(err)
	}

	store := db.Teams()

	createTeam := func(team *types.Team, members ...int32) *types.Team {
		team.CreatorID = johndoe.ID
		if _, err := store.CreateTeam(internalCtx, team); err != nil {
			t.Fatal(err)
		}
		for _, m := range members {
			if err := store.CreateTeamMember(internalCtx, &types.TeamMember{TeamID: team.ID, UserID: m}); err != nil {
				t.Fatal(err)
			}
		}
		return team
	}

	engineeringTeam := createTeam(&types.Team{Name: "engineering"}, johndoe.ID)
	salesTeam := createTeam(&types.Team{Name: "sales"})
	supportTeam := createTeam(&types.Team{Name: "support"}, johndoe.ID)
	ownTeam := createTeam(&types.Team{Name: "sgown", ParentTeamID: engineeringTeam.ID}, alice.ID, alex.ID)
	batchesTeam := createTeam(&types.Team{Name: "batches", ParentTeamID: engineeringTeam.ID}, johndoe.ID, alice.ID)

	t.Run("GetByID", func(t *testing.T) {
		for _, want := range []*types.Team{engineeringTeam, salesTeam, supportTeam, ownTeam, batchesTeam} {
			have, err := store.GetTeamByID(internalCtx, want.ID)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, have); diff != "" {
				t.Fatal(diff)
			}
		}
		t.Run("not found error", func(t *testing.T) {
			_, err := store.GetTeamByID(internalCtx, 100000)
			if err == nil {
				t.Fatal("no error for not found team")
			}
			var tnfe TeamNotFoundError
			if !errors.As(err, &tnfe) {
				t.Fatalf("invalid error returned, expected not found got %v", err)
			}
		})
	})

	t.Run("GetByName", func(t *testing.T) {
		for _, want := range []*types.Team{engineeringTeam, salesTeam, supportTeam, ownTeam, batchesTeam} {
			have, err := store.GetTeamByName(internalCtx, want.Name)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, have); diff != "" {
				t.Fatal(diff)
			}
		}
		t.Run("not found error", func(t *testing.T) {
			_, err := store.GetTeamByName(internalCtx, "definitelynotateam")
			if err == nil {
				t.Fatal("no error for not found team")
			}
			var tnfe TeamNotFoundError
			if !errors.As(err, &tnfe) {
				t.Fatalf("invalid error returned, expected not found got %v", err)
			}
		})
	})

	t.Run("ListCountTeams", func(t *testing.T) {
		allTeams := []*types.Team{engineeringTeam, salesTeam, supportTeam, ownTeam, batchesTeam}

		// Get all.
		haveTeams, haveCursor, err := store.ListTeams(internalCtx, ListTeamsOpts{})
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(allTeams, haveTeams); diff != "" {
			t.Fatal(diff)
		}

		if haveCursor != 0 {
			t.Fatal("incorrect cursor returned")
		}

		// Test cursor pagination.
		var lastCursor int32
		for i := range len(allTeams) {
			t.Run(fmt.Sprintf("List 1 %s", allTeams[i].Name), func(t *testing.T) {
				opts := ListTeamsOpts{LimitOffset: &LimitOffset{Limit: 1}, Cursor: lastCursor}
				teams, c, err := store.ListTeams(internalCtx, opts)
				if err != nil {
					t.Fatal(err)
				}
				lastCursor = c

				if diff := cmp.Diff(allTeams[i], teams[0]); diff != "" {
					t.Fatal(diff)
				}
			})
		}

		// Test global count.
		have, err := store.CountTeams(internalCtx, ListTeamsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if have, want := have, int32(len(allTeams)); have != want {
			t.Fatalf("incorrect number of teams returned have=%d want=%d", have, want)
		}

		t.Run("WithParentID", func(t *testing.T) {
			engineeringTeams := []*types.Team{ownTeam, batchesTeam}

			// Get all.
			haveTeams, haveCursor, err := store.ListTeams(internalCtx, ListTeamsOpts{WithParentID: engineeringTeam.ID})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(engineeringTeams, haveTeams); diff != "" {
				t.Fatal(diff)
			}

			if haveCursor != 0 {
				t.Fatal("incorrect cursor returned")
			}
		})

		t.Run("RootOnly", func(t *testing.T) {
			rootTeams := []*types.Team{engineeringTeam, salesTeam, supportTeam}

			// Get all.
			haveTeams, haveCursor, err := store.ListTeams(internalCtx, ListTeamsOpts{RootOnly: true})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(rootTeams, haveTeams); diff != "" {
				t.Fatal(diff)
			}

			if haveCursor != 0 {
				t.Fatal("incorrect cursor returned")
			}
		})

		t.Run("Search", func(t *testing.T) {
			for _, team := range allTeams {
				opts := ListTeamsOpts{Search: team.Name[:3]}
				teams, _, err := store.ListTeams(internalCtx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if len(teams) != 1 {
					t.Fatalf("expected exactly 1 team, got %d", len(teams))
				}

				if diff := cmp.Diff(team, teams[0]); diff != "" {
					t.Fatal(diff)
				}
			}
		})

		t.Run("ForUserMember", func(t *testing.T) {
			johnTeams := []*types.Team{engineeringTeam, supportTeam, batchesTeam}
			aliceTeams := []*types.Team{ownTeam, batchesTeam}
			alexTeams := []*types.Team{ownTeam}

			t.Run("johndoe", func(t *testing.T) {
				haveTeams, haveCursor, err := store.ListTeams(internalCtx, ListTeamsOpts{ForUserMember: johndoe.ID})
				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(johnTeams, haveTeams); diff != "" {
					t.Fatal(diff)
				}

				if haveCursor != 0 {
					t.Fatal("incorrect cursor returned")
				}
			})

			t.Run("alice", func(t *testing.T) {
				haveTeams, haveCursor, err := store.ListTeams(internalCtx, ListTeamsOpts{ForUserMember: alice.ID})
				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(aliceTeams, haveTeams); diff != "" {
					t.Fatal(diff)
				}

				if haveCursor != 0 {
					t.Fatal("incorrect cursor returned")
				}
			})

			t.Run("alex", func(t *testing.T) {
				haveTeams, haveCursor, err := store.ListTeams(internalCtx, ListTeamsOpts{ForUserMember: alex.ID})
				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(alexTeams, haveTeams); diff != "" {
					t.Fatal(diff)
				}

				if haveCursor != 0 {
					t.Fatal("incorrect cursor returned")
				}
			})
		})

		t.Run("ExceptAncestorID", func(t *testing.T) {
			teams, cursor, err := store.ListTeams(internalCtx, ListTeamsOpts{ExceptAncestorID: engineeringTeam.ID})
			if err != nil {
				t.Fatal(err)
			}
			if cursor != 0 {
				t.Fatal("incorrect cursor returned")
			}
			want := []*types.Team{salesTeam, supportTeam}
			sort.Slice(teams, func(i, j int) bool { return teams[i].ID < teams[j].ID })
			sort.Slice(want, func(i, j int) bool { return want[i].ID < want[j].ID })
			if diff := cmp.Diff(want, teams); diff != "" {
				t.Errorf("non-ancestors -want+got: %s", diff)
			}
		})

		t.Run("ExceptAncestorID contains", func(t *testing.T) {
			contains, err := store.ContainsTeam(internalCtx, salesTeam.ID, ListTeamsOpts{ExceptAncestorID: engineeringTeam.ID})
			if err != nil {
				t.Fatal(err)
			}
			if !contains {
				t.Errorf("sales team is expected to be contained in all teams except the sub-tree rooted at engineering team")
			}
		})

		t.Run("ExceptAncestorID does not contain", func(t *testing.T) {
			for _, team := range []*types.Team{ownTeam, engineeringTeam} {
				contains, err := store.ContainsTeam(internalCtx, ownTeam.ID, ListTeamsOpts{ExceptAncestorID: engineeringTeam.ID})
				if err != nil {
					t.Fatal(err)
				}
				if contains {
					t.Errorf("%q team is descendant of engineering, so is expected to be outside of list of teams excluding engineering descendants", team.Name)
				}
			}
		})
	})

	t.Run("ListCountTeamMembers", func(t *testing.T) {
		allTeams := map[*types.Team][]int32{
			engineeringTeam: {johndoe.ID},
			salesTeam:       {},
			batchesTeam:     {johndoe.ID, alice.ID},
		}

		err := db.Users().Delete(internalCtx, alex.ID)
		if err != nil {
			t.Fatal(err)
		}

		for team, wantMembers := range allTeams {
			haveMemberTypes, haveCursor, err := store.ListTeamMembers(internalCtx, ListTeamMembersOpts{TeamID: team.ID})
			if err != nil {
				t.Fatal(err)
			}

			haveMembers := []int32{}
			for _, member := range haveMemberTypes {
				haveMembers = append(haveMembers, member.UserID)
			}

			if diff := cmp.Diff(wantMembers, haveMembers); diff != "" {
				t.Fatal(diff)
			}

			if haveCursor != nil {
				t.Fatal("incorrect cursor returned")
			}

			have, err := store.CountTeamMembers(internalCtx, ListTeamMembersOpts{TeamID: team.ID})
			if err != nil {
				t.Fatal(err)
			}
			if have, want := have, int32(len(wantMembers)); have != want {
				t.Fatalf("incorrect number of teams returned have=%d want=%d", have, want)
			}

			// Test cursor pagination.
			var lastCursor TeamMemberListCursor
			for i := range len(wantMembers) {
				t.Run(fmt.Sprintf("List 1 %s", team.Name), func(t *testing.T) {
					opts := ListTeamMembersOpts{LimitOffset: &LimitOffset{Limit: 1}, Cursor: lastCursor, TeamID: team.ID}
					members, c, err := store.ListTeamMembers(internalCtx, opts)
					if err != nil {
						t.Fatal(err)
					}
					if c != nil {
						lastCursor = *c
					} else {
						lastCursor = TeamMemberListCursor{}
					}

					if len(members) != 1 {
						t.Fatalf("expected exactly 1 member, got %d", len(members))
					}

					if diff := cmp.Diff(wantMembers[i], members[0].UserID); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		}

		t.Run("Search", func(t *testing.T) {
			// Search for john in the team that contains both john and alice: batchesTeam
			opts := ListTeamMembersOpts{TeamID: batchesTeam.ID, Search: johndoe.Username[:3]}
			members, _, err := store.ListTeamMembers(internalCtx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if len(members) != 1 {
				t.Fatalf("expected exactly 1 member, got %d", len(members))
			}

			if diff := cmp.Diff(johndoe.ID, members[0].UserID); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("IsMember", func(t *testing.T) {
			opts := ListTeamMembersOpts{TeamID: batchesTeam.ID}
			members, _, err := store.ListTeamMembers(internalCtx, opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(members) != 2 {
				t.Fatalf("expected exactly 2 members, got %d", len(members))
			}

			for _, m := range members {
				ok, err := store.IsTeamMember(internalCtx, batchesTeam.ID, m.UserID)
				if err != nil {
					t.Fatal(err)
				}
				if !ok {
					t.Fatalf("expected %d to be a member but isn't", m.UserID)
				}
			}

			ok, err := store.IsTeamMember(internalCtx, batchesTeam.ID, 999999)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				t.Fatal("expected not a member but was truthy")
			}
		})
	})
}

func TestTeamNotFoundError(t *testing.T) {
	err := TeamNotFoundError{}
	if have := errcode.IsNotFound(err); !have {
		t.Error("TeamNotFoundError does not say it represents a not found error")
	}
}
