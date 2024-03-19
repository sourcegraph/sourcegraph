package limiter

import (
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	t.Run("zero value does not block", func(t *testing.T) {
		var l Limiter
		for range 100 {
			l.Acquire()
		}
		// if we blocked, this test would time out
	})

	t.Run("acquire and release does not block", func(t *testing.T) {
		l := New(1)
		for range 100 {
			l.Acquire()
			l.Release()
		}
		// if we blocked, this test would time out
	})

	t.Run("full limiter blocks", func(t *testing.T) {
		l := New(1)
		l.Acquire() // does not block

		done := make(chan struct{})
		go func() {
			defer close(done)
			l.Acquire() // should block forever
		}()

		select {
		case <-done:
			t.Fatal("expected acquire to block forever")
		case <-time.After(10 * time.Millisecond):
		}

		l.Release()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("expected release to unblock Acquire")
		}
	})
}
