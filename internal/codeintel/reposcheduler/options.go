package reposcheduler

import "time"

type RepositoryBatchOptions struct {
	// Time to wait before making repository visible again
	// after previous scan. Note that this delay can be superseded if repository
	// has been changed in the meantime
	ProcessDelay time.Duration

	// If allowGlobalPolicies is false, then configuration policies that do not specify a repository name
	// or patterns will be ignored.
	// When true, such policies apply over all repositories known to the instance.
	AllowGlobalPolicies bool

	// This optional limit controls how many repositores will be considered by matching
	// via global policy. As global policy matches large sets of repositories,
	// this limit allows reducing the number of repositories that will be considered
	// for scanning.
	GlobalPolicyRepositoriesMatchLimit *int

	// The maximum number repositories that will be returned in a batch
	Limit int
}

func NewBatchOptions(processDelay time.Duration, allowGlobalPolicies bool, repositoryMatchLimit *int, limit int) RepositoryBatchOptions {
	return RepositoryBatchOptions{
		ProcessDelay:                       processDelay,
		AllowGlobalPolicies:                allowGlobalPolicies,
		GlobalPolicyRepositoriesMatchLimit: repositoryMatchLimit,
		Limit:                              limit,
	}
}
