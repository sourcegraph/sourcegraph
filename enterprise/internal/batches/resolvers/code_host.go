package resolvers

import (
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
)

type batchChangesCodeHostResolver struct {
	codeHost   *batches.CodeHost
	credential *database.UserCredential
}

var _ graphqlbackend.BatchChangesCodeHostResolver = &batchChangesCodeHostResolver{}

func (c *batchChangesCodeHostResolver) ExternalServiceKind() string {
	return extsvc.TypeToKind(c.codeHost.ExternalServiceType)
}

func (c *batchChangesCodeHostResolver) ExternalServiceURL() string {
	return c.codeHost.ExternalServiceID
}

func (c *batchChangesCodeHostResolver) Credential() graphqlbackend.BatchChangesCredentialResolver {
	if c.credential != nil {
		return &batchChangesCredentialResolver{credential: c.credential}
	}
	return nil
}

func (c *batchChangesCodeHostResolver) RequiresSSH() bool {
	return c.codeHost.RequiresSSH
}
