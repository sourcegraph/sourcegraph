package streaming

import (
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/sourcegraph/sourcegraph/internal/search/result"
)

type SearchEvent struct {
	Results result.Matches
	Stats   Stats
}

type Sender interface {
	Send(SearchEvent)
}

type StreamFunc func(SearchEvent)

func (f StreamFunc) Send(se SearchEvent) {
	f(se)
}

// NewAggregatingStream returns a stream that collects all the events
// sent to it. The aggregated event can be retrieved with Get().
func NewAggregatingStream() *aggregatingStream {
	return &aggregatingStream{}
}

type aggregatingStream struct {
	sync.Mutex
	SearchEvent
}

func (c *aggregatingStream) Send(event SearchEvent) {
	c.Lock()
	c.Results = append(c.Results, event.Results...)
	c.Stats.Update(&event.Stats)
	c.Unlock()
}

func NewNullStream() Sender {
	return StreamFunc(func(SearchEvent) {})
}

func NewStatsObservingStream(s Sender) *statsObservingStream {
	return &statsObservingStream{
		parent: s,
	}
}

type statsObservingStream struct {
	parent Sender

	sync.Mutex
	Stats
}

func (s *statsObservingStream) Send(event SearchEvent) {
	s.Lock()
	s.Stats.Update(&event.Stats)
	s.Unlock()
	s.parent.Send(event)
}

func NewResultCountingStream(s Sender) *resultCountingStream {
	return &resultCountingStream{
		parent: s,
	}
}

type resultCountingStream struct {
	parent Sender
	count  atomic.Int64
}

func (c *resultCountingStream) Send(event SearchEvent) {
	c.count.Add(int64(event.Results.ResultCount()))
	c.parent.Send(event)
}

func (c *resultCountingStream) Count() int {
	return int(c.count.Load())
}

// NewBatchingStream returns a stream that batches results sent to it, holding
// delaying their forwarding by a max time of maxDelay, then sending the batched
// event to the parent stream. The first event is passed through without delay.
// When there will be no more events sent on the batching stream, Done() must be
// called to flush the remaining batched events.
func NewBatchingStream(maxDelay time.Duration, parent Sender) *batchingStream {
	return &batchingStream{
		parent:   parent,
		maxDelay: maxDelay,
	}
}

type batchingStream struct {
	parent   Sender
	maxDelay time.Duration

	mu             sync.Mutex
	sentFirstEvent bool
	dirty          bool
	batch          SearchEvent
	timer          *time.Timer
	flushScheduled bool
}

func (s *batchingStream) Send(event SearchEvent) {
	s.mu.Lock()

	// Update the batch
	s.batch.Results = append(s.batch.Results, event.Results...)
	s.batch.Stats.Update(&event.Stats)
	s.dirty = true

	// If this is our first event with results, flush immediately
	if !s.sentFirstEvent && len(event.Results) > 0 {
		s.sentFirstEvent = true
		s.flush()
		s.mu.Unlock()
		return
	}

	if s.timer == nil {
		// Create a timer and schedule a flush
		s.timer = time.AfterFunc(s.maxDelay, func() {
			s.mu.Lock()
			s.flush()
			s.flushScheduled = false
			s.mu.Unlock()
		})
		s.flushScheduled = true
	} else if !s.flushScheduled {
		// Reuse the timer, scheduling a new flush
		s.timer.Reset(s.maxDelay)
		s.flushScheduled = true
	}
	// If neither of those conditions is true,
	// a flush has already been scheduled and
	// we're good to go.
	s.mu.Unlock()
}

// Done should be called when no more events will be sent down
// the stream. It flushes any events that are currently batched
// and cancels any scheduled flush.
func (s *batchingStream) Done() {
	s.mu.Lock()
	// Cancel any scheduled flush
	if s.timer != nil {
		s.timer.Stop()
	}

	s.flush()
	s.mu.Unlock()
}

// flush sends the currently batched events to the parent stream. The caller must hold
// a lock on the batching stream.
func (s *batchingStream) flush() {
	if s.dirty {
		s.parent.Send(s.batch)
		s.batch = SearchEvent{}
		s.dirty = false
	}
}
