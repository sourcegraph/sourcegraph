package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"

	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type FeatureFlagResolver struct {
	db    database.DB
	inner *featureflag.FeatureFlag
}

func (f *FeatureFlagResolver) ToFeatureFlagBoolean() (*FeatureFlagBooleanResolver, bool) {
	if f.inner.Bool != nil {
		return &FeatureFlagBooleanResolver{f.db, f.inner}, true
	}
	return nil, false
}

func (f *FeatureFlagResolver) ToFeatureFlagRollout() (*FeatureFlagRolloutResolver, bool) {
	if f.inner.Rollout != nil {
		return &FeatureFlagRolloutResolver{f.db, f.inner}, true
	}
	return nil, false
}

type FeatureFlagBooleanResolver struct {
	db database.DB
	// Invariant: inner.Bool is non-nil
	inner *featureflag.FeatureFlag
}

func (f *FeatureFlagBooleanResolver) Name() string { return f.inner.Name }
func (f *FeatureFlagBooleanResolver) Value() bool  { return f.inner.Bool.Value }
func (f *FeatureFlagBooleanResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: f.inner.CreatedAt}
}
func (f *FeatureFlagBooleanResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: f.inner.UpdatedAt}
}
func (f *FeatureFlagBooleanResolver) Overrides(ctx context.Context) ([]*FeatureFlagOverrideResolver, error) {
	overrides, err := f.db.FeatureFlags().GetOverridesForFlag(ctx, f.inner.Name)
	if err != nil {
		return nil, err
	}
	return overridesToResolvers(f.db, overrides), nil
}

type FeatureFlagRolloutResolver struct {
	db database.DB
	// Invariant: inner.Rollout is non-nil
	inner *featureflag.FeatureFlag
}

func (f *FeatureFlagRolloutResolver) Name() string              { return f.inner.Name }
func (f *FeatureFlagRolloutResolver) RolloutBasisPoints() int32 { return f.inner.Rollout.Rollout }
func (f *FeatureFlagRolloutResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: f.inner.CreatedAt}
}
func (f *FeatureFlagRolloutResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: f.inner.UpdatedAt}
}
func (f *FeatureFlagRolloutResolver) Overrides(ctx context.Context) ([]*FeatureFlagOverrideResolver, error) {
	overrides, err := f.db.FeatureFlags().GetOverridesForFlag(ctx, f.inner.Name)
	if err != nil {
		return nil, err
	}
	return overridesToResolvers(f.db, overrides), nil
}

func overridesToResolvers(db database.DB, input []*featureflag.Override) []*FeatureFlagOverrideResolver {
	res := make([]*FeatureFlagOverrideResolver, 0, len(input))
	for _, flag := range input {
		res = append(res, &FeatureFlagOverrideResolver{db, flag})
	}
	return res
}

type FeatureFlagOverrideResolver struct {
	db    database.DB
	inner *featureflag.Override
}

func (f *FeatureFlagOverrideResolver) TargetFlag(ctx context.Context) (*FeatureFlagResolver, error) {
	res, err := f.db.FeatureFlags().GetFeatureFlag(ctx, f.inner.FlagName)
	return &FeatureFlagResolver{f.db, res}, err
}
func (f *FeatureFlagOverrideResolver) Value() bool { return f.inner.Value }
func (f *FeatureFlagOverrideResolver) Namespace(ctx context.Context) (*NamespaceResolver, error) {
	if f.inner.UserID != nil {
		u, err := UserByIDInt32(ctx, f.db, *f.inner.UserID)
		return &NamespaceResolver{u}, err
	} else if f.inner.OrgID != nil {
		o, err := OrgByIDInt32(ctx, f.db, *f.inner.OrgID)
		return &NamespaceResolver{o}, err
	}
	return nil, errors.Errorf("one of userID or orgID must be set")
}
func (f *FeatureFlagOverrideResolver) ID() graphql.ID {
	return marshalOverrideID(overrideSpec{
		UserID:   f.inner.UserID,
		OrgID:    f.inner.OrgID,
		FlagName: f.inner.FlagName,
	})
}

type overrideSpec struct {
	UserID, OrgID *int32
	FlagName      string
}

func marshalOverrideID(spec overrideSpec) graphql.ID {
	return relay.MarshalID("FeatureFlagOverride", spec)
}

func unmarshalOverrideID(id graphql.ID) (spec overrideSpec, err error) {
	err = relay.UnmarshalSpec(id, &spec)
	return
}

type EvaluatedFeatureFlagResolver struct {
	name  string
	value bool
}

func (e *EvaluatedFeatureFlagResolver) Name() string {
	return e.name
}

func (e *EvaluatedFeatureFlagResolver) Value() bool {
	return e.value
}

func (r *schemaResolver) EvaluateFeatureFlag(ctx context.Context, args *struct {
	FlagName string
}) *bool {
	flagSet := featureflag.FromContext(ctx)
	if v, ok := flagSet.GetBool(args.FlagName); ok {
		return &v
	}
	return nil
}

func (r *schemaResolver) EvaluatedFeatureFlags(ctx context.Context) []*EvaluatedFeatureFlagResolver {
	return evaluatedFlagsToResolvers(featureflag.GetEvaluatedFlagSet(ctx))
}

func evaluatedFlagsToResolvers(input map[string]bool) []*EvaluatedFeatureFlagResolver {
	res := make([]*EvaluatedFeatureFlagResolver, 0, len(input))
	for k, v := range input {
		res = append(res, &EvaluatedFeatureFlagResolver{name: k, value: v})
	}
	return res
}

