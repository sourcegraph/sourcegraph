package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/lib/batches"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/externallink"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
)

type CreateBatchChangeArgs struct {
	BatchSpec         graphql.ID
	PublicationStates *[]ChangesetSpecPublicationStateInput
}

type ApplyBatchChangeArgs struct {
	BatchSpec         graphql.ID
	EnsureBatchChange *graphql.ID
	PublicationStates *[]ChangesetSpecPublicationStateInput
}

type ChangesetSpecPublicationStateInput struct {
	ChangesetSpec    graphql.ID
	PublicationState batches.PublishedValue
}

type ListBatchChangesArgs struct {
	First               int32
	After               *string
	State               *string
	States              *[]string
	ViewerCanAdminister *bool

	Namespace *graphql.ID
	Repo      *graphql.ID
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

type CreateChangesetSpecsArgs struct {
	ChangesetSpecs []string
}

type CreateChangesetSpecArgs struct {
	ChangesetSpec string
}

type CreateBatchSpecArgs struct {
	Namespace graphql.ID

	BatchSpec      string
	ChangesetSpecs []graphql.ID
}

type CreateEmptyBatchChangeArgs struct {
	Namespace graphql.ID
	Name      string
}

type UpsertEmptyBatchChangeArgs struct {
	Namespace graphql.ID
	Name      string
}

type CreateBatchSpecFromRawArgs struct {
	BatchSpec        string
	AllowIgnored     bool
	AllowUnsupported bool
	Execute          bool
	NoCache          bool
	Namespace        graphql.ID
	BatchChange      graphql.ID
}

type ReplaceBatchSpecInputArgs struct {
	PreviousSpec     graphql.ID
	BatchSpec        string
	AllowIgnored     bool
	AllowUnsupported bool
	Execute          bool
	NoCache          bool
}

type UpsertBatchSpecInputArgs = CreateBatchSpecFromRawArgs

type DeleteBatchSpecArgs struct {
	BatchSpec graphql.ID
}

type ExecuteBatchSpecArgs struct {
	BatchSpec graphql.ID
	NoCache   *bool
	AutoApply bool
}

type CancelBatchSpecExecutionArgs struct {
	BatchSpec graphql.ID
}

type CancelBatchSpecWorkspaceExecutionArgs struct {
	BatchSpecWorkspaces []graphql.ID
}

type RetryBatchSpecWorkspaceExecutionArgs struct {
	BatchSpecWorkspaces []graphql.ID
}

type RetryBatchSpecExecutionArgs struct {
	BatchSpec        graphql.ID
	IncludeCompleted bool
}

type EnqueueBatchSpecWorkspaceExecutionArgs struct {
	BatchSpecWorkspaces []graphql.ID
}

type ToggleBatchSpecAutoApplyArgs struct {
	BatchSpec graphql.ID
	Value     bool
}

type ChangesetSpecsConnectionArgs struct {
	First int32
	After *string
}

type ChangesetApplyPreviewConnectionArgs struct {
	First  int32
	After  *string
	Search *string
	// CurrentState is a value of type btypes.ChangesetState.
	CurrentState *string
	// Action is a value of type btypes.ReconcilerOperation.
	Action            *string
	PublicationStates *[]ChangesetSpecPublicationStateInput
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
	User                *graphql.ID
	Username            *string
	Credential          string
}

type DeleteBatchChangesCredentialArgs struct {
	BatchChangesCredential graphql.ID
}

type ListBatchChangesCodeHostsArgs struct {
	First  int32
	After  *string
	UserID *int32
}

type ListViewerBatchChangesCodeHostsArgs struct {
	First                 int32
	After                 *string
	OnlyWithoutCredential bool
	OnlyWithoutWebhooks   bool
}

type BulkOperationBaseArgs struct {
	BatchChange graphql.ID
	Changesets  []graphql.ID
}

type DetachChangesetsArgs struct {
	BulkOperationBaseArgs
}

type ListBatchChangeBulkOperationArgs struct {
	First        int32
	After        *string
	CreatedAfter *gqlutil.DateTime
}

type CreateChangesetCommentsArgs struct {
	BulkOperationBaseArgs
	Body string
}

type ReenqueueChangesetsArgs struct {
	BulkOperationBaseArgs
}

type MergeChangesetsArgs struct {
	BulkOperationBaseArgs
	Squash bool
}

type CloseChangesetsArgs struct {
	BulkOperationBaseArgs
}

type PublishChangesetsArgs struct {
	BulkOperationBaseArgs
	Draft bool
}

type ResolveWorkspacesForBatchSpecArgs struct {
	BatchSpec string
}

type ListImportingChangesetsArgs struct {
	First  int32
	After  *string
	Search *string
}

type BatchSpecWorkspaceStepArgs struct {
	Index int32
}

type BatchChangesResolver interface {
	//
	// MUTATIONS
	//
	CreateBatchChange(ctx context.Context, args *CreateBatchChangeArgs) (BatchChangeResolver, error)
	CreateBatchSpec(ctx context.Context, args *CreateBatchSpecArgs) (BatchSpecResolver, error)
	CreateEmptyBatchChange(ctx context.Context, args *CreateEmptyBatchChangeArgs) (BatchChangeResolver, error)
	UpsertEmptyBatchChange(ctx context.Context, args *UpsertEmptyBatchChangeArgs) (BatchChangeResolver, error)
	CreateBatchSpecFromRaw(ctx context.Context, args *CreateBatchSpecFromRawArgs) (BatchSpecResolver, error)
	ReplaceBatchSpecInput(ctx context.Context, args *ReplaceBatchSpecInputArgs) (BatchSpecResolver, error)
	UpsertBatchSpecInput(ctx context.Context, args *UpsertBatchSpecInputArgs) (BatchSpecResolver, error)
	DeleteBatchSpec(ctx context.Context, args *DeleteBatchSpecArgs) (*EmptyResponse, error)
	ExecuteBatchSpec(ctx context.Context, args *ExecuteBatchSpecArgs) (BatchSpecResolver, error)
	CancelBatchSpecExecution(ctx context.Context, args *CancelBatchSpecExecutionArgs) (BatchSpecResolver, error)
	CancelBatchSpecWorkspaceExecution(ctx context.Context, args *CancelBatchSpecWorkspaceExecutionArgs) (*EmptyResponse, error)
	RetryBatchSpecWorkspaceExecution(ctx context.Context, args *RetryBatchSpecWorkspaceExecutionArgs) (*EmptyResponse, error)
	RetryBatchSpecExecution(ctx context.Context, args *RetryBatchSpecExecutionArgs) (BatchSpecResolver, error)
	EnqueueBatchSpecWorkspaceExecution(ctx context.Context, args *EnqueueBatchSpecWorkspaceExecutionArgs) (*EmptyResponse, error)
	ToggleBatchSpecAutoApply(ctx context.Context, args *ToggleBatchSpecAutoApplyArgs) (BatchSpecResolver, error)

	ApplyBatchChange(ctx context.Context, args *ApplyBatchChangeArgs) (BatchChangeResolver, error)
	CloseBatchChange(ctx context.Context, args *CloseBatchChangeArgs) (BatchChangeResolver, error)
	MoveBatchChange(ctx context.Context, args *MoveBatchChangeArgs) (BatchChangeResolver, error)
	DeleteBatchChange(ctx context.Context, args *DeleteBatchChangeArgs) (*EmptyResponse, error)
	CreateBatchChangesCredential(ctx context.Context, args *CreateBatchChangesCredentialArgs) (BatchChangesCredentialResolver, error)
	DeleteBatchChangesCredential(ctx context.Context, args *DeleteBatchChangesCredentialArgs) (*EmptyResponse, error)

	CreateChangesetSpec(ctx context.Context, args *CreateChangesetSpecArgs) (ChangesetSpecResolver, error)
	CreateChangesetSpecs(ctx context.Context, args *CreateChangesetSpecsArgs) ([]ChangesetSpecResolver, error)
	SyncChangeset(ctx context.Context, args *SyncChangesetArgs) (*EmptyResponse, error)
	ReenqueueChangeset(ctx context.Context, args *ReenqueueChangesetArgs) (ChangesetResolver, error)
	DetachChangesets(ctx context.Context, args *DetachChangesetsArgs) (BulkOperationResolver, error)
	CreateChangesetComments(ctx context.Context, args *CreateChangesetCommentsArgs) (BulkOperationResolver, error)
	ReenqueueChangesets(ctx context.Context, args *ReenqueueChangesetsArgs) (BulkOperationResolver, error)
	MergeChangesets(ctx context.Context, args *MergeChangesetsArgs) (BulkOperationResolver, error)
	CloseChangesets(ctx context.Context, args *CloseChangesetsArgs) (BulkOperationResolver, error)
	PublishChangesets(ctx context.Context, args *PublishChangesetsArgs) (BulkOperationResolver, error)

	// Queries
	BatchChange(ctx context.Context, args *BatchChangeArgs) (BatchChangeResolver, error)
	BatchChanges(cx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error)

	GlobalChangesetsStats(cx context.Context) (GlobalChangesetsStatsResolver, error)

	BatchChangesCodeHosts(ctx context.Context, args *ListBatchChangesCodeHostsArgs) (BatchChangesCodeHostConnectionResolver, error)
	RepoChangesetsStats(ctx context.Context, repo *graphql.ID) (RepoChangesetsStatsResolver, error)
	RepoDiffStat(ctx context.Context, repo *graphql.ID) (*DiffStat, error)

	BatchSpecs(cx context.Context, args *ListBatchSpecArgs) (BatchSpecConnectionResolver, error)
	AvailableBulkOperations(ctx context.Context, args *AvailableBulkOperationsArgs) ([]string, error)

	ResolveWorkspacesForBatchSpec(ctx context.Context, args *ResolveWorkspacesForBatchSpecArgs) ([]ResolvedBatchSpecWorkspaceResolver, error)

	CheckBatchChangesCredential(ctx context.Context, args *CheckBatchChangesCredentialArgs) (*EmptyResponse, error)

	MaxUnlicensedChangesets(ctx context.Context) int32

	GetChangesetsByIDs(ctx context.Context, args *GetChangesetsByIDsArgs) (graphqlutil.SliceConnectionResolver[ChangesetResolver], error)

	NodeResolvers() map[string]NodeByIDFunc
}

type BulkOperationConnectionResolver interface {
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
	Nodes(ctx context.Context) ([]BulkOperationResolver, error)
}

type BulkOperationResolver interface {
	ID() graphql.ID
	Type() (string, error)
	State() string
	Progress() float64
	Errors(ctx context.Context) ([]ChangesetJobErrorResolver, error)
	Initiator(ctx context.Context) (*UserResolver, error)
	ChangesetCount() int32
	CreatedAt() gqlutil.DateTime
	FinishedAt() *gqlutil.DateTime
}

type ChangesetJobErrorResolver interface {
	Changeset() ChangesetResolver
	Error() *string
}

type BatchSpecResolver interface {
	ID() graphql.ID

	OriginalInput() (string, error)
	ParsedInput() (JSONValue, error)
	ChangesetSpecs(ctx context.Context, args *ChangesetSpecsConnectionArgs) (ChangesetSpecConnectionResolver, error)
	ApplyPreview(ctx context.Context, args *ChangesetApplyPreviewConnectionArgs) (ChangesetApplyPreviewConnectionResolver, error)

	Description() BatchChangeDescriptionResolver

	Creator(context.Context) (*UserResolver, error)
	CreatedAt() gqlutil.DateTime
	Namespace(context.Context) (*NamespaceResolver, error)

	ExpiresAt() *gqlutil.DateTime

	ApplyURL(ctx context.Context) (*string, error)

	ViewerCanAdminister(context.Context) (bool, error)

	DiffStat(ctx context.Context) (*DiffStat, error)

	AppliesToBatchChange(ctx context.Context) (BatchChangeResolver, error)

	SupersedingBatchSpec(context.Context) (BatchSpecResolver, error)

	ViewerBatchChangesCodeHosts(ctx context.Context, args *ListViewerBatchChangesCodeHostsArgs) (BatchChangesCodeHostConnectionResolver, error)

	AutoApplyEnabled() bool
	State(context.Context) (string, error)
	StartedAt(ctx context.Context) (*gqlutil.DateTime, error)
	FinishedAt(ctx context.Context) (*gqlutil.DateTime, error)
	FailureMessage(ctx context.Context) (*string, error)
	WorkspaceResolution(ctx context.Context) (BatchSpecWorkspaceResolutionResolver, error)
	ImportingChangesets(ctx context.Context, args *ListImportingChangesetsArgs) (ChangesetSpecConnectionResolver, error)

	AllowIgnored() *bool
	AllowUnsupported() *bool
	NoCache() *bool

	ViewerCanRetry(context.Context) (bool, error)

	Source() string

	Files(ctx context.Context, args *ListBatchSpecWorkspaceFilesArgs) (BatchSpecWorkspaceFileConnectionResolver, error)
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
	// Operations returns a slice of btypes.ReconcilerOperation.
	Operations(ctx context.Context) ([]string, error)
	Delta(ctx context.Context) (ChangesetSpecDeltaResolver, error)
	Targets() VisibleApplyPreviewTargetsResolver
}

type HiddenChangesetApplyPreviewResolver interface {
	// Operations returns a slice of btypes.ReconcilerOperation.
	Operations(ctx context.Context) ([]string, error)
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
	Archive() int32
	Reattach() int32

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
	// Type returns a value of type btypes.ChangesetSpecDescriptionType.
	Type() string
	ExpiresAt() *gqlutil.DateTime

	ToHiddenChangesetSpec() (HiddenChangesetSpecResolver, bool)
	ToVisibleChangesetSpec() (VisibleChangesetSpecResolver, bool)
}

type HiddenChangesetSpecResolver interface {
	ChangesetSpecResolver
}

type VisibleChangesetSpecResolver interface {
	ChangesetSpecResolver

	Description(ctx context.Context) (ChangesetDescription, error)
	Workspace(ctx context.Context) (BatchSpecWorkspaceResolver, error)

	ForkTarget() ForkTargetInterface
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

	HeadRef() string

	Title() string
	Body() string

	Diff(ctx context.Context) (PreviewRepositoryComparisonResolver, error)
	DiffStat() *DiffStat

	Commits() []GitCommitDescriptionResolver

	Published() *batches.PublishedValue
}

type GitCommitDescriptionResolver interface {
	Message() string
	Subject() string
	Body() *string
	Author() *PersonResolver
	Diff() string
}

type ForkTargetInterface interface {
	PushUser() bool
	Namespace() *string
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
	RequiresUsername() bool
	SupportsCommitSigning() bool
	HasWebhooks() bool
	Credential() BatchChangesCredentialResolver
	CommitSigningConfiguration(context.Context) (CommitSigningConfigResolver, error)
}

type BatchChangesCredentialResolver interface {
	ID() graphql.ID
	ExternalServiceKind() string
	ExternalServiceURL() string
	IsGitHubApp() bool
	GitHubAppID() int
	SSHPublicKey(ctx context.Context) (*string, error)
	CreatedAt() gqlutil.DateTime
	IsSiteCredential() bool
}

// Only GitHubApps are supported for commit signing for now.
type CommitSigningConfigResolver interface {
	ToGitHubApp() (GitHubAppResolver, bool)
}

type ChangesetCountsArgs struct {
	From            *gqlutil.DateTime
	To              *gqlutil.DateTime
	IncludeArchived bool
}

type ListChangesetsArgs struct {
	First int32
	After *string
	// PublicationState is a value of type *btypes.ChangesetPublicationState.
	PublicationState *string
	// ReconcilerState is a slice of *btypes.ReconcilerState.
	ReconcilerState *[]string
	// ExternalState is a value of type *btypes.ChangesetExternalState.
	ExternalState *string
	// State is a value of type *btypes.ChangesetState.
	State *string
	// onlyClosable indicates the user only wants open and draft changesets to be returned
	OnlyClosable *bool
	// ReviewState is a value of type *btypes.ChangesetReviewState.
	ReviewState *string
	// CheckState is a value of type *btypes.ChangesetCheckState.
	CheckState                     *string
	OnlyPublishedByThisBatchChange *bool
	Search                         *string

	OnlyArchived bool
	Repo         *graphql.ID
}

type ListBatchSpecArgs struct {
	First                       int32
	After                       *string
	IncludeLocallyExecutedSpecs *bool
	ExcludeEmptySpecs           *bool
}

type ListBatchSpecWorkspaceFilesArgs struct {
	First int32
	After *string
}

type AvailableBulkOperationsArgs struct {
	BulkOperationBaseArgs
}

type CheckBatchChangesCredentialArgs struct {
	BatchChangesCredential graphql.ID
}

type GetChangesetsByIDsArgs struct {
	BulkOperationBaseArgs
}

type ListWorkspacesArgs struct {
	First   int32
	After   *string
	OrderBy *string
	Search  *string
	State   *string
}

type ListRecentlyCompletedWorkspacesArgs struct {
	First int32
	After *string
}

type ListRecentlyErroredWorkspacesArgs struct {
	First int32
	After *string
}

type BatchSpecWorkspaceStepOutputLinesArgs struct {
	First int32
	After *string
}

type BatchChangeResolver interface {
	ID() graphql.ID
	Name() string
	Description() *string
	State() string
	Creator(ctx context.Context) (*UserResolver, error)
	LastApplier(ctx context.Context) (*UserResolver, error)
	LastAppliedAt() *gqlutil.DateTime
	ViewerCanAdminister(ctx context.Context) (bool, error)
	URL(ctx context.Context) (string, error)
	Namespace(ctx context.Context) (n NamespaceResolver, err error)
	CreatedAt() gqlutil.DateTime
	UpdatedAt() gqlutil.DateTime
	ChangesetsStats(ctx context.Context) (ChangesetsStatsResolver, error)
	Changesets(ctx context.Context, args *ListChangesetsArgs) (ChangesetsConnectionResolver, error)
	ChangesetCountsOverTime(ctx context.Context, args *ChangesetCountsArgs) ([]ChangesetCountsResolver, error)
	ClosedAt() *gqlutil.DateTime
	DiffStat(ctx context.Context) (*DiffStat, error)
	CurrentSpec(ctx context.Context) (BatchSpecResolver, error)
	BulkOperations(ctx context.Context, args *ListBatchChangeBulkOperationArgs) (BulkOperationConnectionResolver, error)
	BatchSpecs(ctx context.Context, args *ListBatchSpecArgs) (BatchSpecConnectionResolver, error)
}

type BatchChangesConnectionResolver interface {
	Nodes(ctx context.Context) ([]BatchChangeResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type BatchSpecConnectionResolver interface {
	Nodes(ctx context.Context) ([]BatchSpecResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type BatchSpecWorkspaceFileConnectionResolver interface {
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
	Nodes(ctx context.Context) ([]BatchWorkspaceFileResolver, error)
}

type BatchWorkspaceFileResolver interface {
	ID() graphql.ID
	ModifiedAt() gqlutil.DateTime
	CreatedAt() gqlutil.DateTime
	UpdatedAt() gqlutil.DateTime

	Path() string
	Name() string
	IsDirectory() bool
	Content(ctx context.Context, args *GitTreeContentPageArgs) (string, error)
	ByteSize(ctx context.Context) (int32, error)
	TotalLines(ctx context.Context) (int32, error)
	Binary(ctx context.Context) (bool, error)
	RichHTML(ctx context.Context, args *GitTreeContentPageArgs) (string, error)
	URL(ctx context.Context) (string, error)
	CanonicalURL() string
	ChangelistURL(ctx context.Context) (*string, error)
	ExternalURLs(ctx context.Context) ([]*externallink.Resolver, error)
	Highlight(ctx context.Context, args *HighlightArgs) (*HighlightedFileResolver, error)
	Languages() ([]string, error)

	ToGitBlob() (*GitBlobResolver, bool)
	ToVirtualFile() (*VirtualFileResolver, bool)
	ToBatchSpecWorkspaceFile() (BatchWorkspaceFileResolver, bool)
}

type CommonChangesetsStatsResolver interface {
	Unpublished() int32
	Draft() int32
	Open() int32
	Merged() int32
	Closed() int32
	Total() int32
}

type RepoChangesetsStatsResolver interface {
	CommonChangesetsStatsResolver
}

type GlobalChangesetsStatsResolver interface {
	CommonChangesetsStatsResolver
}

type ChangesetsStatsResolver interface {
	CommonChangesetsStatsResolver
	Retrying() int32
	Failed() int32
	Scheduled() int32
	Processing() int32
	Deleted() int32
	Archived() int32
	IsCompleted() bool
	PercentComplete() int32
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

	CreatedAt() gqlutil.DateTime
	UpdatedAt() gqlutil.DateTime
	NextSyncAt(ctx context.Context) (*gqlutil.DateTime, error)
	// State returns a value of type *btypes.ChangesetState.
	State() string
	BatchChanges(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error)

	ToExternalChangeset() (ExternalChangesetResolver, bool)
	ToHiddenExternalChangeset() (HiddenExternalChangesetResolver, bool)
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

	OwnedByBatchChange() *graphql.ID

	// If the changeset is a fork, this corresponds to the namespace of the fork.
	ForkNamespace() *string
	// If the changeset is a fork, this corresponds to the name of the fork.
	ForkName() *string

	CommitVerification(context.Context) (CommitVerificationResolver, error)

	// ReviewState returns a value of type *btypes.ChangesetReviewState.
	ReviewState(context.Context) *string
	// CheckState returns a value of type *btypes.ChangesetCheckState.
	CheckState() *string
	Repository(ctx context.Context) *RepositoryResolver

	Events(ctx context.Context, args *ChangesetEventsConnectionArgs) (ChangesetEventsConnectionResolver, error)
	Diff(ctx context.Context) (RepositoryComparisonInterface, error)
	DiffStat(ctx context.Context) (*DiffStat, error)
	Labels(ctx context.Context) ([]ChangesetLabelResolver, error)

	Error() *string
	SyncerError() *string
	ScheduleEstimateAt(ctx context.Context) (*gqlutil.DateTime, error)

	CurrentSpec(ctx context.Context) (VisibleChangesetSpecResolver, error)
}

// Only GitHubApps are supported for commit signing for now.
type CommitVerificationResolver interface {
	ToGitHubCommitVerification() (GitHubCommitVerificationResolver, bool)
}

type GitHubCommitVerificationResolver interface {
	Verified() bool
	Reason() string
	Signature() string
	Payload() string
}

type ChangesetEventsConnectionResolver interface {
	Nodes(ctx context.Context) ([]ChangesetEventResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type ChangesetEventResolver interface {
	ID() graphql.ID
	Changeset() ExternalChangesetResolver
	CreatedAt() gqlutil.DateTime
}

type ChangesetCountsResolver interface {
	Date() gqlutil.DateTime
	Total() int32
	Merged() int32
	Closed() int32
	Draft() int32
	Open() int32
	OpenApproved() int32
	OpenChangesRequested() int32
	OpenPending() int32
}

type BatchSpecWorkspaceResolutionResolver interface {
	State() string
	StartedAt() *gqlutil.DateTime
	FinishedAt() *gqlutil.DateTime
	FailureMessage() *string

	Workspaces(ctx context.Context, args *ListWorkspacesArgs) (BatchSpecWorkspaceConnectionResolver, error)

	RecentlyCompleted(ctx context.Context, args *ListRecentlyCompletedWorkspacesArgs) BatchSpecWorkspaceConnectionResolver
	RecentlyErrored(ctx context.Context, args *ListRecentlyErroredWorkspacesArgs) BatchSpecWorkspaceConnectionResolver
}

type BatchSpecWorkspaceConnectionResolver interface {
	Nodes(ctx context.Context) ([]BatchSpecWorkspaceResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
	Stats(ctx context.Context) (BatchSpecWorkspacesStatsResolver, error)
}

type BatchSpecWorkspacesStatsResolver interface {
	Errored() int32
	Completed() int32
	Processing() int32
	Queued() int32
	Ignored() int32
}

type BatchSpecWorkspaceResolver interface {
	ID() graphql.ID

	State() string
	QueuedAt() *gqlutil.DateTime
	StartedAt() *gqlutil.DateTime
	FinishedAt() *gqlutil.DateTime
	CachedResultFound() bool
	StepCacheResultCount() int32
	BatchSpec(ctx context.Context) (BatchSpecResolver, error)
	OnlyFetchWorkspace() bool
	Ignored() bool
	Unsupported() bool
	DiffStat(ctx context.Context) (*DiffStat, error)
	PlaceInQueue() *int32
	PlaceInGlobalQueue() *int32

	ToHiddenBatchSpecWorkspace() (HiddenBatchSpecWorkspaceResolver, bool)
	ToVisibleBatchSpecWorkspace() (VisibleBatchSpecWorkspaceResolver, bool)
}

type HiddenBatchSpecWorkspaceResolver interface {
	BatchSpecWorkspaceResolver
}

type VisibleBatchSpecWorkspaceResolver interface {
	BatchSpecWorkspaceResolver

	FailureMessage() *string
	Stages() BatchSpecWorkspaceStagesResolver
	Repository(ctx context.Context) (*RepositoryResolver, error)
	Branch(ctx context.Context) (*GitRefResolver, error)
	Path() string
	Step(args BatchSpecWorkspaceStepArgs) (BatchSpecWorkspaceStepResolver, error)
	Steps() ([]BatchSpecWorkspaceStepResolver, error)
	SearchResultPaths() []string
	ChangesetSpecs(ctx context.Context) (*[]VisibleChangesetSpecResolver, error)
	Executor(ctx context.Context) (*ExecutorResolver, error)
}

type ResolvedBatchSpecWorkspaceResolver interface {
	OnlyFetchWorkspace() bool
	Ignored() bool
	Unsupported() bool
	Repository() *RepositoryResolver
	Branch(ctx context.Context) *GitRefResolver
	Path() string
	SearchResultPaths() []string
}

type BatchSpecWorkspaceStagesResolver interface {
	Setup() []ExecutionLogEntryResolver
	SrcExec() []ExecutionLogEntryResolver
	Teardown() []ExecutionLogEntryResolver
}

type BatchSpecWorkspaceStepOutputLineConnectionResolver interface {
	TotalCount() (int32, error)
	PageInfo() (*graphqlutil.PageInfo, error)
	Nodes() ([]string, error)
}

type BatchSpecWorkspaceStepResolver interface {
	Number() int32
	Run() string
	Container() string
	IfCondition() *string
	CachedResultFound() bool
	Skipped() bool
	OutputLines(ctx context.Context, args *BatchSpecWorkspaceStepOutputLinesArgs) BatchSpecWorkspaceStepOutputLineConnectionResolver

	StartedAt() *gqlutil.DateTime
	FinishedAt() *gqlutil.DateTime

	ExitCode() *int32
	Environment() ([]BatchSpecWorkspaceEnvironmentVariableResolver, error)
	OutputVariables() *[]BatchSpecWorkspaceOutputVariableResolver

	DiffStat(ctx context.Context) (*DiffStat, error)
	Diff(ctx context.Context) (PreviewRepositoryComparisonResolver, error)
}

type BatchSpecWorkspaceEnvironmentVariableResolver interface {
	Name() string
	Value() *string
}

type BatchSpecWorkspaceOutputVariableResolver interface {
	Name() string
	Value() JSONValue
}
