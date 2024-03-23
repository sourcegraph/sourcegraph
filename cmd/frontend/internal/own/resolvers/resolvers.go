package resolvers

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/own"
	"github.com/sourcegraph/sourcegraph/internal/own/codeowners"
	codeownerspb "github.com/sourcegraph/sourcegraph/internal/own/codeowners/v1"
	"github.com/sourcegraph/sourcegraph/internal/rbac"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func New(db database.DB, gitserver gitserver.Client, logger log.Logger) graphqlbackend.OwnResolver {
	return &ownResolver{
		db:           db,
		gitserver:    gitserver,
		ownServiceFn: func() own.Service { return own.NewService(gitserver, db) },
		logger:       logger,
	}
}

func NewWithService(db database.DB, gitserver gitserver.Client, ownService own.Service, logger log.Logger) graphqlbackend.OwnResolver {
	return &ownResolver{
		db:           db,
		gitserver:    gitserver,
		ownServiceFn: func() own.Service { return ownService },
		logger:       logger,
	}
}

var (
	_ graphqlbackend.OwnResolver                              = &ownResolver{}
	_ graphqlbackend.OwnershipReasonResolver                  = &ownershipReasonResolver{}
	_ graphqlbackend.RecentContributorOwnershipSignalResolver = &recentContributorOwnershipSignal{}
	_ graphqlbackend.SimpleOwnReasonResolver                  = &recentContributorOwnershipSignal{}
	_ graphqlbackend.RecentViewOwnershipSignalResolver        = &recentViewOwnershipSignal{}
	_ graphqlbackend.SimpleOwnReasonResolver                  = &recentViewOwnershipSignal{}
	_ graphqlbackend.AssignedOwnerResolver                    = &assignedOwner{}
	_ graphqlbackend.SimpleOwnReasonResolver                  = &assignedOwner{}
	_ graphqlbackend.SimpleOwnReasonResolver                  = &codeownersFileEntryResolver{}
)

type ownResolver struct {
	db           database.DB
	gitserver    gitserver.Client
	ownServiceFn func() own.Service
	logger       log.Logger
}

func (r *ownResolver) ownService() own.Service {
	return r.ownServiceFn()
}

type ownershipReason struct {
	codeownersRule           *codeownerspb.Rule
	codeownersSource         codeowners.RulesetSource
	recentContributionsCount int
	recentViewsCount         int
	assignedOwnerPath        []string
}

type ownershipReasonResolver struct {
	resolver graphqlbackend.SimpleOwnReasonResolver
}

func (o *ownershipReasonResolver) Title() (string, error) {
	return o.resolver.Title()
}

func (o *ownershipReasonResolver) Description() (string, error) {
	return o.resolver.Description()
}

func (o *ownershipReasonResolver) ToCodeownersFileEntry() (res graphqlbackend.CodeownersFileEntryResolver, ok bool) {
	res, ok = o.resolver.(*codeownersFileEntryResolver)
	return
}

func (o *ownershipReasonResolver) ToRecentContributorOwnershipSignal() (res graphqlbackend.RecentContributorOwnershipSignalResolver, ok bool) {
	res, ok = o.resolver.(*recentContributorOwnershipSignal)
	return
}

func (o *ownershipReasonResolver) ToRecentViewOwnershipSignal() (res graphqlbackend.RecentViewOwnershipSignalResolver, ok bool) {
	res, ok = o.resolver.(*recentViewOwnershipSignal)
	return
}

func (o *ownershipReasonResolver) ToAssignedOwner() (res graphqlbackend.AssignedOwnerResolver, ok bool) {
	res, ok = o.resolver.(*assignedOwner)
	return
}

func (o *ownershipReasonResolver) makesAnOwner() bool {
	_, makesAnOwner := o.resolver.(*codeownersFileEntryResolver)
	_, makesAnAssignedOwner := o.resolver.(*assignedOwner)
	return makesAnOwner || makesAnAssignedOwner
}

