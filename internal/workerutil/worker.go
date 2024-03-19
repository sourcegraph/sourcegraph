package workerutil

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/derision-test/glock"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/goroutine/recorder"
	"github.com/sourcegraph/sourcegraph/internal/hostname"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ErrJobAlreadyExists occurs when a duplicate job identifier is dequeued.
var ErrJobAlreadyExists = errors.New("job already exists")

// Worker is a generic consumer of records from the workerutil store.
type Worker[T Record] struct {
	store            Store[T]
	handler          Handler[T]
	options          WorkerOptions
	dequeueClock     glock.Clock
	heartbeatClock   glock.Clock
	shutdownClock    glock.Clock
	numDequeues      int             // tracks number of dequeue attempts
	handlerSemaphore chan struct{}   // tracks available handler slots
	rootCtx          context.Context // root context passed to the handler
	dequeueCtx       context.Context // context used for dequeue loop (based on root)
	dequeueCancel    func()          // cancels the dequeue context
	wg               sync.WaitGroup  // tracks active handler routines
	finished         chan struct{}   // signals that Start has finished
	runningIDSet     *IDSet          // tracks the running job IDs to heartbeat
	jobName          string
	recorder         *recorder.Recorder
}

// dummyType is only for this compile-time test.
type dummyType struct{}

func (d dummyType) RecordID() int { return 0 }

func (d dummyType) RecordUID() string {
	return strconv.Itoa(0)
}

var _ recorder.Recordable = &Worker[dummyType]{}

type WorkerOptions struct {
	// Name denotes the name of the worker used to distinguish log messages and
	// emitted metrics. The worker constructor will fail if this field is not
	// supplied.
	Name string

	// Description describes the worker for logging purposes.
	Description string

	// WorkerHostname denotes the hostname of the instance/container the worker
	// is running on. If not supplied, it will be derived from either the `HOSTNAME`
	// env var, or else from os.Hostname()
	WorkerHostname string

	// NumHandlers is the maximum number of handlers that can be invoked
	// concurrently. The underlying store will not be queried while the current
	// number of handlers exceeds this value.
	NumHandlers int

	// NumTotalJobs is the maximum number of jobs that will be dequeued by the worker.
	// After this number of dequeue attempts has been made, no more dequeues will be
	// attempted. Currently dequeued jobs will finish, and the Start method of the
	// worker will unblock. If not set, there is no limit.
	NumTotalJobs int

	// MaxActiveTime is the maximum time that can be spent by the worker dequeueing
	// records to be handled. After this duration has elapsed, no more dequeues will
	// be attempted. Currently dequeued jobs will finish, and the Start method of the
	// worker will unblock. If not set, there is no limit.
	MaxActiveTime time.Duration

	// Interval is the frequency to poll the underlying store for new work.
	Interval time.Duration

	// HeartbeatInterval is the interval between heartbeat updates to a job's last_heartbeat_at field. This
	// field is periodically updated while being actively processed to signal to other workers that the
	// record is neither pending nor abandoned.
	HeartbeatInterval time.Duration

	// MaximumRuntimePerJob is the maximum wall time that can be spent on a single job.
	MaximumRuntimePerJob time.Duration

	// Metrics configures logging, tracing, and metrics for the work loop.
	Metrics WorkerObservability
}

func NewWorker[T Record](ctx context.Context, store Store[T], handler Handler[T], options WorkerOptions) *Worker[T] {
	clock := glock.NewRealClock()
	return newWorker(ctx, store, handler, options, clock, clock, clock)
}

