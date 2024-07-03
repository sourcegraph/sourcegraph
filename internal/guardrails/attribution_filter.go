package guardrails

import (
	"context"
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// AttributionTest is a predicate that tells whether given snippet can be freely used.
// Returning false indicates that some attribution of snippet was found, and the snippet
// should be used with caution. The operation can be long-running, and should respect
// context cancellation.
type AttributionTest func(context.Context, string) (bool, error)

// CompletionEventSink is where selected events end up being streamed to.
type CompletionEventSink func(types.CompletionResponse) error

// CompletionsFilter is used to filter out the completions with potential risk
// due to attribution.
//
// Example usage:
// ```
//
//	sink := func (e types.CompletionResponse) error {
//	  // stream completions response back to the client
//	}
//
//	test := func (ctx context.Context, snippet string) bool {
//	  // execute attribution search and return true/false.
//	}
//
// filter := NewCompletionsFilter(sink, test)
// // as completion events arrive:
// err := filter.Send(ctx, event)
// // all send operations finished:
// err := filter.WaitDone(ctx)
// ```
type CompletionsFilter interface {
	// Send is invoked each time new completion prefix arrives as the completion
	// is being yielded by the LLM.
	Send(context.Context, types.CompletionResponse) error

	// WaitDone is called after all the completions have finished arriving
	// in order to await on async attribution within time limits.
	// In other words all calls to `Send` will have finished. Then `WaitDone`
	// is called and it will wait for remaining attribution search if context
	// time limits permit.
	WaitDone(context.Context) error
}

type completionsFilter struct {
	config CompletionsFilterConfig
	// attributionRun synchronizes on attribution to run only once.
	attributionRun sync.Once
	// attributionFinished has to have len=1 and stores error from attribution
	// run once it's finished.
	attributionFinished chan error
	// mu guards all the following fields
	mu sync.Mutex
	// sendingBlocked = true indicates that eventWriter should not be used anymore
	sendingBlocked bool
	// attributionResult is nil if attribution search has not finished yet,
	// true if it finished, and result was permissive, and false otherwise
	attributionResult *bool
	// mostRecentCompletion memoized. We keep the most recent
	// completion passed over to Send while attribution is running. Once attribution
	// finishes, we carry on sending from the most recent completion.
	mostRecentCompletion types.CompletionResponse
	// totalCompletion tracks the whole completion string in memory, in delta mode
	// we otherwise cannot keep track of the whole string to search for.
	totalCompletion string
}

// CompletionsFilterConfig assembles parameters needed to create new CompletionFilter.
type CompletionsFilterConfig struct {
	Sink             CompletionEventSink
	Test             AttributionTest
	AttributionError func(err error)
}

// NewCompletionsFilter returns a fully initialized streaming filter.
// This filter should be used for only a single code completions streaming
// since it keeps internal state. Public methods are synchronized.
func NewCompletionsFilter(config CompletionsFilterConfig) (CompletionsFilter, error) {
	if config.Sink == nil || config.Test == nil || config.AttributionError == nil {
		return nil, errors.New("Attribution filtering misconfigured.")
	}
	return &completionsFilter{
		config:              config,
		attributionFinished: make(chan error, 1),
	}, nil
}

// Send is invoked each time new completion prefix arrives as the completion
// is being yielded by the LLM.
func (a *completionsFilter) Send(ctx context.Context, e types.CompletionResponse) error {
	a.setTotalCompletion(e)
	if err := ctx.Err(); err != nil {
		a.blockSending()
		return err
	}
	if a.attributionResultPermissive() {
		return a.send(e)
	}
	if a.smallEnough() {
		return a.send(e)
	}
	a.setMostRecentCompletion(e)
	a.attributionRun.Do(func() {
		go a.runAttribution(ctx)
	})
	return nil
}

// WaitDone is called after all the completions have finished arriving
// in order to await on async attribution within time limits.
// In other words all calls to `Send` will have finished. Then `WaitDone`
// is called and it will wait for remaining attribution search if context
// time limits permit.
func (a *completionsFilter) WaitDone(ctx context.Context) error {
	// If attribution never run, we're done.
	a.attributionRun.Do(func() {
		close(a.attributionFinished)
	})
	select {
	case <-ctx.Done():
		a.blockSending()
		return ctx.Err()
	case err := <-a.attributionFinished:
		return err
	}
}

func (a *completionsFilter) setTotalCompletion(e types.CompletionResponse) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(e.DeltaText) > 0 {
		a.totalCompletion += e.DeltaText
	} else if len(e.Completion) > 0 {
		a.totalCompletion = e.Completion
	}
}

// attributionResultPermissive states whether attribution search
// finished and is permissive - that is no attribution was found.
func (a *completionsFilter) attributionResultPermissive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.attributionResult != nil && *a.attributionResult
}

// setAttributionResult is invoked to note the result of attribution search.
// If true, attribution is emtpy, and the snippet is safe to be used.
// If false, attribution is not empty, and the snippet should be used with caution.
func (a *completionsFilter) setAttributionResult(attributionResult bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.attributionResult = &attributionResult
}

// send invokes eventWriter if available. Sending is only enabled
// as long as the http stream is still being served - in other words
// call to `WaitDone` has not returned.
func (a *completionsFilter) send(e types.CompletionResponse) error {
	if !a.canSend() {
		return nil
	}
	return a.config.Sink(e)
}

// smallEnough indicates whether snippet size is small enough not to consider
// it for attribution search. At this point we run attribution search for
// snippets 10 lines long or longer.
func (a *completionsFilter) smallEnough() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return !NewThreshold().ShouldSearch(a.totalCompletion)
}

// getMostRecentCompletion returns the last completion event to be fed to `Send`.
func (a *completionsFilter) getMostRecentCompletion() types.CompletionResponse {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.mostRecentCompletion
}

// setMostRecentCompletion overwrites the last completion event passed to `Send`.
func (a *completionsFilter) setMostRecentCompletion(e types.CompletionResponse) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Copy so we don't modify the incoming event:
	next := e
	next.DeltaText = a.mostRecentCompletion.DeltaText + e.DeltaText
	a.mostRecentCompletion = next
}

// runAttribution performs attribution search and updates the state of the object
// after finishing. It's run within `attributionRun` to ensure it only runs once.
func (a *completionsFilter) runAttribution(ctx context.Context) {
	a.mu.Lock()
	totalCompletion := a.totalCompletion
	a.mu.Unlock()

	result, err := a.config.Test(ctx, totalCompletion)
	if err != nil {
		a.config.AttributionError(err)
	}
	a.setAttributionResult(result)
	if result {
		err := a.send(a.getMostRecentCompletion())
		a.attributionFinished <- err
	}
	close(a.attributionFinished)
}

// blockSending prevents any operations on the event writer.
func (a *completionsFilter) blockSending() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sendingBlocked = true
}

// canSend states whether event writer can be used.
func (a *completionsFilter) canSend() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.sendingBlocked
}

func NoopCompletionsFilter(sink CompletionEventSink) CompletionsFilter {
	return noopCompletionsFilter{
		sink: sink,
	}
}

type noopCompletionsFilter struct {
	sink CompletionEventSink
}

func (f noopCompletionsFilter) Send(ctx context.Context, e types.CompletionResponse) error {
	return f.sink(e)
}
func (f noopCompletionsFilter) WaitDone(ctx context.Context) error { return nil }
