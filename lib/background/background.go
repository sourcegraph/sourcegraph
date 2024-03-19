package background

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Routine represents a component of a binary that consists of a long
// running process with a graceful shutdown mechanism.
type Routine interface {
	// Start begins the long-running process. This routine may also implement
	// a Stop method that should signal this process the application is going
	// to shut down.
	Start()

	// Stop signals the Start method to stop accepting new work and complete its
	// current work. This method can but is not required to block until Start has
	// returned.
	Stop()
}

// Monitor will start the given background routines in their own
// goroutine. If the given context is canceled or a signal is received, the Stop
// method of each routine will be called. This method blocks until the Stop methods
// of each routine have returned. Two signals will cause the app to shutdown
// immediately.
func Monitor(ctx context.Context, routines ...Routine) {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	monitorBackgroundRoutines(ctx, signals, routines...)
}

func monitorBackgroundRoutines(ctx context.Context, signals <-chan os.Signal, routines ...Routine) {
	wg := &sync.WaitGroup{}
	startAll(wg, routines...)
	waitForSignal(ctx, signals)
	stopAll(wg, routines...)
	wg.Wait()
}

// startAll calls each routine's Start method in its own goroutine and registers
// each running goroutine with the given waitgroup.
func startAll(wg *sync.WaitGroup, routines ...Routine) {
	for _, r := range routines {
		t := r
		wg.Add(1)
		Go(func() { defer wg.Done(); t.Start() })
	}
}

// stopAll calls each routine's Stop method in its own goroutine and registers
// each running goroutine with the given waitgroup.
func stopAll(wg *sync.WaitGroup, routines ...Routine) {
	for _, r := range routines {
		t := r
		wg.Add(1)
		Go(func() { defer wg.Done(); t.Stop() })
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

// CombinedRoutine is a list of routines which are started and stopped in unison.
type CombinedRoutine []Routine

func (r CombinedRoutine) Start() {
	wg := &sync.WaitGroup{}
	startAll(wg, r...)
	wg.Wait()
}

func (r CombinedRoutine) Stop() {
	wg := &sync.WaitGroup{}
	stopAll(wg, r...)
	wg.Wait()
}

// LIFOStopRoutine is a list of routines which are started in unison, but stopped
// sequentially last-in-first-out (the last Routine is stopped, and once it
// successfully stops, the next routine is stopped).
//
// This is useful for services where subprocessors should be stopped before the
// primary service stops for a graceful shutdown.
type LIFOStopRoutine []Routine

func (r LIFOStopRoutine) Start() { CombinedRoutine(r).Start() }

func (r LIFOStopRoutine) Stop() {
	for i := len(r) - 1; i >= 0; i -= 1 {
		r[i].Stop()
	}
}

// NoopRoutine does nothing for start or stop.
func NoopRoutine() Routine {
	return CallbackRoutine{}
}

// CallbackRoutine calls the StartFunc and StopFunc callbacks to implement a
// Routine. Each callback may be nil.
type CallbackRoutine struct {
	StartFunc func()
	StopFunc  func()
}

func (r CallbackRoutine) Start() {
	if r.StartFunc != nil {
		r.StartFunc()
	}
}

func (r CallbackRoutine) Stop() {
	if r.StopFunc != nil {
		r.StopFunc()
	}
}