func newWorker[T Record](ctx context.Context, store Store[T], handler Handler[T], options WorkerOptions, mainClock, heartbeatClock, shutdownClock glock.Clock) *Worker[T] {
	if options.Name == "" {
		panic("no name supplied to github.com/sourcegraph/sourcegraph/internal/workerutil:newWorker")
	}
	if options.WorkerHostname == "" {
		options.WorkerHostname = hostname.Get()
	}

	// Initialize the logger
	if options.Metrics.logger == nil {
		options.Metrics.logger = log.Scoped("worker." + options.Name)
	}
	options.Metrics.logger = options.Metrics.logger.With(log.String("name", options.Name))

	dequeueContext, cancel := context.WithCancel(ctx)

	handlerSemaphore := make(chan struct{}, options.NumHandlers)
	for range options.NumHandlers {
		handlerSemaphore <- struct{}{}
	}

	return &Worker[T]{
		store:            store,
		handler:          handler,
		options:          options,
		dequeueClock:     mainClock,
		heartbeatClock:   heartbeatClock,
		shutdownClock:    shutdownClock,
		handlerSemaphore: handlerSemaphore,
		rootCtx:          ctx,
		dequeueCtx:       dequeueContext,
		dequeueCancel:    cancel,
		finished:         make(chan struct{}),
		runningIDSet:     newIDSet(),
	}
}

// Start begins polling for work from the underlying store and processing records.
func (w *Worker[T]) Start() {
	if w.recorder != nil {
		go w.recorder.LogStart(w)
	}
	defer close(w.finished)

	// Create a background routine that periodically writes the current time to the running records.
	// This will keep the records claimed by the active worker for a small amount of time so that
	// it will not be processed by a second worker concurrently.
	go func() {
		for {
			select {
			case <-w.finished:
				// All jobs finished. Heart can rest now :comfy:
				return
			case <-w.heartbeatClock.After(w.options.HeartbeatInterval):
			}

			ids := w.runningIDSet.Slice()
			knownIDs, canceledIDs, err := w.store.Heartbeat(w.rootCtx, ids)
			if err != nil {
				w.options.Metrics.logger.Error("Failed to refresh heartbeats",
					log.Strings("ids", ids),
					log.Error(err))
				// Bail out and restart the for loop.
				continue
			}
			knownIDsMap := map[string]struct{}{}
			for _, id := range knownIDs {
				knownIDsMap[id] = struct{}{}
			}

			for _, id := range ids {
				if _, ok := knownIDsMap[id]; !ok {
					if w.runningIDSet.Remove(id) {
						w.options.Metrics.logger.Error("Removed unknown job from running set",
							log.String("id", id))
					}
				}
			}

			if len(canceledIDs) > 0 {
				w.options.Metrics.logger.Info("Found jobs to cancel", log.Strings("IDs", canceledIDs))
			}

			for _, id := range canceledIDs {
				w.runningIDSet.Cancel(id)
			}
		}
	}()

	var shutdownChan <-chan time.Time
	if w.options.MaxActiveTime > 0 {
		shutdownChan = w.shutdownClock.After(w.options.MaxActiveTime)
	} else {
		shutdownChan = make(chan time.Time)
	}

	var reason string

loop:
	for {
		if w.options.NumTotalJobs != 0 && w.numDequeues >= w.options.NumTotalJobs {
			reason = "NumTotalJobs dequeued"
			break loop
		}

		ok, err := w.dequeueAndHandle()
		if err != nil {
			// Note that both rootCtx and dequeueCtx are used in the dequeueAndHandle
			// method, but only dequeueCtx errors can be forwarded. The rootCtx is only
			// used within a Go routine, so its error cannot be returned synchronously.
			if w.dequeueCtx.Err() != nil && errors.Is(err, w.dequeueCtx.Err()) {
				// If the error is due to the loop being shut down, just break
				break loop
			}

			w.options.Metrics.logger.Error("Failed to dequeue and handle record",
				log.String("name", w.options.Name),
				log.Error(err))
		}

		delay := w.options.Interval
		if ok {
			// If we had a successful dequeue, do not wait the poll interval.
			// Just attempt to get another handler routine and process the next
			// unit of work immediately.
			delay = 0

			// Count the number of successful dequeues, but do not count only
			// attempts. As we do this on a timed loop, we will end up just
			// sloppily counting the active time instead of the number of jobs
			// (with data) that were seen.
			w.numDequeues++
		}

		select {
		case <-w.dequeueClock.After(delay):
		case <-w.dequeueCtx.Done():
			break loop
		case <-shutdownChan:
			reason = "MaxActiveTime elapsed"
			break loop
		}
	}

	w.options.Metrics.logger.Info("Shutting down dequeue loop", log.String("reason", reason))
	w.wg.Wait()
}