func (r *ownResolver) GitBlobOwnership(
	ctx context.Context,
	blob *graphqlbackend.GitTreeEntryResolver,
	args graphqlbackend.ListOwnershipArgs,
) (graphqlbackend.OwnershipConnectionResolver, error) {
	if blob == nil {
		return nil, errors.New("cannot resolve git tree")
	}
	var rrs []reasonAndReference
	// Evaluate CODEOWNERS rules.
	if args.IncludeReason(graphqlbackend.CodeownersFileEntry) {
		co, err := r.computeCodeowners(ctx, blob)
		if err != nil {
			return nil, err
		}
		rrs = append(rrs, co...)
	}

	repoID := blob.Repository().IDInt32()

	// Retrieve recent contributors signals.
	if args.IncludeReason(graphqlbackend.RecentContributorOwnershipSignal) {
		contribResolvers, err := computeRecentContributorSignals(ctx, r.db, blob.Path(), repoID)
		if err != nil {
			return nil, err
		}
		rrs = append(rrs, contribResolvers...)
	}

	// Retrieve recent view signals.
	if args.IncludeReason(graphqlbackend.RecentViewOwnershipSignal) {
		viewerResolvers, err := computeRecentViewSignals(ctx, r.db, blob.Path(), repoID)
		if err != nil {
			return nil, err
		}
		rrs = append(rrs, viewerResolvers...)
	}

	if args.IncludeReason(graphqlbackend.AssignedOwner) {
		// Retrieve assigned owners.
		assignedOwners, err := r.computeAssignedOwners(ctx, blob, repoID)
		if err != nil {
			return nil, err
		}
		rrs = append(rrs, assignedOwners...)

		// Retrieve assigned teams.
		assignedTeams, err := r.computeAssignedTeams(ctx, blob, repoID)
		if err != nil {
			return nil, err
		}
		rrs = append(rrs, assignedTeams...)
	}

	return r.ownershipConnection(ctx, args, rrs, blob.Repository(), blob.Path())
}

// repoRootPath is the path that designates all the aggregate signals
// for a repository.
const repoRootPath = ""

// GitCommitOwnership retrieves ownership signals (not CODEOWNERS data)
// aggregated for the whole repository.
//
// It's a commit ownership rather than repo ownership because
// from the resolution point of view repo needs to be versioned
// at a certain commit to compute signals. At this point, however
// signals are not versioned yet, so every commit gets the same data.
func (r *ownResolver) GitCommitOwnership(
	ctx context.Context,
	commit *graphqlbackend.GitCommitResolver,
	args graphqlbackend.ListOwnershipArgs,
) (graphqlbackend.OwnershipConnectionResolver, error) {
	if commit == nil {
		return nil, errors.New("cannot resolve git commit")
	}
	repoID := commit.Repository().IDInt32()
	// Retrieve recent contributors signals.
	rrs, err := computeRecentContributorSignals(ctx, r.db, repoRootPath, repoID)
	if err != nil {
		return nil, err
	}

	// Retrieve recent view signals.
	viewerResolvers, err := computeRecentViewSignals(ctx, r.db, repoRootPath, repoID)
	if err != nil {
		return nil, err
	}
	rrs = append(rrs, viewerResolvers...)

	return r.ownershipConnection(ctx, args, rrs, commit.Repository(), "")
}

func (r *ownResolver) GitTreeOwnership(
	ctx context.Context,
	tree *graphqlbackend.GitTreeEntryResolver,
	args graphqlbackend.ListOwnershipArgs,
) (graphqlbackend.OwnershipConnectionResolver, error) {
	if tree == nil {
		return nil, errors.New("cannot resolve git tree")
	}
	// Retrieve recent contributors signals.
	repoID := tree.Repository().IDInt32()
	rrs, err := computeRecentContributorSignals(ctx, r.db, tree.Path(), repoID)
	if err != nil {
		return nil, err
	}

	// Retrieve recent view signals.
	viewerResolvers, err := computeRecentViewSignals(ctx, r.db, tree.Path(), repoID)
	if err != nil {
		return nil, err
	}
	rrs = append(rrs, viewerResolvers...)

	// Retrieve assigned owners.
	assignedOwners, err := r.computeAssignedOwners(ctx, tree, repoID)
	if err != nil {
		return nil, err
	}
	rrs = append(rrs, assignedOwners...)

	// Retrieve assigned teams.
	assignedTeams, err := r.computeAssignedTeams(ctx, tree, repoID)
	if err != nil {
		return nil, err
	}
	rrs = append(rrs, assignedTeams...)

	return r.ownershipConnection(ctx, args, rrs, tree.Repository(), tree.Path())
}

