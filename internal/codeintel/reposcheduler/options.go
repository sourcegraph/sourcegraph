package reposcheduler

import "time"

type RepositoryBatchOptions struct {
	ProcessDelay         time.Duration
	AllowGlobalPolicies  bool
	RepositoryMatchLimit *int
	Limit                int
}

func NewBatchOptions(processDelay time.Duration, allowGlobalPolicies bool, repositoryMatchLimit *int, limit int) RepositoryBatchOptions {
	return RepositoryBatchOptions{
		ProcessDelay:         processDelay,
		AllowGlobalPolicies:  allowGlobalPolicies,
		RepositoryMatchLimit: repositoryMatchLimit,
		Limit:                limit,
	}
}