// Stop will cause the worker loop to exit after the current iteration. This is done by canceling the
// context passed to the dequeue operations (but not the handler operations). This method blocks until
// all handler goroutines have exited.
func (w *Worker[T]) Stop() {
	if w.recorder != nil {
		go w.recorder.LogStop(w)
	}
	w.dequeueCancel()
	w.Wait()
}

// Wait blocks until all handler goroutines have exited.
func (w *Worker[T]) Wait() {
	<-w.finished
}

// dequeueAndHandle selects a queued record to process. This method returns false if no such record
// can be dequeued and returns an error only on failure to dequeue a new record - no handler errors
// will bubble up.
func (w *Worker[T]) dequeueAndHandle() (dequeued bool, err error) {
	select {
	// If we block here we are waiting for a handler to exit so that we do not
	// exceed our configured concurrency limit.
	case <-w.handlerSemaphore:
	case <-w.dequeueCtx.Done():
		return false, w.dequeueCtx.Err()
	}
	defer func() {
		if !dequeued {
			// Ensure that if we do not dequeue a record successfully we do not
			// leak from the semaphore. This will happen if the pre dequeue hook
			// fails, if the dequeue call fails, or if there are no records to
			// process.
			w.handlerSemaphore <- struct{}{}
		}
	}()

	dequeueable, extraDequeueArguments, err := w.preDequeueHook(w.dequeueCtx)
	if err != nil {
		return false, errors.Wrap(err, "Handler.PreDequeueHook")
	}
	if !dequeueable {
		// Hook declined to dequeue a record
		return false, nil
	}

	// Select a queued record to process and the transaction that holds it
	record, dequeued, err := w.store.Dequeue(w.dequeueCtx, w.options.WorkerHostname, extraDequeueArguments)
	if err != nil {
		return false, errors.Wrap(err, "store.Dequeue")
	}
	if !dequeued {
		// Nothing to process
		return false, nil
	}

	// Create context and span based on the root context
	workerSpan, workerCtxWithSpan := trace.New(
		// TODO tail-based sampling once its a thing, until then, we can configure on a per-job basis
		policy.WithShouldTrace(w.rootCtx, w.options.Metrics.traceSampler(record)),
		w.options.Name,
	)
	handleCtx, cancel := context.WithCancel(workerCtxWithSpan)
	processLog := trace.Logger(workerCtxWithSpan, w.options.Metrics.logger)

	// Register the record as running so it is included in heartbeat updates.
	if !w.runningIDSet.Add(record.RecordUID(), cancel) {
		workerSpan.EndWithErr(&ErrJobAlreadyExists)
		return false, ErrJobAlreadyExists
	}

	// Set up observability
	w.options.Metrics.numJobs.Inc()
	processLog.Debug("Dequeued record for processing", log.String("id", record.RecordUID()))
	processArgs := observation.Args{
		Attrs: []attribute.KeyValue{attribute.String("record.id", record.RecordUID())},
	}

	if hook, ok := w.handler.(WithHooks[T]); ok {
		preCtx, prehandleLogger, endObservation := w.options.Metrics.operations.preHandle.With(handleCtx, nil, processArgs)
		// Open namespace for logger to avoid key collisions on fields
		hook.PreHandle(preCtx, prehandleLogger.With(log.Namespace("prehandle")), record)
		endObservation(1, observation.Args{})
	}

	w.wg.Add(1)

	go func() {
		defer func() {
			if hook, ok := w.handler.(WithHooks[T]); ok {
				// Don't use handleCtx here, the record is already not owned by
				// this worker anymore at this point. Tracing hierarchy is still correct,
				// as handleCtx used in preHandle/handle is at the same level as
				// workerCtxWithSpan
				postCtx, posthandleLogger, endObservation := w.options.Metrics.operations.postHandle.With(workerCtxWithSpan, nil, processArgs)
				defer endObservation(1, observation.Args{})
				// Open namespace for logger to avoid key collisions on fields
				hook.PostHandle(postCtx, posthandleLogger.With(log.Namespace("posthandle")), record)
			}

			// Remove the record from the set of running jobs, so it is not included
			// in heartbeat updates anymore.
			defer w.runningIDSet.Remove(record.RecordUID())
			w.options.Metrics.numJobs.Dec()
			w.handlerSemaphore <- struct{}{}
			w.wg.Done()
			workerSpan.End()
		}()

		if err := w.handle(handleCtx, workerCtxWithSpan, record); err != nil {
			processLog.Error("Failed to finalize record", log.Error(err))
		}
	}()

	return true, nil
}

