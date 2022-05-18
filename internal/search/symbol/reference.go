package symbol

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
)

type ReferenceSearcherJob struct {
	//RepoOptions search.RepoOptions
}

func (s *ReferenceSearcherJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {

	stream.Send(streaming.SearchEvent{
		Results: result.Matches{&result.FileMatch{
			LineMatches: []*result.LineMatch{{
				Preview: "hi",
			}},
		}},
	})

	return nil, nil
}

func (*ReferenceSearcherJob) Name() string {
	return "ReferenceSearcherJob"
}
