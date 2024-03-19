package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log/logtest"

	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func testStoreBatchSpecExecutionCacheEntries(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	entries := make([]*btypes.BatchSpecExecutionCacheEntry, 0, 3)
	for i := range cap(entries) {
		job := &btypes.BatchSpecExecutionCacheEntry{
			UserID: 900 + int32(i),
			Key:    fmt.Sprintf("check-out-this-cache-key-%d", i),
			Value:  fmt.Sprintf("what-about-this-cache-value-huh-%d", i),
		}

		entries = append(entries, job)
	}

	t.Run("Create", func(t *testing.T) {
		for _, job := range entries {
			if err := s.CreateBatchSpecExecutionCacheEntry(ctx, job); err != nil {
				t.Fatal(err)
			}

			have := job
			if have.ID == 0 {
				t.Fatal("ID should not be zero")
			}

			want := have
			want.CreatedAt = clock.Now()

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})

	t.Run("List", func(t *testing.T) {
		t.Run("ListByUserIDAndKeys", func(t *testing.T) {
			for i, job := range entries {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					cs, err := s.ListBatchSpecExecutionCacheEntries(ctx, ListBatchSpecExecutionCacheEntriesOpts{
						UserID: job.UserID,
						Keys:   []string{job.Key},
					})
					if err != nil {
						t.Fatal(err)
					}
					if len(cs) != 1 {
						t.Fatal("cache entry not found")
					}
					have := cs[0]

					if diff := cmp.Diff(have, job); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})
	})

	t.Run("CreateWithConflictingKey", func(t *testing.T) {
		clock.Add(1 * time.Minute)

		keyConflict := &btypes.BatchSpecExecutionCacheEntry{
			UserID: entries[0].UserID,
			Key:    entries[0].Key,
			Value:  "new value",
		}
		if err := s.CreateBatchSpecExecutionCacheEntry(ctx, keyConflict); err != nil {
			t.Fatal(err)
		}

		reloaded, err := s.ListBatchSpecExecutionCacheEntries(ctx, ListBatchSpecExecutionCacheEntriesOpts{
			UserID: keyConflict.UserID,
			Keys:   []string{keyConflict.Key},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(reloaded) != 1 {
			t.Fatal("cache entry not found")
		}
		reloadedEntry := reloaded[0]

		if diff := cmp.Diff(reloadedEntry, keyConflict); diff != "" {
			t.Fatal(diff)
		}

		if reloadedEntry.CreatedAt.Equal(entries[0].CreatedAt) {
			t.Fatal("CreatedAt not updated")
		}
	})

	t.Run("MarkUsedBatchSpecExecutionCacheEntries", func(t *testing.T) {
		entry := &btypes.BatchSpecExecutionCacheEntry{
			UserID: 9999,
			Key:    "the-amazing-cache-key",
			Value:  "the-mysterious-cache-value",
		}

		if err := s.CreateBatchSpecExecutionCacheEntry(ctx, entry); err != nil {
			t.Fatal(err)
		}

		if err := s.MarkUsedBatchSpecExecutionCacheEntries(ctx, []int64{entry.ID}); err != nil {
			t.Fatal(err)
		}

		reloaded, err := s.ListBatchSpecExecutionCacheEntries(ctx, ListBatchSpecExecutionCacheEntriesOpts{
			UserID: entry.UserID,
			Keys:   []string{entry.Key},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(reloaded) != 1 {
			t.Fatal("cache entry not found")
		}
		reloadedEntry := reloaded[0]

		if want, have := clock.Now(), reloadedEntry.LastUsedAt; !have.Equal(want) {
			t.Fatalf("entry.LastUsedAt is wrong.\n\twant=%s\n\thave=%s", want, have)
		}
	})
}

func TestStore_CleanBatchSpecExecutionCacheEntries(t *testing.T) {
	// Separate test function because we want a clean DB

	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))
	c := &bt.TestClock{Time: timeutil.Now()}
	s := NewWithClock(db, &observation.TestContext, nil, c.Now)
	user := bt.CreateTestUser(t, db, true)

	maxSize := 10 * 1024 // 10kb

	for i := 0; i < 20; i += 1 {
		entry := &btypes.BatchSpecExecutionCacheEntry{
			UserID: user.ID,
			Key:    fmt.Sprintf("cache-key-%d", i),
			Value:  strings.Repeat("a", 1024),
		}

		if err := s.CreateBatchSpecExecutionCacheEntry(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}

	totalSize, _, err := basestore.ScanFirstInt(s.Query(ctx, sqlf.Sprintf("SELECT sum(octet_length(value)) AS total FROM batch_spec_execution_cache_entries")))
	if err != nil {
		t.Fatal(err)
	}
	if totalSize != maxSize*2 {
		t.Fatalf("totalsize wrong=%d", totalSize)
	}

	if err := s.CleanBatchSpecExecutionCacheEntries(ctx, int64(maxSize)); err != nil {
		t.Fatal(err)
	}

	entriesLeft, _, err := basestore.ScanFirstInt(s.Query(ctx, sqlf.Sprintf("SELECT count(*) FROM batch_spec_execution_cache_entries")))
	if err != nil {
		t.Fatal(err)
	}

	wantLeft := 10
	if entriesLeft != wantLeft {
		t.Fatalf("wrong number of entries left. want=%d, have=%d", wantLeft, entriesLeft)
	}

	totalSize, _, err = basestore.ScanFirstInt(s.Query(ctx, sqlf.Sprintf("SELECT sum(octet_length(value)) AS total FROM batch_spec_execution_cache_entries")))
	if err != nil {
		t.Fatal(err)
	}
	if totalSize != maxSize {
		t.Fatalf("totalsize wrong=%d", totalSize)
	}
}
