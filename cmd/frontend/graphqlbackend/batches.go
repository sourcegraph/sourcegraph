package graphqlbackend

import (
	"context"
	"errors"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/externallink"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/batches"
)

// TODO(campaigns-deprecation)
type CreateCampaignArgs struct {
	CampaignSpec graphql.ID
}

// TODO(campaigns-deprecation)
type CreateCampaignSpecArgs struct {
	Namespace graphql.ID

	CampaignSpec   string
	ChangesetSpecs []graphql.ID
}

// TODO(campaigns-deprecation)
type ApplyCampaignArgs struct {
	CampaignSpec   graphql.ID
	EnsureCampaign *graphql.ID
}

// TODO(campaigns-deprecation)
type CloseCampaignArgs struct {
	Campaign        graphql.ID
	CloseChangesets bool
}

// TODO(campaigns-deprecation)
type MoveCampaignArgs struct {
	Campaign     graphql.ID
	NewName      *string
	NewNamespace *graphql.ID
}

// TODO(campaigns-deprecation)
type DeleteCampaignArgs struct {
	Campaign graphql.ID
}

// TODO(campaigns-deprecation)
type CreateCampaignsCredentialArgs struct {
	ExternalServiceKind string
	ExternalServiceURL  string
	User                graphql.ID
	Credential          string
}

// TODO(campaigns-deprecation)
type DeleteCampaignsCredentialArgs struct {
	CampaignsCredential graphql.ID
}

// TODO(campaigns-deprecation)
type ListCampaignsCodeHostsArgs struct {
	First  int32
	After  *string
	UserID int32
}

// TODO(campaigns-deprecation)
type ListViewerCampaignsCodeHostsArgs struct {
	First                 int32
	After                 *string
	OnlyWithoutCredential bool
}

