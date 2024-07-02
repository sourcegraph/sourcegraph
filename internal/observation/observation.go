// Package observation provides a unified way to wrap an operation with logging, tracing, and metrics.
//
// To learn more, refer to "How to add observability": https://docs-legacy.sourcegraph.com/dev/how-to/add_observability
package observation

import (
	"context"
	"sync"
	"time"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/honey"
	"github.com/sourcegraph/sourcegraph/internal/hostname"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type ErrorFilterBehaviour uint8

const (
	EmitForNone    ErrorFilterBehaviour = 0
	EmitForMetrics ErrorFilterBehaviour = 1 << iota
	EmitForLogs
	EmitForTraces
	EmitForHoney
	EmitForSentry
	EmitForAllExceptLogs = EmitForMetrics | EmitForSentry | EmitForTraces | EmitForHoney

	EmitForDefault = EmitForMetrics | EmitForLogs | EmitForTraces | EmitForHoney
)

func (b ErrorFilterBehaviour) Without(e ErrorFilterBehaviour) ErrorFilterBehaviour {
	return b ^ e
}

// Op configures an Operation instance.
type Op struct {
	// Metrics sets the RED metrics triplet used to monitor & track metrics for this operation.
	// This field is optional, with `nil` meaning no metrics will be tracked for this.
	Metrics *metrics.REDMetrics
	// Name configures the trace and error log names. This string should be of the
	// format {GroupName}.{OperationName}, where both sections are title cased
	// (e.g. Store.GetRepoByID).
	Name string
	// Description is a simple description for this Op.
	Description string
	// MetricLabelValues that apply for every invocation of this operation.
	MetricLabelValues []string
	// Attributes that apply for every invocation of this operation.
	Attrs []attribute.KeyValue
	// ErrorFilter returns true for any error that should be converted to nil
	// for the purposes of metrics and tracing. If this field is not set then
	// error values are unaltered.
	//
	// This is useful when, for example, a revision not found error is expected by
	// a process interfacing with gitserver. Such an error should not be treated as
	// an unexpected value in metrics and traces but should be handled higher up in
	// the stack.
	ErrorFilter func(err error) ErrorFilterBehaviour
}

// Operation represents an interesting section of code that can be invoked. It has an
// embedded Logger that can be used directly.
type Operation struct {
	context      *Context
	metrics      *metrics.REDMetrics
	errorFilter  func(err error) ErrorFilterBehaviour
	name         string
	kebabName    string
	metricLabels []string
	attributes   []attribute.KeyValue

	// Logger is a logger scoped to this operation. Must not be nil.
	log.Logger
}

// TraceLogger is returned from With and can be used to add timestamped key and
// value pairs into a related span. It has an embedded Logger that can be used
// directly to log messages in the context of a trace.
type TraceLogger interface {
	// AddEvent logs an event with name and fields on the trace.
	AddEvent(name string, attributes ...attribute.KeyValue)

	// SetAttributes adds attributes to the trace, and also applies fields to the
	// underlying Logger.
	SetAttributes(attributes ...attribute.KeyValue)

	// Logger is a logger scoped to this trace.
	log.Logger
}

// TestTraceLogger creates an empty TraceLogger that can be used for testing. The logger
// should be 'logtest.Scoped(t)'.
func TestTraceLogger(logger log.Logger) TraceLogger {
	tr, _ := trace.New(context.Background(), "test")
	return &traceLogger{
		Logger: logger,
		trace:  tr,
	}
}

type traceLogger struct {
	opName  string
	event   honey.Event
	trace   trace.Trace
	context *Context

	log.Logger
}

// initWithTags adds tags to everything except the underlying Logger, which should
// already have init fields due to being spawned from a parent Logger.
func (t *traceLogger) initWithTags(attrs ...attribute.KeyValue) {
	if honey.Enabled() {
		for _, field := range attrs {
			t.event.AddField(t.opName+"."+toSnakeCase(string(field.Key)), field.Value.AsInterface())
		}
	}
	t.trace.SetAttributes(attrs...)
}

func (t *traceLogger) AddEvent(name string, attributes ...attribute.KeyValue) {
	if honey.Enabled() && t.context.HoneyDataset != nil {
		event := t.context.HoneyDataset.EventWithFields(map[string]any{
			"operation":            toSnakeCase(name),
			"meta.hostname":        hostname.Get(),
			"meta.version":         version.Version(),
			"meta.annotation_type": "span_event",
			"trace.trace_id":       t.event.Fields()["trace.trace_id"],
			"trace.parent_id":      t.event.Fields()["trace.span_id"],
		})
		for _, attr := range attributes {
			event.AddField(t.opName+"."+toSnakeCase(string(attr.Key)), attr.Value.AsInterface())
		}
		// if sample rate > 1 for this dataset, then theres a possibility that this event
		// won't be sent but the "parent" may be sent.
		event.Send()
	}
	t.trace.AddEvent(name, attributes...)
}

func (t *traceLogger) SetAttributes(attributes ...attribute.KeyValue) {
	if honey.Enabled() {
		for _, attr := range attributes {
			t.event.AddField(t.opName+"."+toSnakeCase(string(attr.Key)), attr.Value)
		}
	}
	t.trace.SetAttributes(attributes...)
	t.Logger = t.Logger.With(attributesToLogFields(attributes)...)
}

// FinishFunc is the shape of the function returned by With and should be invoked within
// a defer directly before the observed function returns or when a context is cancelled
// with OnCancel.
type FinishFunc func(count float64, args Args)

// OnCancel allows for ending an observation when a context is cancelled as opposed to the
// more common scenario of when the observed function returns through a defer. This can
// be used for continuing an observation beyond the lifetime of a function if that function
// returns more units of work that you want to observe as part of the original function.
func (f FinishFunc) OnCancel(ctx context.Context, count float64, args Args) {
	go func() {
		<-ctx.Done()
		f(count, args)
	}()
}

// ErrCollector represents multiple errors and additional log fields that arose from those errors.
// This type is thread-safe.
type ErrCollector struct {
	mu         sync.Mutex
	errs       error
	extraAttrs []attribute.KeyValue
}

func NewErrorCollector() *ErrCollector { return &ErrCollector{errs: nil} }

func (e *ErrCollector) Collect(err *error, attrs ...attribute.KeyValue) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err != nil && *err != nil {
		e.errs = errors.Append(e.errs, *err)
		e.extraAttrs = append(e.extraAttrs, attrs...)
	}
}

