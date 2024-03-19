package goroutine

import (
	"context"
	"sync"
	"time"

	"github.com/derision-test/glock"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/sourcegraph/sourcegraph/internal/goroutine/recorder"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type (
	getIntervalFunc    func() time.Duration
	getConcurrencyFunc func() int
)

// PeriodicGoroutine represents a goroutine whose main behavior is reinvoked periodically.
//
// See
// https://sourcegraph.com/docs/dev/background-information/backgroundroutine
// for more information and a step-by-step guide on how to implement a
// PeriodicBackgroundRoutine.
type PeriodicGoroutine struct {
	name              string
	description       string
	jobName           string
	recorder          *recorder.Recorder
	getInterval       getIntervalFunc
	initialDelay      time.Duration
	getConcurrency    getConcurrencyFunc
	handler           Handler
	operation         *observation.Operation
	clock             glock.Clock
	concurrencyClock  glock.Clock
	ctx               context.Context    // root context passed to the handler
	cancel            context.CancelFunc // cancels the root context
	finished          chan struct{}      // signals that Start has finished
	reinvocationsLock sync.Mutex
	reinvocations     int
}

var _ recorder.Recordable = &PeriodicGoroutine{}

// Handler represents the main behavior of a PeriodicGoroutine. Additional
// interfaces like ErrorHandler can also be implemented.
type Handler interface {
	// Handle performs an action with the given context.
	Handle(ctx context.Context) error
}

// ErrorHandler is an optional extension of the Handler interface.
type ErrorHandler interface {
	// HandleError is called with error values returned from Handle. This will not
	// be called with error values due to a context cancellation during a graceful
	// shutdown.
	HandleError(err error)
}

// Finalizer is an optional extension of the Handler interface.
type Finalizer interface {
	// OnShutdown is called after the last call to Handle during a graceful shutdown.
	OnShutdown()
}

// HandlerFunc wraps a function, so it can be used as a Handler.
type HandlerFunc func(ctx context.Context) error

func (f HandlerFunc) Handle(ctx context.Context) error {
	return f(ctx)
}

type Option func(*PeriodicGoroutine)

func WithName(name string) Option {
	return func(p *PeriodicGoroutine) { p.name = name }
}

func WithDescription(description string) Option {
	return func(p *PeriodicGoroutine) { p.description = description }
}

func WithInterval(interval time.Duration) Option {
	return WithIntervalFunc(func() time.Duration { return interval })
}

func WithIntervalFunc(getInterval getIntervalFunc) Option {
	return func(p *PeriodicGoroutine) { p.getInterval = getInterval }
}

func WithConcurrency(concurrency int) Option {
	return WithConcurrencyFunc(func() int { return concurrency })
}

func WithConcurrencyFunc(getConcurrency getConcurrencyFunc) Option {
	return func(p *PeriodicGoroutine) { p.getConcurrency = getConcurrency }
}

func WithOperation(operation *observation.Operation) Option {
	return func(p *PeriodicGoroutine) { p.operation = operation }
}

// WithInitialDelay sets the initial delay before the first invocation of the handler.
func WithInitialDelay(delay time.Duration) Option {
	return func(p *PeriodicGoroutine) { p.initialDelay = delay }
}

// NewPeriodicGoroutine creates a new PeriodicGoroutine with the given handler. The context provided will propagate into
// the executing goroutine and will terminate the goroutine if cancelled.
func NewPeriodicGoroutine(ctx context.Context, handler Handler, options ...Option) *PeriodicGoroutine {
	r := newDefaultPeriodicRoutine()
	for _, o := range options {
		o(r)
	}

	ctx, cancel := context.WithCancel(ctx)
	r.ctx = ctx
	r.cancel = cancel
	r.finished = make(chan struct{})
	r.handler = handler

	// If no operation is provided, create a default one that only handles logging.
	// We disable tracing and metrics by default - if any of these should be
	// enabled, caller should use goroutine.WithOperation
	if r.operation == nil {
		r.operation = observation.NewContext(
			log.Scoped("periodic"),
			observation.Tracer(noop.NewTracerProvider().Tracer("noop")),
			observation.Metrics(metrics.NoOpRegisterer),
		).Operation(observation.Op{
			Name:        r.name,
			Description: r.description,
		})
	}

	return r
}

