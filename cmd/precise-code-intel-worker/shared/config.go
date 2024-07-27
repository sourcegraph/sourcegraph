package shared

import (
	"runtime"
	"strconv"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/lsifuploadstore"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Config struct {
	env.BaseConfig

	WorkerPollInterval    time.Duration
	WorkerConcurrency     int
	WorkerBudget          int64
	MaximumRuntimePerJob  time.Duration
	LSIFUploadStoreConfig *lsifuploadstore.Config
}

func (c *Config) Load() {
	c.LSIFUploadStoreConfig = &lsifuploadstore.Config{}
	c.LSIFUploadStoreConfig.Load()

	c.WorkerPollInterval = c.GetInterval("PRECISE_CODE_INTEL_WORKER_POLL_INTERVAL", "1s", "Interval between queries to the upload queue.")
	// If the worker has multiple cores available, let's make sure we're making good use of those.
	// As of 2024 July 15, I/O takes about 2x the time as CPU processing,
	// (see https://github.com/sourcegraph/sourcegraph/pull/61826)
	// so try to spin up more goroutines since we're not worried about
	// context switching overhead.
	c.WorkerConcurrency = c.GetInt("PRECISE_CODE_INTEL_WORKER_CONCURRENCY", strconv.Itoa(2*runtime.GOMAXPROCS(-1)), "The maximum number of indexes that can be processed concurrently.")
	c.WorkerBudget = int64(c.GetInt("PRECISE_CODE_INTEL_WORKER_BUDGET", "0", "The amount of compressed input data (in bytes) a worker can process concurrently. Zero acts as an infinite budget."))
	c.MaximumRuntimePerJob = c.GetInterval("PRECISE_CODE_INTEL_WORKER_MAXIMUM_RUNTIME_PER_JOB", "25m", "The maximum time a single LSIF processing job can take.")
}

func (c *Config) Validate() error {
	var errs error
	errs = errors.Append(errs, c.BaseConfig.Validate())
	errs = errors.Append(errs, c.LSIFUploadStoreConfig.Validate())
	return errs
}
