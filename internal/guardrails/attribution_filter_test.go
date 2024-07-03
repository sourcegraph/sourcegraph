package guardrails_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/guardrails"
)

type fakeClient struct {
	mu     sync.Mutex
	events []types.CompletionResponse
	err    error
}

func (s *fakeClient) stream(e types.CompletionResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return s.err
}

func (s *fakeClient) trimmedDiffs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var prefix string
	var diffs []string
	for _, e := range s.events {
		if e.DeltaText != "" {
			diffs = append(diffs, strings.Split(strings.Trim(e.DeltaText, "\n"), "\n")...)
		} else {
			diffStr := strings.TrimSpace(strings.TrimPrefix(e.Completion, prefix))
			diffs = append(diffs, strings.Split(diffStr, "\n")...)
			prefix = e.Completion
		}
	}
	return diffs
}

type fakeSearch struct {
	mu       sync.Mutex
	snippets []string
	response chan bool
}

func (s *fakeSearch) test(_ context.Context, snippet string) (bool, error) {
	s.mu.Lock()
	s.snippets = append(s.snippets, snippet)
	s.mu.Unlock()
	return <-s.response, nil
}

type eventOrder []event

func (o eventOrder) replay(ctx context.Context, delta bool, f guardrails.CompletionsFilter) error {
	var completionPrefix string
	for _, e := range o {
		if s := e.NextCompletionLine(); s != nil {
			if delta {
				if err := f.Send(ctx, types.CompletionResponse{
					DeltaText: "\n" + *s,
				}); err != nil {
					return err
				}
			} else {
				completionPrefix = fmt.Sprintf("%s\n%s", completionPrefix, *s)
				if err := f.Send(ctx, types.CompletionResponse{
					Completion: completionPrefix,
				}); err != nil {
					return err
				}
			}
		}
		e.Run()
	}
	return f.WaitDone(ctx)
}

type event interface {
	Run()
	NextCompletionLine() *string
}

type nextLine string

func (_ nextLine) Run()                        {}
func (n nextLine) NextCompletionLine() *string { s := string(n); return &s }

type searchFinishes struct {
	search        *fakeSearch
	canUseSnippet bool
}

func (f searchFinishes) Run()                        { f.search.response <- f.canUseSnippet }
func (_ searchFinishes) NextCompletionLine() *string { return nil }

type contextCancelled func()

func (c contextCancelled) Run()                        { c() }
func (_ contextCancelled) NextCompletionLine() *string { return nil }

func bothImplementations(
	t *testing.T,
	test func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool),
) {
	for implementationName, factory := range map[string]func(guardrails.CompletionsFilterConfig) (guardrails.CompletionsFilter, error){
		"guardrails.NewCompletionsFilter (old)":  guardrails.NewCompletionsFilter,
		"guardrails.NewCompletionsFilter2 (new)": guardrails.NewCompletionsFilter2,
	} {
		for delta, testNameSuffix := range map[bool]string{false: " (full response)", true: " (delta)"} {
			client := &fakeClient{}
			search := &fakeSearch{response: make(chan bool)}
			config := guardrails.CompletionsFilterConfig{
				Sink:             client.stream,
				Test:             search.test,
				AttributionError: func(error) {},
			}
			filter, err := factory(config)
			require.NoError(t, err)
			t.Run(implementationName+testNameSuffix, func(t *testing.T) { test(t, filter, client, search, delta) })
		}
	}
}

func TestAttributionNotFound(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx := context.Background()
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			nextLine("10"),
			searchFinishes{search: search, canUseSnippet: true},
		}
		require.NoError(t, o.replay(ctx, delta, f))
		got := client.trimmedDiffs()
		want := []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// Completion with lines 9 and 10 came potentially together
			// after attribution search finished
			"9", "10",
		}
		require.Equal(t, want, got)
	})
}

func TestAttributionFound(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx := context.Background()
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			nextLine("10"),
			searchFinishes{search: search, canUseSnippet: false},
		}
		require.NoError(t, o.replay(ctx, delta, f))
		got := client.trimmedDiffs()
		want := []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// Completion with lines 9 and 10 never arrives,
			// as attribution was found
			// "9", "10",
		}
		require.Equal(t, want, got)
	})
}