func (e *ErrCollector) Error() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.errs == nil {
		return ""
	}
	return e.errs.Error()
}

func (e *ErrCollector) Unwrap() error {
	// ErrCollector wraps collected errors, for compatibility with errors.HasType,
	// errors.Is etc it has to implement Unwrap to return the inner errors the
	// collector stores.

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.errs
}

// appendExtraAttrs appends the extra attributes stored in the ErrCollector to the provided base slice.
func (e *ErrCollector) appendExtraAttrs(base []attribute.KeyValue) []attribute.KeyValue {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append(base, e.extraAttrs...)
}

// Args configures the observation behavior of an invocation of an operation.
type Args struct {
	// MetricLabelValues that apply only to this invocation of the operation.
	MetricLabelValues []string

	// Attributes that only apply to this invocation of the operation
	Attrs []attribute.KeyValue
}

// WithErrors prepares the necessary timers, loggers, and metrics to observe the invocation of an
// operation. This method returns a modified context, an multi-error capturing type and a function to be deferred until the
// end of the operation. It can be used with FinishFunc.OnCancel to capture multiple async errors.
func (op *Operation) WithErrors(ctx context.Context, root *error, args Args) (context.Context, *ErrCollector, FinishFunc) {
	ctx, collector, _, endObservation := op.WithErrorsAndLogger(ctx, root, args)
	return ctx, collector, endObservation
}

// WithErrorsAndLogger prepares the necessary timers, loggers, and metrics to observe the invocation of an
// operation. This method returns a modified context, an multi-error capturing type, a function that will add a log field
// to the active trace, and a function to be deferred until the end of the operation. It can be used with
// FinishFunc.OnCancel to capture multiple async errors.
func (op *Operation) WithErrorsAndLogger(ctx context.Context, root *error, args Args) (context.Context, *ErrCollector, TraceLogger, FinishFunc) {
	errTracer := NewErrorCollector()
	err := error(errTracer)

	ctx, traceLogger, endObservation := op.With(ctx, &err, args)

	// to avoid recursion stack overflow, we need a new binding
	endFunc := endObservation

	if root != nil {
		endFunc = func(count float64, args Args) {
			if *root != nil {
				errTracer.Collect(root)
			}
			endObservation(count, args)
		}
	}
	return ctx, errTracer, traceLogger, endFunc
}

