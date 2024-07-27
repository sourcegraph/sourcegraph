package main

import (
	"encoding/json"

	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
)

const DefaultChannel = "#william-buildchecker-webhook-test"

type Config struct {
	BigQueryProjectID   string
	BigQueryDatasetID   string
	SlackToken          string
	TeamChannelMapping  map[string]string
	LookbackWindowWeeks int
}

func (c *Config) Load(env *runtime.Env) {
	c.SlackToken = env.Get("SLACK_TOKEN", "", "")
	c.BigQueryProjectID = env.Get("BIGQUERY_PROJECT_ID", "", "")
	c.BigQueryDatasetID = env.Get("BIGQUERY_DATASET_ID", "", "")
	c.LookbackWindowWeeks = env.GetInt("LOOKBACK_WINDOW_WEEKS", "2", "")

	c.TeamChannelMapping = make(map[string]string)
	if err := json.Unmarshal([]byte(env.Get("TEAM_CHANNEL_MAPPING", "{}", "")), &c.TeamChannelMapping); err != nil {
		env.AddError(err)
	}
}