func newDefaultPeriodicRoutine() *PeriodicGoroutine {
	return &PeriodicGoroutine{
		name:             "<unnamed periodic routine>",
		description:      "<no description provided>",
		getInterval:      func() time.Duration { return time.Second },
		getConcurrency:   func() int { return 1 },
		operation:        nil,
		clock:            glock.NewRealClock(),
		concurrencyClock: glock.NewRealClock(),
	}
}

func (r *PeriodicGoroutine) Name() string                                 { return r.name }
func (r *PeriodicGoroutine) Type() recorder.RoutineType                   { return typeFromOperations(r.operation) }
func (r *PeriodicGoroutine) Description() string                          { return r.description }
func (r *PeriodicGoroutine) Interval() time.Duration                      { return r.getInterval() }
func (r *PeriodicGoroutine) Concurrency() int                             { return r.getConcurrency() }
func (r *PeriodicGoroutine) JobName() string                              { return r.jobName }
func (r *PeriodicGoroutine) SetJobName(jobName string)                    { r.jobName = jobName }
func (r *PeriodicGoroutine) RegisterRecorder(recorder *recorder.Recorder) { r.recorder = recorder }

// Start begins the process of calling the registered handler in a loop. This process will
// wait the interval supplied at construction between invocations.
func (r *PeriodicGoroutine) Start() {
	if r.recorder != nil {
		go r.recorder.LogStart(r)
	}
	defer close(r.finished)

	r.runHandlerPool()

	if h, ok := r.handler.(Finalizer); ok {
		h.OnShutdown()
	}
}

// Stop will cancel the context passed to the handler function to stop the current
// iteration of work, then break the loop in the Start method so that no new work
// is accepted. This method blocks until Start has returned.
func (r *PeriodicGoroutine) Stop() {
	if r.recorder != nil {
		go r.recorder.LogStop(r)
	}
	r.cancel()
	<-r.finished
}

func (r *PeriodicGoroutine) runHandlerPool() {
	drain := func() {}

	for concurrency := range r.concurrencyUpdates() {
		// drain previous pool
		drain()

		// create new pool with updated concurrency
		drain = r.startPool(concurrency)
	}

	// channel closed, drain pool
	drain()
}

const concurrencyRecheckInterval = time.Second * 30

func (r *PeriodicGoroutine) concurrencyUpdates() <-chan int {
	var (
		ch        = make(chan int, 1)
		prevValue = r.getConcurrency()
	)

	ch <- prevValue

	go func() {
		defer close(ch)

		for {
			select {
			case <-r.concurrencyClock.After(concurrencyRecheckInterval):
			case <-r.ctx.Done():
				return
			}

			newValue := r.getConcurrency()
			if newValue == prevValue {
				continue
			}

			prevValue = newValue
			forciblyWriteToBufferedChannel(ch, newValue)
		}
	}()

	return ch
}

func (r *PeriodicGoroutine) startPool(concurrency int) func() {
	g := conc.NewWaitGroup()
	ctx, cancel := context.WithCancel(context.Background())

	for range concurrency {
		g.Go(func() { r.runHandlerPeriodically(ctx) })
	}

	return func() {
		cancel()
		g.Wait()
	}
}

func (r *PeriodicGoroutine) runHandlerPeriodically(monitorCtx context.Context) {
	// Create a ctx based on r.ctx that gets canceled when monitorCtx is canceled
	// This ensures that we don't block inside of runHandlerAndDetermineBackoff
	// below when one of the exit conditions are met.

	handlerCtx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	go func() {
		<-monitorCtx.Done()
		cancel()
	}()

	select {
	// Initial delay sleep - might be a zero-duration value if it wasn't set,
	// but this gives us a nice chance to check the context to see if we should
	// exit naturally.
	case <-r.clock.After(r.initialDelay):

	case <-r.ctx.Done():
		// Goroutine is shutting down
		return

	case <-monitorCtx.Done():
		// Caller is requesting we return to resize the pool
		return
	}

	for {
		interval, ok := r.runHandlerAndDetermineBackoff(handlerCtx)
		if !ok {
			// Goroutine is shutting down
			// (the handler returned the context's error)
			return
		}

		select {
		// Sleep - might be a zero-duration value if we're immediately reinvoking,
		// but this gives us a nice chance to check the context to see if we should
		// exit naturally.
		case <-r.clock.After(interval):

		case <-r.ctx.Done():
			// Goroutine is shutting down
			return

		case <-monitorCtx.Done():
			// Caller is requesting we return to resize the pool
			return
		}
	}
}

