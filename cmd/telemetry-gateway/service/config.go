package service

import (
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
)

type Config struct {
	Events struct {
		PubSub struct {
			Enabled   bool
			ProjectID *string
			TopicID   *string
		}

		StreamPublishConcurrency int
	}

	SAMS struct {
		ServerURL    string
		ClientID     string
		ClientSecret string
	}
}

func (c *Config) Load(env *runtime.Env) {
	c.Events.PubSub.Enabled = env.GetBool("TELEMETRY_GATEWAY_EVENTS_PUBSUB_ENABLED", "true",
		"If false, logs Pub/Sub messages instead of actually sending them")
	c.Events.PubSub.ProjectID = env.GetOptional("TELEMETRY_GATEWAY_EVENTS_PUBSUB_PROJECT_ID",
		"The project ID for the Pub/Sub.")
	c.Events.PubSub.TopicID = env.GetOptional("TELEMETRY_GATEWAY_EVENTS_PUBSUB_TOPIC_ID",
		"The topic ID for the Pub/Sub.")
	c.Events.StreamPublishConcurrency = env.GetInt("TELEMETRY_GATEWAY_EVENTS_STREAM_PUBLISH_CONCURRENCY", "250",
		"Per-stream concurrent publishing limit.")

	c.SAMS.ServerURL = env.Get("TELEMETRY_GATEWAY_SAMS_SERVER_URL", "https://accounts.sourcegraph.com",
		"Sourcegraph Accounts Management System URL")
	c.SAMS.ClientID = env.Get("TELEMETRY_GATEWAY_SAMS_CLIENT_ID", "",
		"Sourcegraph Accounts Management System client ID")
	c.SAMS.ClientSecret = env.Get("TELEMETRY_GATEWAY_SAMS_CLIENT_SECRET", "",
		"Sourcegraph Accounts Management System client secret")
}