// handle processes the given record. This method returns an error only if there is an issue updating
// the record to a terminal state - no handler errors will bubble up.
func (w *Worker[T]) handle(ctx, workerContext context.Context, record T) (err error) {
	var handleErr error
	ctx, handleLog, endOperation := w.options.Metrics.operations.handle.With(ctx, &handleErr, observation.Args{})
	defer func() {
		// prioritize handleErr in `operations.handle.With` without bubbling handleErr up if non-nil
		if handleErr == nil && err != nil {
			handleErr = err
		}
		endOperation(1, observation.Args{})
	}()

	// If a maximum runtime is configured, set a deadline on the handle context.
	if w.options.MaximumRuntimePerJob > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, time.Now().Add(w.options.MaximumRuntimePerJob))
		defer cancel()
	}

	// Open namespace for logger to avoid key collisions on fields
	start := time.Now()
	handleErr = w.handler.Handle(ctx, handleLog.With(log.Namespace("handle")), record)

	if w.options.MaximumRuntimePerJob > 0 && errors.Is(handleErr, context.DeadlineExceeded) {
		handleErr = errors.Wrap(handleErr, fmt.Sprintf("job exceeded maximum execution time of %s", w.options.MaximumRuntimePerJob))
	}
	duration := time.Since(start)
	if w.recorder != nil {
		go w.recorder.LogRun(w, duration, handleErr)
	}

	if errcode.IsNonRetryable(handleErr) || handleErr != nil && w.isJobCanceled(record.RecordUID(), handleErr, ctx.Err()) {
		if marked, markErr := w.store.MarkFailed(workerContext, record, handleErr.Error()); markErr != nil {
			return errors.Wrap(markErr, "store.MarkFailed")
		} else if marked {
			handleLog.Warn("Marked record as failed", log.Error(handleErr))
		}
	} else if handleErr != nil {
		if marked, markErr := w.store.MarkErrored(workerContext, record, handleErr.Error()); markErr != nil {
			return errors.Wrap(markErr, "store.MarkErrored")
		} else if marked {
			handleLog.Warn("Marked record as errored", log.Error(handleErr))
		}
	} else {
		if marked, markErr := w.store.MarkComplete(workerContext, record); markErr != nil {
			return errors.Wrap(markErr, "store.MarkComplete")
		} else if marked {
			handleLog.Debug("Marked record as complete")
		}
	}

	handleLog.Debug("Handled record")
	return nil
}

// isJobCanceled returns true if the job has been canceled through the Cancel interface.
// If the context is canceled, and the job is still part of the running ID set,
// we know that it has been canceled for that reason.
func (w *Worker[T]) isJobCanceled(id string, handleErr, ctxErr error) bool {
	return errors.Is(handleErr, ctxErr) && w.runningIDSet.Has(id) && !errors.Is(handleErr, context.DeadlineExceeded)
}

// preDequeueHook invokes the handler's pre-dequeue hook if it exists.
func (w *Worker[T]) preDequeueHook(ctx context.Context) (dequeueable bool, extraDequeueArguments any, err error) {
	if o, ok := w.handler.(WithPreDequeue); ok {
		return o.PreDequeue(ctx, w.options.Metrics.logger)
	}

	return true, nil, nil
}

func (w *Worker[T]) Name() string {
	return w.options.Name
}

func (w *Worker[T]) Type() recorder.RoutineType {
	return recorder.DBBackedRoutine
}

func (w *Worker[T]) JobName() string {
	return w.jobName
}

func (w *Worker[T]) SetJobName(jobName string) {
	w.jobName = jobName
}

func (w *Worker[T]) Description() string {
	return w.options.Description
}

func (w *Worker[T]) Interval() time.Duration {
	return w.options.Interval
}

func (w *Worker[T]) RegisterRecorder(r *recorder.Recorder) {
	w.recorder = r
}
