package shared

import (
	"net"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func LoadConfig() *Config {
	var config Config
	config.Load()
	return &config
}

type Config struct {
	env.BaseConfig

	ListenAddress string

	SyncRepoStateInterval        time.Duration
	SyncRepoStateBatchSize       int
	SyncRepoStateUpdatePerSecond int

	JanitorReposDesiredPercentFree        int
	JanitorInterval                       time.Duration
	JanitorDisableDeleteReposOnWrongShard bool
}

func (c *Config) Load() {
	c.ListenAddress = c.GetOptional("GITSERVER_ADDR", "The address under which the gitserver API listens. Can include a port.")
	// Fall back to a reasonable default.
	if c.ListenAddress == "" {
		port := "3178"
		host := ""
		if env.InsecureDev {
			host = "127.0.0.1"
		}
		c.ListenAddress = net.JoinHostPort(host, port)
	}

	c.SyncRepoStateInterval = c.GetInterval("SRC_REPOS_SYNC_STATE_INTERVAL", "10m", "Interval between state syncs")
	c.SyncRepoStateBatchSize = c.GetInt("SRC_REPOS_SYNC_STATE_BATCH_SIZE", "500", "Number of updates to perform per batch")
	c.SyncRepoStateUpdatePerSecond = c.GetInt("SRC_REPOS_SYNC_STATE_UPSERT_PER_SEC", "500", "The number of updated rows allowed per second across all gitserver instances")

	// Align these variables with the 'disk_space_remaining' alerts in monitoring
	c.JanitorReposDesiredPercentFree = c.GetInt("SRC_REPOS_DESIRED_PERCENT_FREE", "10", "Target percentage of free space on disk.")
	if c.JanitorReposDesiredPercentFree < 0 {
		c.AddError(errors.Errorf("negative value given for SRC_REPOS_DESIRED_PERCENT_FREE: %d", c.JanitorReposDesiredPercentFree))
	}
	if c.JanitorReposDesiredPercentFree > 100 {
		c.AddError(errors.Errorf("excessively high value given for SRC_REPOS_DESIRED_PERCENT_FREE: %d", c.JanitorReposDesiredPercentFree))
	}

	c.JanitorInterval = c.GetInterval("SRC_REPOS_JANITOR_INTERVAL", "1m", "Interval between cleanup runs")
	c.JanitorDisableDeleteReposOnWrongShard = c.GetBool("SRC_REPOS_JANITOR_DISABLE_DELETE_REPOS_ON_WRONG_SHARD", "false", "Disable deleting repos on wrong shard")
}
