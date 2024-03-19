package smartsearch

import (
	"context"
	"strconv"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/search"
	alertobserver "github.com/sourcegraph/sourcegraph/internal/search/alert"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/mockjob"
	"github.com/sourcegraph/sourcegraph/internal/search/limits"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
)

func TestNewSmartSearchJob_Run(t *testing.T) {
	// Setup: A child job that sends the same result
	mockJob := mockjob.NewMockJob()
	mockJob.RunFunc.SetDefaultHook(func(ctx context.Context, _ job.RuntimeClients, s streaming.Sender) (*search.Alert, error) {
		s.Send(streaming.SearchEvent{
			Results: []result.Match{&result.FileMatch{
				File: result.File{Path: "haut-medoc"},
			}},
		})
		return nil, nil
	})

	mockAutoQuery := &autoQuery{description: "mock", query: query.Basic{}}

	j := FeelingLuckySearchJob{
		initialJob: mockJob,
		generators: []next{func() (*autoQuery, next) { return mockAutoQuery, nil }},
		newGeneratedJob: func(*autoQuery) job.Job {
			return mockJob
		},
	}

	var sent []result.Match
	stream := streaming.StreamFunc(func(e streaming.SearchEvent) {
		sent = append(sent, e.Results...)
	})

	t.Run("deduplicate results returned by generated jobs", func(t *testing.T) {
		j.Run(context.Background(), job.RuntimeClients{}, stream)
		require.Equal(t, 1, len(sent))
	})
}

func TestGeneratedSearchJob(t *testing.T) {
	mockJob := mockjob.NewMockJob()
	setMockJobResultSize := func(n int) {
		mockJob.RunFunc.SetDefaultHook(func(ctx context.Context, _ job.RuntimeClients, s streaming.Sender) (*search.Alert, error) {
			for i := range n {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				default:
				}
				s.Send(streaming.SearchEvent{
					Results: []result.Match{&result.FileMatch{
						File: result.File{Path: strconv.Itoa(i)},
					}},
				})
			}
			return nil, nil
		})
	}

	test := func(resultSize int) string {
		setMockJobResultSize(resultSize)
		q, _ := query.ParseStandard("test")
		mockQuery, _ := query.ToBasicQuery(q)
		notifier := &notifier{autoQuery: &autoQuery{description: "test", query: mockQuery}}
		j := &generatedSearchJob{
			Child:           mockJob,
			NewNotification: notifier.New,
		}
		_, err := j.Run(context.Background(), job.RuntimeClients{}, streaming.NewAggregatingStream())
		if err == nil {
			return ""
		}
		return err.(*alertobserver.ErrLuckyQueries).ProposedQueries[0].Annotations[search.ResultCount]
	}

	autogold.Expect(autogold.Raw("")).Equal(t, autogold.Raw(test(0)))
	autogold.Expect(autogold.Raw("1 result")).Equal(t, autogold.Raw(test(1)))
	autogold.Expect(autogold.Raw("10000+ results")).Equal(t, autogold.Raw(test(limits.DefaultMaxSearchResultsStreaming)))
}

func TestNewSmartSearchJob_ResultCount(t *testing.T) {
	// This test ensures the invariant that generated queries do not run if
	// at least RESULT_THRESHOLD results are emitted by the initial job. If
	// less than RESULT_THRESHOLD results are seen, the logic will run a
	// generated query, which always panics.
	mockJob := mockjob.NewMockJob()
	mockJob.RunFunc.SetDefaultHook(func(ctx context.Context, _ job.RuntimeClients, s streaming.Sender) (*search.Alert, error) {
		for i := range RESULT_THRESHOLD {
			s.Send(streaming.SearchEvent{
				Results: []result.Match{&result.FileMatch{
					File: result.File{Path: strconv.Itoa(i)},
				}},
			})
		}
		return nil, nil
	})

	mockAutoQuery := &autoQuery{description: "mock", query: query.Basic{}}

	j := FeelingLuckySearchJob{
		initialJob: mockJob,
		generators: []next{func() (*autoQuery, next) { return mockAutoQuery, nil }},
		newGeneratedJob: func(*autoQuery) job.Job {
			return mockjob.NewStrictMockJob() // always panic, and should never get run.
		},
	}

	var sent []result.Match
	stream := streaming.StreamFunc(func(e streaming.SearchEvent) {
		sent = append(sent, e.Results...)
	})

	t.Run("do not run generated queries over RESULT_THRESHOLD", func(t *testing.T) {
		j.Run(context.Background(), job.RuntimeClients{}, stream)
		require.Equal(t, RESULT_THRESHOLD, len(sent))
	})
}
