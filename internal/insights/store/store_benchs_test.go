package store

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	edb "github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func initializeData(ctx context.Context, store *Store, repos, times int, withCapture int) string {
	var cv []*string

	if withCapture > 0 {
		for i := range withCapture {
			cv = append(cv, pointers.Ptr(fmt.Sprintf("%d", i)))
		}
	} else {
		cv = append(cv, nil)
	}

	seriesID := rand.String(8)
	currentTime := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	var records []RecordSeriesPointArgs
	for range times {
		for j := range repos {
			repoName := fmt.Sprintf("repo-%d", j)
			id := api.RepoID(j)
			for _, val := range cv {
				records = append(records, RecordSeriesPointArgs{
					SeriesID: seriesID,
					Point: SeriesPoint{
						SeriesID: seriesID,
						Time:     currentTime,
						Value:    float64(rand.Intn(500)),
						Capture:  val,
					},
					RepoName:    &repoName,
					RepoID:      &id,
					PersistMode: RecordMode,
				})
			}
		}

		currentTime = currentTime.AddDate(0, 1, 0)
	}
	if err := store.RecordSeriesPoints(ctx, records); err != nil {
		panic(err)
	}

	return seriesID
}

func generateRepoRestrictions(numberRestricted, totalNumber int) []api.RepoID {
	seen := map[int]struct{}{}
	restrictions := []api.RepoID{}
	upperBound := totalNumber
	for range numberRestricted {
		randomID := rand.Intn(upperBound - 1)
		if _, ok := seen[randomID]; ok {
			restrictions = append(restrictions, api.RepoID(upperBound))
			upperBound = upperBound - 1 // we use the upper bound as our fallback random number, so we always have a hit
		} else {
			seen[randomID] = struct{}{}
			restrictions = append(restrictions, api.RepoID(randomID))
		}
	}
	return restrictions
}

func TestCompareLoadMethods(t *testing.T) {
	toStr := func(pts []SeriesPoint) []string {
		var elems []string
		for _, pt := range pts {
			var cap string
			if pt.Capture != nil {
				cap = *pt.Capture
			}

			elems = append(elems, fmt.Sprintf("%s-%s-%s-%f", pt.SeriesID, cap, pt.Time, pt.Value))
		}
		return elems
	}
	testCases := []struct {
		name         string
		repos        int
		capture      int
		restrictions int
		times        int
	}{
		{
			name:  "no capture values",
			repos: 500,
			times: 5,
		},
		{
			name:    "with captured values",
			repos:   500,
			times:   5,
			capture: 2,
		},
		{
			name:         "with restrictions",
			repos:        500,
			times:        5,
			restrictions: 250,
		},
		{
			name:         "all restricted",
			repos:        100,
			times:        2,
			restrictions: 100,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := logtest.Scoped(t)
			ctx := context.Background()
			clock := timeutil.Now
			insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)
			postgres := database.NewDB(logger, dbtest.NewDB(t))
			permStore := NewInsightPermissionStore(postgres)
			store := NewWithClock(insightsDB, permStore, clock)

			seriesID := initializeData(ctx, store, tc.repos, tc.times, tc.capture)

			opts := SeriesPointsOpts{SeriesID: &seriesID, Excluded: generateRepoRestrictions(tc.restrictions, tc.repos)}

			db, _ := store.SeriesPoints(ctx, opts)
			mem, _ := store.LoadSeriesInMem(ctx, opts)

			dbStr := toStr(db)
			memStr := toStr(mem)

			sort.Slice(dbStr, func(i, j int) bool {
				return dbStr[i] < dbStr[j]
			})

			sort.Slice(memStr, func(i, j int) bool {
				return memStr[i] < memStr[j]
			})
			require.ElementsMatchf(t, dbStr, memStr, "db aggregation not equal to mem aggregation")
		})
	}
}

func BenchmarkLoadTimes(b *testing.B) {
	benchmarks := []struct {
		name         string
		repos        int
		capture      int
		restrictions int
		times        int
	}{
		{
			name:  "1000 repos no capture no restrictions", // 1000 * 12 = 12,000 samples
			repos: 1000,
			times: 12,
		},
		{
			name:  "10000 repos no capture no restrictions", // 10,000 * 12 = 120,000 samples
			repos: 10000,
			times: 12,
		},
		{
			name:  "100000 repos no capture no restrictions", // 100,000 * 12 = 1,200,000 samples
			repos: 100000,
			times: 12,
		},
		{
			name:    "1000 repos 100 capture no restrictions", // 1000 * 100 * 12 = 1,200,000 samples
			repos:   1000,
			times:   12,
			capture: 100,
		},
		{
			name:    "500 repos 200 capture no restrictions", // 500 * 200 * 12 = 1,200,000 samples
			repos:   500,
			times:   12,
			capture: 200,
		},
		{
			name:         "1000 repos no capture 500 restrictions",
			repos:        1000,
			times:        12,
			restrictions: 500,
		},
		{
			name:         "1000 repos 200 capture 500 restrictions",
			repos:        1000,
			times:        12,
			capture:      200,
			restrictions: 500,
		},
	}
	for _, bm := range benchmarks {
		logger := logtest.Scoped(b)
		ctx := context.Background()
		clock := timeutil.Now
		insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, b), logger)
		postgres := database.NewDB(logger, dbtest.NewDB(b))
		permStore := NewInsightPermissionStore(postgres)
		store := NewWithClock(insightsDB, permStore, clock)

		seriesID := initializeData(ctx, store, bm.repos, bm.times, bm.capture)

		opts := SeriesPointsOpts{SeriesID: &seriesID, Excluded: generateRepoRestrictions(bm.restrictions, bm.repos)}

		b.ResetTimer()

		b.Run("in-db-aggregate-"+bm.name, func(b *testing.B) {
			for range b.N {
				_, err := store.SeriesPoints(ctx, opts)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("in-mem-aggregate-"+bm.name, func(b *testing.B) {
			for range b.N {
				_, err := store.LoadSeriesInMem(ctx, opts)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
