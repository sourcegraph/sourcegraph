package a8n

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/sourcegraph/pkg/db/dbtest"
)

var dsn = flag.String("dsn", "", "Database connection string to use in integration tests")

func TestStore(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	d, cleanup := dbtest.NewDB(t, *dsn)
	defer cleanup()

	tx, done := dbtest.NewTx(t, d)
	defer done()

	s := NewStore(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	s.now = func() time.Time {
		return now.UTC().Truncate(time.Microsecond)
	}

	ctx := context.Background()

	t.Run("Campaigns", func(t *testing.T) {
		campaigns := make([]*Campaign, 0, 3)

		t.Run("Create", func(t *testing.T) {
			for i := 0; i < cap(campaigns); i++ {
				c := &Campaign{
					Name:         fmt.Sprintf("Upgrade ES-Lint %d", i),
					Description:  "All the Javascripts are belong to us",
					AuthorID:     23,
					ChangeSetIDs: []int64{int64(i) + 1},
				}

				if i%2 == 0 {
					c.NamespaceOrgID = 23
				} else {
					c.NamespaceUserID = 42
				}

				want := c.Clone()
				have := c

				err := s.CreateCampaign(ctx, have)
				if err != nil {
					t.Fatal(err)
				}

				if have.ID == 0 {
					t.Fatal("ID should not be zero")
				}

				want.ID = have.ID
				want.CreatedAt = now
				want.UpdatedAt = now

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}

				campaigns = append(campaigns, c)
			}
		})

		t.Run("Count", func(t *testing.T) {
			count, err := s.CountCampaigns(ctx, CountCampaignsOpts{})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, int64(len(campaigns)); have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}

			count, err = s.CountCampaigns(ctx, CountCampaignsOpts{ChangeSetID: 1})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, int64(1); have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}
		})

		t.Run("List", func(t *testing.T) {
			for i := 1; i <= len(campaigns); i++ {
				opts := ListCampaignsOpts{ChangeSetID: int64(i)}

				ts, next, err := s.ListCampaigns(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if have, want := next, int64(0); have != want {
					t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
				}

				have, want := ts, campaigns[i-1:i]
				if len(have) != len(want) {
					t.Fatalf("listed %d campaigns, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			for i := 1; i <= len(campaigns); i++ {
				cs, next, err := s.ListCampaigns(ctx, ListCampaignsOpts{Limit: i})
				if err != nil {
					t.Fatal(err)
				}

				{
					have, want := next, int64(0)
					if i < len(campaigns) {
						want = campaigns[i].ID
					}

					if have != want {
						t.Fatalf("limit: %v: have next %v, want %v", i, have, want)
					}
				}

				{
					have, want := cs, campaigns[:i]
					if len(have) != len(want) {
						t.Fatalf("listed %d campaigns, want: %d", len(have), len(want))
					}

					if diff := cmp.Diff(have, want); diff != "" {
						t.Fatal(diff)
					}
				}
			}
		})

		t.Run("Update", func(t *testing.T) {
			for _, c := range campaigns {
				c.Name += "-updated"
				c.Description += "-updated"
				c.AuthorID++

				if c.NamespaceUserID != 0 {
					c.NamespaceUserID++
				}

				if c.NamespaceOrgID != 0 {
					c.NamespaceOrgID++
				}

				now = now.Add(time.Second)
				want := c
				want.UpdatedAt = now

				have := c.Clone()
				if err := s.UpdateCampaign(ctx, have); err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}

				// Test that duplicates are not introduced.
				have.ChangeSetIDs = append(have.ChangeSetIDs, have.ChangeSetIDs...)
				if err := s.UpdateCampaign(ctx, have); err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}

				// Test we can add to the set.
				have.ChangeSetIDs = append(have.ChangeSetIDs, 42)
				want.ChangeSetIDs = append(want.ChangeSetIDs, 42)

				if err := s.UpdateCampaign(ctx, have); err != nil {
					t.Fatal(err)
				}

				sort.Slice(have.ChangeSetIDs, func(a, b int) bool {
					return have.ChangeSetIDs[a] < have.ChangeSetIDs[b]
				})

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}

				// Test we can remove from the set.
				have.ChangeSetIDs = have.ChangeSetIDs[:0]
				want.ChangeSetIDs = want.ChangeSetIDs[:0]

				if err := s.UpdateCampaign(ctx, have); err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}
			}
		})

		t.Run("Get", func(t *testing.T) {
			t.Run("ByID", func(t *testing.T) {
				want := campaigns[0]
				opts := GetCampaignOpts{ID: want.ID}

				have, err := s.GetCampaign(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}
			})

			t.Run("NoResults", func(t *testing.T) {
				opts := GetCampaignOpts{ID: 0xdeadbeef}

				_, have := s.GetCampaign(ctx, opts)
				want := ErrNoResults

				if have != want {
					t.Fatalf("have err %v, want %v", have, want)
				}
			})
		})
	})

	t.Run("ChangeSets", func(t *testing.T) {
		changesets := make([]*ChangeSet, 0, 3)
		t.Run("Create", func(t *testing.T) {
			for i := 0; i < cap(changesets); i++ {
				th := &ChangeSet{
					RepoID:      42,
					CreatedAt:   now,
					UpdatedAt:   now,
					Metadata:    []byte("{}"),
					CampaignIDs: []int64{int64(i) + 1},
				}

				want := th.Clone()
				have := th

				err := s.CreateChangeSet(ctx, have)
				if err != nil {
					t.Fatal(err)
				}

				if have.ID == 0 {
					t.Fatal("id should not be zero")
				}

				want.ID = have.ID
				want.CreatedAt = now
				want.UpdatedAt = now

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}

				changesets = append(changesets, th)
			}
		})

		t.Run("Count", func(t *testing.T) {
			count, err := s.CountChangeSets(ctx, CountChangeSetsOpts{})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, int64(len(changesets)); have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}

			count, err = s.CountChangeSets(ctx, CountChangeSetsOpts{CampaignID: 1})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, int64(1); have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}
		})

		t.Run("List", func(t *testing.T) {
			for i := 1; i <= len(changesets); i++ {
				opts := ListChangeSetsOpts{CampaignID: int64(i)}

				ts, next, err := s.ListChangeSets(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if have, want := next, int64(0); have != want {
					t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
				}

				have, want := ts, changesets[i-1:i]
				if len(have) != len(want) {
					t.Fatalf("listed %d changesets, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			for i := 1; i <= len(changesets); i++ {
				ts, next, err := s.ListChangeSets(ctx, ListChangeSetsOpts{Limit: i})
				if err != nil {
					t.Fatal(err)
				}

				{
					have, want := next, int64(0)
					if i < len(changesets) {
						want = changesets[i].ID
					}

					if have != want {
						t.Fatalf("limit: %v: have next %v, want %v", i, have, want)
					}
				}

				{
					have, want := ts, changesets[:i]
					if len(have) != len(want) {
						t.Fatalf("listed %d changesets, want: %d", len(have), len(want))
					}

					if diff := cmp.Diff(have, want); diff != "" {
						t.Fatal(diff)
					}
				}
			}
		})
	})
}
