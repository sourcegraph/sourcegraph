package jobselector

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/shared"
)

type InferenceService interface {
	InferIndexJobs(ctx context.Context, repo api.RepoID, commit, overrideScript string) (*shared.InferenceResult, error)
}
