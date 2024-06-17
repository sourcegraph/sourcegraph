package resolvers

import (
	"context"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/githubapp"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	ghstore "github.com/sourcegraph/sourcegraph/internal/github_apps/store"
	ghtypes "github.com/sourcegraph/sourcegraph/internal/github_apps/types"
	"github.com/sourcegraph/sourcegraph/internal/types"
	itypes "github.com/sourcegraph/sourcegraph/internal/types"
)

type batchChangesCodeHostResolver struct {
	db         database.DB
	store      *store.Store
	codeHost   *btypes.CodeHost
	credential graphqlbackend.BatchChangesCredentialResolver
	logger     log.Logger
}

var _ graphqlbackend.BatchChangesCodeHostResolver = &batchChangesCodeHostResolver{}

func (c *batchChangesCodeHostResolver) ExternalServiceKind() string {
	return extsvc.TypeToKind(c.codeHost.ExternalServiceType)
}

func (c *batchChangesCodeHostResolver) ExternalServiceURL() string {
	return c.codeHost.ExternalServiceID
}

func (c *batchChangesCodeHostResolver) Credential() graphqlbackend.BatchChangesCredentialResolver {
	return c.credential
}

func (c *batchChangesCodeHostResolver) CommitSigningConfiguration(ctx context.Context) (graphqlbackend.CommitSigningConfigResolver, error) {
	switch c.codeHost.ExternalServiceType {
	case extsvc.TypeGitHub:
		gstore := ghstore.GitHubAppsWith(c.store.Store)
		domain := itypes.BatchesGitHubAppDomain
		ghapp, err := gstore.GetByDomainAndKind(ctx, domain, c.codeHost.ExternalServiceID, types.CommitSigningGitHubAppKind)
		if err != nil {
			if _, ok := err.(ghstore.ErrNoGitHubAppFound); ok {
				return nil, nil
			} else {
				return nil, err
			}
		}
		return &commitSigningConfigResolver{
			db:        c.db,
			githubApp: ghapp,
			logger:    c.logger,
		}, nil
	}
	return nil, nil
}

func (c *batchChangesCodeHostResolver) RequiresSSH() bool {
	return c.codeHost.RequiresSSH
}

func (c *batchChangesCodeHostResolver) RequiresUsername() bool {
	switch c.codeHost.ExternalServiceType {
	case extsvc.TypeBitbucketCloud, extsvc.TypeAzureDevOps, extsvc.TypeGerrit, extsvc.TypePerforce:
		return true
	}

	return false
}

func (c *batchChangesCodeHostResolver) SupportsCommitSigning() bool {
	return c.codeHost.ExternalServiceType == extsvc.TypeGitHub
}

func (c *batchChangesCodeHostResolver) HasWebhooks() bool {
	return c.codeHost.HasWebhooks
}

var _ graphqlbackend.CommitSigningConfigResolver = &commitSigningConfigResolver{}

type commitSigningConfigResolver struct {
	logger    log.Logger
	db        database.DB
	githubApp *ghtypes.GitHubApp
}

func (c *commitSigningConfigResolver) ToGitHubApp() (graphqlbackend.GitHubAppResolver, bool) {
	if c.githubApp != nil {
		return githubapp.NewGitHubAppResolver(c.db, c.githubApp, c.logger), true
	}

	return nil, false
}
