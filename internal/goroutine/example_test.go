package goroutine

import (
	"context"
	"fmt"
	"time"
)

type exampleRoutine struct {
	done chan struct{}
}

func (m *exampleRoutine) Start() {
	for {
		select {
		case <-m.done:
			fmt.Println("done!")
			return
		default:
		}

		fmt.Println("Hello there!")
		time.Sleep(200 * time.Millisecond)
	}
}

func (m *exampleRoutine) Stop() {
	m.done <- struct{}{}
}

func ExampleBackgroundRoutine() {
	r := &exampleRoutine{
		done: make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())

	go MonitorBackgroundRoutines(ctx, r)

	time.Sleep(500 * time.Millisecond)
	cancel()
	time.Sleep(200 * time.Millisecond)
}

func ExamplePeriodicGoroutine() {
	h := NewHandlerWithErrorMessage("example background routine", func(ctx context.Context) error {
		fmt.Println("Hello from the background!")
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())

	r := NewPeriodicGoroutine(ctx, 200*time.Millisecond, h)

	go MonitorBackgroundRoutines(ctx, r)

	time.Sleep(500 * time.Millisecond)
	cancel()
	time.Sleep(200 * time.Millisecond)
}
