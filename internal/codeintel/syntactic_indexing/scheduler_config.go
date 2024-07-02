package syntactic_indexing

import (
	"time"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

type SchedulerConfig struct {
	env.BaseConfig

	SchedulerInterval      time.Duration
	RepositoryProcessDelay time.Duration
	RepositoryBatchSize    int
	PolicyBatchSize        int
}

func (c *SchedulerConfig) Load() {
	intervalName := env.ChooseFallbackVariableName("CODEINTEL_SYNTACTIC_INDEXING_SCHEDULER_INTERVAL")
	repositoryProcessDelayName := env.ChooseFallbackVariableName("CODEINTEL_SYNTACTIC_INDEXING_SCHEDULER_REPOSITORY_PROCESS_DELAY")
	repositoryBatchSizeName := env.ChooseFallbackVariableName("CODEINTEL_SYNTACTIC_INDEXING_SCHEDULER_REPOSITORY_BATCH_SIZE")
	policyBatchSizeName := env.ChooseFallbackVariableName("CODEINTEL_SYNTACTIC_INDEXING_SCHEDULER_POLICY_BATCH_SIZE")

	c.SchedulerInterval = c.GetInterval(intervalName, "5s" /* TODO!!!! revert to 2m*/, "How frequently to run the auto-indexing scheduling routine.")
	c.RepositoryProcessDelay = c.GetInterval(repositoryProcessDelayName, "24h", "The minimum frequency that the same repository can be considered for auto-index scheduling.")
	c.RepositoryBatchSize = c.GetInt(repositoryBatchSizeName, "2500", "The number of repositories to consider for auto-indexing scheduling at a time.")
	c.PolicyBatchSize = c.GetInt(policyBatchSizeName, "100", "The number of policies to consider for auto-indexing scheduling at a time.")
}
