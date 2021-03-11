package api

import (
	"context"
	"errors"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func init() {
	graphqlbackend.NodeToRegistryExtension = func(node interface{}) (graphqlbackend.RegistryExtension, bool) {
		switch n := node.(type) {
		case *registryExtensionRemoteResolver:
			return n, true
		case graphqlbackend.RegistryExtension:
			return n, true
		default:
			return nil, false
		}
	}

	graphqlbackend.RegistryExtensionByID = registryExtensionByID
}

// RegistryExtensionID identifies a registry extension, either locally or on a remote
// registry. Exactly 1 field must be set.
type RegistryExtensionID struct {
	LocalID  int32                      `json:"l,omitempty"`
	RemoteID *registryExtensionRemoteID `json:"r,omitempty"`
}

func MarshalRegistryExtensionID(id RegistryExtensionID) graphql.ID {
	return relay.MarshalID("RegistryExtension", id)
}

func UnmarshalRegistryExtensionID(id graphql.ID) (registryExtensionID RegistryExtensionID, err error) {
	err = relay.UnmarshalSpec(id, &registryExtensionID)
	return
}

// RegistryExtensionByIDInt32 looks up and returns the registry extension in the database with the
// given ID. If no such extension exists, an error is returned. The func is nil when there is no
// local registry.
var RegistryExtensionByIDInt32 func(context.Context, dbutil.DB, int32) (graphqlbackend.RegistryExtension, error)

func registryExtensionByID(ctx context.Context, db dbutil.DB, id graphql.ID) (graphqlbackend.RegistryExtension, error) {
	registryExtensionID, err := UnmarshalRegistryExtensionID(id)
	if err != nil {
		return nil, err
	}
	switch {
	case registryExtensionID.LocalID != 0 && RegistryExtensionByIDInt32 != nil:
		return RegistryExtensionByIDInt32(ctx, db, registryExtensionID.LocalID)
	case registryExtensionID.RemoteID != nil:
		x, err := getRemoteRegistryExtension(ctx, "uuid", registryExtensionID.RemoteID.UUID)
		if err != nil {
			return nil, err
		}
		return &registryExtensionRemoteResolver{v: x}, nil
	default:
		return nil, errors.New("invalid registry extension ID")
	}
}

// RegistryPublisherByID looks up and returns the registry publisher by GraphQL ID. If there is no
// local registry, it is not implemented.
var RegistryPublisherByID func(context.Context, graphql.ID) (graphqlbackend.RegistryPublisher, error)
