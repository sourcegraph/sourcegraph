package syncer

import (
	"context"
	"sort"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

// loadExternalService looks up all external services that are connected to the given repo.
// The first external service to have a token configured will be returned then.
// If no external service matching the above criteria is found, an error is returned.
func loadExternalService(ctx context.Context, esStore ExternalServiceStore, repo *types.Repo) (*types.ExternalService, error) {
	es, err := esStore.List(ctx, database.ExternalServicesListOptions{
		// Consider all available external services for this repo.
		IDs: repo.ExternalServiceIDs(),
	})
	if err != nil {
		return nil, err
	}

	// Sort the external services so user owned external service go last.
	// This also retains the initial ORDER BY ID DESC.
	sort.SliceStable(es, func(i, j int) bool {
		return es[i].NamespaceUserID == 0 && es[i].ID > es[j].ID
	})

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
