package background

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hexops/autogold"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/discovery"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

type testParams struct {
	settings              *api.Settings
	numRepos              int
	frames                int
	recordSleepOperations bool
	haveData              bool
}

type testResults struct {
	allReposIteratorCalls int
	sleeps                int
	totalSleepTimeMS      int
	reposGetByName        int
	operations            []string
}

func testHistoricalEnqueuer(t *testing.T, p *testParams) *testResults {
	r := &testResults{}
	ctx := context.Background()
	clock := func() time.Time {
		baseNow, err := time.Parse(time.RFC3339, "2021-01-01T00:00:01Z")
		if err != nil {
			panic(err)
		}
		return baseNow
	}
	sleep := func(d time.Duration) {
		r.sleeps++
		r.totalSleepTimeMS += int(d / time.Millisecond)
		if p.recordSleepOperations {
			r.operations = append(r.operations, "sleep()")
		}
	}

	settingStore := discovery.NewMockSettingStore()
	if p.settings != nil {
		settingStore.GetLatestFunc.SetDefaultReturn(p.settings, nil)
	}

	insightsStore := store.NewMockInterface()
	insightsStore.DistinctSeriesWithDataFunc.SetDefaultHook(func(ctx context.Context, from time.Time, to time.Time) ([]string, error) {
		if p.haveData {
			insights, err := discovery.Discover(ctx, settingStore)
			if err != nil {
				panic(err)
			}
			var haveData []string
			for _, insight := range insights {
				for _, series := range insight.Series {
					seriesID, err := discovery.EncodeSeriesID(series)
					if err != nil {
						panic(err)
					}
					haveData = append(haveData, seriesID)
				}
			}
			return haveData, nil
		}
		return []string{}, nil
	})
	insightsStore.RecordSeriesPointFunc.SetDefaultHook(func(ctx context.Context, args store.RecordSeriesPointArgs) error {
		r.operations = append(r.operations, fmt.Sprintf("recordSeriesPoint(point=%v, repoName=%v)", args.Point.String(), *args.RepoName))
		return nil
	})

	repoStore := NewMockRepoStore()
	repos := map[api.RepoName]*types.Repo{}
	for i := 0; i < p.numRepos; i++ {
		name := api.RepoName(fmt.Sprintf("repo/%d", i))
		repos[name] = &types.Repo{
			ID:   api.RepoID(i),
			Name: name,
		}
	}
	repoStore.GetByNameFunc.SetDefaultHook(func(ctx context.Context, name api.RepoName) (*types.Repo, error) {
		r.reposGetByName++
		return repos[name], nil
	})

	enqueueQueryRunnerJob := func(ctx context.Context, job *queryrunner.Job) error {
		r.operations = append(r.operations, fmt.Sprintf(`enqueueQueryRunnerJob("%s", "%s")`, job.RecordTime.Format(time.RFC3339), job.SearchQuery))
		return nil
	}

	allReposIterator := func(ctx context.Context, each func(repoName string) error) error {
		r.allReposIteratorCalls++
		for i := 0; i < p.numRepos; i++ {
			if err := each(fmt.Sprintf("repo/%d", i)); err != nil {
				return err
			}
		}
		return nil
	}

	gitFirstEverCommit := func(ctx context.Context, repoName api.RepoName) (*git.Commit, error) {
		if repoName == "repo/1" {
			daysAgo := clock().Add(-3 * 24 * time.Hour)
			return &git.Commit{Author: git.Signature{Date: daysAgo}}, nil
		}
		yearsAgo := clock().Add(-2 * 365 * 24 * time.Hour)
		return &git.Commit{Author: git.Signature{Date: yearsAgo}}, nil
	}

	gitFindNearestCommit := func(ctx context.Context, repoName api.RepoName, revSpec string, target time.Time) (*git.Commit, error) {
		nearby := target.Add(-2 * 24 * time.Hour)
		return &git.Commit{Author: git.Signature{Date: nearby}}, nil
	}

	historicalEnqueuer := &historicalEnqueuer{
		now:                   clock,
		sleep:                 sleep,
		settingStore:          settingStore,
		insightsStore:         insightsStore,
		repoStore:             repoStore,
		enqueueQueryRunnerJob: enqueueQueryRunnerJob,
		allReposIterator:      allReposIterator,
		gitFirstEverCommit:    gitFirstEverCommit,
		gitFindNearestCommit:  gitFindNearestCommit,

		framesToBackfill: p.frames,
		frameLength:      7 * 24 * time.Hour,
	}

	// If we do an iteration without any insights or repos, we should expect no sleep calls to be made.
	if err := historicalEnqueuer.Handler(ctx); err != nil {
		t.Fatal(err)
	}
	return r
}

