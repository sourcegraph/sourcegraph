package internal

import (
	"context"
)

// Limiter implements a counting semaphore.
type Limiter struct {
	concurrency int
	ch          chan struct{}
}

// NewLimiter creates a new limiter with the given maximum concurrency.
func NewLimiter(concurrency int) *Limiter {
	ch := make(chan struct{}, concurrency)
	for range concurrency {
		ch <- struct{}{}
	}

	return &Limiter{concurrency, ch}
}

// Acquire blocks until it can acquire a value from the inner channel.
func (l *Limiter) Acquire(ctx context.Context) error {
	select {
	case <-l.ch:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release adds a value back to the limiter, unblocking one waiter.
func (l *Limiter) Release() {
	l.ch <- struct{}{}
}

// Close closes the underlying channel.
func (l *Limiter) Close() {
	// Drain the channel before close
	for range l.concurrency {
		<-l.ch
	}

	close(l.ch)
}
