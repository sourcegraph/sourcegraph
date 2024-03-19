package events

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"google.golang.org/protobuf/types/known/structpb"

	telemetrygatewayv1 "github.com/sourcegraph/sourcegraph/internal/telemetrygateway/v1"
)

func TestExtractPubSubAttributes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		event *telemetrygatewayv1.Event

		expect autogold.Value
	}{
		{
			name: "basic",
			event: &telemetrygatewayv1.Event{
				Feature: "cody.feature",
				Action:  "chat",
			},
			expect: autogold.Expect(map[string]string{
				"event.action": "chat", "event.feature": "cody.feature",
				"event.hasPrivateMetadata": "false",
				"publisher.source":         "licensed_instance",
			}),
		},
		{
			name: "has privateMetadata",
			event: &telemetrygatewayv1.Event{
				Feature: "cody.feature",
				Action:  "chat",
				Parameters: &telemetrygatewayv1.EventParameters{
					PrivateMetadata: &structpb.Struct{
						Fields: map[string]*structpb.Value{},
					},
				},
			},
			expect: autogold.Expect(map[string]string{
				"event.action": "chat", "event.feature": "cody.feature",
				"event.hasPrivateMetadata": "true",
				"publisher.source":         "licensed_instance",
			}),
		},
		{
			name: "recordsPrivateMetadataTranscript",
			event: &telemetrygatewayv1.Event{
				Feature: "cody.feature",
				Action:  "chat",
				Parameters: &telemetrygatewayv1.EventParameters{
					Metadata: map[string]float64{
						"recordsPrivateMetadataTranscript": 1,
					},
					PrivateMetadata: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"transcript": structpb.NewStringValue("foobar"),
						},
					},
				},
			},
			expect: autogold.Expect(map[string]string{
				"event.action": "chat", "event.feature": "cody.feature",
				"event.hasPrivateMetadata":               "true",
				"event.recordsPrivateMetadataTranscript": "true",
				"publisher.source":                       "licensed_instance",
			}),
		},
		{
			name: "metadata that don't get added to attributes",
			event: &telemetrygatewayv1.Event{
				Feature: "cody.feature",
				Action:  "chat",
				Parameters: &telemetrygatewayv1.EventParameters{
					Metadata: map[string]float64{
						"anotherMetadata":  3,
						"recordTranscript": 1,
					},
				},
			},
			expect: autogold.Expect(map[string]string{
				"event.action": "chat", "event.feature": "cody.feature",
				"event.hasPrivateMetadata": "false",
				"publisher.source":         "licensed_instance",
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.expect.Equal(t, extractPubSubAttributes("licensed_instance", tc.event))
		})
	}
}
