# Developing a background routine

Background routines are long running processes in our backend binaries.

They are defined in the [`goroutine`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b946a20/-/blob/internal/goroutine/background.go?subtree=true#L22) package.

Examples:

- [`worker.NewWorker`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+worker.NewWorker&patternType=literal), for example, produces a background routine, which in this case is a background worker.
- [`batches.newSpecExpireWorker`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/enterprise/internal/batches/background/spec_expire.go?subtree=true#L13-27) returns a [`goroutine.PeriodicGoroutine`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b946a20362ee7dfedb3b1fbc7f8bb002135d7283/-/blob/internal/goroutine/periodic.go?subtree=true#L14:78), which means it's invoked periodically.
- [out-of-band migrations](oobmigrations.md) are implemented as background routines.
- [`HardDeleter`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b946a20362ee7dfedb3b1fbc7f8bb002135d7283/-/blob/enterprise/cmd/frontend/internal/codeintel/background/janitor/hard_delete.go?subtree=true#L33) is a periodic background routine that periodically hard-deletes soft-deleted upload records.

See also the [godocs.io examples for the `goroutine` package](https://godocs.io/github.com/sourcegraph/sourcegraph/internal/goroutine).

## Adding a background routine

### Step 1: Implement the `goroutine.BackgroundRoutine` interface

In order to be managed by the utilities in the `goroutine` package, a background routine needs to implement the [`goroutine.BackgroundRoutine`](https://github.com/sourcegraph/sourcegraph/blob/b946a20362ee7dfedb3b1fbc7f8bb002135d7283/internal/goroutine/background.go#L20-L29) interface.

```go
type myRoutine struct {
	done chan struct{}
}

func (m *myRoutine) Start() {
	for {
		select {
		case <-m.done:
			fmt.Println("done!")
			return
		default:
		}

		fmt.Println("Hello there!")
		time.Sleep(1 * time.Second)
	}
}

func (m *myRoutine) Stop() {
	m.done <- struct{}{}
}
```

### Step 2: Start and monitor the background routine

With `myRoutine` defined, we can start and monitor it in a separate goroutine:

```go
func main() {
	r := &myRoutine{
		done: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Make sure to run this in a separate goroutine
	go goroutine.MonitorBackgroundRoutines(ctx, r)

	// Give the background routine some time to do its work...
	time.Sleep(2 * time.Second)

	// ... and then cancel the context.
	cancel()

	// The routine will stop now, but let's give it some time to print its
	// message
	time.Sleep(1 * time.Second)
}
```

Canceling the `ctx` will signal to `MonitorBackgroundRoutines` that it should `Stop()` the background routines.

If we run this we'll see the following output:

```
Hello there!
Hello there!
done!
```

## Adding a periodic background routine

### Step 1: Define a handler

Use the `goroutine.NewHandlerWithErrorMessage` helper to create a handler function that implements the `goroutine.HandlerFunc` interface:

```go
myHandler := goroutine.NewHandlerWithErrorMessage("this is my cool handler", func(ctx context.Context) error {
	fmt.Println("Hello from the background!")

	// Talk to the database, send HTTP requests, etc.
	return nil
})
```

### Step 2: Create periodic goroutine from handler

With `myHandler` defined, we can create a periodic goroutine using `goroutine.NewPeriodicGoroutine`:

```go
myPeriodicGoroutine := goroutine.NewPeriodicGoroutine(ctx, 2*time.Minute, myHandler)
```

### Step 3: Start and monitor the background routine

The last step is to start the routine in a goroutine and monitor it:

```go
go goroutine.MonitorBackgroundRoutines(ctx, myPeriodicGoroutine)
```