func (r *ownResolver) GitTreeOwnershipStats(_ context.Context, tree *graphqlbackend.GitTreeEntryResolver) (graphqlbackend.OwnershipStatsResolver, error) {
	if tree == nil {
		return nil, errors.New("cannot resolve git tree")
	}
	return &ownStatsResolver{
		db: r.db,
		opts: database.TreeLocationOpts{
			RepoID: tree.Repository().IDInt32(),
			Path:   tree.Path(),
		},
	}, nil
}

func (r *ownResolver) InstanceOwnershipStats(_ context.Context) (graphqlbackend.OwnershipStatsResolver, error) {
	return &ownStatsResolver{db: r.db}, nil
}

func (r *ownResolver) PersonOwnerField(_ *graphqlbackend.PersonResolver) string {
	return "owner"
}

func (r *ownResolver) UserOwnerField(_ *graphqlbackend.UserResolver) string {
	return "owner"
}

func (r *ownResolver) TeamOwnerField(_ *graphqlbackend.TeamResolver) string {
	return "owner"
}

func (r *ownResolver) NodeResolvers() map[string]graphqlbackend.NodeByIDFunc {
	return map[string]graphqlbackend.NodeByIDFunc{
		codeownersIngestedFileKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			// codeowners ingested files are identified by repo ID at the moment.
			var repoID api.RepoID
			if err := relay.UnmarshalSpec(id, &repoID); err != nil {
				return nil, errors.Wrap(err, "could not unmarshal repository ID")
			}
			return r.RepoIngestedCodeowners(ctx, repoID)
		},
	}
}

type reasonAndReference struct {
	reason    ownershipReason
	reference own.Reference
}

type reasonsAndOwner struct {
	reasons []ownershipReason
	owner   codeowners.ResolvedOwner
}

func (ro reasonsAndOwner) String() string {
	var b bytes.Buffer
	fmt.Fprint(&b, ro.owner.Identifier())
	for _, r := range ro.reasons {
		if r.codeownersRule != nil {
			fmt.Fprint(&b, " codeowners")
		}
		if len(r.assignedOwnerPath) > 0 {
			fmt.Fprint(&b, " assigned-owner")
		}
		if r.recentContributionsCount > 0 {
			fmt.Fprint(&b, " recent-contributor")
		}
		if r.recentViewsCount > 0 {
			fmt.Fprint(&b, " recent-viewer")
		}
	}
	return b.String()
}

func (ro reasonsAndOwner) order() int {
	var ownershipReasons, reasons, contributions, views int
	for _, r := range ro.reasons {
		if len(r.assignedOwnerPath) > 0 || r.codeownersRule != nil {
			ownershipReasons++
		}
		reasons++
		contributions += r.recentContributionsCount
		views += r.recentViewsCount
	}
	// Smaller numbers are ordered in front, so take negative score.
	return -(100000*ownershipReasons +
		1000*reasons +
		10*contributions +
		views)
}

func (ro reasonsAndOwner) isOwner() bool {
	for _, r := range ro.reasons {
		if len(r.assignedOwnerPath) > 0 || r.codeownersRule != nil {
			return true
		}
	}
	return false
}

