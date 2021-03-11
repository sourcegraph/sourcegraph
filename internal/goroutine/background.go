package goroutine

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// StartableRoutine represents a component of a binary that consists of a long
// running process.
type StartableRoutine interface {
	// Start begins the long-running process. This routine may also implement
	// a Stop method that should signal this process the application is going
	// to shut down.
	Start()
}

// BackgroundRoutine represents a component of a binary that consists of a long
// running process with a graceful shutdown mechanism.
//
// See
// https://docs.sourcegraph.com/dev/background-information/backgroundroutine
// for more information and a step-by-step guide on how to implement a
// BackgroundRoutine.
type BackgroundRoutine interface {
	StartableRoutine

	// Stop signals the Start method to stop accepting new work and complete its
	// current work. This method can but is not required to block until Start has
	// returned.
	Stop()
}

// MonitorBackgroundRoutines will start the given background routines in their own
// goroutine. If the given context is canceled or a signal is received, the Stop
// method of each routine will be called. This method blocks until the Stop methods
// of each routine have returned. Two signals will cause the app to shutdown
// immediately.
func MonitorBackgroundRoutines(ctx context.Context, routines ...BackgroundRoutine) {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGINT)
	monitorBackgroundRoutines(ctx, signals, routines...)
}

func monitorBackgroundRoutines(ctx context.Context, signals <-chan os.Signal, routines ...BackgroundRoutine) {
	wg := &sync.WaitGroup{}
	startAll(wg, routines...)
	waitForSignal(ctx, signals)
	stopAll(wg, routines...)
	wg.Wait()
}

// startAll calls each routine's Start method in its own goroutine and registers
// each running goroutine with the given waitgroup.
func startAll(wg *sync.WaitGroup, routines ...BackgroundRoutine) {
	for _, r := range routines {
		t := r
		wg.Add(1)
		Go(func() { defer wg.Done(); t.Start() })
	}
}

// stopAll calls each routine's Stop method in its own goroutine and and registers
// each running goroutine with the given waitgroup.
func stopAll(wg *sync.WaitGroup, routines ...BackgroundRoutine) {
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
	for i := 0; i < numSignals; i++ {
		<-signals
	}

	exiter()
}

// CombinedRoutine is a list of routines which are started and stopped in unison.
type CombinedRoutine []BackgroundRoutine

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

type noopRoutine struct{}

func (r noopRoutine) Start() {}
func (r noopRoutine) Stop()  {}

// NoopRoutine does nothing for start or stop.
func NoopRoutine() BackgroundRoutine {
	return noopRoutine{}
}
