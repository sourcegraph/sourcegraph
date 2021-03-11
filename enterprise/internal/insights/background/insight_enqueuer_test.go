package background

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hexops/autogold"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/discovery"
	"github.com/sourcegraph/sourcegraph/internal/api"
)

var testRealGlobalSettings = &api.Settings{ID: 1, Contents: `{
	"insights": [
		{
		  "title": "fmt usage",
		  "description": "fmt.Errorf/fmt.Printf usage",
		  "series": [
			{
			  "label": "fmt.Errorf",
			  "search": "errorf",
			},
			{
			  "label": "printf",
			  "search": "fmt.Printf",
			},
			{
			  "label": "duplicate",
			  "search": "errorf",
			}
		  ]
		},
		{
			"title": "gitserver usage",
			"description": "gitserver exec & close usage",
			"series": [
			  {
				"label": "exec",
				"search": "gitserver.Exec",
			  },
			  {
				"label": "close",
				"search": "gitserver.Close",
			  },
			  {
				"label": "duplicate",
				"search": "gitserver.Close",
			  }
			]
		  }
		]
	}
`}

// Test_discoverAndEnqueueInsights tests that insight discovery and job enqueueing works and
// adheres to a few properties:
//
// 1. Webhook insights are not enqueued (not yet supported.)
// 2. Duplicate insights are deduplicated / do not submit multiple jobs.
// 3. Jobs are scheduled not to all run at the same time.
//
func Test_discoverAndEnqueueInsights(t *testing.T) {
	// Setup the setting store and job enqueuer mocks.
	ctx := context.Background()
	settingStore := discovery.NewMockSettingStore()
	settingStore.GetLatestFunc.SetDefaultReturn(testRealGlobalSettings, nil)
	var enqueued []*queryrunner.Job
	enqueueQueryRunnerJob := func(ctx context.Context, job *queryrunner.Job) error {
		enqueued = append(enqueued, job)
		return nil
	}

	// Create a fake clock so the times reported in our test data do not change and can be easily verified.
	now, err := time.Parse(time.RFC3339, "2020-03-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return now }

	if err := discoverAndEnqueueInsights(ctx, clock, settingStore, enqueueQueryRunnerJob); err != nil {
		t.Fatal(err)
	}

	// JSON marshal to keep times formatted nicely.
	enqueuedJSON, err := json.MarshalIndent(enqueued, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	autogold.Want("0", `[
  {
    "SeriesID": "s:087855E6A24440837303FD8A252E9893E8ABDFECA55B61AC83DA1B521906626E",
    "SearchQuery": "errorf count:9999999",
    "RecordTime": null,
    "ID": 0,
    "State": "queued",
    "FailureMessage": null,
    "StartedAt": null,
    "FinishedAt": null,
    "ProcessAfter": "2020-03-01T00:00:00Z",
    "NumResets": 0,
    "NumFailures": 0,
    "ExecutionLogs": null
  },
  {
    "SeriesID": "s:7FBD292BF97936C4B6397688CFFB05DEA95E650C3D5B653AAEA8F77BBD25CE93",
    "SearchQuery": "fmt.Printf count:9999999",
    "RecordTime": null,
    "ID": 0,
    "State": "queued",
    "FailureMessage": null,
    "StartedAt": null,
    "FinishedAt": null,
    "ProcessAfter": "2020-03-01T00:00:30Z",
    "NumResets": 0,
    "NumFailures": 0,
    "ExecutionLogs": null
  },
  {
    "SeriesID": "s:FB8CFBB7C7C28834957FBE1B830EDD79C5E710FD55B0ACF246C0D7267C5462B4",
    "SearchQuery": "gitserver.Exec count:9999999",
    "RecordTime": null,
    "ID": 0,
    "State": "queued",
    "FailureMessage": null,
    "StartedAt": null,
    "FinishedAt": null,
    "ProcessAfter": "2020-03-01T00:01:00Z",
    "NumResets": 0,
    "NumFailures": 0,
    "ExecutionLogs": null
  },
  {
    "SeriesID": "s:2B55C7CE2EB30BFFAF1F0276E525B36BB71908E3893A27F416F62A3E23542566",
    "SearchQuery": "gitserver.Close count:9999999",
    "RecordTime": null,
    "ID": 0,
    "State": "queued",
    "FailureMessage": null,
    "StartedAt": null,
    "FinishedAt": null,
    "ProcessAfter": "2020-03-01T00:01:30Z",
    "NumResets": 0,
    "NumFailures": 0,
    "ExecutionLogs": null
  }
]`).Equal(t, string(enqueuedJSON))
}
