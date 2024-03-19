package events_test

import (
	"context"
	"encoding/json"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/telemetry-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/internal/pubsub/pubsubtest"
	telemetrygatewayv1 "github.com/sourcegraph/sourcegraph/internal/telemetrygateway/v1"
)

func TestPublish(t *testing.T) {
	done := make(chan struct{})
	memTopic := pubsubtest.NewMemoryTopicClient()

	// Emulate semi-random blockage to emulate concurrency
	var count atomic.Int32
	memTopic.PrePublishHook = func() {
		count.Add(1)
		if count.Load()%2 == 0 {
			<-done
		}
	}

	const concurrency = 50
	publisher, err := events.NewPublisherForStream(
		logtest.Scoped(t),
		memTopic,
		&telemetrygatewayv1.RecordEventsRequestMetadata{
			Identifier: &telemetrygatewayv1.Identifier{
				Identifier: &telemetrygatewayv1.Identifier_LicensedInstance{
					LicensedInstance: &telemetrygatewayv1.Identifier_LicensedInstanceIdentifier{},
				},
			},
		},
		events.PublishStreamOptions{
			ConcurrencyLimit: concurrency,
		})
	require.NoError(t, err)

	// Check the evaluated source
	assert.Equal(t, "licensed_instance", publisher.GetSourceName())

	events := make([]*telemetrygatewayv1.Event, concurrency)
	for i := range events {
		events[i] = &telemetrygatewayv1.Event{
			Id:        strconv.Itoa(i),
			Feature:   t.Name(),
			Action:    strconv.Itoa(i),
			Timestamp: timestamppb.Now(),
		}
	}

	// Collect sets of things we expect
	eventResults := make(map[string]bool)
	for _, e := range events {
		eventResults[e.Id] = false
	}
	publishedEvents := make(map[string]bool)
	for _, e := range events {
		publishedEvents[e.Id] = false
	}

	// Publish the events, blocking some goroutines for a bit
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()
	results := publisher.Publish(context.Background(), events)

	// Collect all the results we got
	for _, r := range results {
		eventResults[r.EventID] = true
	}

	// Collect all the messages we published
	for _, m := range memTopic.Messages {
		var payload map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(m.Data, &payload))

		var event struct {
			Id      string
			Feature string
			Action  string
		}
		require.NoError(t, json.Unmarshal(payload["event"], &event))
		publishedEvents[event.Id] = true

		assert.Equal(t, event.Feature, m.Attributes["event.feature"])
		assert.Equal(t, event.Action, m.Attributes["event.action"])
	}

	// Make our assertions - all events should be have results or be published
	for eventID, found := range eventResults {
		if !found {
			t.Errorf("expected event result %q", eventID)
		}
	}
	for eventID, published := range publishedEvents {
		if !published {
			t.Errorf("expected published event %q", eventID)
		}
	}
}
