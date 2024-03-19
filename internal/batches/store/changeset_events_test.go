package store

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
)

func testStoreChangesetEvents(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	issueComment := &github.IssueComment{
		DatabaseID: 443827703,
		Author: github.Actor{
			AvatarURL: "https://avatars0.githubusercontent.com/u/1976?v=4",
			Login:     "sqs",
			URL:       "https://github.com/sqs",
		},
		Editor:              nil,
		AuthorAssociation:   "MEMBER",
		Body:                "> Just to be sure: you mean the \"searchFilters\" \"Filters\" should be lowercase, not the \"Search Filters\" from the description, right?\r\n\r\nNo, the prose “Search Filters” should have the F lowercased to fit with our style guide preference for sentence case over title case. (Can’t find this comment on the GitHub mobile interface anymore so quoting the email.)",
		URL:                 "https://github.com/sourcegraph/sourcegraph/pull/999#issuecomment-443827703",
		CreatedAt:           clock.Now(),
		UpdatedAt:           clock.Now(),
		IncludesCreatedEdit: false,
	}

	events := make([]*btypes.ChangesetEvent, 0, 3)
	kinds := []btypes.ChangesetEventKind{
		btypes.ChangesetEventKindGitHubCommented,
		btypes.ChangesetEventKindGitHubClosed,
		btypes.ChangesetEventKindGitHubAssigned,
	}

	t.Run("Upsert", func(t *testing.T) {
		for i := range cap(events) {
			e := &btypes.ChangesetEvent{
				ChangesetID: int64(i + 1),
				Kind:        kinds[i],
				Key:         issueComment.Key(),
				CreatedAt:   clock.Now(),
				Metadata:    issueComment,
			}

			events = append(events, e)
		}

		// Verify that no duplicates are introduced and no error is returned.
		for range 2 {
			err := s.UpsertChangesetEvents(ctx, events...)
			if err != nil {
				t.Fatal(err)
			}
		}

		for _, have := range events {
			if have.ID == 0 {
				t.Fatal("id should not be zero")
			}

			want := have.Clone()

			want.ID = have.ID
			want.CreatedAt = clock.Now()
			want.UpdatedAt = clock.Now()

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := s.CountChangesetEvents(ctx, CountChangesetEventsOpts{})
		if err != nil {
			t.Fatal(err)
		}

		if have, want := count, len(events); have != want {
			t.Fatalf("have count: %d, want: %d", have, want)
		}

		count, err = s.CountChangesetEvents(ctx, CountChangesetEventsOpts{ChangesetID: 1})
		if err != nil {
			t.Fatal(err)
		}

		if have, want := count, 1; have != want {
			t.Fatalf("have count: %d, want: %d", have, want)
		}
	})

	t.Run("Get", func(t *testing.T) {
		t.Run("ByID", func(t *testing.T) {
			want := events[0]
			opts := GetChangesetEventOpts{ID: want.ID}

			have, err := s.GetChangesetEvent(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("ByKey", func(t *testing.T) {
			want := events[0]
			opts := GetChangesetEventOpts{
				ChangesetID: want.ChangesetID,
				Kind:        want.Kind,
				Key:         want.Key,
			}

			have, err := s.GetChangesetEvent(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("NoResults", func(t *testing.T) {
			opts := GetChangesetEventOpts{ID: 0xdeadbeef}

			_, have := s.GetChangesetEvent(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("ByChangesetIDs", func(t *testing.T) {
			for i := 1; i <= len(events); i++ {
				opts := ListChangesetEventsOpts{ChangesetIDs: []int64{int64(i)}}

				ts, next, err := s.ListChangesetEvents(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if have, want := next, int64(0); have != want {
					t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
				}

				have, want := ts, events[i-1:i]
				if len(have) != len(want) {
					t.Fatalf("listed %d events, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			{
				opts := ListChangesetEventsOpts{ChangesetIDs: []int64{}}

				for i := 1; i <= len(events); i++ {
					opts.ChangesetIDs = append(opts.ChangesetIDs, int64(i))
				}

				ts, next, err := s.ListChangesetEvents(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if have, want := next, int64(0); have != want {
					t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
				}

				have, want := ts, events
				if len(have) != len(want) {
					t.Fatalf("listed %d events, want: %d", len(have), len(want))
				}
			}
		})

		t.Run("ByKinds", func(t *testing.T) {
			for _, k := range kinds {
				opts := ListChangesetEventsOpts{Kinds: []btypes.ChangesetEventKind{k}}

				ts, next, err := s.ListChangesetEvents(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if have, want := next, int64(0); have != want {
					t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
				}

				if have, want := len(ts), 1; have != want {
					t.Fatalf("listed %d events for %q, want: %d", have, k, want)
				}

				if have, want := ts[0].Kind, k; have != want {
					t.Fatalf("listed %q events, want of kind: %q", have, want)
				}
			}

			{
				opts := ListChangesetEventsOpts{Kinds: []btypes.ChangesetEventKind{}}

				for _, e := range events {
					opts.Kinds = append(opts.Kinds, e.Kind)
				}

				ts, next, err := s.ListChangesetEvents(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if have, want := next, int64(0); have != want {
					t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
				}

				have, want := ts, events
				if len(have) != len(want) {
					t.Fatalf("listed %d events, want: %d", len(have), len(want))
				}
			}
		})

		t.Run("WithLimit", func(t *testing.T) {
			for i := 1; i <= len(events); i++ {
				cs, next, err := s.ListChangesetEvents(ctx, ListChangesetEventsOpts{LimitOpts: LimitOpts{Limit: i}})
				if err != nil {
					t.Fatal(err)
				}

				{
					have, want := next, int64(0)
					if i < len(events) {
						want = events[i].ID
					}

					if have != want {
						t.Fatalf("limit: %v: have next %v, want %v", i, have, want)
					}
				}

				{
					have, want := cs, events[:i]
					if len(have) != len(want) {
						t.Fatalf("listed %d events, want: %d", len(have), len(want))
					}

					if diff := cmp.Diff(have, want); diff != "" {
						t.Fatal(diff)
					}
				}
			}
		})

		t.Run("WithCursor", func(t *testing.T) {
			var cursor int64
			for i := 1; i <= len(events); i++ {
				opts := ListChangesetEventsOpts{Cursor: cursor, LimitOpts: LimitOpts{Limit: 1}}
				have, next, err := s.ListChangesetEvents(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := events[i-1 : i]
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}

				cursor = next
			}
		})

		t.Run("EmptyResultListingAll", func(t *testing.T) {
			opts := ListChangesetEventsOpts{ChangesetIDs: []int64{99999}}

			ts, next, err := s.ListChangesetEvents(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if have, want := next, int64(0); have != want {
				t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
			}

			if len(ts) != 0 {
				t.Fatalf("listed %d events, want: %d", len(ts), 0)
			}
		})
	})
}
