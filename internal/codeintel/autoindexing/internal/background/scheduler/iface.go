package scheduler

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/policies"
	policiesshared "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
)

type PolicyMatcher interface {
	CommitsDescribedByPolicy(ctx context.Context, repositoryID int, repoName api.RepoName, policies []policiesshared.ConfigurationPolicy, now time.Time, filterCommits ...string) (map[string][]policies.PolicyMatch, error)
}

type PoliciesService interface {
	GetConfigurationPolicies(ctx context.Context, opts policiesshared.GetConfigurationPoliciesOptions) ([]policiesshared.ConfigurationPolicy, int, error)
}

type IndexEnqueuer interface {
	QueueIndexes(ctx context.Context, repositoryID int, rev, configuration string, force, bypassLimit bool) (_ []uploadsshared.Index, err error)
	QueueIndexesForPackage(ctx context.Context, pkg dependencies.MinimialVersionedPackageRepo) (err error)
}
