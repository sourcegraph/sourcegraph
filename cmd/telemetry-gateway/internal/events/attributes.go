package events

import (
	"strconv"
	"strings"

	telemetrygatewayv1 "github.com/sourcegraph/sourcegraph/internal/telemetrygateway/v1"
)

// extractPubSubAttributes extracts attributes from the event for use in the
// published pub/sub message as attributes. This makes it easiser to build
// routing of events in our data pipelines.
func extractPubSubAttributes(publisherSource string, event *telemetrygatewayv1.Event) map[string]string {
	attributes := map[string]string{
		"publisher.source": publisherSource,
		"event.feature":    event.Feature,
		"event.action":     event.Action,
		"event.hasPrivateMetadata": strconv.FormatBool(
			event.GetParameters().GetPrivateMetadata() != nil),
	}

	// Metadata that has 'recordsPrivateMetadata' as a prefix, e.g.
	// 'recordsPrivateMetadataTranscript' or 'recordsPrivateMetadataSearch', is
	// used to indicate if the event's private metadata (if exported) carries
	// special data for processing in our data pipelines.
	const recordsSpecialDataPrefix = "recordsPrivateMetadata"
	for k, v := range event.GetParameters().GetMetadata() {
		if strings.HasPrefix(k, recordsSpecialDataPrefix) &&
			// There must be more to the name after the prefix
			len(k) > len(recordsSpecialDataPrefix)+1 {
			if v > 0 {
				attributes["event."+k] = "true"
			}
		}
	}

	return attributes
}