func TestAttributionNotFoundMoreDataAfter(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx := context.Background()
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			nextLine("10"),
			searchFinishes{search: search, canUseSnippet: true},
			nextLine("11"),
			nextLine("12"),
		}
		require.NoError(t, o.replay(ctx, delta, f))
		got := client.trimmedDiffs()
		want := []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// Completion with lines 9 and 10 came potentially together
			// after attribution search finished
			"9", "10",
			// Lines 11 and 12 came after search finished, they
			// are streamed through.
			"11", "12",
		}
		require.Equal(t, want, got)
	})
}

func TestAttributionFoundMoreDataAfter(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx := context.Background()
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			nextLine("10"),
			searchFinishes{search: search, canUseSnippet: false},
			nextLine("11"),
			nextLine("12"),
		}
		require.NoError(t, o.replay(ctx, delta, f))
		got := client.trimmedDiffs()
		want := []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// No lines beyond 8 comve since attribution search
			// disallowed it:
			// "9", "10", "11", "12"
		}
		require.Equal(t, want, got)
	})
}

func TestTimeout(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx, cancel := context.WithCancel(context.Background())
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			contextCancelled(cancel),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			nextLine("10"),
		}
		require.ErrorIs(t, o.replay(ctx, delta, f), context.Canceled)
		got := client.trimmedDiffs()
		want := []string{
			"1", "2", "3", "4", "5",
			// Request cancelled before the rest of events arrived:
			// "6", "7", "8", "9", "10",
		}
		require.Equal(t, want, got)
	})
}

func TestTimeoutAfterAttributionFound(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx, cancel := context.WithCancel(context.Background())
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			nextLine("10"),
			searchFinishes{search: search, canUseSnippet: true},
			nextLine("11"),
			contextCancelled(cancel),
			nextLine("12"),
		}
		require.ErrorIs(t, o.replay(ctx, delta, f), context.Canceled)
		t.Skip("TODO(#59863) Still sometimes flakes in not returning lines past 8.")
		got := client.trimmedDiffs()
		want := []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// Completion with lines 9 and 10 arrive potentially
			// together, as attribution was found
			"9", "10",
			// Line 11 manages to arrive while request finishes.
			"11",
			// Timeout. Line 12 never arrives:
			// "12",
		}
		require.Equal(t, want, got)
	})
}

func TestTimeoutBeforeAttributionFound(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx, cancel := context.WithCancel(context.Background())
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			contextCancelled(cancel),
			nextLine("10"),
			searchFinishes{search: search, canUseSnippet: false},
			nextLine("11"),
		}
		require.ErrorIs(t, o.replay(ctx, delta, f), context.Canceled)
		got := client.trimmedDiffs()
		want := []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// Completion with lines 9 and 10 never arrives,
			// because attribution response arrives only after
			// time runs out. Same with the subsequent line.
			// "9", "10", "11"
		}
		require.Equal(t, want, got)
	})
}

func TestAttributionSearchFinishesAfterWaitDoneIsCalled(t *testing.T) {
	bothImplementations(t, func(t *testing.T, f guardrails.CompletionsFilter, client *fakeClient, search *fakeSearch, delta bool) {
		ctx := context.Background()
		o := eventOrder{
			nextLine("1"),
			nextLine("2"),
			nextLine("3"),
			nextLine("4"),
			nextLine("5"),
			nextLine("6"),
			nextLine("7"),
			nextLine("8"),
			nextLine("9"),
			nextLine("10"),
			nextLine("11"),
		}
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			require.NoError(t, o.replay(ctx, delta, f))
			wg.Done()
		}()
		want := []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// Lines that came over while attribution runs
			// not streamed yet
		}
		for got := []string{}; reflect.DeepEqual(want, got); got = client.trimmedDiffs() {
			time.Sleep(10 * time.Millisecond) // Poor man's awaitility.
		}
		search.response <- true // Finish attribution search.
		wg.Wait()               // WaitDone returns.
		got := client.trimmedDiffs()
		want = []string{
			"1", "2", "3", "4", "5", "6", "7", "8",
			// Lines that came over while attribution runs
			// not streamed as part of WaitDone.
			"9", "10", "11",
		}
		require.Equal(t, want, got)
	})
}
