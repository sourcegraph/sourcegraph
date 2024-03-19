package goroutine

import (
	"context"
	"testing"
	"time"

	"github.com/derision-test/glock"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func withClock(clock glock.Clock) Option {
	return func(p *PeriodicGoroutine) { p.clock = clock }
}

func withConcurrencyClock(clock glock.Clock) Option {
	return func(p *PeriodicGoroutine) { p.concurrencyClock = clock }
}

func TestPeriodicGoroutine(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandler()
	called := make(chan struct{}, 1)

	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		called <- struct{}{}
		return nil
	})

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithInterval(time.Second),
		withClock(clock),
	)
	go goroutine.Start()
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 4 {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 4, calls)
	}
}

func TestPeriodicGoroutineReinvoke(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandler()
	called := make(chan struct{}, 1)

	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		called <- struct{}{}
		return ErrReinvokeImmediately
	})

	witnessHandler := func() {
		for range maxConsecutiveReinvocations {
			<-called
		}
	}

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithInterval(time.Second),
		withClock(clock),
	)
	go goroutine.Start()
	witnessHandler()
	clock.BlockingAdvance(time.Second)
	witnessHandler()
	clock.BlockingAdvance(time.Second)
	witnessHandler()
	clock.BlockingAdvance(time.Second)
	witnessHandler()
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 4*maxConsecutiveReinvocations {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 4*maxConsecutiveReinvocations, calls)
	}
}

func TestPeriodicGoroutineWithDynamicInterval(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandler()
	called := make(chan struct{}, 1)

	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		called <- struct{}{}
		return nil
	})

	seconds := 1

	// intervals: 1 -> 2 -> 3 ...
	getInterval := func() time.Duration {
		duration := time.Duration(seconds) * time.Second
		seconds += 1
		return duration
	}

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithIntervalFunc(getInterval),
		withClock(clock),
	)
	go goroutine.Start()
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(2 * time.Second)
	<-called
	clock.BlockingAdvance(3 * time.Second)
	<-called
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 4 {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 4, calls)
	}
}

func TestPeriodicGoroutineWithInitialDelay(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandler()
	called := make(chan struct{}, 1)

	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		called <- struct{}{}
		return nil
	})

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithInterval(time.Second),
		WithInitialDelay(2*time.Second),
		withClock(clock),
	)
	go goroutine.Start()
	clock.BlockingAdvance(time.Second)
	select {
	case <-called:
		t.Error("unexpected handler invocation")
	default:
	}
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 3 {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 3, calls)
	}
}

func TestPeriodicGoroutineConcurrency(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandler()
	called := make(chan struct{})
	concurrency := 4

	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		called <- struct{}{}
		return nil
	})

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithConcurrency(concurrency),
		withClock(clock),
	)
	go goroutine.Start()

	for range concurrency {
		<-called
		clock.BlockingAdvance(time.Second)
	}

	for range concurrency {
		<-called
		clock.BlockingAdvance(time.Second)
	}

	for range concurrency {
		<-called
	}

	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 3*concurrency {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 3*concurrency, calls)
	}
}

func TestPeriodicGoroutineWithDynamicConcurrency(t *testing.T) {
	clock := glock.NewMockClock()
	concurrencyClock := glock.NewMockClock()
	handler := NewMockHandler()
	called := make(chan struct{})
	exit := make(chan struct{})

	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		select {
		case called <- struct{}{}:
			return nil

		case <-ctx.Done():
			select {
			case exit <- struct{}{}:
			default:
			}

			return ctx.Err()
		}
	})

	concurrency := 0

	// concurrency: 1 -> 2 -> 3 ...
	getConcurrency := func() int {
		concurrency += 1
		return concurrency
	}

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithConcurrencyFunc(getConcurrency),
		withClock(clock),
		withConcurrencyClock(concurrencyClock),
	)
	go goroutine.Start()

	for poolSize := 1; poolSize < 3; poolSize++ {
		// Ensure each of the handlers can be called independently.
		// Adding an additional channel read would block as each of
		// the monitor routines would be waiting on the clock tick.
		for range poolSize {
			<-called
		}

		// Resize the pool
		clock.BlockingAdvance(time.Second)                           // invoke but block one handler
		concurrencyClock.BlockingAdvance(concurrencyRecheckInterval) // trigger drain of the old pool
		<-exit                                                       // wait for blocked handler to exit
	}

	goroutine.Stop()

	// N.B.: no need for assertions here as getting through the test at all to this
	// point without some permanent blockage shows that each of the pool sizes behave
	// as expected.
}

