package shared

import (
	"net"
	"strconv"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/lsifuploadstore"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type IndexingWorkerConfig struct {
	env.BaseConfig
	PollInterval          time.Duration
	Concurrency           int
	MaximumRuntimePerJob  time.Duration
	CliPath               string
	LSIFUploadStoreConfig *lsifuploadstore.Config
}

type Config struct {
	env.BaseConfig

	IndexingWorkerConfig *IndexingWorkerConfig

	ListenAddress string
}

const DefaultPort = 3288

func (c *IndexingWorkerConfig) Load() {
	c.LSIFUploadStoreConfig = &lsifuploadstore.Config{}
	c.LSIFUploadStoreConfig.Load()

	c.PollInterval = c.GetInterval("SYNTACTIC_CODE_INTEL_INDEXING_POLL_INTERVAL", "1s", "Interval between queries to the repository queue")
	c.Concurrency = c.GetInt("SYNTACTIC_CODE_INTEL_INDEXING_CONCURRENCY", "1", "The maximum number of repositories that can be processed concurrently.")
	c.MaximumRuntimePerJob = c.GetInterval("SYNTACTIC_CODE_INTEL_INDEXING_MAXIMUM_RUNTIME_PER_JOB", "5m", "The maximum time a single repository indexing job can take")
	c.CliPath = c.Get("SCIP_SYNTAX_PATH", "scip-syntax", "TODO: fill in description")
}

func (c *Config) Load() {
	c.IndexingWorkerConfig = &IndexingWorkerConfig{}
	c.IndexingWorkerConfig.Load()
	c.ListenAddress = c.GetOptional("SYNTACTIC_CODE_INTEL_WORKER_ADDR", "The address under which the syntactic codeintel worker API listens. Can include a port.")
	// Fall back to a reasonable default.
	if c.ListenAddress == "" {
		port := strconv.Itoa(DefaultPort)
		host := ""
		if env.InsecureDev {
			host = "127.0.0.1"
		}
		c.ListenAddress = net.JoinHostPort(host, port)
	}
}

func (c *Config) Validate() error {
	var errs error
	errs = errors.Append(errs, c.BaseConfig.Validate())
	errs = errors.Append(errs, c.IndexingWorkerConfig.Validate())
	return errs
}