const maxConsecutiveReinvocations = 100

func (r *PeriodicGoroutine) runHandlerAndDetermineBackoff(ctx context.Context) (time.Duration, bool) {
	handlerErr := r.runHandler(ctx)
	if handlerErr != nil {
		if isShutdownError(ctx, handlerErr) {
			// Caller is exiting
			return 0, false
		}

		if filteredErr := errorFilter(ctx, handlerErr); filteredErr != nil {
			// It's a real error, see if we need to handle it
			if h, ok := r.handler.(ErrorHandler); ok {
				h.HandleError(filteredErr)
			}
		}
	}

	return r.getNextInterval(isReinvokeImmediatelyError(handlerErr)), true
}

func (r *PeriodicGoroutine) getNextInterval(tryReinvoke bool) time.Duration {
	r.reinvocationsLock.Lock()
	defer r.reinvocationsLock.Unlock()

	if tryReinvoke {
		r.reinvocations++

		if r.reinvocations < maxConsecutiveReinvocations {
			// Return zero, do not sleep any significant time
			return 0
		}
	}

	// We're not immediately re-invoking or we would've exited earlier.
	// Reset our count so we can begin fresh on the next call
	r.reinvocations = 0

	// Return our configured interval
	return r.getInterval()
}

func (r *PeriodicGoroutine) runHandler(ctx context.Context) error {
	return r.withOperation(ctx, func(ctx context.Context) error {
		return r.withRecorder(ctx, r.handler.Handle)
	})
}

func (r *PeriodicGoroutine) withOperation(ctx context.Context, f func(ctx context.Context) error) error {
	if r.operation == nil {
		return f(ctx)
	}

	var observedError error
	ctx, _, endObservation := r.operation.With(ctx, &observedError, observation.Args{})
	err := f(ctx)
	observedError = errorFilter(ctx, err)
	endObservation(1, observation.Args{})

	return err
}

func (r *PeriodicGoroutine) withRecorder(ctx context.Context, f func(ctx context.Context) error) error {
	if r.recorder == nil {
		return runAndConvertPanicToError(ctx, f)
	}

	start := time.Now()
	err := runAndConvertPanicToError(ctx, f)
	duration := time.Since(start)

	go func() {
		r.recorder.SaveKnownRoutine(r)
		r.recorder.LogRun(r, duration, errorFilter(ctx, err))
	}()

	return err
}

// runAndConvertPanicToError invokes f with the given ctx and recovers any panics
// by turning them into an error instead.
func runAndConvertPanicToError(ctx context.Context, f func(ctx context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = errors.Wrap(e, "panic occurred")
			} else {
				err = errors.Newf("panic occurred: %v", r)
			}
		}
	}()
	return f(ctx)
}

func typeFromOperations(operation *observation.Operation) recorder.RoutineType {
	if operation != nil {
		return recorder.PeriodicWithMetrics
	}

	return recorder.PeriodicRoutine
}

func isShutdownError(ctx context.Context, err error) bool {
	return ctx.Err() != nil && errors.Is(err, ctx.Err())
}

var ErrReinvokeImmediately = errors.New("periodic handler wishes to be immediately re-invoked")

func isReinvokeImmediatelyError(err error) bool {
	return errors.Is(err, ErrReinvokeImmediately)
}

func errorFilter(ctx context.Context, err error) error {
	if isShutdownError(ctx, err) || isReinvokeImmediatelyError(err) {
		return nil
	}

	return err
}

func forciblyWriteToBufferedChannel[T any](ch chan T, value T) {
	for {
		select {
		case ch <- value:
			// Write succeeded
			return

		default:
			select {
			// Buffer is full
			// Pop item if we can and retry the write on the next iteration
			case <-ch:
			default:
			}
		}
	}
}
