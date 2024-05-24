package shared

import (
	"net"
	"path/filepath"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/hostname"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func LoadConfig() *Config {
	var config Config
	config.Load()
	return &config
}

type Config struct {
	env.BaseConfig

	ReposDir         string
	CoursierCacheDir string

	// ExternalAddress is the name of this gitserver as it would appear in
	// SRC_GIT_SERVERS.
	//
	// Note: we can't just rely on the listen address since more than likely
	// gitserver is behind a k8s service.
	ExternalAddress string

	ListenAddress string

	SyncRepoStateInterval        time.Duration
	SyncRepoStateBatchSize       int
	SyncRepoStateUpdatePerSecond int

	JanitorReposDesiredPercentFree        int
	JanitorInterval                       time.Duration
	JanitorDisableDeleteReposOnWrongShard bool

	EnableExperimentalJanitor      bool
	ExperimentalJanitorConcurrency int

	ExhaustiveRequestLoggingEnabled bool
}

func (c *Config) Load() {
	c.ReposDir = c.Get("SRC_REPOS_DIR", "/data/repos", "Root dir containing repos.")
	if c.ReposDir == "" {
		c.AddError(errors.New("SRC_REPOS_DIR is required"))
	}

	// if COURSIER_CACHE_DIR is set, try create that dir and use it. If not set, use the SRC_REPOS_DIR value (or default).
	c.CoursierCacheDir = c.GetOptional("COURSIER_CACHE_DIR", "Directory in which coursier data is cached for JVM package repos.")
	if c.CoursierCacheDir == "" && c.ReposDir != "" {
		c.CoursierCacheDir = filepath.Join(c.ReposDir, "coursier")
	}

	// First we check for it being explicitly set. This should only be
	// happening in environments were we run gitserver on localhost.
	// Otherwise we assume we can reach gitserver via its hostname / its
	// hostname is a prefix of the reachable address (see hostnameMatch).
	c.ExternalAddress = c.Get("GITSERVER_EXTERNAL_ADDR", hostname.Get(), "The name of this gitserver as it would appear in SRC_GIT_SERVERS.")

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

	c.ExhaustiveRequestLoggingEnabled = c.GetBool("SRC_GITSERVER_EXHAUSTIVE_LOGGING_ENABLED", "false", "Enable exhaustive request logging in gitserver")

	c.EnableExperimentalJanitor = c.GetBool("SRC_GITSERVER_ENABLE_EXPERIMENTAL_JANITOR", "false", "Enable experimental janitor. DO NOT USE THIS IN PRODUCTION, IT MIGHT CORRUPT ALL REPOS AND RECOVERY WILL REQUIRE A FULL RECLONE OF ALL REPOS.")
	c.ExperimentalJanitorConcurrency = c.GetInt("SRC_GITSERVER_EXPERIMENTAL_JANITOR_CONCURRENCY", "1", "Concurrency of experimental janitor, up to N repos will be optimized in parallel.")
	// TODO: Verify that ExperimentalJanitorConcurrency is > 0.
}