// With prepares the necessary timers, loggers, and metrics to observe the invocation
// of an operation. This method returns a modified context, a function that will add a log field
// to the active trace, and a function to be deferred until the end of the operation.
func (op *Operation) With(ctx context.Context, err *error, args Args) (context.Context, TraceLogger, FinishFunc) {
	parentTraceContext := trace.Context(ctx)
	start := time.Now()
	tr, ctx := op.startTrace(ctx)

	event := honey.NonSendingEvent()
	snakecaseOpName := toSnakeCase(op.name)
	if op.context.HoneyDataset != nil {
		event = op.context.HoneyDataset.EventWithFields(map[string]any{
			"operation":     snakecaseOpName,
			"meta.hostname": hostname.Get(),
			"meta.version":  version.Version(),
		})
	}

	logger := op.Logger.With(attributesToLogFields(args.Attrs)...)

	if traceContext := trace.Context(ctx); traceContext.TraceID != "" {
		event.AddField("trace.trace_id", traceContext.TraceID)
		event.AddField("trace.span_id", traceContext.SpanID)
		if parentTraceContext.SpanID != "" {
			event.AddField("trace.parent_id", parentTraceContext.SpanID)
		}
		logger = logger.WithTrace(traceContext)
	}

	trLogger := &traceLogger{
		context: op.context,
		opName:  snakecaseOpName,
		event:   event,
		trace:   tr,
		Logger:  logger,
	}

	if mergedFields := mergeAttrs(op.attributes, args.Attrs); len(mergedFields) > 0 {
		trLogger.initWithTags(mergedFields...)
	}

	return ctx, trLogger, func(count float64, finishArgs Args) {
		since := time.Since(start)
		elapsed := since.Seconds()
		elapsedMs := since.Milliseconds()
		defaultFinishFields := []attribute.KeyValue{attribute.Float64("count", count), attribute.Float64("elapsed", elapsed)}
		finishAttrs := mergeAttrs(defaultFinishFields, finishArgs.Attrs)
		metricLabels := mergeLabels(op.metricLabels, args.MetricLabelValues, finishArgs.MetricLabelValues)

		if multi := new(ErrCollector); err != nil && errors.As(*err, &multi) {
			if multi.Error() == "" {
				err = nil
			}

			multi.appendExtraAttrs(finishAttrs)
		}

		var (
			logErr     = op.applyErrorFilter(err, EmitForLogs)
			metricsErr = op.applyErrorFilter(err, EmitForMetrics)
			traceErr   = op.applyErrorFilter(err, EmitForTraces)
			honeyErr   = op.applyErrorFilter(err, EmitForHoney)

			emitToSentry = op.applyErrorFilter(err, EmitForSentry) != nil
		)

		// already has all the other log fields
		op.emitErrorLogs(trLogger, logErr, finishAttrs, emitToSentry)
		// op. and args.LogFields already added at start
		op.emitHoneyEvent(honeyErr, snakecaseOpName, event, finishArgs.Attrs, elapsedMs)

		op.emitMetrics(metricsErr, count, elapsed, metricLabels)

		op.finishTrace(traceErr, tr, finishAttrs)
	}
}

// startTrace creates a new Trace object and returns the wrapped context. This returns
// an unmodified context and a nil startTrace if no tracer was supplied on the observation context.
func (op *Operation) startTrace(ctx context.Context) (trace.Trace, context.Context) {
	tracer := op.context.Tracer
	if tracer == nil {
		tracer = trace.GetTracer()
	}
	return trace.NewInTracer(ctx, tracer, op.kebabName)
}

// emitErrorLogs will log as message if the operation has failed. This log contains the error
// as well as all of the log fields attached ot the operation, the args to With, and the args
// to the finish function.
func (op *Operation) emitErrorLogs(trLogger TraceLogger, err *error, attrs []attribute.KeyValue, emitToSentry bool) {
	if err == nil || *err == nil {
		return
	}

	errField := log.Error(*err)
	if !emitToSentry {
		// only fields of type ErrorType end up in sentry
		errField = log.String("error", (*err).Error())
	}
	fields := append(attributesToLogFields(attrs), errField)

	trLogger.
		AddCallerSkip(2). // callback() -> emitErrorLogs() -> Logger
		Error("operation.error", fields...)
}

func (op *Operation) emitHoneyEvent(err *error, opName string, event honey.Event, attrs []attribute.KeyValue, duration int64) {
	if err != nil && *err != nil {
		event.AddField("error", (*err).Error())
	}

	event.AddField("duration_ms", duration)

	for _, attr := range attrs {
		event.AddField(opName+"."+toSnakeCase(string(attr.Key)), attr.Value.AsInterface())
	}

	event.Send()
}

// emitMetrics will emit observe the duration, operation/result, and error counter metrics
// for this operation. This does nothing if no metric was supplied to the observation.
func (op *Operation) emitMetrics(err *error, count, elapsed float64, labels []string) {
	if op.metrics == nil {
		return
	}

	op.metrics.Observe(elapsed, count, err, labels...)
}

// finishTrace will set the error value, log additional fields supplied after the operation's
// execution, and finalize the trace span. This does nothing if no trace was constructed at
// the start of the operation.
func (op *Operation) finishTrace(err *error, tr trace.Trace, attrs []attribute.KeyValue) {
	if err != nil {
		tr.SetError(*err)
	}

	tr.SetAttributes(attrs...)
	tr.End()
}

// applyErrorFilter returns nil if the given error does not pass the registered error filter.
// The original value is returned otherwise.
func (op *Operation) applyErrorFilter(err *error, behaviour ErrorFilterBehaviour) *error {
	if op.errorFilter != nil && err != nil && op.errorFilter(*err)&behaviour == 0 {
		return nil
	}

	return err
}
