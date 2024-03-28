package executors

import (
	"context"
	"fmt"
	"testing"
	"time"

	executortypes "github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

var defaultTime = time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)

type cacheEntry struct {
	timestamp       string
	jobId           string
	shouldBeDeleted bool
}

func Test_multiqueueCacheCleaner_Handle(t *testing.T) {
	tests := []struct {
		name         string
		cacheEntries map[string][]cacheEntry
	}{
		{
			name: "nothing deleted",
			cacheEntries: map[string][]cacheEntry{
				"batches": {{
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 2).UnixNano()),
					jobId:           "batches-1",
					shouldBeDeleted: false,
				}, {
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 1).UnixNano()),
					jobId:           "batches-2",
					shouldBeDeleted: false,
				}},
				"codeintel": {{
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 2).UnixNano()),
					jobId:           "codeintel-1",
					shouldBeDeleted: false,
				}, {
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 1).UnixNano()),
					jobId:           "codeintel-2",
					shouldBeDeleted: false,
				}},
			},
		},
		{
			name: "one entry for each deleted",
			cacheEntries: map[string][]cacheEntry{
				"batches": {{
					// past the 5 minute TTL
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 6).UnixNano()),
					jobId:           "batches-1",
					shouldBeDeleted: false,
				}, {
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 1).UnixNano()),
					jobId:           "batches-2",
					shouldBeDeleted: false,
				}},
				"codeintel": {{
					// past the 5 minute TTL
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 6).UnixNano()),
					jobId:           "codeintel-1",
					shouldBeDeleted: false,
				}, {
					timestamp:       fmt.Sprint(defaultTime.Add(-time.Minute * 1).UnixNano()),
					jobId:           "codeintel-2",
					shouldBeDeleted: false,
				}},
			},
		},
	}
	for _, tt := range tests {
		ctx := context.TODO()
		t.Run(tt.name, func(t *testing.T) {
			rcache.SetupForTest(t)
			m := &multiqueueCacheCleaner{
				cache:      rcache.New(executortypes.DequeueCachePrefix, false),
				windowSize: executortypes.DequeueTtl,
			}
			timeNow = func() time.Time {
				return defaultTime
			}

			for queue, dequeues := range tt.cacheEntries {
				for _, dequeue := range dequeues {
					if err := m.cache.SetHashItem(queue, dequeue.timestamp, dequeue.jobId); err != nil {
						t.Fatalf("unexpected error setting test cache data: %s", err)
					}
				}
			}

			expectedCacheSizePerQueue := make(map[string]int, len(tt.cacheEntries))
			for queue, entries := range tt.cacheEntries {
				for _, entry := range entries {
					if !entry.shouldBeDeleted {
						expectedCacheSizePerQueue[queue]++
					}
				}
			}

			if err := m.Handle(ctx); err != nil {
				t.Fatalf("unexpected error cleaning the cache: %s", err)
			}

			for queue, size := range expectedCacheSizePerQueue {
				items, err := m.cache.GetHashAll(queue)
				if err != nil {
					t.Fatalf("unexpected error getting all cache items for queue %s: %s", queue, err)
				}
				if len(items) != size {
					t.Errorf("expected %d cache items for queue %s but found %d", size, queue, len(items))
				}
			}
		})
	}
}
