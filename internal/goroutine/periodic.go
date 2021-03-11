package goroutine

import (
	"context"
	"errors"
	"time"

	"github.com/efritz/glock"
	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// PeriodicGoroutine represents a goroutine whose main behavior is reinvoked periodically.
//
// See
// https://docs.sourcegraph.com/dev/background-information/backgroundroutine
// for more information and a step-by-step guide on how to implement a
// PeriodicBackgroundRoutine.
type PeriodicGoroutine struct {
	interval  time.Duration
	handler   Handler
	operation *observation.Operation
	clock     glock.Clock
	ctx       context.Context    // root context passed to the handler
	cancel    context.CancelFunc // cancels the root context
	finished  chan struct{}      // signals that Start has finished
}

var _ BackgroundRoutine = &PeriodicGoroutine{}

// Handler represents the main behavior of a PeriodicGoroutine.
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

// HandlerFunc wraps a function so it can be used as a Handler.
type HandlerFunc func(ctx context.Context) error

func (f HandlerFunc) Handle(ctx context.Context) error {
	return f(ctx)
}

type simpleHandler struct {
	name    string
	handler HandlerFunc
}

// NewHandlerWithErrorMessage wraps the given function to be used as a handler, and
// prints a canned failure message containing the given name.
func NewHandlerWithErrorMessage(name string, handler func(ctx context.Context) error) Handler {
	return &simpleHandler{handler: handler, name: name}
}

func (h *simpleHandler) Handle(ctx context.Context) error {
	return h.handler(ctx)
}

func (h *simpleHandler) HandleError(err error) {
	log15.Error("An error occurred in a background task", "handler", h.name, "error", err)
}

// NewPeriodicGoroutine creates a new PeriodicGoroutine with the given handler.
func NewPeriodicGoroutine(ctx context.Context, interval time.Duration, handler Handler) *PeriodicGoroutine {
	return NewPeriodicGoroutineWithMetrics(ctx, interval, handler, nil)
}

// NewPeriodicGoroutine creates a new PeriodicGoroutine with the given handler.
func NewPeriodicGoroutineWithMetrics(ctx context.Context, interval time.Duration, handler Handler, operation *observation.Operation) *PeriodicGoroutine {
	return newPeriodicGoroutine(ctx, interval, handler, operation, glock.NewRealClock())
}

func newPeriodicGoroutine(ctx context.Context, interval time.Duration, handler Handler, operation *observation.Operation, clock glock.Clock) *PeriodicGoroutine {
	ctx, cancel := context.WithCancel(ctx)

	return &PeriodicGoroutine{
		handler:   handler,
		interval:  interval,
		operation: operation,
		clock:     clock,
		ctx:       ctx,
		cancel:    cancel,
		finished:  make(chan struct{}),
	}
}

// Start begins the process of calling the registered handler in a loop. This process will
// wait the interval supplied at construction between invocations.
func (r *PeriodicGoroutine) Start() {
	defer close(r.finished)

loop:
	for {
		if shutdown, err := runPeriodicHandler(r.ctx, r.handler, r.operation); shutdown {
			break
		} else if h, ok := r.handler.(ErrorHandler); ok && err != nil {
			h.HandleError(err)
		}

		select {
		case <-r.clock.After(r.interval):
		case <-r.ctx.Done():
			break loop
		}
	}

	if h, ok := r.handler.(Finalizer); ok {
		h.OnShutdown()
	}
}

// Stop will cancel the context passed to the handler function to stop the current
// iteration of work, then break the loop in the Start method so that no new work
// is accepted. This method blocks until Start has returned.
func (r *PeriodicGoroutine) Stop() {
	r.cancel()
	<-r.finished
}

func runPeriodicHandler(ctx context.Context, handler Handler, operation *observation.Operation) (_ bool, err error) {
	if operation != nil {
		tmpCtx, endObservation := operation.With(ctx, &err, observation.Args{})
		defer endObservation(1, observation.Args{})
		ctx = tmpCtx
	}

	err = handler.Handle(ctx)

	// If the error is due to the loop being shut down, just break
	for ex := err; ex != nil; ex = errors.Unwrap(ex) {
		if err == ctx.Err() {
			return true, nil
		}
	}

	return false, err
}
