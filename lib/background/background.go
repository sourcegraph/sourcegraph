package background

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Routine represents a background process that consists of a long-running
// process with a graceful shutdown mechanism.
type Routine interface {
	// Name returns the human-readable name of the routine.
	Name() string
	// Start begins the long-running process. This routine may also implement a Stop
	// method that should signal this process the application is going to shut down.
	Start()
	// Stop signals the Start method to stop accepting new work and complete its
	// current work. This method can but is not required to block until Start has
	// returned. The method should respect the context deadline passed to it for
	// proper graceful shutdown.
	Stop(ctx context.Context) error
}

// Monitor will start the given background routines in their own goroutine. If
// the given context is canceled or a signal is received, the Stop method of
// each routine will be called. This method blocks until the Stop methods of
// each routine have returned. Two signals will cause the app to shut down
// immediately.
//
// This function is only returned when routines are signaled to stop with
// potential errors from stopping routines.
func Monitor(ctx context.Context, routines ...Routine) error {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	return monitorBackgroundRoutines(ctx, signals, routines...)
}

func monitorBackgroundRoutines(ctx context.Context, signals <-chan os.Signal, routines ...Routine) error {
	wg := &sync.WaitGroup{}
	startAll(wg, routines...)
	waitForSignal(ctx, signals)
	return stopAll(ctx, wg, routines...)
}

// startAll calls each routine's Start method in its own goroutine and registers
// each running goroutine with the given waitgroup. It DOES NOT wait for the
// routines to finish starting, so the caller must wait for the waitgroup (if
// desired).
func startAll(wg *sync.WaitGroup, routines ...Routine) {
	for _, r := range routines {
		t := r
		wg.Add(1)
		Go(func() { defer wg.Done(); t.Start() })
	}
}

// stopAll calls each routine's Stop method in its own goroutine and registers
// each running goroutine with the given waitgroup.
//
// It initiates all goroutines to Stop concurrently. Individual implementations
// are expected to handle cancellation correctly.
//
// It waits for all goroutines to complete until the context's deadline is exceeded.
func stopAll(ctx context.Context, wg *sync.WaitGroup, routines ...Routine) error {
	var stopErrs error
	var stopErrsLock sync.Mutex
	for _, r := range routines {
		wg.Add(1)
		Go(func() {
			defer wg.Done()
			if err := r.Stop(ctx); err != nil {
				stopErrsLock.Lock()
				stopErrs = errors.Append(stopErrs,
					errors.Wrapf(err, "stop routine %q", errors.Safe(r.Name())))
				stopErrsLock.Unlock()
			}
		})
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		// If there is no deadline, wait until all work is complete.
		wg.Wait()
		return stopErrs
	}

	// Otherwise, wait until the work completes or the deadline is exceeded.
	// We need to specifically wait for the deadline event, instead of the
	// general ctx.Done() event, as the latter also handles explicit cancellation
	// that can prevent goroutine Stops from being called at all, which is not
	// desirable.
	contextTimeout := time.NewTimer(time.Until(deadline))
	defer contextTimeout.Stop()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		return stopErrs
	case <-contextTimeout.C:
		// Deadline is exceeded, we now need to wait for the context to also be
		// cancelled by its internal deadline.
		<-ctx.Done()
		stopErrsLock.Lock()
		defer stopErrsLock.Unlock()
		if stopErrs != nil {
			return errors.Wrapf(ctx.Err(), "unable to stop routines gracefully with partial errors: %v", stopErrs)
		}
		return errors.Wrap(ctx.Err(), "unable to stop routines gracefully")
	}
}

// waitForSignal blocks until the given context is canceled or signal has been
// received on the given channel. If two signals are received, os.Exit(0) will
// be called immediately.
func waitForSignal(ctx context.Context, signals <-chan os.Signal) {
	select {
	case <-ctx.Done():
		go exitAfterSignals(signals, 2)

	case <-signals:
		go exitAfterSignals(signals, 1)
	}
}

