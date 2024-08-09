package background

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Make the exiter a no-op in tests
func init() { exiter = func() {} }

func TestMonitorBackgroundRoutinesSignal(t *testing.T) {
	r1 := NewMockRoutine()
	r2 := NewMockRoutine()
	r3 := NewMockRoutine()

	signals := make(chan os.Signal, 1)
	defer close(signals)
	unblocked := make(chan struct{})

	go func() {
		defer close(unblocked)
		err := monitorBackgroundRoutines(context.Background(), signals, r1, r2, r3)
		require.NoError(t, err)
	}()

	signals <- syscall.SIGINT
	<-unblocked

	for _, r := range []*MockRoutine{r1, r2, r3} {
		if calls := len(r.StartFunc.History()); calls != 1 {
			t.Errorf("unexpected number of calls to start. want=%d have=%d", 1, calls)
		}
		if calls := len(r.StopFunc.History()); calls != 1 {
			t.Errorf("unexpected number of calls to stop. want=%d have=%d", 1, calls)
		}
	}
}

func TestMonitorBackgroundRoutinesContextCancel(t *testing.T) {
	// fnIfErrCancelled emulates the behaviour a routine SHOULD exhibit if
	// Stop is correctly implemented: attempt to stop until context cancellation.
	// If the context is cancelled, return the error.
	fnIfErrCancelled := func(ctx context.Context) error {
		<-ctx.Done()
		return errors.Wrap(ctx.Err(), "context cancelled")
	}

	r1 := NewMockRoutine()
	r1.NameFunc.SetDefaultReturn("r1")
	r1.StopFunc.SetDefaultHook(fnIfErrCancelled)

	r2 := NewMockRoutine()
	r2.NameFunc.SetDefaultReturn("r2")
	r2.StopFunc.SetDefaultHook(fnIfErrCancelled)

	r3 := NewMockRoutine()
	r3.NameFunc.SetDefaultReturn("r3")
	r3.StopFunc.SetDefaultHook(fnIfErrCancelled)

	signals := make(chan os.Signal, 1)
	defer close(signals)
	unblocked := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(unblocked)
		err := monitorBackgroundRoutines(ctx, signals, r1, r2, r3)
		assert.ErrorIs(t, err, context.Canceled)
	}()

	cancel()
	<-unblocked

	for _, r := range []*MockRoutine{r1, r2, r3} {
		if calls := len(r.StartFunc.History()); calls != 1 {
			t.Errorf("unexpected number of calls to start. want=%d have=%d", 1, calls)
		}
		if calls := len(r.StopFunc.History()); calls != 1 {
			t.Errorf("unexpected number of calls to stop. want=%d have=%d", 1, calls)
		}
	}
}

func TestCombinedRoutine_Name(t *testing.T) {
	r1 := NewMockRoutine()
	r1.NameFunc.SetDefaultReturn("r1")
	r2 := NewMockRoutine()
	r2.NameFunc.SetDefaultReturn("r2")
	rs := CombinedRoutine{r1, r2}
	assert.Equal(t, `combined["r1" "r2"]`, rs.Name())
}

func TestCombinedRoutines(t *testing.T) {
	t.Run("successful stop", func(t *testing.T) {
		r1 := NewMockRoutine()
		r2 := NewMockRoutine()
		rs := CombinedRoutine{r1, r2}
		rs.Start()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))
		defer cancel()
		err := rs.Stop(ctx)
		require.NoError(t, err)
		mockrequire.Called(t, r1.StopFunc)
		mockrequire.Called(t, r2.StopFunc)
	})

	t.Run("stop with error", func(t *testing.T) {
		r1 := NewMockRoutine()
		r2 := NewMockRoutine()
		r2.NameFunc.SetDefaultReturn("mock")
		r2.StopFunc.SetDefaultReturn(errors.New("stop error"))
		rs := CombinedRoutine{r1, r2}
		rs.Start()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))
		defer cancel()
		err := rs.Stop(ctx)
		assert.EqualError(t, err, `stop routine "mock": stop error`)
		mockrequire.Called(t, r1.StopFunc)
		mockrequire.Called(t, r2.StopFunc)
	})

	t.Run("stop with timeout", func(t *testing.T) {
		r1 := NewMockRoutine()
		r1.NameFunc.SetDefaultReturn("mock")
		r1.StopFunc.SetDefaultReturn(errors.New("stop error"))
		r2 := NewMockRoutine()
		r2.StopFunc.PushHook(func(context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		rs := CombinedRoutine{r1, r2}
		rs.Start()

		// Context deadline is 50ms, which is half of the sleep time of r2.
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
		defer cancel()
		err := rs.Stop(ctx)
		assert.EqualError(t, err, `unable to stop routines gracefully with partial errors: stop routine "mock": stop error: context deadline exceeded`)

		// Although the caller doesn't wait, each routine should still be stopped if
		// they get the chance. We wait long enough to avoid flakiness.
		time.Sleep(100 * time.Millisecond)
		mockrequire.Called(t, r1.StopFunc)
		mockrequire.Called(t, r2.StopFunc)
	})
}

func TestLIFOStopRoutine(t *testing.T) {
	// use an unguarded slice because LIFOStopRoutine should only stop in sequence
	var stopped []string
	r1 := NewMockRoutine()
	r1.StopFunc.PushHook(func(context.Context) error {
		stopped = append(stopped, "r1")
		return nil
	})
	r2 := NewMockRoutine()
	r2.StopFunc.PushHook(func(context.Context) error {
		stopped = append(stopped, "r2")
		return nil
	})
	r3 := NewMockRoutine()
	r3.StopFunc.PushHook(func(context.Context) error {
		stopped = append(stopped, "r3")
		return nil
	})

	r := LIFOStopRoutine{r1, r2, r3}
	err := r.Stop(context.Background())
	require.NoError(t, err)
	// stops in reverse
	assert.Equal(t, []string{"r3", "r2", "r1"}, stopped)
}

func TestFIFOStopRoutine(t *testing.T) {
	// use an unguarded slice because FIFOSTopRoutine should only stop in sequence
	var stopped []string
	r1 := NewMockRoutine()
	r1.StopFunc.PushHook(func(context.Context) error {
		stopped = append(stopped, "r1")
		return nil
	})
	r2 := NewMockRoutine()
	r2.StopFunc.PushHook(func(context.Context) error {
		stopped = append(stopped, "r2")
		return nil
	})
	r3 := NewMockRoutine()
	r3.StopFunc.PushHook(func(context.Context) error {
		stopped = append(stopped, "r3")
		return nil
	})

	r := FIFOSTopRoutine{r1, r2, r3}
	err := r.Stop(context.Background())
	require.NoError(t, err)
	// stops in order
	assert.Equal(t, []string{"r1", "r2", "r3"}, stopped)
}
