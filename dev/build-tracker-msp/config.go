package main

import (
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
)

const DefaultChannel = "#william-buildchecker-webhook-test"

type Config struct {
	BuildkiteToken string
	SlackToken     string
	GithubToken    string
	SlackChannel   string
	Production     bool
	DebugPassword  string
}

func (c *Config) Load(env *runtime.Env) {
	c.BuildkiteToken = env.Get("BUILDKITE_WEBHOOK_TOKEN", "", "")
	c.SlackToken = env.Get("SLACK_TOKEN", "", "")
	c.GithubToken = env.Get("GITHUB_TOKEN", "", "")
	c.SlackChannel = env.Get("SLACK_CHANNEL", DefaultChannel, "")
	c.Production = env.GetBool("BUILDTRACKER_PRODUCTION", "false", "")

	if c.Production {
		c.DebugPassword = env.Get("BUIDLTRACKER_DEBUG_PASSWORD", "", "")
	}
}
