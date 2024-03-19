# How to add logging

This guide describes best practices for adding logging to Sourcegraph's backend components.

> NOTE: For how to *use* Sourcegraph's observability and an overview of our observability features, refer to the [observability for site administrators documentation](../../admin/observability/index.md).

The recommended logger at Sourcegraph is [`github.com/sourcegraph/log`](https://sourcegraph.com/github.com/sourcegraph/log), which exports:

1. A standardized, strongly-typed, and structured logging interface, [`log.Logger`](https://sourcegraph.com/github.com/sourcegraph/log/-/blob/logger.go)
   1. Production output from this logger (`SRC_LOG_FORMAT=json`) complies with the [OpenTelemetry log data model](https://opentelemetry.io/docs/reference/specification/logs/data-model/) (also see: [Logging: OpenTelemetry](../../admin/observability/logs.md#opentelemetry))
   2. `log.Logger` has a variety of constructors for spawning new loggers with context, namely `Scoped`, `WithTrace`, and `WithFields`.
2. An initialization function to be called in `func main()`, `log.Init()`, that must be called.
   1. Log level can be configured with `SRC_LOG_LEVEL` (also see: [Logging: Log levels](../../admin/observability/logs.md#log-levels))
   2. Do not use this in an `init()` function—we want to explicitly avoid tying logger instances as a compile-time dependency.
3. A getter to retrieve a `log.Logger` instance, [`log.Scoped`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+log.Scoped+lang:go&patternType=literal), and [`(Logger).Scoped`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+logger.Scoped+lang:go&patternType=literal) for [creating sub-loggers](#sub-loggers).
4. [Testing utilities](#testing-usage)

Logging is also available via the all-in-one `internal/observation` package: [How to add observability](add_observability.md)

> NOTE: Sourcegraph's new logging standards are still a work in progress—please leave a comment in [this discussion](https://github.com/sourcegraph/sourcegraph/discussions/33248) if you have any feedback or ideas!

## Core concepts

1. `github.com/sourcegraph/log` intentionally does not export directly usable (global) log functions. Users should hold their own references to `Logger` instances, so that fields can be attached and log output can be more useful with additional context, and pass `Logger`s to components as required.
   1. Do not call the package-level `log.Scoped` wherever you need to log—consider passing a `Logger` around explicitly, and [creating sub-scoped loggers using `(Logger).Scoped()`](#scoped-loggers).
   2. [Do not add `Logger` to `context.Context`](https://dave.cheney.net/2017/01/26/context-is-for-cancelation).
   3. [Do not create package-level logger instances](https://dave.cheney.net/2017/01/23/the-package-level-logger-anti-pattern).
2. `github.com/sourcegraph/log` should export everything that is required for logging—do not directly import a third-party logging package such as `zap`, `log15`, or the standard `log` library.

## Handling loggers

Before creating loggers, you must initialize `github.com/sourcegraph/log` package within your program's `main()` function.
Initialization includes metadata about your service, as well as log sinks to tee log entries to additional destinations.

For example, a typical initialization process looks like this:

```go
import (
  "github.com/sourcegraph/log"
  
  "github.com/sourcegraph/sourcegraph/internal/env"
  "github.com/sourcegraph/sourcegraph/internal/version"
  "github.com/sourcegraph/sourcegraph/internal/hostname"
)

func main() {
  // If unintialized, calls to `log.Scoped` will return a no-op logger in production, or
  // panic in development. It returns a callback to flush the logger buffer, if any, that
  // you should make sure to call before application exit (namely via `defer`)
  //
  // Repeated calls to `log.Init` will panic. Make sure to call this exactly once in `main`!
  liblog := log.Init(log.Resource{
    Name:       env.MyName,
    Version:    version.Version(),
    InstanceID: hostname.Get(),
  }, log.NewSentrySink())
  defer liblog.Sync()

  // Now we register a hook to update log sinks.
  //
  // Note that the call to conf.Watch must be run in a goroutine, because the initial call
  // is blocking if a connection cannot be established.
  conf.Init()
  go conf.Watch(liblog.Update(conf.GetLogSinks))
}
```

### Basic conventions

- The logger parameter should either be immediately after `ctx context.Context`, or be the first parameter.
- In some cases there might already be a `log` module imported. Use the alias `sglog` to refer to `github.com/sourcegraph/log`, for example `import sglog "github.com/sourcegraph/log"`.
- When testing, provide [test loggers](#testing-usage) for improved output management.
- For more conventions, see relevant subsections in this guide, such as [top-level loggers](#top-level-loggers), [sub-loggers](#sub-loggers), and [writing log messages](#writing-log-messages).

### Top-level loggers

Once initialized, you can use `log.Scoped()` to create some top-level loggers to propagate. From each logger, you can:

- [create sub-loggers](#sub-loggers)
- [write log entries](#writing-log-messages)

The first top-level scoped logger is typically `"server"`, since most logging is related to server initialization and service-level logging—the name of the service itself is already logged as part of the [`Resource` field](https://opentelemetry.io/docs/reference/specification/logs/data-model/#field-resource) provided during initialization.

Background jobs, etc. can have additional top-level loggers created that better describe each components.

### Sub-loggers

When your service starts logging, obtain a `log.Logger` instance, attach some relevant context, and start propagating your logger for use.
From a parent logger, you can create sub-loggers that have additional context attached to them using functions on the `log.Logger` instance that returns a new `log.Logger` instance:

1. [Scoped loggers](#scoped-loggers)
2. [Fields sub-loggers](#fields-sub-loggers)
3. [Traced sub-loggers](#traced-sub-loggers)

All the above mechanisms attach metadata to *all* log entries created by the sub-logger, and they do not modify the parent logger.
Using sub-loggers allows you to easily trace, for example, the execution of an event or a particular execution type by looking for shared log fields.

> WARNING: All sub-logger constructors and functions on the `log.Logger` type (e.g. `(Logger).Soped(...)`, `(Logger).With(...)`, `(Logger).WithTrace(...)`) **do not** modify the underlying logger—you must hold and use the reference to the returned `log.Logger` instance.

#### Scoped loggers

Scopes are used to identify the component of a system a log message comes from, and generally should provide enough information for an uninitiated reader (such as a new teammate, or a Sourcegraph administrator) to get a rough idea the context in which a log message might have occurred.

There are several ways to create scoped loggers:

- a top-level scoped logger, from the package-level `log.Scoped()` function, is used mostly in a `main()`-type function or in places where no logger has been propagated.
- a scoped sub-logger, which can be created from an existing logger with `(Logger).Scoped()`. The sub-scope is appended onto the parent scope with a `.` separator.

In general:

- From the caller, only add a scope if, as a caller, you care that the log output enough to want to differentiate it
  - For example, if you create multiple clients for a service, you will probably want to create a separate scoped logger for each
- From the callee, add a scope if you will be logging output that should be meaningfully differentiated (e.g. inside a client, or inside a significant helper function)
- Scope names should be `CamelCase` or `camelCase`, and the scope description should follow [the same conventions as a log message](#writing-log-messages).

Example:

```go
func NewClient() *Client {
  return &Client{logger: log.Scoped("Client", "client for a certain thing")}
}

func (p *Client) Request(logger log.Logger) {
  requestLog := p.logger.Scoped("Request", "executes a request") // creates scope "Public.Process"

  requestLog.Info("starting tasks")
  // ... things ...

  requestLog.Debug("mid checkpoint")

  helperFunc(requestLog)

  // ... things ...
  requestLog.Info("finalizing some things!")
}

func helperFunc(logger log.Logger) {
  // This is a small helper function, so no need to create another scope
  logger.Info("I'm helping!")
}
```

#### Fields sub-loggers

In a particular scope, some fields might be repeatedly emitted—most commonly, you might want to associate a set of log entries to an ID of some sort (user ID, iteration ID, etc). In these scenarios you should create a sub-logger with prepended fields by using `logger.With(...fields)`, and use it directly to maintain relevant context:

```go
func (w *Worker) DoBigThing(ctx context.Context, id int) {
  doLog := w.logger.Scoped(ctx).
    With(log.Int("id", id))

  // subsequent entries will have trace and params attached
  doLog.Info("starting the big thing")
  // {
  //   "InstrumentationScope": "...",
  //   "Message": "starting the big thing",
  //   "Attributes": { "id": 1 },
  // }

  // pass the logger to maintain context across function boundaries
  doSubTask(doLog, /*... */)
}
```

#### Traced sub-loggers

Traced loggers are loggers with trace context (trace and span IDs) attached to them. These loggers can be created in several ways:

1. [`internal/trace.Logger(ctx, logger)`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/trace/logger.go) creates a sub-logger with trace context from `context.Context`
2. [`(internal/observation.Operation).With`](add_observability.md) creates a sub-logger with trace context and fields from the operation
3. `(Logger).WithTrace(...)` creates a sub-logger with explicitly provided trace context

For example:

```go
import (
  "github.com/sourcegraph/log"

  "github.com/sourcegraph/sourcegraph/internal/trace"
)

func (w *Worker) DoBigThing(ctx context.Context, id int) {
  // ...

  // start a trace in the context
  tr, ctx := trace.New(ctx, /* ... */)
  logger := trace.Logger(ctx, w.logger)
  doSubTask(logger, /*... */)
}

func doSubTask(logger log.Logger) {
  logger.Info("a message")
  // {
  //   "InstrumentationScope": "...",
  //   "TraceID": "...",
  //   "Message": "a message",
  // }
}
```

Note that if `trace.Logger` finds an `internal/trace.Trace`, it will use that instead, and also apply the trace family as the logger scope.
If the result proves problematic, you can bypass the behaviour by using `WithTrace` with `internal/trace.Context(...)` directly:

```go
func doSomething(ctx context.Context, logger log.Logger) {
  logger = logger.WithTrace(trace.Context(ctx))
}
```

## Writing log messages

The message in a log line should generally be in lowercase, and should generally not have ending punctuation.

```go
logger.Info("this is my lowercase log line",
  log.String("someField", value))
logger.Error("this is an error",
  log.Error(err))
```

If writing log messages that, for example, indicate the results of a function, simply use the Go conventions for naming (i.e. just copy the function name).

If multiple log lines have similar components (such as a message prefix, or the same log fields) prefer to [create a sub-logger](#sub-loggers) instead. For example:

- instead of repeating a message prefix to e.g. indicate a component, [create a scoped sub-logger](#scoped-loggers) instead
- instead of adding the same field to multiple log calls, [create a fields sub-logger](#fields-sub-loggers) instead

> WARNING: Field constructors provided by the `sourcegraph/log` package, for example `log.Error(error)`, **do not** create log entries—they create fields (`type log.Field`) that are intended to be provided to `Logger` functions like `(Logger).Info` and so on.

### Log levels

Guidance on when to use each log level is available on the docstrings of each respective logging function on `Logger`:

<div class="embed">
  <iframe src="https://sourcegraph.com/embed/notebooks/Tm90ZWJvb2s6MTE3Nw=="
    style="width:100%;height:520px" frameborder="0" sandbox="allow-scripts allow-same-origin allow-popups">
  </iframe>
</div>

## Production usage

See [observability: logs](../../admin/observability/logs.md) in the administration docs.

### Automatic error reporting with Sentry

If the optional sink [`log.NewSentrySink()`](https://doctree.org/github.com/sourcegraph/log/-/go/-//?id=NewSentrySink) is passed when initializing the logger, when an error is passed in a field to the logger with `log.Error(err)`, it will be reported to Sentry automatically if and only if the log level is above or equal to `Warn`.
The log message and all fields will be used to annotate the error report and the logger scope will be used as a tag, which being indexed by Sentry, enables to group reports. 

For example, the Sentry search query `is:unresolved scope:*codeintel*` will surface all error reports coming from errors that were logged by loggers whose scope includes `codeintel`.

If multiple error fields are passed, an individual report will be created for each of them.

The Sentry project to which the reports are sent is configured through the `log.sentry.backendDSN` site-config entry.

## Development usage

With `SRC_DEVELOPMENT=true` and `SRC_LOG_FORMAT=condensed` or `SRC_LOG_FORMAT=console`, loggers will generate a human-readable summary format like the following:

```none
DEBUG TestLogger log/logger_test.go:22 a debug message
INFO TestLogger log/logger_test.go:26 hello world {"some": "field", "hello": "world"}
INFO TestLogger log/logger_test.go:29 goodbye {"TraceId": "1234abcde", "some": "field", "world": "hello"}
WARN TestLogger log/logger_test.go:30 another message {"TraceId": "1234abcde", "some": "field"}
ERROR TestLogger log/logger_test.go:32 object of fields {"TraceId": "1234abcde", "some": "field", "object": {"field1": "value", "field2": "value"}}
```

This format omits fields like OpenTelemetry `Resource` and renders certain field types in a more friendly manner. Levels are also coloured, and the caller link with `filename:line` should be clickable in iTerm and VS Code such that you can jump straight to the source of the log entry.

Additionally, in `SRC_DEVELOPMENT=true` using `log.Scoped` without calling `log.Init` will panic (in production, a no-op logger will be returned).

## Testing usage

For testing purposes, we also provide:

1. A getter to retrieve a `log.Logger` instance and a callback to programmatically iterate log output, [`logtest.Scoped`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+logtest.Scoped+lang:go&patternType=literal)
   1. The standard `log.Scoped` will also work after `logtest.Init`
   2. Programatically iterable logs are available from the `logtest.Captured` logger instance
   3. Can be provided in code that accepts [injected loggers](#injected-loggers)
2. An *optional* initialization function to be called in `func TestMain(*testing.M)`, [`logtest.Init`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+logtest.Init+lang:go&patternType=literal).
   1. This is required for testing code that [*does not* accept injected loggers](#instantiated-loggers)

### Injected loggers

If the code you are testing accepts `Logger` instances as a parameter, you can skip the above and simply use `logtest.Scoped` to instantiate a `Logger` to provide. You can also use `logtest.Captured`, which also provides a callback for exporting logs, though be wary of making overly strict assertions on log entries to avoid brittle tests:

```go
import (
  "testing"

  "github.com/sourcegraph/log/logtest"
)

func TestFooBar(t *testing.T) {
  logger, exportLogs := logtest.Captured(t)

  t.Run("test my thing", func(t *testing.T) {
    fooBar(logger)
  })

  // export log entries that were written during this time
  logs := exportLogs()
}
```

When writing a test, ensure that `logtest.Scope` in the tightest scope possible. This ensures that if a test fails, that the logging is closely tied to the test that failed. Especially if you testcase has sub tests with `t.Run`, prefer to created the test logger inside `t.Run`.

Alternatively, `logtest.NoOp()` creates a logger that can be used to silence output. Levels can also be adjusted using `(Logger).IncreaseLevel`.

### Instantiated loggers

In the absense of `log.Init` in `main()`, testing code that instantiates loggers with `log.Scoped` (as opposed to `(Logger).Scoped`), `github.com/sourcegraph/log` can be initialized using `libtest` in packages that use `log.Scoped`:

```go
import (
  "os"
  "testing"

  "github.com/sourcegraph/log/logtest"
)

func TestMain(m *testing.M) {
  logtest.Init(m)
  os.Exit(m.Run())
}
```

You can control the log level used during initialization with `logtest.InitWithLevel`.
