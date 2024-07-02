package guardrails

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewCompletionsFilter2 returns a fully initialized streaming filter.
// This filter should be used for only a single code completions streaming
// since it keeps internal state. `Send` and `WaitDone` methods
// are expected to be called from single goroutine,
// and `WaitDone` is only invoked after all `Send` calls have finished.
func NewCompletionsFilter2(config CompletionsFilterConfig) (CompletionsFilter, error) {
	if config.Sink == nil || config.Test == nil || config.AttributionError == nil {
		return nil, errors.New("Attribution filtering misconfigured.")
	}
	return &attributionRunFilter{
		config:                config,
		closeOnSearchFinished: make(chan struct{}),
	}, nil
}

// attributionRunFilter implementation of CompletionsFilter that runs attribution search for snippets
// aboce certain threshold defined by `SnippetLowerBound`.
// It's inspired by an idea of [communicating sequential processes](https://en.wikipedia.org/wiki/Communicating_sequential_processes):
//   - Attribution search is going to run async of passing down the completion from the LLM.
//   - But all the work of actually forwarding LLM completions happens only on the _caller_
//     thread, that is the one that controls the filter by calling `Send` and `WaitDone`.
//     Hopefully this will ensure proper desctruction of response sending and no races
//     with attribution search.
//   - The synchronization with attribution search happens on 3 elements:
//     1.  `sync.Once` is used to ensure attribution search is only fired once.
//     This simplifies logic of starting search once snippet passed a given threshold.
//     2.  `atomic.Bool` is set to true once we confirm no attribution was found.
//     This makes it real easy for `Send` to make a decision on the spot whether or not
//     to forward a completion back to the client.
//     3.  `chan struct{}` is closed on attribution search finishing.
//     This is a robust way for `WaitDone` to wait on either context cancellation (timeout)
//     or attribution search finishing (via select). Channel closing happens
//     in the attribution search routine, which is fired via sync.Once, so no chance
//     of multiple goroutines closing the channel.
type attributionRunFilter struct {
	config CompletionsFilterConfig
	// Just to make sure we have run attribution once.
	attributionSearch sync.Once
	// Attribution search result, true = successful, false is any of {not run, pending, failed, error}.
	attributionSucceeded atomic.Bool
	// Channel that the attribution routine closes when attribution is finished.
	closeOnSearchFinished chan struct{}
	// Last seen completion that was not sent. Unsynchronized - only referred
	// to from Send and WaitDone, which are executed in the same request handler routine.
	last *types.CompletionResponse
	// totalCompletion tracks the whole completion string in memory, in delta mode
	// we otherwise cannot keep track of the whole string to search for.
	totalCompletion string
}

// Send forwards the completion to the client if it can given its current idea
// about attribution status. Otherwise it memoizes completion in `attributionRunFilter#last`.
func (d *attributionRunFilter) Send(ctx context.Context, r types.CompletionResponse) error {
	if len(r.DeltaText) > 0 {
		d.totalCompletion += r.DeltaText
	} else if len(r.Completion) > 0 {
		d.totalCompletion = r.Completion
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if d.shortEnough() {
		d.last = nil // last seen completion was sent
		return d.config.Sink(r)
	}

	d.attributionSearch.Do(func() { go d.runAttribution(ctx, d.totalCompletion) })

	if d.last != nil {
		r.DeltaText = d.last.DeltaText + r.DeltaText
	}

	if d.attributionSucceeded.Load() {
		d.last = nil // last seen completion was sent
		return d.config.Sink(r)
	}

	d.last = &r // last seen completion not sent
	return nil
}

// WaitDone awaits either attribution search finishing or timeout.
// The caller calls WaitDone only after all calls to send, so LLM is done streaming.
// This is why in case of attribution search finishing it's enough for us
// to send the last memoized completion here.
func (d *attributionRunFilter) WaitDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// Request cancelled, return.
		return ctx.Err()
	case <-d.closeOnSearchFinished:
		// When search finishes successfully and last seen completion was not sent, send it now, and finish.
		if d.attributionSucceeded.Load() && d.last != nil {
			return d.config.Sink(*d.last)
		}
		return nil
	}
}

func (d *attributionRunFilter) shortEnough() bool {
	return !NewThreshold().ShouldSearch(d.totalCompletion)
}

// runAttribution is a blocking function that defines the goroutine
// that runs attribution search and then synchronizes with main thread
// by setting `atomic.Bool` for flagging and closing `chan struct{}`
// to notify `WaitDone` if needed.
func (d *attributionRunFilter) runAttribution(ctx context.Context, snippet string) {
	defer close(d.closeOnSearchFinished)
	canUse, err := d.config.Test(ctx, snippet)
	if err != nil {
		d.config.AttributionError(err)
		return
	}
	if canUse {
		d.attributionSucceeded.Store(true)
	}
}
