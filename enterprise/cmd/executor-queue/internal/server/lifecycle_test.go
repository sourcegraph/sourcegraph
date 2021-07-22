package server

import (
	"context"
	"testing"
	"time"

	"github.com/derision-test/glock"
	"github.com/google/go-cmp/cmp"

	apiclient "github.com/sourcegraph/sourcegraph/enterprise/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	workerstoremocks "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store/mocks"
)

func TestHeartbeat(t *testing.T) {
	store1 := workerstoremocks.NewMockStore()
	recordTransformer := func(ctx context.Context, record workerutil.Record) (apiclient.Job, error) {
		return apiclient.Job{ID: record.RecordID()}, nil
	}

	store1.DequeueFunc.PushReturn(testRecord{ID: 41}, true, nil)
	store1.DequeueFunc.PushReturn(testRecord{ID: 42}, true, nil)
	store1.DequeueFunc.PushReturn(testRecord{ID: 43}, true, nil)
	store1.DequeueFunc.PushReturn(testRecord{ID: 44}, true, nil)

	clock := glock.NewMockClock()
	handler := newHandler(Options{UnreportedMaxAge: time.Second}, QueueOptions{Store: store1, RecordTransformer: recordTransformer}, "q1", clock)

	_, dequeued1, _ := handler.dequeue(context.Background(), "deadbeef", "test")
	_, dequeued2, _ := handler.dequeue(context.Background(), "deadveal", "test")
	_, dequeued3, _ := handler.dequeue(context.Background(), "deadbeef", "test")
	_, dequeued4, _ := handler.dequeue(context.Background(), "deadveal", "test")
	if !dequeued1 || !dequeued2 || !dequeued3 || !dequeued4 {
		t.Fatalf("failed to dequeue records")
	}

	// missing all jobs, but they're less than UnreportedMaxAge
	clock.Advance(time.Second / 2)
	if _, err := handler.heartbeat(context.Background(), "deadbeef", []int{}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}
	if _, err := handler.heartbeat(context.Background(), "deadveal", []int{}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}

	// missing no jobs
	clock.Advance(time.Minute * 2)
	if _, err := handler.heartbeat(context.Background(), "deadbeef", []int{41, 43}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}
	if _, err := handler.heartbeat(context.Background(), "deadveal", []int{42, 44}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}

	// missing one deadbeef jobs
	clock.Advance(time.Minute * 2)
	if _, err := handler.heartbeat(context.Background(), "deadbeef", []int{41}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}
	if _, err := handler.heartbeat(context.Background(), "deadveal", []int{42, 44}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}

	// missing two deadveal jobs
	clock.Advance(time.Minute * 2)
	if _, err := handler.heartbeat(context.Background(), "deadbeef", []int{41}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}
	if _, err := handler.heartbeat(context.Background(), "deadveal", []int{}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	}

	// unknown jobs
	clock.Advance(time.Minute * 2)
	if unknownIDs, err := handler.heartbeat(context.Background(), "deadbeef", []int{41, 43, 45}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	} else if diff := cmp.Diff([]int{43, 45}, unknownIDs); diff != "" {
		t.Errorf("unexpected unknown ids (-want +got):\n%s", diff)
	}
	if unknownIDs, err := handler.heartbeat(context.Background(), "deadveal", []int{42, 44, 45}); err != nil {
		t.Fatalf("unexpected error performing heartbeat: %s", err)
	} else if diff := cmp.Diff([]int{42, 44, 45}, unknownIDs); diff != "" {
		t.Errorf("unexpected unknown ids (-want +got):\n%s", diff)
	}
}
