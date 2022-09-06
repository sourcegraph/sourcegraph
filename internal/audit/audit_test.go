package audit

import (
	"context"
	"testing"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
)

func TestLog(t *testing.T) {
	ctx := context.Background()
	ctx = actor.WithActor(ctx, &actor.Actor{UID: 1})

	testCases := []struct {
		name              string
		client            *requestclient.Client
		additionalContext []log.Field
		expectedEntry     map[string]interface{}
	}{
		{
			name: "fully populated audit data",
			client: &requestclient.Client{
				IP:           "192.168.0.1",
				ForwardedFor: "192.168.0.1",
			},
			additionalContext: []log.Field{log.String("additional", "stuff")},
			expectedEntry: map[string]interface{}{
				"audit":        "true",
				"audit.entity": "test entity",
				"audit.actor": map[string]interface{}{
					"actorUID":        "1",
					"ip":              "192.168.0.1",
					"X-Forwarded-For": "192.168.0.1",
				},
				"additional": "stuff",
			},
		},
		{
			name:              "missing client info",
			client:            nil,
			additionalContext: []log.Field{log.String("additional", "stuff")},
			expectedEntry: map[string]interface{}{
				"audit":        "true",
				"audit.entity": "test entity",
				"audit.actor": map[string]interface{}{
					"actorUID":        "1",
					"ip":              "unknown",
					"X-Forwarded-For": "unknown",
				},
				"additional": "stuff",
			},
		},
		{
			name: "no additional context",
			client: &requestclient.Client{
				IP:           "192.168.0.1",
				ForwardedFor: "192.168.0.1",
			},
			additionalContext: nil,
			expectedEntry: map[string]interface{}{
				"audit":        "true",
				"audit.entity": "test entity",
				"audit.actor": map[string]interface{}{
					"actorUID":        "1",
					"ip":              "192.168.0.1",
					"X-Forwarded-For": "192.168.0.1",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = actor.WithActor(ctx, &actor.Actor{UID: 1})
			ctx = requestclient.WithClient(ctx, tc.client)

			fields := &Record{
				Entity: "test entity",
				Action: "test audit action",
				Fields: tc.additionalContext,
			}

			logger, exportLogs := logtest.Captured(t)

			Log(logger, ctx, fields)

			logs := exportLogs()
			if len(logs) != 1 {
				t.Fatal("expected to capture one log exactly")
			}

			assert.Equal(t, "test audit action", logs[0].Message)
			assert.Equal(t, tc.expectedEntry, logs[0].Fields)
		})
	}
}
