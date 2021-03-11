package reconciler

import (
	"context"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

// TODO: copy-pasted
func loadExternalService(ctx context.Context, esStore *database.ExternalServiceStore, repo *types.Repo) (*types.ExternalService, error) {
	es, err := esStore.List(ctx, database.ExternalServicesListOptions{IDs: repo.ExternalServiceIDs()})
	if err != nil {
		return nil, err
	}

	for _, e := range es {
		cfg, err := e.Configuration()
		if err != nil {
			return nil, err
		}

		switch cfg := cfg.(type) {
		case *schema.GitHubConnection:
			if cfg.Token != "" {
				return e, nil
			}
		case *schema.BitbucketServerConnection:
			if cfg.Token != "" {
				return e, nil
			}
		case *schema.GitLabConnection:
			if cfg.Token != "" {
				return e, nil
			}
		}
	}

	return nil, errors.Errorf("no external services found for repo %q", repo.Name)
}
