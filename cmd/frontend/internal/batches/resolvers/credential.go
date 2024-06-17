package resolvers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const batchChangesCredentialIDKind = "BatchChangesCredential"

const (
	siteCredentialPrefix = "site"
	userCredentialPrefix = "user"
)

func marshalBatchChangesCredentialID(id int64, isSiteCredential bool) graphql.ID {
	var idStr string
	if isSiteCredential {
		idStr = fmt.Sprintf("%s:%d", siteCredentialPrefix, id)
	} else {
		idStr = fmt.Sprintf("%s:%d", userCredentialPrefix, id)
	}
	return relay.MarshalID(batchChangesCredentialIDKind, idStr)
}

func unmarshalBatchChangesCredentialID(id graphql.ID) (credentialID int64, isSiteCredential bool, err error) {
	var strID string
	if err := relay.UnmarshalSpec(id, &strID); err != nil {
		return credentialID, isSiteCredential, err
	}

	parts := strings.SplitN(strID, ":", 2)
	if len(parts) != 2 {
		return credentialID, isSiteCredential, errors.New("invalid id")
	}

	kind := parts[0]
	switch strings.ToLower(kind) {
	case siteCredentialPrefix:
		isSiteCredential = true
	case userCredentialPrefix:
	default:
		return credentialID, isSiteCredential, errors.Errorf("invalid id, unsupported credential kind %q", kind)
	}

	parsedID, err := strconv.Atoi(parts[1])
	return int64(parsedID), isSiteCredential, err
}

func commentSSHKey(ssh auth.AuthenticatorWithSSH) string {
	url := globals.ExternalURL()
	if url != nil && url.Host != "" {
		return strings.TrimRight(ssh.SSHPublicKey(), "\n") + " Sourcegraph " + url.Host
	}
	return ssh.SSHPublicKey()
}

type batchChangesUserCredentialResolver struct {
	credential *database.UserCredential
}

var _ graphqlbackend.BatchChangesCredentialResolver = &batchChangesUserCredentialResolver{}

func (c *batchChangesUserCredentialResolver) ID() graphql.ID {
	return marshalBatchChangesCredentialID(c.credential.ID, false)
}

func (c *batchChangesUserCredentialResolver) ExternalServiceKind() string {
	return extsvc.TypeToKind(c.credential.ExternalServiceType)
}

func (c *batchChangesUserCredentialResolver) ExternalServiceURL() string {
	// This is usually the code host URL.
	return c.credential.ExternalServiceID
}

func (c *batchChangesUserCredentialResolver) SSHPublicKey(ctx context.Context) (*string, error) {
	a, err := c.credential.Authenticator(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving authenticator")
	}

	if ssh, ok := a.(auth.AuthenticatorWithSSH); ok {
		publicKey := commentSSHKey(ssh)
		return &publicKey, nil
	}
	return nil, nil
}

func (c *batchChangesUserCredentialResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: c.credential.CreatedAt}
}

func (c *batchChangesUserCredentialResolver) IsSiteCredential() bool {
	return false
}

func (c *batchChangesUserCredentialResolver) authenticator(ctx context.Context) (auth.Authenticator, error) {
	return c.credential.Authenticator(ctx, nil)
}

func (c *batchChangesUserCredentialResolver) IsGitHubApp() bool {
	return c.credential.GitHubAppID != 0
}

type batchChangesSiteCredentialResolver struct {
	credential *btypes.SiteCredential
}

func (c *batchChangesUserCredentialResolver) GitHubAppID() int {
	return c.credential.GitHubAppID
}

var _ graphqlbackend.BatchChangesCredentialResolver = &batchChangesSiteCredentialResolver{}

func (c *batchChangesSiteCredentialResolver) ID() graphql.ID {
	return marshalBatchChangesCredentialID(c.credential.ID, true)
}

func (c *batchChangesSiteCredentialResolver) ExternalServiceKind() string {
	return extsvc.TypeToKind(c.credential.ExternalServiceType)
}

func (c *batchChangesSiteCredentialResolver) ExternalServiceURL() string {
	// This is usually the code host URL.
	return c.credential.ExternalServiceID
}

func (c *batchChangesSiteCredentialResolver) SSHPublicKey(ctx context.Context) (*string, error) {
	a, err := c.credential.Authenticator(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "decrypting authenticator")
	}

	if ssh, ok := a.(auth.AuthenticatorWithSSH); ok {
		publicKey := commentSSHKey(ssh)
		return &publicKey, nil
	}
	return nil, nil
}

func (c *batchChangesSiteCredentialResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: c.credential.CreatedAt}
}

func (c *batchChangesSiteCredentialResolver) IsSiteCredential() bool {
	return true
}

func (c *batchChangesSiteCredentialResolver) authenticator(ctx context.Context) (auth.Authenticator, error) {
	return c.credential.Authenticator(ctx)
}

func (c *batchChangesSiteCredentialResolver) IsGitHubApp() bool {
	return false
}

func (c *batchChangesSiteCredentialResolver) GitHubAppID() int {
	return 0
}