// ownershipConnection handles ordering and pagination of given set of ownerships.
func (r *ownResolver) ownershipConnection(
	ctx context.Context,
	args graphqlbackend.ListOwnershipArgs,
	ownerships []reasonAndReference,
	repo *graphqlbackend.RepositoryResolver,
	path string,
) (*ownershipConnectionResolver, error) {
	// 1. Resolve ownership references
	bag := own.EmptyBag()
	for _, r := range ownerships {
		bag.Add(r.reference)
	}
	bag.Resolve(ctx, r.db)
	// 2. Group reasons by resolved owners
	ownersByKey := map[string]*reasonsAndOwner{}
	for _, r := range ownerships {
		resolvedOwner, found := bag.FindResolved(r.reference)
		if !found {
			if guess := r.reference.ResolutionGuess(); guess != nil {
				resolvedOwner = guess
			}
		}
		if resolvedOwner == nil {
			// TODO: Log - this is a bug
			continue
		}
		ownerKey := resolvedOwner.Identifier()
		ownerInfo, ok := ownersByKey[ownerKey]
		if !ok {
			ownerInfo = &reasonsAndOwner{owner: resolvedOwner}
			ownersByKey[ownerKey] = ownerInfo
		}
		ownerInfo.reasons = append(ownerInfo.reasons, r.reason)
	}
	var owners []reasonsAndOwner
	for _, o := range ownersByKey {
		owners = append(owners, *o)
	}

	// 3. Order ownerships for deterministic pagination:
	sort.Slice(owners, func(i, j int) bool {
		o, p := owners[i], owners[j]
		if x, y := o.order(), p.order(); x != y {
			return x < y
		}
		return o.owner.Identifier() < p.owner.Identifier()
	})

	// 4. Compute counts
	total := len(owners)
	var totalOwners int
	for _, o := range owners {
		if o.isOwner() {
			totalOwners++
		}
	}

	// 5. Apply pagination from the parameter & compute next cursor:
	cursor, err := graphqlutil.DecodeCursor(args.After)
	if err != nil {
		return nil, err
	}
	for cursor != "" && len(owners) > 0 && owners[0].owner.Identifier() != cursor {
		owners = owners[1:]
	}
	var next *string
	if args.First != nil && len(owners) > int(*args.First) {
		c := owners[*args.First].owner.Identifier()
		next = &c
		owners = owners[:*args.First]
	}

	// 6. Assemble the connection resolver object:
	return &ownershipConnectionResolver{
		db:          r.db,
		total:       total,
		totalOwners: totalOwners,
		next:        next,
		owners:      owners,
		gitserver:   r.gitserver,
		repo:        repo,
		path:        path,
	}, nil
}

type ownStatsResolver struct {
	db           database.DB
	opts         database.TreeLocationOpts
	once         sync.Once
	ownCounts    database.PathAggregateCounts
	ownCountsErr error
}

func (r *ownStatsResolver) computeOwnCounts(ctx context.Context) (database.PathAggregateCounts, error) {
	r.once.Do(func() {
		r.ownCounts, r.ownCountsErr = r.db.OwnershipStats().QueryAggregateCounts(ctx, r.opts)
	})
	return r.ownCounts, r.ownCountsErr
}

func (r *ownStatsResolver) TotalFiles(ctx context.Context) (int32, error) {
	return r.db.RepoPaths().AggregateFileCount(ctx, r.opts)
}

func (r *ownStatsResolver) TotalCodeownedFiles(ctx context.Context) (int32, error) {
	counts, err := r.computeOwnCounts(ctx)
	if err != nil {
		return 0, err
	}
	return int32(counts.CodeownedFileCount), nil
}

func (r *ownStatsResolver) TotalOwnedFiles(ctx context.Context) (int32, error) {
	counts, err := r.computeOwnCounts(ctx)
	if err != nil {
		return 0, err
	}
	return int32(counts.TotalOwnedFileCount), nil
}

func (r *ownStatsResolver) TotalAssignedOwnershipFiles(ctx context.Context) (int32, error) {
	counts, err := r.computeOwnCounts(ctx)
	if err != nil {
		return 0, err
	}
	return int32(counts.AssignedOwnershipFileCount), nil
}

func (r *ownStatsResolver) UpdatedAt(ctx context.Context) (*gqlutil.DateTime, error) {
	counts, err := r.computeOwnCounts(ctx)
	if err != nil {
		return nil, err
	}
	return gqlutil.FromTime(counts.UpdatedAt), nil
}

type ownershipConnectionResolver struct {
	db          database.DB
	total       int
	totalOwners int
	next        *string
	owners      []reasonsAndOwner
	gitserver   gitserver.Client
	repo        *graphqlbackend.RepositoryResolver
	path        string
}

func (r *ownershipConnectionResolver) TotalCount(_ context.Context) (int32, error) {
	return int32(r.total), nil
}

func (r *ownershipConnectionResolver) TotalOwners(_ context.Context) (int32, error) {
	return int32(r.totalOwners), nil
}