func Test_historicalEnqueuer(t *testing.T) {
	// Test that when no insights are defined, no work or sleeping is performed.
	t.Run("no_insights_no_repos", func(t *testing.T) {
		want := autogold.Want("no_insights_no_repos", &testResults{})
		want.Equal(t, testHistoricalEnqueuer(t, &testParams{}))
	})

	// Test that when insights are defined, but no repos exist, no work or sleeping is performed.
	t.Run("some_insights_no_repos", func(t *testing.T) {
		want := autogold.Want("some_insights_no_repos", &testResults{})
		want.Equal(t, testHistoricalEnqueuer(t, &testParams{
			settings: testRealGlobalSettings,
		}))
	})

	// Test that when there is no work to perform (because all insights have historical data) that
	// no work is performed.
	t.Run("no_work", func(t *testing.T) {
		want := autogold.Want("no_work", &testResults{
			allReposIteratorCalls: 2, sleeps: 20,
			reposGetByName: 4,
			operations: []string{
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
				"sleep()",
			},
		})
		want.Equal(t, testHistoricalEnqueuer(t, &testParams{
			settings:              testRealGlobalSettings,
			numRepos:              2,
			frames:                2,
			recordSleepOperations: true,
			haveData:              true,
		}))
	})
	// Test that when insights AND repos exist:
	//
	// * We sleep() between enqueueing jobs
	// * We enqueue a job for every timeframe*repo*series
	// * repo/1 is only enqueued once, because its oldest commit is 3 days ago.
	// * repo/1 has zero data points directly recorded for points in time before its oldest commit.
	// * We enqueue jobs to build out historical data in most-recent to oldest order.
	//
	t.Run("no_data", func(t *testing.T) {
		want := autogold.Want("no_data", &testResults{
			allReposIteratorCalls: 2, sleeps: 20,
			reposGetByName: 4,
			operations: []string{
				"sleep()",
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "errorf count:9999999 repo:^repo/0$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "fmt.Printf count:9999999 repo:^repo/0$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "gitserver.Exec count:9999999 repo:^repo/0$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "gitserver.Close count:9999999 repo:^repo/0$@")`,
				"sleep()",
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "errorf count:9999999 repo:^repo/1$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "fmt.Printf count:9999999 repo:^repo/1$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "gitserver.Exec count:9999999 repo:^repo/1$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-28T12:00:01Z", "gitserver.Close count:9999999 repo:^repo/1$@")`,
				"sleep()",
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-21T12:00:01Z", "errorf count:9999999 repo:^repo/0$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-21T12:00:01Z", "fmt.Printf count:9999999 repo:^repo/0$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-21T12:00:01Z", "gitserver.Exec count:9999999 repo:^repo/0$@")`,
				"sleep()",
				`enqueueQueryRunnerJob("2020-12-21T12:00:01Z", "gitserver.Close count:9999999 repo:^repo/0$@")`,
				"sleep()",
				"sleep()",
				`recordSeriesPoint(point=SeriesPoint{Time: "2020-12-21 12:00:01 +0000 UTC", Value: 0, Metadata: }, repoName=repo/1)`,
				"sleep()",
				`recordSeriesPoint(point=SeriesPoint{Time: "2020-12-21 12:00:01 +0000 UTC", Value: 0, Metadata: }, repoName=repo/1)`,
				"sleep()",
				`recordSeriesPoint(point=SeriesPoint{Time: "2020-12-21 12:00:01 +0000 UTC", Value: 0, Metadata: }, repoName=repo/1)`,
				"sleep()",
				`recordSeriesPoint(point=SeriesPoint{Time: "2020-12-21 12:00:01 +0000 UTC", Value: 0, Metadata: }, repoName=repo/1)`,
			},
		})
		want.Equal(t, testHistoricalEnqueuer(t, &testParams{
			settings:              testRealGlobalSettings,
			numRepos:              2,
			frames:                2,
			recordSleepOperations: true,
		}))
	})
}