func TestPeriodicGoroutineError(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandlerWithErrorHandler()

	calls := 0
	called := make(chan struct{}, 1)
	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) (err error) {
		if calls == 0 {
			err = errors.New("oops")
		}

		calls++
		called <- struct{}{}
		return err
	})

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithInterval(time.Second),
		withClock(clock),
	)
	go goroutine.Start()
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 4 {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 4, calls)
	}

	if calls := len(handler.HandleErrorFunc.History()); calls != 1 {
		t.Errorf("unexpected number of error handler invocations. want=%d have=%d", 1, calls)
	}
}

func TestPeriodicGoroutinePanic(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandlerWithErrorHandler()

	calls := 0
	called := make(chan struct{}, 1)
	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		calls++
		defer func() {
			called <- struct{}{}
		}()

		if calls == 1 {
			panic("oops")
		}

		return nil
	})

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithInterval(time.Second),
		withClock(clock),
	)
	go goroutine.Start()

	clock.BlockingAdvance(time.Second)
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("first call didn't happen within 1s")
	}
	// Run a second time to make sure it actually is invoked again after the
	// panic. Periodic goroutines turn panics into errors (analogous to
	// goroutine.Go which silences panics), and we expect to keep running a periodic
	// routine after a panic.
	clock.BlockingAdvance(time.Second)
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("second call didn't happen within 1s")
	}
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 2 {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 4, calls)
	}
	if calls := len(handler.HandleErrorFunc.History()); calls != 1 {
		t.Errorf("unexpected number of error handler invocations. want=%d have=%d", 4, calls)
	}
}

func TestPeriodicGoroutineContextError(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandlerWithErrorHandler()

	called := make(chan struct{}, 1)
	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		called <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	})

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithInterval(time.Second),
		withClock(clock),
	)
	go goroutine.Start()
	<-called
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 1 {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 1, calls)
	}

	if calls := len(handler.HandleErrorFunc.History()); calls != 0 {
		t.Errorf("unexpected number of error handler invocations. want=%d have=%d", 0, calls)
	}
}

func TestPeriodicGoroutineFinalizer(t *testing.T) {
	clock := glock.NewMockClock()
	handler := NewMockHandlerWithFinalizer()

	called := make(chan struct{}, 1)
	handler.HandleFunc.SetDefaultHook(func(ctx context.Context) error {
		called <- struct{}{}
		return nil
	})

	goroutine := NewPeriodicGoroutine(
		context.Background(),
		handler,
		WithName(t.Name()),
		WithInterval(time.Second),
		withClock(clock),
	)
	go goroutine.Start()
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	clock.BlockingAdvance(time.Second)
	<-called
	goroutine.Stop()

	if calls := len(handler.HandleFunc.History()); calls != 4 {
		t.Errorf("unexpected number of handler invocations. want=%d have=%d", 4, calls)
	}

	if calls := len(handler.OnShutdownFunc.History()); calls != 1 {
		t.Errorf("unexpected number of finalizer invocations. want=%d have=%d", 1, calls)
	}
}

type MockHandlerWithErrorHandler struct {
	*MockHandler
	*MockErrorHandler
}

func NewMockHandlerWithErrorHandler() *MockHandlerWithErrorHandler {
	return &MockHandlerWithErrorHandler{
		MockHandler:      NewMockHandler(),
		MockErrorHandler: NewMockErrorHandler(),
	}
}

type MockHandlerWithFinalizer struct {
	*MockHandler
	*MockFinalizer
}

func NewMockHandlerWithFinalizer() *MockHandlerWithFinalizer {
	return &MockHandlerWithFinalizer{
		MockHandler:   NewMockHandler(),
		MockFinalizer: NewMockFinalizer(),
	}
}