func (r *ownershipConnectionResolver) PageInfo(_ context.Context) (*graphqlutil.PageInfo, error) {
	return graphqlutil.EncodeCursor(r.next), nil
}

func (r *ownershipConnectionResolver) Nodes(_ context.Context) ([]graphqlbackend.OwnershipResolver, error) {
	var rs []graphqlbackend.OwnershipResolver
	for _, o := range r.owners {
		rs = append(rs, &ownershipResolver{
			db:            r.db,
			gitserver:     r.gitserver,
			repo:          r.repo,
			resolvedOwner: o.owner,
			reasons:       o.reasons,
			path:          r.path,
		})
	}
	return rs, nil
}

type ownershipResolver struct {
	db            database.DB
	gitserver     gitserver.Client
	resolvedOwner codeowners.ResolvedOwner
	path          string
	repo          *graphqlbackend.RepositoryResolver
	reasons       []ownershipReason
}

func (r *ownershipResolver) Owner(_ context.Context) (graphqlbackend.OwnerResolver, error) {
	return &ownerResolver{
		db:            r.db,
		resolvedOwner: r.resolvedOwner,
	}, nil
}

func (r *ownershipResolver) Reasons(_ context.Context) ([]graphqlbackend.OwnershipReasonResolver, error) {
	var rs []graphqlbackend.OwnershipReasonResolver
	for _, reason := range r.reasons {
		if reason.codeownersRule != nil {
			rs = append(rs, &ownershipReasonResolver{
				resolver: &codeownersFileEntryResolver{
					db:              r.db,
					gitserverClient: r.gitserver,
					source:          reason.codeownersSource,
					repo:            r.repo,
					matchLineNumber: reason.codeownersRule.GetLineNumber(),
				},
			})

		}
		for _, p := range reason.assignedOwnerPath {
			rs = append(rs, &ownershipReasonResolver{
				resolver: &assignedOwner{
					directMatch: r.path == p,
				},
			})
		}
		if reason.recentContributionsCount > 0 {
			rs = append(rs, &ownershipReasonResolver{
				resolver: &recentContributorOwnershipSignal{
					total: int32(reason.recentContributionsCount),
				},
			})
		}
		if reason.recentViewsCount > 0 {
			rs = append(rs, &ownershipReasonResolver{
				resolver: &recentViewOwnershipSignal{
					total: int32(reason.recentViewsCount),
				},
			})
		}
	}
	return rs, nil
}

type ownerResolver struct {
	db            database.DB
	resolvedOwner codeowners.ResolvedOwner
}

func (r *ownerResolver) OwnerField(_ context.Context) (string, error) { return "owner", nil }

func (r *ownerResolver) ToPerson() (*graphqlbackend.PersonResolver, bool) {
	if r.resolvedOwner.Type() != codeowners.OwnerTypePerson {
		return nil, false
	}
	person, ok := r.resolvedOwner.(*codeowners.Person)
	if !ok {
		return nil, false
	}
	if person.User != nil {
		return graphqlbackend.NewPersonResolverFromUser(r.db, person.GetEmail(), person.User), true
	}
	const includeUserInfo = true
	return graphqlbackend.NewPersonResolver(r.db, person.Handle, person.GetEmail(), includeUserInfo), true
}

func (r *ownerResolver) ToTeam() (*graphqlbackend.TeamResolver, bool) {
	if r.resolvedOwner.Type() != codeowners.OwnerTypeTeam {
		return nil, false
	}
	resolvedTeam, ok := r.resolvedOwner.(*codeowners.Team)
	if !ok {
		return nil, false
	}
	if resolvedTeam.Team != nil {
		return graphqlbackend.NewTeamResolver(r.db, resolvedTeam.Team), true
	}
	// The sourcegraph instance does not have that team in the database.
	// It might be an unimported code-host team, a guess or both.
	return graphqlbackend.NewTeamResolver(r.db, &types.Team{
		Name:        resolvedTeam.Identifier(),
		DisplayName: resolvedTeam.Identifier(),
	}), true
}