// exiter exits the process with a status code of zero. This is declared here
// so it can be replaced by tests without risk of aborting the tests without
// a good indication to the calling program that the tests didn't in fact pass.
var exiter = func() { os.Exit(0) }

// exitAfterSignals waits for a number of signals on the given channel, then
// calls os.Exit(0) to exit the program.
func exitAfterSignals(signals <-chan os.Signal, numSignals int) {
	for range numSignals {
		<-signals
	}

	exiter()
}

// CombinedRoutine is a list of routines which are started and stopped in
// unison.
type CombinedRoutine []Routine

func (rs CombinedRoutine) Name() string {
	names := make([]string, 0, len(rs))
	for _, r := range rs {
		names = append(names, r.Name())
	}
	return fmt.Sprintf("combined%q", names) // [a b c] -> combined["one" "two" "three"]
}

// Start starts all routines, it does not wait for the routines to finish
// starting.
func (rs CombinedRoutine) Start() {
	startAll(&sync.WaitGroup{}, rs...)
}

// Stop attempts to gracefully stopping all routines. It attempts to collect all
// the errors returned from the routines, and respects the context deadline
// passed to it and gives up waiting when context deadline exceeded.
func (rs CombinedRoutine) Stop(ctx context.Context) error {
	wg := &sync.WaitGroup{}
	return stopAll(ctx, wg, rs...)
}

// LIFOStopRoutine is a list of routines which are started in unison, but stopped
// sequentially first-in-first-out order (the first Routine is stopped, and once
// it successfully stops, the next routine is stopped).
//
// This is useful for services where subprocessors should be stopped before the
// primary service stops for a graceful shutdown.
type FIFOSTopRoutine []Routine

func (r FIFOSTopRoutine) Name() string { return "fifo" }

func (r FIFOSTopRoutine) Start() { CombinedRoutine(r).Start() }

func (r FIFOSTopRoutine) Stop(ctx context.Context) error {
	// Pass self inverted into LIFOStopRoutine
	slices.Reverse(r)
	return LIFOStopRoutine(r).Stop(ctx)
}

// LIFOStopRoutine is a list of routines which are started in unison, but stopped
// sequentially last-in-first-out order (the last Routine is stopped, and once it
// successfully stops, the next routine is stopped).
//
// This is useful for services where subprocessors should be stopped before the
// primary service stops for a graceful shutdown.
type LIFOStopRoutine []Routine

func (r LIFOStopRoutine) Name() string { return "lifo" }

func (r LIFOStopRoutine) Start() { CombinedRoutine(r).Start() }

func (r LIFOStopRoutine) Stop(ctx context.Context) error {
	var stopErr error
	for i := len(r) - 1; i >= 0; i -= 1 {
		err := r[i].Stop(ctx)
		if err != nil {
			stopErr = errors.Append(stopErr,
				errors.Wrapf(err, "stop routine %q", errors.Safe(r[i].Name())))
		}
	}
	return stopErr
}

// NoopRoutine return a background routine that does nothing for start or stop.
// If the name is empty, it will default to "noop".
func NoopRoutine(name string) Routine {
	if name == "" {
		name = "noop"
	}
	return CallbackRoutine{
		NameFunc: func() string { return name },
	}
}

// CallbackRoutine calls the StartFunc and StopFunc callbacks to implement a
// Routine. Each callback may be nil.
type CallbackRoutine struct {
	NameFunc  func() string
	StartFunc func()
	StopFunc  func(ctx context.Context) error
}

func (r CallbackRoutine) Name() string {
	if r.NameFunc != nil {
		return r.NameFunc()
	}
	return "callback"
}

func (r CallbackRoutine) Start() {
	if r.StartFunc != nil {
		r.StartFunc()
	}
}

func (r CallbackRoutine) Stop(ctx context.Context) error {
	if r.StopFunc != nil {
		return r.StopFunc(ctx)
	}
	return nil
}
