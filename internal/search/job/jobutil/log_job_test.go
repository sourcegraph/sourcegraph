package jobutil_test

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/internal/search/job/mockjob"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/schema"
)

type fakeEventLogStore struct {
	database.EventLogStore
	events []*database.Event
}

func (s *fakeEventLogStore) BulkInsert(_ context.Context, newEvents []*database.Event) error {
	s.events = append(s.events, newEvents...)
	return nil
}

func (s *fakeEventLogStore) loggedEventNames() []string {
	var names []string
	for _, e := range s.events {
		var present bool
		for _, n := range names {
			present = present || e.Name == n
		}
		if !present {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names
}

func TestOwnSearchEventNames(t *testing.T) {
	for literal, wantEventNames := range map[string][]string{
		"file:has.owner(one@example.com)": {"FileHasOwnersSearch", "search.latencies.file"},
		"select:file.owners":              {"SelectFileOwnersSearch", "search.latencies.repo"},
	} {
		t.Run(literal, func(t *testing.T) {
			q, err := query.ParseLiteral(literal)
			if err != nil {
				t.Fatalf("ParseLiteral: %s", err)
			}
			inputs := &search.Inputs{
				UserSettings:        &schema.Settings{},
				PatternType:         query.SearchTypeLiteral,
				Protocol:            search.Streaming,
				OnSourcegraphDotCom: true,
				Features: &search.Features{
					CodeOwnershipSearch: true,
				},
				Query: q,
			}
			db := database.NewMockDB()
			eventStore := &fakeEventLogStore{}
			db.EventLogsFunc.SetDefaultReturn(eventStore)
			ctx := actor.WithActor(context.Background(), actor.FromUser(42))
			childJob := mockjob.NewMockJob()
			childJob.RunFunc.SetDefaultHook(func(_ context.Context, _ job.RuntimeClients, s streaming.Sender) (*search.Alert, error) {
				return nil, nil
			})
			voidCollector := streaming.StreamFunc(func(ev streaming.SearchEvent) {})
			logJob := jobutil.NewLogJob(inputs, childJob)
			if _, err := logJob.Run(ctx, job.RuntimeClients{DB: db}, voidCollector); err != nil {
				t.Fatalf("LogJob.Run: %s", err)
			}
			if diff := cmp.Diff(wantEventNames, eventStore.loggedEventNames()); diff != "" {
				t.Errorf("logged events, -want+got: %s", diff)
			}

		})
	}
}
