package pipeline

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/derision-test/glock"
	"github.com/hexops/autogold/v2"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	internalGitserver "github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/internal/insights/compression"
	"github.com/sourcegraph/sourcegraph/internal/insights/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/insights/timeseries"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	itypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func makeTestJobGenerator(numJobs int) SearchJobGenerator {
	return func(ctx context.Context, req requestContext) (*requestContext, []*queryrunner.SearchJob, error) {
		jobs := make([]*queryrunner.SearchJob, 0, numJobs)
		recordDate := time.Date(2022, time.April, 1, 0, 0, 0, 0, time.UTC)
		for range numJobs {
			jobs = append(jobs, &queryrunner.SearchJob{
				SeriesID:    req.backfillRequest.Series.SeriesID,
				SearchQuery: "test search",
				RecordTime:  &recordDate,
			})
		}
		return &req, jobs, nil
	}
}

func testSearchHandlerConstValue(ctx context.Context, job *queryrunner.SearchJob, series *types.InsightSeries, recordTime time.Time) ([]store.RecordSeriesPointArgs, error) {
	return []store.RecordSeriesPointArgs{{Point: store.SeriesPoint{Value: 10, Time: *job.RecordTime}}}, nil
}

func makeTestSearchHandlerErr(err error, errorAfterNumReq int) func(ctx context.Context, job *queryrunner.SearchJob, series *types.InsightSeries, recordTime time.Time) ([]store.RecordSeriesPointArgs, error) {
	var called *int
	called = new(int)
	var mu sync.Mutex
	return func(ctx context.Context, job *queryrunner.SearchJob, series *types.InsightSeries, recordTime time.Time) ([]store.RecordSeriesPointArgs, error) {
		mu.Lock()
		defer mu.Unlock()
		if *called >= errorAfterNumReq {
			return nil, err
		}
		*called++
		return []store.RecordSeriesPointArgs{{Point: store.SeriesPoint{Value: 10, Time: *job.RecordTime}}}, nil
	}
}

func testSearchRunnerStep(ctx context.Context, reqContext *requestContext, jobs []*queryrunner.SearchJob) (*requestContext, []store.RecordSeriesPointArgs, error) {
	points := make([]store.RecordSeriesPointArgs, 0, len(jobs))
	for _, job := range jobs {
		newPoints, _ := testSearchHandlerConstValue(ctx, job, reqContext.backfillRequest.Series, *job.RecordTime)
		points = append(points, newPoints...)
	}
	return reqContext, points, nil
}

type testRunCounts struct {
	err         error
	resultCount int
	totalCount  int
}

func TestBackfillStepsConnected(t *testing.T) {
	testCases := []struct {
		name    string
		numJobs int
		want    autogold.Value
	}{
		{"With Jobs", 10, autogold.Expect(testRunCounts{resultCount: 10, totalCount: 100})},
		{"No Jobs", 0, autogold.Expect(testRunCounts{})},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := testRunCounts{}
			countingPersister := func(ctx context.Context, reqContext *requestContext, points []store.RecordSeriesPointArgs) (*requestContext, error) {
				for _, p := range points {
					got.resultCount++
					got.totalCount += int(p.Point.Value)
				}
				return reqContext, nil
			}

			backfiller := newBackfiller(makeTestJobGenerator(tc.numJobs), testSearchRunnerStep, countingPersister, glock.NewMockClock())
			got.err = backfiller.Run(context.Background(), BackfillRequest{Series: &types.InsightSeries{SeriesID: "1"}})
			tc.want.Equal(t, got)
		})
	}
}

type fakeCommitClient struct {
	firstCommit   func(ctx context.Context, repoName api.RepoName) (*gitdomain.Commit, error)
	recentCommits func(ctx context.Context, repoName api.RepoName, target time.Time, revision string) ([]*gitdomain.Commit, error)
}

var _ GitCommitClient = (*fakeCommitClient)(nil)

func (f *fakeCommitClient) FirstCommit(ctx context.Context, repoName api.RepoName) (*gitdomain.Commit, error) {
	return f.firstCommit(ctx, repoName)
}

func (f *fakeCommitClient) RecentCommits(ctx context.Context, repoName api.RepoName, target time.Time, revision string) ([]*gitdomain.Commit, error) {
	return f.recentCommits(ctx, repoName, target, revision)
}

