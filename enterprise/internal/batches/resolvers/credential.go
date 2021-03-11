package resolvers

import (
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
)

const batchChangesCredentialIDKind = "BatchChangesCredential"

func marshalBatchChangesCredentialID(id int64) graphql.ID {
	return relay.MarshalID(batchChangesCredentialIDKind, id)
}

func unmarshalBatchChangesCredentialID(id graphql.ID) (cid int64, err error) {
	err = relay.UnmarshalSpec(id, &cid)
	return
}

type batchChangesCredentialResolver struct {
	credential *database.UserCredential
}

var _ graphqlbackend.BatchChangesCredentialResolver = &batchChangesCredentialResolver{}

func (c *batchChangesCredentialResolver) ID() graphql.ID {
	return marshalBatchChangesCredentialID(c.credential.ID)
}

func (c *batchChangesCredentialResolver) ExternalServiceKind() string {
	return extsvc.TypeToKind(c.credential.ExternalServiceType)
}

func (c *batchChangesCredentialResolver) ExternalServiceURL() string {
	// This is usually the code host URL.
	return c.credential.ExternalServiceID
}

func (c *batchChangesCredentialResolver) SSHPublicKey() *string {
	if a, ok := c.credential.Credential.(auth.AuthenticatorWithSSH); ok {
		publicKey := a.SSHPublicKey()
		return &publicKey
	}
	return nil
}

func (c *batchChangesCredentialResolver) CreatedAt() graphqlbackend.DateTime {
	return graphqlbackend.DateTime{Time: c.credential.CreatedAt}
}
