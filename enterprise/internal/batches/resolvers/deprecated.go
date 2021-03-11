package resolvers

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
)

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CampaignByID(ctx context.Context, id graphql.ID) (graphqlbackend.BatchChangeResolver, error) {
	res, err := r.BatchChangeByID(ctx, id)
	if batchChangeRes, ok := res.(*batchChangeResolver); ok {
		batchChangeRes.shouldActAsCampaign = true
		return batchChangeRes, err
	}
	return res, err
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) Campaign(ctx context.Context, args *graphqlbackend.BatchChangeArgs) (graphqlbackend.BatchChangeResolver, error) {
	res, err := r.BatchChange(ctx, args)
	if batchChangeRes, ok := res.(*batchChangeResolver); ok {
		batchChangeRes.shouldActAsCampaign = true
		return batchChangeRes, err
	}
	return res, err
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CampaignSpecByID(ctx context.Context, id graphql.ID) (graphqlbackend.BatchSpecResolver, error) {
	res, err := r.BatchSpecByID(ctx, id)
	if batchSpecRes, ok := res.(*batchSpecResolver); ok {
		batchSpecRes.shouldActAsCampaignSpec = true
		return batchSpecRes, err
	}
	return res, err
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CreateCampaignSpec(ctx context.Context, args *graphqlbackend.CreateCampaignSpecArgs) (graphqlbackend.BatchSpecResolver, error) {
	return r.CreateBatchSpec(ctx, &graphqlbackend.CreateBatchSpecArgs{
		Namespace:      args.Namespace,
		ChangesetSpecs: args.ChangesetSpecs,
		// Use the new method by renaming the args field
		BatchSpec: args.CampaignSpec,
	})
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) MoveCampaign(ctx context.Context, args *graphqlbackend.MoveCampaignArgs) (graphqlbackend.BatchChangeResolver, error) {
	return r.MoveBatchChange(ctx, &graphqlbackend.MoveBatchChangeArgs{
		BatchChange:  args.Campaign,
		NewName:      args.NewName,
		NewNamespace: args.NewNamespace,
	})
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) DeleteCampaign(ctx context.Context, args *graphqlbackend.DeleteCampaignArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	return r.DeleteBatchChange(ctx, &graphqlbackend.DeleteBatchChangeArgs{
		BatchChange: args.Campaign,
	})
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) Campaigns(ctx context.Context, args *graphqlbackend.ListBatchChangesArgs) (graphqlbackend.BatchChangesConnectionResolver, error) {
	return r.BatchChanges(ctx, args)
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CampaignsCodeHosts(ctx context.Context, args *graphqlbackend.ListCampaignsCodeHostsArgs) (graphqlbackend.CampaignsCodeHostConnectionResolver, error) {
	conn, err := r.BatchChangesCodeHosts(ctx, &graphqlbackend.ListBatchChangesCodeHostsArgs{
		First:  args.First,
		After:  args.After,
		UserID: args.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &campaignsCodeHostConnectionResolver{BatchChangesCodeHostConnectionResolver: conn}, nil
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CreateCampaignsCredential(ctx context.Context, args *graphqlbackend.CreateCampaignsCredentialArgs) (_ graphqlbackend.CampaignsCredentialResolver, err error) {
	return r.CreateBatchChangesCredential(ctx, &graphqlbackend.CreateBatchChangesCredentialArgs{
		ExternalServiceKind: args.ExternalServiceKind,
		ExternalServiceURL:  args.ExternalServiceURL,
		User:                args.User,
		Credential:          args.Credential,
	})
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) DeleteCampaignsCredential(ctx context.Context, args *graphqlbackend.DeleteCampaignsCredentialArgs) (_ *graphqlbackend.EmptyResponse, err error) {
	return r.DeleteBatchChangesCredential(ctx, &graphqlbackend.DeleteBatchChangesCredentialArgs{
		BatchChangesCredential: args.CampaignsCredential,
	})
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CampaignsCredentialByID(ctx context.Context, id graphql.ID) (graphqlbackend.CampaignsCredentialResolver, error) {
	return r.BatchChangesCredentialByID(ctx, id)
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CreateCampaign(ctx context.Context, args *graphqlbackend.CreateCampaignArgs) (graphqlbackend.BatchChangeResolver, error) {
	newArgs := &graphqlbackend.CreateBatchChangeArgs{BatchSpec: args.CampaignSpec}
	return r.CreateBatchChange(ctx, newArgs)
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) ApplyCampaign(ctx context.Context, args *graphqlbackend.ApplyCampaignArgs) (graphqlbackend.BatchChangeResolver, error) {
	return r.ApplyBatchChange(ctx, &graphqlbackend.ApplyBatchChangeArgs{
		BatchSpec:         args.CampaignSpec,
		EnsureBatchChange: args.EnsureCampaign,
	})
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *Resolver) CloseCampaign(ctx context.Context, args *graphqlbackend.CloseCampaignArgs) (_ graphqlbackend.BatchChangeResolver, err error) {
	return r.CloseBatchChange(ctx, &graphqlbackend.CloseBatchChangeArgs{
		BatchChange:     args.Campaign,
		CloseChangesets: args.CloseChangesets,
	})
}

// TODO(campaigns-deprecation): Remove when campaigns are fully removed
func (r *batchSpecResolver) ViewerCampaignsCodeHosts(ctx context.Context, args *graphqlbackend.ListViewerCampaignsCodeHostsArgs) (graphqlbackend.CampaignsCodeHostConnectionResolver, error) {
	res, err := r.ViewerBatchChangesCodeHosts(ctx, &graphqlbackend.ListViewerBatchChangesCodeHostsArgs{
		First:                 args.First,
		After:                 args.After,
		OnlyWithoutCredential: args.OnlyWithoutCredential,
	})
	if err != nil {
		return nil, err
	}
	return &campaignsCodeHostConnectionResolver{BatchChangesCodeHostConnectionResolver: res}, nil
}

// TODO(campaigns-deprecation): Remove this wrapper type. It just exists to fulfil the interface
// of graphqlbackend.CampaignsCodeHostConnectionResolver.
type campaignsCodeHostResolver struct {
	graphqlbackend.BatchChangesCodeHostResolver
}

var _ graphqlbackend.CampaignsCodeHostResolver = &campaignsCodeHostResolver{}

func (c *campaignsCodeHostResolver) ExternalServiceKind() string {
	return c.BatchChangesCodeHostResolver.ExternalServiceKind()
}

func (c *campaignsCodeHostResolver) ExternalServiceURL() string {
	return c.BatchChangesCodeHostResolver.ExternalServiceURL()
}

func (c *campaignsCodeHostResolver) Credential() graphqlbackend.CampaignsCredentialResolver {
	return c.BatchChangesCodeHostResolver.Credential()
}

func (c *campaignsCodeHostResolver) RequiresSSH() bool {
	return c.BatchChangesCodeHostResolver.RequiresSSH()
}

type campaignsCodeHostConnectionResolver struct {
	graphqlbackend.BatchChangesCodeHostConnectionResolver
}

var _ graphqlbackend.CampaignsCodeHostConnectionResolver = &campaignsCodeHostConnectionResolver{}

func (c *campaignsCodeHostConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	return c.BatchChangesCodeHostConnectionResolver.TotalCount(ctx)
}

func (c *campaignsCodeHostConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	return c.BatchChangesCodeHostConnectionResolver.PageInfo(ctx)
}

func (c *campaignsCodeHostConnectionResolver) Nodes(ctx context.Context) ([]graphqlbackend.CampaignsCodeHostResolver, error) {
	batchNodes, err := c.BatchChangesCodeHostConnectionResolver.Nodes(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]graphqlbackend.CampaignsCodeHostResolver, len(batchNodes))
	for i, ch := range batchNodes {
		nodes[i] = &campaignsCodeHostResolver{BatchChangesCodeHostResolver: ch}
	}
	return nodes, nil
}