func (r *ownResolver) OwnSignalConfigurations(ctx context.Context) ([]graphqlbackend.SignalConfigurationResolver, error) {
	err := auth.CheckCurrentActorIsSiteAdmin(actor.FromContext(ctx), r.db)
	if err != nil {
		return nil, err
	}
	var resolvers []graphqlbackend.SignalConfigurationResolver
	store := r.db.OwnSignalConfigurations()
	configurations, err := store.LoadConfigurations(ctx, database.LoadSignalConfigurationArgs{})
	if err != nil {
		return nil, errors.Wrap(err, "LoadConfigurations")
	}

	for _, configuration := range configurations {
		resolvers = append(resolvers, &signalConfigResolver{config: configuration})
	}

	return resolvers, nil
}

type signalConfigResolver struct {
	config database.SignalConfiguration
}

func (s *signalConfigResolver) Name() string {
	return s.config.Name
}

func (s *signalConfigResolver) Description() string {
	return s.config.Description
}

func (s *signalConfigResolver) IsEnabled() bool {
	return s.config.Enabled
}

func (s *signalConfigResolver) ExcludedRepoPatterns() []string {
	return userifyPatterns(s.config.ExcludedRepoPatterns)
}

func (r *ownResolver) UpdateOwnSignalConfigurations(ctx context.Context, args graphqlbackend.UpdateSignalConfigurationsArgs) ([]graphqlbackend.SignalConfigurationResolver, error) {
	err := auth.CheckCurrentActorIsSiteAdmin(actor.FromContext(ctx), r.db)
	if err != nil {
		return nil, err
	}

	err = r.db.OwnSignalConfigurations().WithTransact(ctx, func(store database.SignalConfigurationStore) error {
		for _, config := range args.Input.Configs {
			if err := store.UpdateConfiguration(ctx, database.UpdateSignalConfigurationArgs{
				Name:                 config.Name,
				ExcludedRepoPatterns: postgresifyPatterns(config.ExcludedRepoPatterns),
				Enabled:              config.Enabled,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return r.OwnSignalConfigurations(ctx)
}

// postgresifyPatterns will convert glob-ish patterns to postgres compatible patterns. For example github.com/* -> github.com/%
func postgresifyPatterns(patterns []string) (results []string) {
	for _, pattern := range patterns {
		results = append(results, strings.ReplaceAll(pattern, "*", "%"))
	}
	return results
}

// userifyPatterns will convert postgres patterns to glob-ish patterns. For example github.com/% -> github.com/*.
func userifyPatterns(patterns []string) (results []string) {
	for _, pattern := range patterns {
		results = append(results, strings.ReplaceAll(pattern, "%", "*"))
	}
	return results
}

func (r *ownResolver) AssignOwner(ctx context.Context, args *graphqlbackend.AssignOwnerOrTeamArgs) (*graphqlbackend.EmptyResponse, error) {
	// Internal actor is a no-op, only a user can assign an owner.
	if actor.FromContext(ctx).IsInternal() {
		return nil, nil
	}
	user, err := r.checkAssignedOwnershipPermission(ctx)
	if err != nil {
		return nil, err
	}
	u, err := unmarshalAssignOwnerArgs(args.Input, userUnmarshalMode)
	if err != nil {
		return nil, err
	}
	whoAssignedUserID := user.ID
	err = r.db.AssignedOwners().Insert(ctx, u.AssignedOwnerOrTeamID, u.RepoID, u.AbsolutePath, whoAssignedUserID)
	if err != nil {
		return nil, errors.Wrap(err, "creating assigned owner")
	}
	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *ownResolver) RemoveAssignedOwner(ctx context.Context, args *graphqlbackend.AssignOwnerOrTeamArgs) (*graphqlbackend.EmptyResponse, error) {
	// Internal actor is a no-op, only a user can remove an assigned owner.
	if actor.FromContext(ctx).IsInternal() {
		return nil, nil
	}
	_, err := r.checkAssignedOwnershipPermission(ctx)
	if err != nil {
		return nil, err
	}
	u, err := unmarshalAssignOwnerArgs(args.Input, userUnmarshalMode)
	if err != nil {
		return nil, err
	}
	err = r.db.AssignedOwners().DeleteOwner(ctx, u.AssignedOwnerOrTeamID, u.RepoID, u.AbsolutePath)
	if err != nil {
		return nil, errors.Wrap(err, "deleting assigned owner")
	}
	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *ownResolver) AssignTeam(ctx context.Context, args *graphqlbackend.AssignOwnerOrTeamArgs) (*graphqlbackend.EmptyResponse, error) {
	// Internal actor is a no-op, only a user can assign an owner.
	if actor.FromContext(ctx).IsInternal() {
		return nil, nil
	}
	user, err := r.checkAssignedOwnershipPermission(ctx)
	if err != nil {
		return nil, err
	}
	t, err := unmarshalAssignOwnerArgs(args.Input, teamUnmarshalMode)
	if err != nil {
		return nil, err
	}
	whoAssignedUserID := user.ID
	err = r.db.AssignedTeams().Insert(ctx, t.AssignedOwnerOrTeamID, t.RepoID, t.AbsolutePath, whoAssignedUserID)
	if err != nil {
		return nil, errors.Wrap(err, "creating assigned team")
	}
	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *ownResolver) RemoveAssignedTeam(ctx context.Context, args *graphqlbackend.AssignOwnerOrTeamArgs) (*graphqlbackend.EmptyResponse, error) {
	// Internal actor is a no-op, only a user can remove an assigned owner.
	if actor.FromContext(ctx).IsInternal() {
		return nil, nil
	}
	_, err := r.checkAssignedOwnershipPermission(ctx)
	if err != nil {
		return nil, err
	}
	t, err := unmarshalAssignOwnerArgs(args.Input, teamUnmarshalMode)
	if err != nil {
		return nil, err
	}
	err = r.db.AssignedTeams().DeleteOwnerTeam(ctx, t.AssignedOwnerOrTeamID, t.RepoID, t.AbsolutePath)
	if err != nil {
		return nil, errors.Wrap(err, "deleting assigned team")
	}
	return &graphqlbackend.EmptyResponse{}, nil
}

// checkAssignedOwnershipPermission checks that the user from the context has
// `rbac.OwnershipAssignPermission` and returns the user if so.
func (r *ownResolver) checkAssignedOwnershipPermission(ctx context.Context) (*types.User, error) {
	// Extracting the user to run an RBAC check and then use their ID as an
	// `whoAssignedUserID`.
	user, err := auth.CurrentUser(ctx, r.db)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, auth.ErrNotAuthenticated
	}
	// Checking if the user has permission to assign an owner.
	if err := rbac.CheckGivenUserHasPermission(ctx, r.db, user, rbac.OwnershipAssignPermission); err != nil {
		return nil, err
	}
	return user, nil
}

type UnmarshalledAssignOwnerArgs struct {
	AssignedOwnerOrTeamID int32
	RepoID                api.RepoID
	AbsolutePath          string
}

type UnmarshalMode string

const (
	userUnmarshalMode UnmarshalMode = "user"
	teamUnmarshalMode UnmarshalMode = "team"
)

func unmarshalAssignOwnerArgs(args graphqlbackend.AssignOwnerOrTeamInput, unmarshalMode UnmarshalMode) (*UnmarshalledAssignOwnerArgs, error) {
	var userOrTeamID int32
	var unmarshalError error
	if unmarshalMode == userUnmarshalMode {
		userOrTeamID, unmarshalError = graphqlbackend.UnmarshalUserID(args.AssignedOwnerID)
	} else if unmarshalMode == teamUnmarshalMode {
		userOrTeamID, unmarshalError = graphqlbackend.UnmarshalTeamID(args.AssignedOwnerID)
	} else {
		return nil, errors.New("only user or team can be assigned ownership")
	}
	if unmarshalError != nil {
		return nil, unmarshalError
	}
	if userOrTeamID == 0 {
		return nil, errors.Newf("assigned %s ID should not be 0", unmarshalMode)
	}
	repoID, err := graphqlbackend.UnmarshalRepositoryID(args.RepoID)
	if err != nil {
		return nil, err
	}
	if repoID == 0 {
		return nil, errors.New("repo ID should not be 0")
	}
	return &UnmarshalledAssignOwnerArgs{
		AssignedOwnerOrTeamID: userOrTeamID,
		RepoID:                repoID,
		AbsolutePath:          args.AbsolutePath,
	}, nil
}