func (f *fakeCommitClient) GitserverClient() internalGitserver.Client {
	return internalGitserver.NewMockClient()
}

func newFakeCommitClient(first *gitdomain.Commit, recents []*gitdomain.Commit) GitCommitClient {
	return &fakeCommitClient{
		firstCommit: func(ctx context.Context, repoName api.RepoName) (*gitdomain.Commit, error) { return first, nil },
		recentCommits: func(ctx context.Context, repoName api.RepoName, target time.Time, revision string) ([]*gitdomain.Commit, error) {
			return recents, nil
		},
	}
}

func TestMakeSearchJobs(t *testing.T) {
	// Setup
	threeWeeks := 24 * 21 * time.Hour
	createdDate := time.Date(2022, time.April, 1, 1, 0, 0, 0, time.UTC)
	firstCommit := gitdomain.Commit{ID: "1", Committer: &gitdomain.Signature{}}
	recentFirstCommit := gitdomain.Commit{ID: "1", Committer: &gitdomain.Signature{}, Author: gitdomain.Signature{Date: createdDate.Add(-1 * threeWeeks)}}
	recentCommits := []*gitdomain.Commit{{ID: "1", Committer: &gitdomain.Signature{}}, {ID: "2", Committer: &gitdomain.Signature{}}}

	series := &types.InsightSeries{
		ID:                  1,
		SeriesID:            "abc",
		Query:               "test query",
		CreatedAt:           createdDate,
		SampleIntervalUnit:  string(types.Week),
		SampleIntervalValue: 1,
	}
	// All the series in this test reuse the same time data, so we will reuse these sampling timestamps across all request objects.
	sampleTimes := timeseries.BuildSampleTimes(12, timeseries.TimeInterval{
		Unit:  types.IntervalUnit(series.SampleIntervalUnit),
		Value: series.SampleIntervalValue,
	}, series.CreatedAt.Truncate(time.Minute))

	t.Log(fmt.Sprintf("sampleTimes: %v", sampleTimes))
	t.Log(fmt.Sprintf("first: %v", createdDate.Add(-1*threeWeeks)))

	backfillReq := &BackfillRequest{
		Series:      series,
		SampleTimes: sampleTimes,
		Repo:        &itypes.MinimalRepo{ID: api.RepoID(1), Name: api.RepoName("testrepo")},
	}

	backfillReqInvalidQuery := &BackfillRequest{
		Series: &types.InsightSeries{
			ID:                  1,
			SeriesID:            "abc",
			Query:               "patterntype:regexp i++",
			CreatedAt:           createdDate,
			SampleIntervalUnit:  string(types.Week),
			SampleIntervalValue: 1,
		},
		SampleTimes: sampleTimes,
		Repo:        &itypes.MinimalRepo{ID: api.RepoID(1), Name: api.RepoName("testrepo")},
	}

	backfillReqRepoQuery := &BackfillRequest{
		Series: &types.InsightSeries{
			ID:                  1,
			SeriesID:            "abc",
			Query:               "test query repo:repoA",
			CreatedAt:           createdDate,
			SampleIntervalUnit:  string(types.Week),
			SampleIntervalValue: 1,
		},
		SampleTimes: sampleTimes,
		Repo:        &itypes.MinimalRepo{ID: api.RepoID(1), Name: api.RepoName("testrepo")},
	}

	basicCommitClient := newFakeCommitClient(&firstCommit, recentCommits)
	// used to simulate a single call to recent commits failing
	recentsErrorAfter := func(times int, commits []*gitdomain.Commit) func(ctx context.Context, repoName api.RepoName, target time.Time, revision string) ([]*gitdomain.Commit, error) {
		var called *int
		called = new(int)
		var mu sync.Mutex
		return func(ctx context.Context, repoName api.RepoName, target time.Time, revision string) ([]*gitdomain.Commit, error) {
			mu.Lock()
			defer mu.Unlock()
			if *called >= times {
				return nil, errors.New("error hit")
			}
			*called++
			return commits, nil
		}
	}

	testCases := []struct {
		name         string
		commitClient GitCommitClient
		backfillReq  *BackfillRequest
		workers      int
		canceled     bool
		want         autogold.Value
	}{
		{
			name:         "Base case single worker",
			commitClient: basicCommitClient, backfillReq: backfillReq, workers: 1,
			want: autogold.Expect([]string{
				"job recordtime:2022-04-01T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-25T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-18T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-11T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-04T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-25T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-18T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-11T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-04T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-01-28T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-01-21T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-01-14T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"error occurred: false",
			}),
		},
		{
			name:         "Base case multiple workers",
			commitClient: basicCommitClient, backfillReq: backfillReq, workers: 5, want: autogold.Expect([]string{
				"job recordtime:2022-04-01T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-25T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-18T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-11T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-04T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-25T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-18T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-11T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-02-04T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-01-28T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-01-21T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-01-14T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"error occurred: false",
			}),
		},
		{
			name:         "First commit during backfill period",
			commitClient: newFakeCommitClient(&recentFirstCommit, recentCommits), backfillReq: backfillReq, workers: 1, want: autogold.Expect([]string{
				"job recordtime:2022-04-01T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-25T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-18T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-11T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"error occurred: false",
			}),
		},
		{
			name:         "First commit during backfill period multiple workers",
			commitClient: newFakeCommitClient(&recentFirstCommit, recentCommits), backfillReq: backfillReq, workers: 5, want: autogold.Expect([]string{
				"job recordtime:2022-04-01T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-25T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-18T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"job recordtime:2022-03-11T01:00:00Z query:fork:no archived:no patterntype:literal count:99999999 test query repo:^testrepo$@1",
				"error occurred: false",
			}),
		},
		{
			name:         "Canceled case single worker",
			commitClient: basicCommitClient, backfillReq: backfillReq, workers: 1, canceled: true, want: autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name:         "Canceled case multiple workers",
			commitClient: basicCommitClient, backfillReq: backfillReq, workers: 5, canceled: true, want: autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name: "Firt commit error",
			commitClient: &fakeCommitClient{
				firstCommit: func(ctx context.Context, repoName api.RepoName) (*gitdomain.Commit, error) {
					return nil, errors.New("somethings wrong")
				},
				recentCommits: basicCommitClient.RecentCommits,
			}, backfillReq: backfillReq, workers: 1, want: autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name: "Empty repo",
			commitClient: &fakeCommitClient{
				firstCommit: func(ctx context.Context, repoName api.RepoName) (*gitdomain.Commit, error) {
					return nil, gitserver.EmptyRepoErr
				},
				recentCommits: basicCommitClient.RecentCommits,
			}, backfillReq: backfillReq, workers: 1, want: autogold.Expect([]string{"error occurred: false"}),
		},
		{
			name: "Error in some jobs single worker",
			commitClient: &fakeCommitClient{
				firstCommit:   basicCommitClient.FirstCommit,
				recentCommits: recentsErrorAfter(6, recentCommits),
			}, backfillReq: backfillReq, workers: 1, want: autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name: "Error in some jobs multiple worker",
			commitClient: &fakeCommitClient{
				firstCommit:   basicCommitClient.FirstCommit,
				recentCommits: recentsErrorAfter(6, recentCommits),
			}, backfillReq: backfillReq, workers: 5, want: autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name:         "Invalid query",
			commitClient: basicCommitClient, backfillReq: backfillReqInvalidQuery, workers: 1, want: autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name:         "Query with repo: in it",
			commitClient: basicCommitClient, backfillReq: backfillReqRepoQuery, workers: 1, want: autogold.Expect([]string{"error occurred: false"}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			unlimitedLimiter := ratelimit.NewInstrumentedLimiter("", rate.NewLimiter(rate.Inf, 100))
			jobsFunc := makeSearchJobsFunc(logtest.NoOp(t), tc.commitClient, &compression.NoopFilter{}, tc.workers, unlimitedLimiter)
			_, jobs, err := jobsFunc(testCtx, requestContext{backfillRequest: tc.backfillReq})
			got := []string{}
			// sorted jobs to make test stable
			sort.SliceStable(jobs, func(i, j int) bool {
				return jobs[i].RecordTime.After(*jobs[j].RecordTime)
			})
			for _, j := range jobs {
				got = append(got, fmt.Sprintf("job recordtime:%s query:%s", j.RecordTime.Format(time.RFC3339Nano), j.SearchQuery))
			}
			got = append(got, fmt.Sprintf("error occurred: %v", err != nil))
			tc.want.Equal(t, got)
		})
	}
}

