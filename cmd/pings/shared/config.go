package shared

import (
	"github.com/sourcegraph/sourcegraph/internal/env"
)

type Config struct {
	env.BaseConfig

	Port int

	PubSub struct {
		ProjectID string
		TopicID   string
	}
}

func (c *Config) Load() {
	c.Port = c.GetInt("PORT", "10086", "Port to serve Pings service on, generally injected by Cloud Run.")
	c.PubSub.ProjectID = c.Get("PINGS_PUBSUB_PROJECT_ID", "", "The project ID for the Pub/Sub.")
	c.PubSub.TopicID = c.Get("PINGS_PUBSUB_TOPIC_ID", "", "The topic ID for the Pub/Sub.")
}
