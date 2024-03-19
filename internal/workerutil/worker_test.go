package workerutil

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/derision-test/glock"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type TestRecord struct {
	ID    int
	State string
}

func (v TestRecord) RecordID() int {
	return v.ID
}

func (v TestRecord) RecordUID() string {
	return strconv.Itoa(v.ID)
}

func TestWorkerHandlerSuccess(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	handler := NewMockHandler[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	options := WorkerOptions{
		Name:           "test",
		WorkerHostname: "test",
		NumHandlers:    1,
		Interval:       time.Second,
		Metrics:        NewMetrics(&observation.TestContext, ""),
	}

	store.DequeueFunc.PushReturn(&TestRecord{ID: 42}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)
	store.MarkCompleteFunc.SetDefaultReturn(true, nil)

	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
	go func() { worker.Start() }()
	dequeueClock.BlockingAdvance(time.Second)
	worker.Stop()

	if callCount := len(handler.HandleFunc.History()); callCount != 1 {
		t.Errorf("unexpected handle call count. want=%d have=%d", 1, callCount)
	} else if arg := handler.HandleFunc.History()[0].Arg2; arg.RecordID() != 42 {
		t.Errorf("unexpected record. want=%d have=%d", 42, arg.RecordID())
	}

	if callCount := len(store.MarkCompleteFunc.History()); callCount != 1 {
		t.Errorf("unexpected mark complete call count. want=%d have=%d", 1, callCount)
	} else if id := store.MarkCompleteFunc.History()[0].Arg1.RecordID(); id != 42 {
		t.Errorf("unexpected id argument to mark complete. want=%v have=%v", 42, id)
	}
}

func TestWorkerHandlerFailure(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	handler := NewMockHandler[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	options := WorkerOptions{
		Name:           "test",
		WorkerHostname: "test",
		NumHandlers:    1,
		Interval:       time.Second,
		Metrics:        NewMetrics(&observation.TestContext, ""),
	}

	store.DequeueFunc.PushReturn(&TestRecord{ID: 42}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)
	store.MarkErroredFunc.SetDefaultReturn(true, nil)
	handler.HandleFunc.SetDefaultReturn(errors.Errorf("oops"))

	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
	go func() { worker.Start() }()
	dequeueClock.BlockingAdvance(time.Second)
	worker.Stop()

	if callCount := len(handler.HandleFunc.History()); callCount != 1 {
		t.Errorf("unexpected handle call count. want=%d have=%d", 1, callCount)
	} else if arg := handler.HandleFunc.History()[0].Arg2; arg.RecordID() != 42 {
		t.Errorf("unexpected record. want=%d have=%d", 42, arg.RecordID())
	}

	if callCount := len(store.MarkErroredFunc.History()); callCount != 1 {
		t.Errorf("unexpected mark errored call count. want=%d have=%d", 1, callCount)
	} else if id := store.MarkErroredFunc.History()[0].Arg1.RecordID(); id != 42 {
		t.Errorf("unexpected id argument to mark errored. want=%v have=%v", 42, id)
	} else if failureMessage := store.MarkErroredFunc.History()[0].Arg2; failureMessage != "oops" {
		t.Errorf("unexpected failure message argument to mark errored. want=%q have=%q", "oops", failureMessage)
	}
}

type nonRetryableTestErr struct{}

func (e nonRetryableTestErr) Error() string      { return "just retry me and see what happens" }
func (e nonRetryableTestErr) NonRetryable() bool { return true }

func TestWorkerHandlerNonRetryableFailure(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	handler := NewMockHandler[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	options := WorkerOptions{
		Name:           "test",
		WorkerHostname: "test",
		NumHandlers:    1,
		Interval:       time.Second,
		Metrics:        NewMetrics(&observation.TestContext, ""),
	}

	store.DequeueFunc.PushReturn(&TestRecord{ID: 42}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)
	store.MarkFailedFunc.SetDefaultReturn(true, nil)

	testErr := nonRetryableTestErr{}
	handler.HandleFunc.SetDefaultReturn(testErr)

	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
	go func() { worker.Start() }()
	dequeueClock.BlockingAdvance(time.Second)
	worker.Stop()

	if callCount := len(handler.HandleFunc.History()); callCount != 1 {
		t.Errorf("unexpected handle call count. want=%d have=%d", 1, callCount)
	} else if arg := handler.HandleFunc.History()[0].Arg2; arg.RecordID() != 42 {
		t.Errorf("unexpected record. want=%d have=%d", 42, arg.RecordID())
	}

	if callCount := len(store.MarkFailedFunc.History()); callCount != 1 {
		t.Errorf("unexpected mark failed call count. want=%d have=%d", 1, callCount)
	} else if id := store.MarkFailedFunc.History()[0].Arg1.RecordID(); id != 42 {
		t.Errorf("unexpected id argument to mark failed. want=%v have=%v", 42, id)
	} else if failureMessage := store.MarkFailedFunc.History()[0].Arg2; failureMessage != testErr.Error() {
		t.Errorf("unexpected failure message argument to mark failed. want=%q have=%q", testErr.Error(), failureMessage)
	}
}

const NumTestRecords = 50

func TestWorkerConcurrent(t *testing.T) {
	runTest := func(numHandlers int) {
		t.Run(fmt.Sprintf("numHandlers=%d", numHandlers), func(t *testing.T) {
			t.Parallel()

			store := NewMockStore[*TestRecord]()
			handler := NewMockHandlerWithHooks[*TestRecord]()
			dequeueClock := glock.NewMockClock()
			heartbeatClock := glock.NewMockClock()
			shutdownClock := glock.NewMockClock()
			options := WorkerOptions{
				Name:           "test",
				WorkerHostname: "test",
				NumHandlers:    numHandlers,
				Interval:       time.Second,
				Metrics:        NewMetrics(&observation.TestContext, ""),
			}

			for i := range NumTestRecords {
				index := i
				store.DequeueFunc.PushReturn(&TestRecord{ID: index}, true, nil)
			}
			store.DequeueFunc.SetDefaultReturn(nil, false, nil)

			var m sync.Mutex
			maxConcurrency := 0
			currentRunning := 0
			collect := func(start bool) {
				m.Lock()
				if start {
					currentRunning++
				} else {
					currentRunning--
				}
				if currentRunning > maxConcurrency {
					maxConcurrency = currentRunning
				}
				m.Unlock()
			}

			handler.PreHandleFunc.SetDefaultHook(func(ctx context.Context, _ log.Logger, record *TestRecord) { collect(true) })
			handler.PostHandleFunc.SetDefaultHook(func(ctx context.Context, _ log.Logger, record *TestRecord) { collect(false) })
			handler.HandleFunc.SetDefaultHook(func(context.Context, log.Logger, *TestRecord) error {
				// Do a _very_ small sleep to make it very unlikely that the scheduler
				// will happen to invoke all of the handlers sequentially.
				<-time.After(time.Millisecond * 10)
				return nil
			})

			worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
			go func() { worker.Start() }()
			for range NumTestRecords {
				dequeueClock.BlockingAdvance(time.Second)
			}
			worker.Stop()

			// We never want to see more concurrently running jobs than configured.
			// Ideally, we would also want to see that the number is exactly equal,
			// but due to the nature of the goroutine scheduler, we can't guarantee that
			// and it causes flakes in our tests.
			if maxConcurrency > numHandlers {
				t.Errorf("unexpected max concurrency. want=%d have=%d", numHandlers, maxConcurrency)
			}
		})
	}

	for numHandlers := 1; numHandlers < NumTestRecords; numHandlers++ {
		runTest(numHandlers)
	}
}

func TestWorkerBlockingPreDequeueHook(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	handler := NewMockHandlerWithPreDequeue[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	options := WorkerOptions{
		Name:           "test",
		WorkerHostname: "test",
		NumHandlers:    1,
		Interval:       time.Second,
		Metrics:        NewMetrics(&observation.TestContext, ""),
	}

	store.DequeueFunc.PushReturn(&TestRecord{ID: 42}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)

	// Block all dequeues
	handler.PreDequeueFunc.SetDefaultReturn(false, nil, nil)

	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
	go func() { worker.Start() }()
	dequeueClock.BlockingAdvance(time.Second)
	worker.Stop()

	if callCount := len(handler.HandleFunc.History()); callCount != 0 {
		t.Errorf("unexpected handle call count. want=%d have=%d", 0, callCount)
	}
}

func TestWorkerConditionalPreDequeueHook(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	handler := NewMockHandlerWithPreDequeue[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	options := WorkerOptions{
		Name:           "test",
		WorkerHostname: "test",
		NumHandlers:    1,
		Interval:       time.Second,
		Metrics:        NewMetrics(&observation.TestContext, ""),
	}

	store.DequeueFunc.PushReturn(&TestRecord{ID: 42}, true, nil)
	store.DequeueFunc.PushReturn(&TestRecord{ID: 43}, true, nil)
	store.DequeueFunc.PushReturn(&TestRecord{ID: 44}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)

	// Return additional arguments
	handler.PreDequeueFunc.PushReturn(true, "A", nil)
	handler.PreDequeueFunc.PushReturn(true, "B", nil)
	handler.PreDequeueFunc.PushReturn(true, "C", nil)

	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
	go func() { worker.Start() }()
	dequeueClock.BlockingAdvance(time.Second)
	dequeueClock.BlockingAdvance(time.Second)
	dequeueClock.BlockingAdvance(time.Second)
	worker.Stop()

	if callCount := len(handler.HandleFunc.History()); callCount != 3 {
		t.Errorf("unexpected handle call count. want=%d have=%d", 3, callCount)
	}

	if callCount := len(store.DequeueFunc.History()); callCount != 3 {
		t.Errorf("unexpected dequeue call count. want=%d have=%d", 3, callCount)
	} else {
		for i, expected := range []string{"A", "B", "C"} {
			if extra := store.DequeueFunc.History()[i].Arg2; extra != expected {
				t.Errorf("unexpected extra argument for dequeue call %d. want=%q have=%q", i, expected, extra)
			}
		}
	}
}

type MockHandlerWithPreDequeue[T Record] struct {
	*MockHandler[T]
	*MockWithPreDequeue
}

func NewMockHandlerWithPreDequeue[T Record]() *MockHandlerWithPreDequeue[T] {
	return &MockHandlerWithPreDequeue[T]{
		MockHandler:        NewMockHandler[T](),
		MockWithPreDequeue: NewMockWithPreDequeue(),
	}
}

type MockHandlerWithHooks[T Record] struct {
	*MockHandler[T]
	*MockWithHooks[T]
}

func NewMockHandlerWithHooks[T Record]() *MockHandlerWithHooks[T] {
	return &MockHandlerWithHooks[T]{
		MockHandler:   NewMockHandler[T](),
		MockWithHooks: NewMockWithHooks[T](),
	}
}

func TestWorkerDequeueHeartbeat(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	store.DequeueFunc.PushReturn(&TestRecord{ID: 42}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)
	store.MarkCompleteFunc.SetDefaultReturn(true, nil)

	handler := NewMockHandler[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	heartbeatInterval := time.Second
	options := WorkerOptions{
		Name:              "test",
		WorkerHostname:    "test",
		NumHandlers:       1,
		HeartbeatInterval: heartbeatInterval,
		Interval:          time.Second,
		Metrics:           NewMetrics(&observation.TestContext, ""),
	}

	dequeued := make(chan struct{})
	doneHandling := make(chan struct{})
	handler.HandleFunc.defaultHook = func(c context.Context, l log.Logger, r *TestRecord) error {
		close(dequeued)
		<-doneHandling
		return nil
	}

	heartbeats := make(chan struct{})
	store.HeartbeatFunc.SetDefaultHook(func(c context.Context, i []string) ([]string, []string, error) {
		heartbeats <- struct{}{}
		return i, nil, nil
	})

	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
	go func() { worker.Start() }()
	t.Cleanup(func() {
		close(doneHandling)
		worker.Stop()
	})
	<-dequeued

	for range []int{1, 2, 3, 4, 5} {
		heartbeatClock.BlockingAdvance(heartbeatInterval)
		select {
		case <-heartbeats:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for heartbeat")
		}
	}
}

func TestWorkerNumTotalJobs(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	handler := NewMockHandler[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	options := WorkerOptions{
		Name:           "test",
		WorkerHostname: "test",
		NumHandlers:    1,
		NumTotalJobs:   5,
		Interval:       time.Second,
		Metrics:        NewMetrics(&observation.TestContext, ""),
	}

	store.DequeueFunc.SetDefaultReturn(&TestRecord{ID: 42}, true, nil)
	store.MarkCompleteFunc.SetDefaultReturn(true, nil)

	// Should process 5 then shut down
	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
	worker.Start()

	if callCount := len(store.DequeueFunc.History()); callCount != 5 {
		t.Errorf("unexpected call count. want=%d have=%d", 5, callCount)
	}
}

func TestWorkerMaxActiveTime(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	handler := NewMockHandler[*TestRecord]()
	dequeueClock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	shutdownClock := glock.NewMockClock()
	options := WorkerOptions{
		Name:           "test",
		WorkerHostname: "test",
		NumHandlers:    1,
		NumTotalJobs:   50,
		MaxActiveTime:  time.Second * 5,
		Interval:       time.Second,
		Metrics:        NewMetrics(&observation.TestContext, ""),
	}

	called := make(chan struct{})
	defer close(called)

	dequeueHook := func(c context.Context, s string, i any) (*TestRecord, bool, error) {
		called <- struct{}{}
		return &TestRecord{ID: 42}, true, nil
	}

	store.DequeueFunc.SetDefaultReturn(nil, false, nil)
	store.MarkCompleteFunc.SetDefaultReturn(true, nil)

	for range 5 {
		store.DequeueFunc.PushHook(dequeueHook)
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, dequeueClock, heartbeatClock, shutdownClock)
		worker.Start()
	}()

	timeout := time.After(time.Second * 5)
	for range 5 {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for dequeues")
		case <-called:
		}
	}

	// Wait for a fixed number of records to be processed, then
	// send a shutdown signal based on time after that. If the
	// goroutine running the worker is released, then it's has
	// shut down properly.
	shutdownClock.BlockingAdvance(time.Second * 5)

	select {
	case <-timeout:
		t.Fatal("timeout waiting for shutdown")
	case <-stopped:
	}

	// Might dequeue 5 or 6 based on timing
	if callCount := len(store.DequeueFunc.History()); callCount != 5 && callCount != 6 {
		t.Errorf("unexpected call count. want=5 or 6 have=%d", callCount)
	}
}

func TestWorkerCancelJobs(t *testing.T) {
	recordID := 42
	store := NewMockStore[*TestRecord]()
	// Return one record from dequeue.
	store.DequeueFunc.PushReturn(&TestRecord{ID: recordID}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)

	// Record when markFailed is called.
	markedFailedCalled := make(chan struct{})
	store.MarkFailedFunc.SetDefaultHook(func(c context.Context, record *TestRecord, s string) (bool, error) {
		close(markedFailedCalled)
		return true, nil
	})

	handler := NewMockHandler[*TestRecord]()
	options := WorkerOptions{
		Name:              "test",
		WorkerHostname:    "test",
		NumHandlers:       1,
		HeartbeatInterval: time.Second,
		Interval:          time.Second,
		Metrics:           NewMetrics(&observation.TestContext, ""),
	}

	dequeued := make(chan struct{})
	doneHandling := make(chan struct{})
	handler.HandleFunc.defaultHook = func(ctx context.Context, l log.Logger, r *TestRecord) error {
		close(dequeued)
		// wait until the context is canceled (through cancelation), or until the test is over.
		select {
		case <-ctx.Done():
		case <-doneHandling:
		}
		return ctx.Err()
	}

	canceledJobsCalled := make(chan struct{})
	store.HeartbeatFunc.SetDefaultHook(func(c context.Context, i []string) ([]string, []string, error) {
		close(canceledJobsCalled)
		// Cancel all jobs.
		return i, i, nil
	})

	clock := glock.NewMockClock()
	heartbeatClock := glock.NewMockClock()
	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, clock, heartbeatClock, clock)
	go func() { worker.Start() }()
	t.Cleanup(func() {
		// Keep the handler working until context is canceled.
		close(doneHandling)
		worker.Stop()
	})

	// Wait until a job has been dequeued.
	select {
	case <-dequeued:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for Dequeue call")
	}
	// Trigger a heartbeat call.
	heartbeatClock.BlockingAdvance(time.Second)
	// Wait for cancelled jobs to be called.
	select {
	case <-canceledJobsCalled:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for CanceledJobs call")
	}
	// Expect that markFailed is called eventually.
	select {
	case <-markedFailedCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for markFailed call")
	}
}

func TestWorkerDeadline(t *testing.T) {
	recordID := 42
	store := NewMockStore[*TestRecord]()
	// Return one record from dequeue.
	store.DequeueFunc.PushReturn(&TestRecord{ID: recordID}, true, nil)
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)

	// Record when markErrored is called.
	markedErroredCalled := make(chan struct{})
	store.MarkErroredFunc.SetDefaultHook(func(c context.Context, record *TestRecord, s string) (bool, error) {
		if !strings.Contains(s, "job exceeded maximum execution time of 10ms") {
			t.Fatal("incorrect error message")
		}
		close(markedErroredCalled)
		return true, nil
	})

	handler := NewMockHandler[*TestRecord]()
	options := WorkerOptions{
		Name:              "test",
		WorkerHostname:    "test",
		NumHandlers:       1,
		HeartbeatInterval: time.Second,
		Interval:          time.Second,
		Metrics:           NewMetrics(&observation.TestContext, ""),
		// The handler runs forever but should be canceled after 10ms.
		MaximumRuntimePerJob: 10 * time.Millisecond,
	}

	dequeued := make(chan struct{})
	doneHandling := make(chan struct{})
	handler.HandleFunc.defaultHook = func(ctx context.Context, l log.Logger, r *TestRecord) error {
		close(dequeued)
		select {
		case <-ctx.Done():
		case <-doneHandling:
		}
		return ctx.Err()
	}

	heartbeats := make(chan struct{})
	store.HeartbeatFunc.SetDefaultHook(func(c context.Context, i []string) ([]string, []string, error) {
		heartbeats <- struct{}{}
		return i, nil, nil
	})

	clock := glock.NewMockClock()
	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, clock, clock, clock)
	go func() { worker.Start() }()
	t.Cleanup(func() {
		// Keep the handler working until context is canceled.
		close(doneHandling)
		worker.Stop()
	})

	// Wait until a job has been dequeued.
	<-dequeued

	// Expect that markErrored is called eventually.
	select {
	case <-markedErroredCalled:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for markErrored call")
	}
}

func TestWorkerStopDrainsDequeueLoopOnly(t *testing.T) {
	store := NewMockStore[*TestRecord]()
	store.DequeueFunc.PushReturn(&TestRecord{ID: 42}, true, nil)
	store.DequeueFunc.PushReturn(&TestRecord{ID: 43}, true, nil) // not dequeued
	store.DequeueFunc.PushReturn(&TestRecord{ID: 44}, true, nil) // not dequeued
	store.DequeueFunc.SetDefaultReturn(nil, false, nil)

	handler := NewMockHandlerWithPreDequeue[*TestRecord]()
	options := WorkerOptions{
		Name:              "test",
		WorkerHostname:    "test",
		NumHandlers:       1,
		HeartbeatInterval: time.Second,
		Interval:          time.Second,
		Metrics:           NewMetrics(&observation.TestContext, ""),
	}

	dequeued := make(chan struct{})
	block := make(chan struct{})
	handler.HandleFunc.defaultHook = func(ctx context.Context, l log.Logger, r *TestRecord) error {
		close(dequeued)
		<-block
		return ctx.Err()
	}

	var dequeueContext context.Context
	handler.PreDequeueFunc.SetDefaultHook(func(ctx context.Context, l log.Logger) (bool, any, error) {
		// Store dequeueContext in outer function so we can tell when Stop has
		// reliably been called. Unfortunately we need to peek a bit into the
		// internals here so we're not dependent on time-based unit tests.
		dequeueContext = ctx
		return true, nil, nil
	})

	clock := glock.NewMockClock()
	worker := newWorker(context.Background(), Store[*TestRecord](store), Handler[*TestRecord](handler), options, clock, clock, clock)
	go func() { worker.Start() }()
	t.Cleanup(func() { worker.Stop() })

	// Wait until a job has been dequeued.
	<-dequeued

	go func() {
		<-dequeueContext.Done()
		block <- struct{}{}
	}()

	// Drain dequeue loop and wait for the one active handler to finish.
	worker.Stop()

	for _, call := range handler.HandleFunc.History() {
		if call.Result0 != nil {
			t.Errorf("unexpected handler error: %s", call.Result0)
		}
	}

	if handlerCallCount := len(handler.HandleFunc.History()); handlerCallCount != 1 {
		t.Errorf("incorrect number of handler calls. want=%d have=%d", 1, handlerCallCount)
	}
}