func (r *schemaResolver) OrganizationFeatureFlagValue(ctx context.Context, args *struct {
	OrgID    graphql.ID
	FlagName string
}) (bool, error) {
	org, err := UnmarshalOrgID(args.OrgID)
	if err != nil {
		return false, err
	}
	// same behavior as if the flag does not exist
	if err := auth.CheckOrgAccess(ctx, r.db, org); err != nil {
		return false, nil
	}

	result, err := r.db.FeatureFlags().GetOrgFeatureFlag(ctx, org, args.FlagName)
	if err != nil {
		return false, err
	}
	return result, nil
}

func (r *schemaResolver) OrganizationFeatureFlagOverrides(ctx context.Context) ([]*FeatureFlagOverrideResolver, error) {
	actor := sgactor.FromContext(ctx)

	if !actor.IsAuthenticated() {
		return nil, errors.New("no current user")
	}

	flags, err := r.db.FeatureFlags().GetOrgOverridesForUser(ctx, actor.UID)
	if err != nil {
		return nil, err
	}

	return overridesToResolvers(r.db, flags), nil
}

func (r *schemaResolver) FeatureFlag(ctx context.Context, args struct {
	Name string
}) (*FeatureFlagResolver, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	ff, err := r.db.FeatureFlags().GetFeatureFlag(ctx, args.Name)
	if err != nil {
		return nil, err
	}

	return &FeatureFlagResolver{r.db, ff}, nil
}

func (r *schemaResolver) FeatureFlags(ctx context.Context) ([]*FeatureFlagResolver, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	flags, err := r.db.FeatureFlags().GetFeatureFlags(ctx)
	if err != nil {
		return nil, err
	}
	return flagsToResolvers(r.db, flags), nil
}

func flagsToResolvers(db database.DB, flags []*featureflag.FeatureFlag) []*FeatureFlagResolver {
	res := make([]*FeatureFlagResolver, 0, len(flags))
	for _, flag := range flags {
		res = append(res, &FeatureFlagResolver{db, flag})
	}
	return res
}

func (r *schemaResolver) CreateFeatureFlag(ctx context.Context, args struct {
	Name               string
	Value              *bool
	RolloutBasisPoints *int32
}) (*FeatureFlagResolver, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	ff := r.db.FeatureFlags()

	var res *featureflag.FeatureFlag
	var err error
	if args.Value != nil {
		res, err = ff.CreateBool(ctx, args.Name, *args.Value)
	} else if args.RolloutBasisPoints != nil {
		res, err = ff.CreateRollout(ctx, args.Name, *args.RolloutBasisPoints)
	} else {
		return nil, errors.Errorf("either 'value' or 'rolloutBasisPoints' must be set")
	}

	return &FeatureFlagResolver{r.db, res}, err
}

func (r *schemaResolver) DeleteFeatureFlag(ctx context.Context, args struct {
	Name string
}) (*EmptyResponse, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	return &EmptyResponse{}, r.db.FeatureFlags().DeleteFeatureFlag(ctx, args.Name)
}

func (r *schemaResolver) UpdateFeatureFlag(ctx context.Context, args struct {
	Name               string
	Value              *bool
	RolloutBasisPoints *int32
}) (*FeatureFlagResolver, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	ff := &featureflag.FeatureFlag{Name: args.Name}
	if args.Value != nil {
		ff.Bool = &featureflag.FeatureFlagBool{Value: *args.Value}
	} else if args.RolloutBasisPoints != nil {
		ff.Rollout = &featureflag.FeatureFlagRollout{Rollout: *args.RolloutBasisPoints}
	} else {
		return nil, errors.Errorf("either 'value' or 'rolloutBasisPoints' must be set")
	}

	res, err := r.db.FeatureFlags().UpdateFeatureFlag(ctx, ff)
	return &FeatureFlagResolver{r.db, res}, err
}

func (r *schemaResolver) CreateFeatureFlagOverride(ctx context.Context, args struct {
	Namespace graphql.ID
	FlagName  string
	Value     bool
}) (*FeatureFlagOverrideResolver, error) {
	fo := &featureflag.Override{
		FlagName: args.FlagName,
		Value:    args.Value,
	}

	var uid, oid int32
	if err := UnmarshalNamespaceID(args.Namespace, &uid, &oid); err != nil {
		return nil, err
	}

	// Allow users to add their own overrides
	if err := auth.CheckSiteAdminOrSameUserFromActor(actor.FromContext(ctx), r.db, uid); err != nil {
		return nil, err
	}

	if uid != 0 {
		fo.UserID = &uid
	} else if oid != 0 {
		fo.OrgID = &oid
	}
	res, err := r.db.FeatureFlags().CreateOverride(ctx, fo)
	return &FeatureFlagOverrideResolver{r.db, res}, err
}

func (r *schemaResolver) DeleteFeatureFlagOverride(ctx context.Context, args struct {
	ID graphql.ID
}) (*EmptyResponse, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	spec, err := unmarshalOverrideID(args.ID)
	if err != nil {
		return &EmptyResponse{}, err
	}
	return &EmptyResponse{}, r.db.FeatureFlags().DeleteOverride(ctx, spec.OrgID, spec.UserID, spec.FlagName)
}

func (r *schemaResolver) UpdateFeatureFlagOverride(ctx context.Context, args struct {
	ID    graphql.ID
	Value bool
}) (*FeatureFlagOverrideResolver, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	spec, err := unmarshalOverrideID(args.ID)
	if err != nil {
		return nil, err
	}

	res, err := r.db.FeatureFlags().UpdateOverride(ctx, spec.OrgID, spec.UserID, spec.FlagName, args.Value)
	return &FeatureFlagOverrideResolver{r.db, res}, err
}
