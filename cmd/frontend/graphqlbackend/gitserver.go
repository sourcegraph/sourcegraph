package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/internal/auth"
)

const gitserverIDKind = "GitserverInstance"

func marshalGitserverID(id string) graphql.ID { return relay.MarshalID(gitserverIDKind, id) }

func unmarshalGitserverID(id graphql.ID) (gitserverID string, err error) {
	err = relay.UnmarshalSpec(id, &gitserverID)
	return
}

func (r *schemaResolver) gitserverByID(ctx context.Context, id graphql.ID) (*gitserverResolver, error) {
	// 🚨 SECURITY: Only site admins can query gitserver information.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	address, err := unmarshalGitserverID(id)
	if err != nil {
		return nil, err
	}

	infos, err := r.gitserverClient.SystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	// Naive implemention that just returns the first gitserver matching the address
	for _, info := range infos {
		if info.Address == address {
			return &gitserverResolver{
				address:             address,
				freeDiskSpaceBytes:  info.FreeSpace,
				totalDiskSpaceBytes: info.TotalSpace,
			}, nil
		}
	}

	return nil, nil
}

func (r *schemaResolver) Gitservers(ctx context.Context) ([]*gitserverResolver, error) {
	// 🚨 SECURITY: Only site admins can query gitserver information.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	infos, err := r.gitserverClient.SystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	var resolvers = make([]*gitserverResolver, 0, len(infos))
	for _, info := range infos {
		resolvers = append(resolvers, &gitserverResolver{
			address:             info.Address,
			freeDiskSpaceBytes:  info.FreeSpace,
			totalDiskSpaceBytes: info.TotalSpace,
		})
	}
	return resolvers, nil
}

type gitserverResolver struct {
	address             string
	freeDiskSpaceBytes  uint64
	totalDiskSpaceBytes uint64
}

// ID returns a unique GraphQL ID for the gitserver instance.
//
// It marshals the gitserver address into an opaque unique string ID.
// This allows the gitserver instance to be uniquely identified in the
// GraphQL schema.
func (g *gitserverResolver) ID() graphql.ID {
	return marshalGitserverID(g.address)
}

// Shard returns the address of the gitserver instance.
func (g *gitserverResolver) Shard() string {
	return g.address
}

// FreeDiskSpaceBytes returns the available free disk space on the gitserver.
//
// The free disk space is returned as a GraphQL BigInt type, representing the
// number of free bytes available.
func (g *gitserverResolver) FreeDiskSpaceBytes() BigInt {
	return BigInt(g.freeDiskSpaceBytes)
}

// TotalDiskSpaceBytes returns the total disk space on the gitserver.
//
// The total space is returned as a GraphQL BigInt type, representing the
// total number of bytes.
func (g *gitserverResolver) TotalDiskSpaceBytes() BigInt {
	return BigInt(g.totalDiskSpaceBytes)
}