// TODO(campaigns-deprecation)
type CampaignsCodeHostConnectionResolver interface {
	Nodes(ctx context.Context) ([]CampaignsCodeHostResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

// TODO(campaigns-deprecation)
type CampaignsCodeHostResolver interface {
	ExternalServiceKind() string
	ExternalServiceURL() string
	RequiresSSH() bool
	Credential() CampaignsCredentialResolver
}

// TODO(campaigns-deprecation)
type CampaignsCredentialResolver interface {
	ID() graphql.ID
	ExternalServiceKind() string
	ExternalServiceURL() string
	SSHPublicKey() *string
	CreatedAt() DateTime
}

type CreateBatchChangeArgs struct {
	BatchSpec graphql.ID
}

type ApplyBatchChangeArgs struct {
	BatchSpec         graphql.ID
	EnsureBatchChange *graphql.ID
}

type ListBatchChangesArgs struct {
	First               int32
	After               *string
	State               *string
	ViewerCanAdminister *bool

	Namespace *graphql.ID
}

type CloseBatchChangeArgs struct {
	BatchChange     graphql.ID
	CloseChangesets bool
}

type MoveBatchChangeArgs struct {
	BatchChange  graphql.ID
	NewName      *string
	NewNamespace *graphql.ID
}

type DeleteBatchChangeArgs struct {
	BatchChange graphql.ID
}

type SyncChangesetArgs struct {
	Changeset graphql.ID
}

type ReenqueueChangesetArgs struct {
	Changeset graphql.ID
}

type CreateChangesetSpecArgs struct {
	ChangesetSpec string
}

type CreateBatchSpecArgs struct {
	Namespace graphql.ID

	BatchSpec      string
	ChangesetSpecs []graphql.ID
}

type ChangesetSpecsConnectionArgs struct {
	First int32
	After *string
}

type ChangesetApplyPreviewConnectionArgs struct {
	First        int32
	After        *string
	Search       *string
	CurrentState *batches.ChangesetState
	Action       *batches.ReconcilerOperation
}

type BatchChangeArgs struct {
	Namespace string
	Name      string
}

type ChangesetEventsConnectionArgs struct {
	First int32
	After *string
}

type CreateBatchChangesCredentialArgs struct {
	ExternalServiceKind string
	ExternalServiceURL  string
	User                graphql.ID
	Credential          string
}

type DeleteBatchChangesCredentialArgs struct {
	BatchChangesCredential graphql.ID
}

type ListBatchChangesCodeHostsArgs struct {
	First  int32
	After  *string
	UserID int32
}

type ListViewerBatchChangesCodeHostsArgs struct {
	First                 int32
	After                 *string
	OnlyWithoutCredential bool
}

type BatchChangesResolver interface {
	//
	// MUTATIONS
	//
	// TODO(campaigns-deprecation)
	CreateCampaign(ctx context.Context, args *CreateCampaignArgs) (BatchChangeResolver, error)
	CreateCampaignSpec(ctx context.Context, args *CreateCampaignSpecArgs) (BatchSpecResolver, error)
	ApplyCampaign(ctx context.Context, args *ApplyCampaignArgs) (BatchChangeResolver, error)
	CloseCampaign(ctx context.Context, args *CloseCampaignArgs) (BatchChangeResolver, error)
	MoveCampaign(ctx context.Context, args *MoveCampaignArgs) (BatchChangeResolver, error)
	DeleteCampaign(ctx context.Context, args *DeleteCampaignArgs) (*EmptyResponse, error)
	CreateCampaignsCredential(ctx context.Context, args *CreateCampaignsCredentialArgs) (CampaignsCredentialResolver, error)
	DeleteCampaignsCredential(ctx context.Context, args *DeleteCampaignsCredentialArgs) (*EmptyResponse, error)
	// New:
	CreateBatchChange(ctx context.Context, args *CreateBatchChangeArgs) (BatchChangeResolver, error)
	CreateBatchSpec(ctx context.Context, args *CreateBatchSpecArgs) (BatchSpecResolver, error)
	ApplyBatchChange(ctx context.Context, args *ApplyBatchChangeArgs) (BatchChangeResolver, error)
	CloseBatchChange(ctx context.Context, args *CloseBatchChangeArgs) (BatchChangeResolver, error)
	MoveBatchChange(ctx context.Context, args *MoveBatchChangeArgs) (BatchChangeResolver, error)
	DeleteBatchChange(ctx context.Context, args *DeleteBatchChangeArgs) (*EmptyResponse, error)
	CreateBatchChangesCredential(ctx context.Context, args *CreateBatchChangesCredentialArgs) (BatchChangesCredentialResolver, error)
	DeleteBatchChangesCredential(ctx context.Context, args *DeleteBatchChangesCredentialArgs) (*EmptyResponse, error)

	CreateChangesetSpec(ctx context.Context, args *CreateChangesetSpecArgs) (ChangesetSpecResolver, error)
	SyncChangeset(ctx context.Context, args *SyncChangesetArgs) (*EmptyResponse, error)
	ReenqueueChangeset(ctx context.Context, args *ReenqueueChangesetArgs) (ChangesetResolver, error)

	// Queries

	// TODO(campaigns-deprecation)
	Campaign(ctx context.Context, args *BatchChangeArgs) (BatchChangeResolver, error)
	Campaigns(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error)
	CampaignByID(ctx context.Context, id graphql.ID) (BatchChangeResolver, error)
	CampaignSpecByID(ctx context.Context, id graphql.ID) (BatchSpecResolver, error)
	CampaignsCredentialByID(ctx context.Context, id graphql.ID) (CampaignsCredentialResolver, error)
	CampaignsCodeHosts(ctx context.Context, args *ListCampaignsCodeHostsArgs) (CampaignsCodeHostConnectionResolver, error)
	// New:
	BatchChange(ctx context.Context, args *BatchChangeArgs) (BatchChangeResolver, error)
	BatchChangeByID(ctx context.Context, id graphql.ID) (BatchChangeResolver, error)
	BatchChanges(cx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error)
	BatchSpecByID(ctx context.Context, id graphql.ID) (BatchSpecResolver, error)

	ChangesetByID(ctx context.Context, id graphql.ID) (ChangesetResolver, error)
	ChangesetSpecByID(ctx context.Context, id graphql.ID) (ChangesetSpecResolver, error)

	BatchChangesCredentialByID(ctx context.Context, id graphql.ID) (BatchChangesCredentialResolver, error)
	BatchChangesCodeHosts(ctx context.Context, args *ListBatchChangesCodeHostsArgs) (BatchChangesCodeHostConnectionResolver, error)
}

type BatchSpecResolver interface {
	ID() graphql.ID

	OriginalInput() (string, error)
	ParsedInput() (JSONValue, error)
	ChangesetSpecs(ctx context.Context, args *ChangesetSpecsConnectionArgs) (ChangesetSpecConnectionResolver, error)
	ApplyPreview(ctx context.Context, args *ChangesetApplyPreviewConnectionArgs) (ChangesetApplyPreviewConnectionResolver, error)

	Description() BatchChangeDescriptionResolver

	Creator(context.Context) (*UserResolver, error)
	CreatedAt() DateTime
	Namespace(context.Context) (*NamespaceResolver, error)

	ExpiresAt() *DateTime

	ApplyURL(ctx context.Context) (string, error)

	ViewerCanAdminister(context.Context) (bool, error)

	DiffStat(ctx context.Context) (*DiffStat, error)

	AppliesToBatchChange(ctx context.Context) (BatchChangeResolver, error)

	SupersedingBatchSpec(context.Context) (BatchSpecResolver, error)

	ViewerBatchChangesCodeHosts(ctx context.Context, args *ListViewerBatchChangesCodeHostsArgs) (BatchChangesCodeHostConnectionResolver, error)

	// TODO(campaigns-deprecation)
	// Defined so that BatchSpecResolver can act as a CampaignSpec:
	AppliesToCampaign(ctx context.Context) (BatchChangeResolver, error)
	SupersedingCampaignSpec(context.Context) (BatchSpecResolver, error)
	ViewerCampaignsCodeHosts(ctx context.Context, args *ListViewerCampaignsCodeHostsArgs) (CampaignsCodeHostConnectionResolver, error)
	// This should be removed once we remove batches. It's here so that in
	// the NodeResolver we can have the same resolver, BatchChangeResolver, act
	// as a Campaign and a BatchChange.
	ActAsCampaignSpec() bool
}

type BatchChangeDescriptionResolver interface {
	Name() string
	Description() string
}

type ChangesetApplyPreviewResolver interface {
	ToVisibleChangesetApplyPreview() (VisibleChangesetApplyPreviewResolver, bool)
	ToHiddenChangesetApplyPreview() (HiddenChangesetApplyPreviewResolver, bool)
}

type VisibleChangesetApplyPreviewResolver interface {
	Operations(ctx context.Context) ([]batches.ReconcilerOperation, error)
	Delta(ctx context.Context) (ChangesetSpecDeltaResolver, error)
	Targets() VisibleApplyPreviewTargetsResolver
}

type HiddenChangesetApplyPreviewResolver interface {
	Operations(ctx context.Context) ([]batches.ReconcilerOperation, error)
	Delta(ctx context.Context) (ChangesetSpecDeltaResolver, error)
	Targets() HiddenApplyPreviewTargetsResolver
}

type VisibleApplyPreviewTargetsResolver interface {
	ToVisibleApplyPreviewTargetsAttach() (VisibleApplyPreviewTargetsAttachResolver, bool)
	ToVisibleApplyPreviewTargetsUpdate() (VisibleApplyPreviewTargetsUpdateResolver, bool)
	ToVisibleApplyPreviewTargetsDetach() (VisibleApplyPreviewTargetsDetachResolver, bool)
}

type VisibleApplyPreviewTargetsAttachResolver interface {
	ChangesetSpec(ctx context.Context) (VisibleChangesetSpecResolver, error)
}
type VisibleApplyPreviewTargetsUpdateResolver interface {
	ChangesetSpec(ctx context.Context) (VisibleChangesetSpecResolver, error)
	Changeset(ctx context.Context) (ExternalChangesetResolver, error)
}
type VisibleApplyPreviewTargetsDetachResolver interface {
	Changeset(ctx context.Context) (ExternalChangesetResolver, error)
}

type HiddenApplyPreviewTargetsResolver interface {
	ToHiddenApplyPreviewTargetsAttach() (HiddenApplyPreviewTargetsAttachResolver, bool)
	ToHiddenApplyPreviewTargetsUpdate() (HiddenApplyPreviewTargetsUpdateResolver, bool)
	ToHiddenApplyPreviewTargetsDetach() (HiddenApplyPreviewTargetsDetachResolver, bool)
}

type HiddenApplyPreviewTargetsAttachResolver interface {
	ChangesetSpec(ctx context.Context) (HiddenChangesetSpecResolver, error)
}
type HiddenApplyPreviewTargetsUpdateResolver interface {
	ChangesetSpec(ctx context.Context) (HiddenChangesetSpecResolver, error)
	Changeset(ctx context.Context) (HiddenExternalChangesetResolver, error)
}
type HiddenApplyPreviewTargetsDetachResolver interface {
	Changeset(ctx context.Context) (HiddenExternalChangesetResolver, error)
}

type ChangesetApplyPreviewConnectionStatsResolver interface {
	Push() int32
	Update() int32
	Undraft() int32
	Publish() int32
	PublishDraft() int32
	Sync() int32
	Import() int32
	Close() int32
	Reopen() int32
	Sleep() int32
	Detach() int32

	Added() int32
	Modified() int32
	Removed() int32
}

type ChangesetApplyPreviewConnectionResolver interface {
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
	Nodes(ctx context.Context) ([]ChangesetApplyPreviewResolver, error)
	Stats(ctx context.Context) (ChangesetApplyPreviewConnectionStatsResolver, error)
}

type ChangesetSpecConnectionResolver interface {
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
	Nodes(ctx context.Context) ([]ChangesetSpecResolver, error)
}

type ChangesetSpecResolver interface {
	ID() graphql.ID
	Type() batches.ChangesetSpecDescriptionType
	ExpiresAt() *DateTime

	ToHiddenChangesetSpec() (HiddenChangesetSpecResolver, bool)
	ToVisibleChangesetSpec() (VisibleChangesetSpecResolver, bool)
}

type HiddenChangesetSpecResolver interface {
	ChangesetSpecResolver
}

type VisibleChangesetSpecResolver interface {
	ChangesetSpecResolver

	Description(ctx context.Context) (ChangesetDescription, error)
}

type ChangesetSpecDeltaResolver interface {
	TitleChanged() bool
	BodyChanged() bool
	Undraft() bool
	BaseRefChanged() bool
	DiffChanged() bool
	CommitMessageChanged() bool
	AuthorNameChanged() bool
	AuthorEmailChanged() bool
}

type ChangesetDescription interface {
	ToExistingChangesetReference() (ExistingChangesetReferenceResolver, bool)
	ToGitBranchChangesetDescription() (GitBranchChangesetDescriptionResolver, bool)
}

type ExistingChangesetReferenceResolver interface {
	BaseRepository() *RepositoryResolver
	ExternalID() string
}

type GitBranchChangesetDescriptionResolver interface {
	BaseRepository() *RepositoryResolver
	BaseRef() string
	BaseRev() string

	HeadRepository() *RepositoryResolver
	HeadRef() string

	Title() string
	Body() string

	Diff(ctx context.Context) (PreviewRepositoryComparisonResolver, error)
	DiffStat() *DiffStat

	Commits() []GitCommitDescriptionResolver

	Published() batches.PublishedValue
}

type GitCommitDescriptionResolver interface {
	Message() string
	Subject() string
	Body() *string
	Author() *PersonResolver
	Diff() string
}

type BatchChangesCodeHostConnectionResolver interface {
	Nodes(ctx context.Context) ([]BatchChangesCodeHostResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type BatchChangesCodeHostResolver interface {
	ExternalServiceKind() string
	ExternalServiceURL() string
	RequiresSSH() bool
	Credential() BatchChangesCredentialResolver
}

type BatchChangesCredentialResolver interface {
	ID() graphql.ID
	ExternalServiceKind() string
	ExternalServiceURL() string
	SSHPublicKey() *string
	CreatedAt() DateTime
}

type ChangesetCountsArgs struct {
	From *DateTime
	To   *DateTime
}

type ListChangesetsArgs struct {
	First            int32
	After            *string
	PublicationState *batches.ChangesetPublicationState
	ReconcilerState  *[]batches.ReconcilerState
	ExternalState    *batches.ChangesetExternalState
	State            *batches.ChangesetState
	ReviewState      *batches.ChangesetReviewState
	CheckState       *batches.ChangesetCheckState
	// old
	OnlyPublishedByThisCampaign *bool
	//new
	OnlyPublishedByThisBatchChange *bool
	Search                         *string
}

type BatchChangeResolver interface {
	ID() graphql.ID
	Name() string
	Description() *string
	InitialApplier(ctx context.Context) (*UserResolver, error)
	LastApplier(ctx context.Context) (*UserResolver, error)
	LastAppliedAt() DateTime
	SpecCreator(ctx context.Context) (*UserResolver, error)
	ViewerCanAdminister(ctx context.Context) (bool, error)
	URL(ctx context.Context) (string, error)
	Namespace(ctx context.Context) (n NamespaceResolver, err error)
	CreatedAt() DateTime
	UpdatedAt() DateTime
	ChangesetsStats(ctx context.Context) (ChangesetsStatsResolver, error)
	Changesets(ctx context.Context, args *ListChangesetsArgs) (ChangesetsConnectionResolver, error)
	ChangesetCountsOverTime(ctx context.Context, args *ChangesetCountsArgs) ([]ChangesetCountsResolver, error)
	ClosedAt() *DateTime
	DiffStat(ctx context.Context) (*DiffStat, error)
	CurrentSpec(ctx context.Context) (BatchSpecResolver, error)

	// TODO(campaigns-deprecation): This should be removed once we remove batches.
	// It's here so that in the NodeResolver we can have the same resolver,
	// BatchChangeResolver, act as a Campaign and a BatchChange.
	ActAsCampaign() bool
}

type BatchChangesConnectionResolver interface {
	Nodes(ctx context.Context) ([]BatchChangeResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type ChangesetsStatsResolver interface {
	Retrying() int32
	Failed() int32
	Processing() int32
	Unpublished() int32
	Draft() int32
	Open() int32
	Merged() int32
	Closed() int32
	Deleted() int32
	Total() int32
}

type ChangesetsConnectionResolver interface {
	Nodes(ctx context.Context) ([]ChangesetResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type ChangesetLabelResolver interface {
	Text() string
	Color() string
	Description() *string
}

// ChangesetResolver is the "interface Changeset" in the GraphQL schema and is
// implemented by ExternalChangesetResolver and HiddenExternalChangesetResolver.
type ChangesetResolver interface {
	ID() graphql.ID

	CreatedAt() DateTime
	UpdatedAt() DateTime
	NextSyncAt(ctx context.Context) (*DateTime, error)
	PublicationState() batches.ChangesetPublicationState
	ReconcilerState() batches.ReconcilerState
	ExternalState() *batches.ChangesetExternalState
	State() (batches.ChangesetState, error)
	BatchChanges(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error)

	ToExternalChangeset() (ExternalChangesetResolver, bool)
	ToHiddenExternalChangeset() (HiddenExternalChangesetResolver, bool)

	// TODO(campaigns-deprecation):
	Campaigns(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error)
}

// HiddenExternalChangesetResolver implements only the common interface,
// ChangesetResolver, to not reveal information to unauthorized users.
//
// Theoretically this type is not necessary, but it's easier to understand the
// implementation of the GraphQL schema if we have a mapping between GraphQL
// types and Go types.
type HiddenExternalChangesetResolver interface {
	ChangesetResolver
}

// ExternalChangesetResolver implements the ChangesetResolver interface and
// additional data.
type ExternalChangesetResolver interface {
	ChangesetResolver

	ExternalID() *string
	Title(context.Context) (*string, error)
	Body(context.Context) (*string, error)
	Author() (*PersonResolver, error)
	ExternalURL() (*externallink.Resolver, error)
	ReviewState(context.Context) *batches.ChangesetReviewState
	CheckState() *batches.ChangesetCheckState
	Repository(ctx context.Context) *RepositoryResolver

	Events(ctx context.Context, args *ChangesetEventsConnectionArgs) (ChangesetEventsConnectionResolver, error)
	Diff(ctx context.Context) (RepositoryComparisonInterface, error)
	DiffStat(ctx context.Context) (*DiffStat, error)
	Labels(ctx context.Context) ([]ChangesetLabelResolver, error)

	Error() *string
	SyncerError() *string

	CurrentSpec(ctx context.Context) (VisibleChangesetSpecResolver, error)
}

type ChangesetEventsConnectionResolver interface {
	Nodes(ctx context.Context) ([]ChangesetEventResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type ChangesetEventResolver interface {
	ID() graphql.ID
	Changeset() ExternalChangesetResolver
	CreatedAt() DateTime
}

type ChangesetCountsResolver interface {
	Date() DateTime
	Total() int32
	Merged() int32
	Closed() int32
	Draft() int32
	Open() int32
	OpenApproved() int32
	OpenChangesRequested() int32
	OpenPending() int32
}

var batchChangesOnlyInEnterprise = errors.New("batch changes are only available in enterprise")

type defaultBatchChangesResolver struct{}

var DefaultBatchChangesResolver BatchChangesResolver = defaultBatchChangesResolver{}

// Mutations
// TODO(campaigns-deprecation):
func (defaultBatchChangesResolver) CreateCampaign(ctx context.Context, args *CreateCampaignArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation):
func (defaultBatchChangesResolver) CreateCampaignSpec(ctx context.Context, args *CreateCampaignSpecArgs) (BatchSpecResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation):
func (defaultBatchChangesResolver) ApplyCampaign(ctx context.Context, args *ApplyCampaignArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation):
func (defaultBatchChangesResolver) CloseCampaign(ctx context.Context, args *CloseCampaignArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation):
func (defaultBatchChangesResolver) MoveCampaign(ctx context.Context, args *MoveCampaignArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation):
func (defaultBatchChangesResolver) DeleteCampaign(ctx context.Context, args *DeleteCampaignArgs) (*EmptyResponse, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) CreateCampaignsCredential(ctx context.Context, args *CreateCampaignsCredentialArgs) (CampaignsCredentialResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) DeleteCampaignsCredential(ctx context.Context, args *DeleteCampaignsCredentialArgs) (*EmptyResponse, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) CreateBatchChange(ctx context.Context, args *CreateBatchChangeArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) CreateBatchSpec(ctx context.Context, args *CreateBatchSpecArgs) (BatchSpecResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) ApplyBatchChange(ctx context.Context, args *ApplyBatchChangeArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) CreateChangesetSpec(ctx context.Context, args *CreateChangesetSpecArgs) (ChangesetSpecResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) CloseBatchChange(ctx context.Context, args *CloseBatchChangeArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) MoveBatchChange(ctx context.Context, args *MoveBatchChangeArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) DeleteBatchChange(ctx context.Context, args *DeleteBatchChangeArgs) (*EmptyResponse, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) SyncChangeset(ctx context.Context, args *SyncChangesetArgs) (*EmptyResponse, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) ReenqueueChangeset(ctx context.Context, args *ReenqueueChangesetArgs) (ChangesetResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) CreateBatchChangesCredential(ctx context.Context, args *CreateBatchChangesCredentialArgs) (BatchChangesCredentialResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) DeleteBatchChangesCredential(ctx context.Context, args *DeleteBatchChangesCredentialArgs) (*EmptyResponse, error) {
	return nil, batchChangesOnlyInEnterprise
}

// Queries
// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) Campaigns(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) Campaign(ctx context.Context, args *BatchChangeArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) CampaignSpecByID(ctx context.Context, id graphql.ID) (BatchSpecResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) CampaignByID(ctx context.Context, id graphql.ID) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) CampaignsCredentialByID(ctx context.Context, id graphql.ID) (CampaignsCredentialResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

// TODO(campaigns-deprecation)
func (defaultBatchChangesResolver) CampaignsCodeHosts(ctx context.Context, args *ListCampaignsCodeHostsArgs) (CampaignsCodeHostConnectionResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) BatchChangeByID(ctx context.Context, id graphql.ID) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) BatchChange(ctx context.Context, args *BatchChangeArgs) (BatchChangeResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) BatchChanges(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) BatchSpecByID(ctx context.Context, id graphql.ID) (BatchSpecResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) ChangesetByID(ctx context.Context, id graphql.ID) (ChangesetResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) ChangesetSpecByID(ctx context.Context, id graphql.ID) (ChangesetSpecResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) BatchChangesCredentialByID(ctx context.Context, id graphql.ID) (BatchChangesCredentialResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}

func (defaultBatchChangesResolver) BatchChangesCodeHosts(ctx context.Context, args *ListBatchChangesCodeHostsArgs) (BatchChangesCodeHostConnectionResolver, error) {
	return nil, batchChangesOnlyInEnterprise
}