func TestMakeRunSearch(t *testing.T) {
	// Setup
	createdDate := time.Date(2022, time.April, 1, 1, 0, 0, 0, time.UTC)

	backfillReq := &BackfillRequest{
		Series: &types.InsightSeries{
			ID:                  1,
			SeriesID:            "abc",
			Query:               "test query",
			CreatedAt:           createdDate,
			SampleIntervalUnit:  string(types.Week),
			SampleIntervalValue: 1,
			GenerationMethod:    types.Search,
		},
		Repo: &itypes.MinimalRepo{ID: api.RepoID(1), Name: api.RepoName("testrepo")},
	}

	// testSearchHandlerConstValue returns 10 for every point
	// testSearchHandlerErr always errors
	defaultHandlers := map[types.GenerationMethod]queryrunner.InsightsHandler{
		types.Search: testSearchHandlerConstValue,
	}
	recordTime1 := time.Date(2022, time.April, 21, 0, 0, 0, 0, time.UTC)
	recordTime2 := time.Date(2022, time.April, 14, 0, 0, 0, 0, time.UTC)
	recordTime3 := time.Date(2022, time.April, 7, 0, 0, 0, 0, time.UTC)
	recordTime4 := time.Date(2022, time.April, 1, 0, 0, 0, 0, time.UTC)

	jobs := []*queryrunner.SearchJob{{RecordTime: &recordTime1}, {RecordTime: &recordTime2}, {RecordTime: &recordTime3}, {RecordTime: &recordTime4}}

	testCases := []struct {
		name        string
		backfillReq *BackfillRequest
		workers     int
		cancled     bool
		handlers    map[types.GenerationMethod]queryrunner.InsightsHandler
		jobs        []*queryrunner.SearchJob
		want        autogold.Value
	}{
		{
			name:        "base case single worker",
			backfillReq: backfillReq,
			workers:     1,
			handlers:    defaultHandlers,
			jobs:        jobs,
			want: autogold.Expect([]string{
				"point pointtime:2022-04-21T00:00:00Z value:10",
				"point pointtime:2022-04-14T00:00:00Z value:10",
				"point pointtime:2022-04-07T00:00:00Z value:10",
				"point pointtime:2022-04-01T00:00:00Z value:10",
				"error occurred: false",
			}),
		},
		{
			name:        "base case multiple worker",
			backfillReq: backfillReq,
			workers:     2,
			handlers:    defaultHandlers,
			jobs:        jobs,
			want: autogold.Expect([]string{
				"point pointtime:2022-04-21T00:00:00Z value:10",
				"point pointtime:2022-04-14T00:00:00Z value:10",
				"point pointtime:2022-04-07T00:00:00Z value:10",
				"point pointtime:2022-04-01T00:00:00Z value:10",
				"error occurred: false",
			}),
		},
		{
			name:        "canceled context",
			backfillReq: backfillReq,
			workers:     1,
			handlers:    defaultHandlers,
			cancled:     true,
			jobs:        jobs,
			want:        autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name:        "some search fail single worker",
			backfillReq: backfillReq,
			workers:     1,
			handlers:    map[types.GenerationMethod]queryrunner.InsightsHandler{types.Search: makeTestSearchHandlerErr(errors.New("search error"), 2)},
			jobs:        jobs,
			want:        autogold.Expect([]string{"error occurred: true"}),
		},
		{
			name:        "some search fail multiple worker",
			backfillReq: backfillReq,
			workers:     2,
			handlers:    map[types.GenerationMethod]queryrunner.InsightsHandler{types.Search: makeTestSearchHandlerErr(errors.New("search error"), 2)},
			jobs:        jobs,
			want:        autogold.Expect([]string{"error occurred: true"}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancled {
				cancel()
			}
			unlimitedLimiter := ratelimit.NewInstrumentedLimiter("", rate.NewLimiter(rate.Inf, 100))
			searchFunc := makeRunSearchFunc(tc.handlers, tc.workers, unlimitedLimiter)

			_, points, err := searchFunc(testCtx, &requestContext{backfillRequest: backfillReq}, tc.jobs)

			got := []string{}
			// sorted points to make test stable
			sort.SliceStable(points, func(i, j int) bool {
				return points[i].Point.Time.After(points[j].Point.Time)
			})
			for _, p := range points {
				got = append(got, fmt.Sprintf("point pointtime:%s value:%d", p.Point.Time.Format(time.RFC3339Nano), int(p.Point.Value)))
			}
			got = append(got, fmt.Sprintf("error occurred: %v", err != nil))
			tc.want.Equal(t, got)
		})
	}
}
