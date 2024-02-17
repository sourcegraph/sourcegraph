package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/graph-gophers/graphql-go"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v2"

	sglog "github.com/sourcegraph/log"

	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/batches/global"
	bgql "github.com/sourcegraph/sourcegraph/internal/batches/graphql"
	"github.com/sourcegraph/sourcegraph/internal/batches/sources"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/batches/webhooks"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	extsvcauth "github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/types"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	"github.com/sourcegraph/sourcegraph/lib/batches/template"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ErrNameNotUnique is returned by CreateEmptyBatchChange if the combination of name and
// namespace provided are already used by another batch change.
var ErrNameNotUnique = errors.New("a batch change with this name already exists in this namespace")

// New returns a Service.
func New(store *store.Store) *Service {
	return NewWithClock(store, store.Clock())
}

// NewWithClock returns a Service the given clock used
// to generate timestamps.
func NewWithClock(store *store.Store, clock func() time.Time) *Service {
	logger := sglog.Scoped("batches.Service")
	svc := &Service{
		logger: logger,
		store:  store,
		sourcer: sources.NewSourcer(httpcli.NewExternalClientFactory(
			httpcli.NewLoggingMiddleware(logger.Scoped("sourcer")),
		)),
		clock:      clock,
		operations: newOperations(store.ObservationCtx()),
	}

	return svc
}

type Service struct {
	logger     sglog.Logger
	store      *store.Store
	sourcer    sources.Sourcer
	operations *operations
	clock      func() time.Time
}

type operations struct {
	createBatchSpec                      *observation.Operation
	createBatchSpecFromRaw               *observation.Operation
	executeBatchSpec                     *observation.Operation
	cancelBatchSpec                      *observation.Operation
	replaceBatchSpecInput                *observation.Operation
	upsertBatchSpecInput                 *observation.Operation
	retryBatchSpecWorkspaces             *observation.Operation
	retryBatchSpecExecution              *observation.Operation
	createChangesetSpec                  *observation.Operation
	getBatchChangeMatchingBatchSpec      *observation.Operation
	getNewestBatchSpec                   *observation.Operation
	moveBatchChange                      *observation.Operation
	closeBatchChange                     *observation.Operation
	deleteBatchChange                    *observation.Operation
	enqueueChangesetSync                 *observation.Operation
	reenqueueChangeset                   *observation.Operation
	checkNamespaceAccess                 *observation.Operation
	fetchUsernameForBitbucketServerToken *observation.Operation
	validateAuthenticator                *observation.Operation
	createChangesetJobs                  *observation.Operation
	applyBatchChange                     *observation.Operation
	reconcileBatchChange                 *observation.Operation
	validateChangesetSpecs               *observation.Operation
}

var (
	singletonOperations *operations
	operationsOnce      sync.Once
)

// newOperations generates a singleton of the operations struct.
// TODO: We should create one per observationCtx.
func newOperations(observationCtx *observation.Context) *operations {
	operationsOnce.Do(func() {
		m := metrics.NewREDMetrics(
			observationCtx.Registerer,
			"batches_service",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)

		op := func(name string) *observation.Operation {
			return observationCtx.Operation(observation.Op{
				Name:              fmt.Sprintf("batches.service.%s", name),
				MetricLabelValues: []string{name},
				Metrics:           m,
			})
		}

		singletonOperations = &operations{
			createBatchSpec:                      op("CreateBatchSpec"),
			createBatchSpecFromRaw:               op("CreateBatchSpecFromRaw"),
			executeBatchSpec:                     op("ExecuteBatchSpec"),
			cancelBatchSpec:                      op("CancelBatchSpec"),
			replaceBatchSpecInput:                op("ReplaceBatchSpecInput"),
			upsertBatchSpecInput:                 op("UpsertBatchSpecInput"),
			retryBatchSpecWorkspaces:             op("RetryBatchSpecWorkspaces"),
			retryBatchSpecExecution:              op("RetryBatchSpecExecution"),
			createChangesetSpec:                  op("CreateChangesetSpec"),
			getBatchChangeMatchingBatchSpec:      op("GetBatchChangeMatchingBatchSpec"),
			getNewestBatchSpec:                   op("GetNewestBatchSpec"),
			moveBatchChange:                      op("MoveBatchChange"),
			closeBatchChange:                     op("CloseBatchChange"),
			deleteBatchChange:                    op("DeleteBatchChange"),
			enqueueChangesetSync:                 op("EnqueueChangesetSync"),
			reenqueueChangeset:                   op("ReenqueueChangeset"),
			checkNamespaceAccess:                 op("CheckNamespaceAccess"),
			fetchUsernameForBitbucketServerToken: op("FetchUsernameForBitbucketServerToken"),
			validateAuthenticator:                op("ValidateAuthenticator"),
			createChangesetJobs:                  op("CreateChangesetJobs"),
			applyBatchChange:                     op("ApplyBatchChange"),
			reconcileBatchChange:                 op("ReconcileBatchChange"),
			validateChangesetSpecs:               op("ValidateChangesetSpecs"),
		}
	})

	return singletonOperations
}

// WithStore returns a copy of the Service with its store attribute set to the
// given Store.
func (s *Service) WithStore(store *store.Store) *Service {
	return &Service{logger: s.logger, store: store, sourcer: s.sourcer, clock: s.clock, operations: s.operations}
}

// checkViewerCanAdminister checks if the current user can administer a batch change in the context of its creator and the namespace it belongs to, if the namespace is an organization.
//
// If it belongs to a user (orgID == 0), the user can administer a batch change only if they are its creator or a site admin.
// If it belongs to an org (orgID != 0), we check the org settings for the `orgs.allMembersBatchChangesAdmin` field:
//   - If true, the user can administer a batch change if they are a member of that org or a site admin.
//   - If false, the user can administer a batch change only if they are its creator or a site admin.
//
// allowOrgMemberAccess is a boolean argument used to indicate when we simply want to check for org namespace access
func (s *Service) checkViewerCanAdminister(ctx context.Context, orgID, creatorID int32, allowOrgMemberAccess bool) error {
	db := s.store.DatabaseDB()
	if orgID != 0 {
		// We retrieve the setting for `orgs.allMembersBatchChangesAdmin` from Settings instead of SiteConfig because
		// multiple orgs could have different values for the field. Because it's an org-specific field, it's added
		// as part of org Settings.
		settings, err := db.Settings().GetLatest(ctx, api.SettingsSubject{Org: &orgID})
		if err != nil {
			return err
		}

		var allMembersBatchChangesAdmin bool
		if settings != nil {
			var orgSettings schema.Settings
			if err := jsonc.Unmarshal(settings.Contents, &orgSettings); err != nil {
				return err
			}

			if orgSettings.OrgsAllMembersBatchChangesAdmin != nil {
				allMembersBatchChangesAdmin = *orgSettings.OrgsAllMembersBatchChangesAdmin
			}
		}

		if allMembersBatchChangesAdmin || allowOrgMemberAccess {
			if err := auth.CheckOrgAccessOrSiteAdmin(ctx, db, orgID); err != nil {
				return err
			}
			return nil
		}
	}

	// 🚨 SECURITY: Unless the org setting override is true, only the author of the batch change or a site admin should be able to perform this operation.
	if err := auth.CheckSiteAdminOrSameUser(ctx, db, creatorID); err != nil {
		return err
	}

	return nil
}

// CheckViewerCanAdminister checks whether the current user can administer a
// batch change in the given namespace.
func (s *Service) CheckViewerCanAdminister(ctx context.Context, namespaceUserID, namespaceOrgID int32) (bool, error) {
	err := s.checkViewerCanAdminister(ctx, namespaceOrgID, namespaceUserID, true)
	if err != nil && (err == auth.ErrNotAnOrgMember || errcode.IsUnauthorized(err)) {
		// These errors indicate that the viewer is valid, but that they simply
		// don't have access to administer this batch change. We don't want to
		// propagate that error to the caller.
		return false, nil
	}
	return err == nil, err
}

type CreateEmptyBatchChangeOpts struct {
	NamespaceUserID int32
	NamespaceOrgID  int32

	Name string
}

// CreateEmptyBatchChange creates a new batch change with an empty batch spec. It enforces
// namespace permissions of the caller and validates that the combination of name +
// namespace is unique.
func (s *Service) CreateEmptyBatchChange(ctx context.Context, opts CreateEmptyBatchChangeOpts) (batchChange *btypes.BatchChange, err error) {
	// 🚨 SECURITY: Check whether the current user has access to either one of
	// the namespaces.
	err = s.CheckNamespaceAccess(ctx, opts.NamespaceUserID, opts.NamespaceOrgID)
	if err != nil {
		return nil, err
	}

	// Construct and parse the batch spec YAML of just the provided name to validate the
	// pattern of the name is okay
	rawSpec, err := yaml.Marshal(struct {
		Name string `yaml:"name"`
	}{Name: opts.Name})
	if err != nil {
		return nil, errors.Wrap(err, "marshalling name")
	}
	// TODO: Should name require a minimum length?
	spec, err := batcheslib.ParseBatchSpec(rawSpec)
	if err != nil {
		return nil, err
	}

	actor := sgactor.FromContext(ctx)
	// Actor is guaranteed to be set here, because CheckNamespaceAccess above enforces it.

	batchSpec := &btypes.BatchSpec{
		RawSpec:         string(rawSpec),
		Spec:            spec,
		NamespaceUserID: opts.NamespaceUserID,
		NamespaceOrgID:  opts.NamespaceOrgID,
		UserID:          actor.UID,
		CreatedFromRaw:  true,
	}

	// The combination of name + namespace must be unique
	// TODO: Should name be case-insensitive unique? i.e. should "foo" and "Foo"
	// be considered unique?
	batchChange, err = s.GetBatchChangeMatchingBatchSpec(ctx, batchSpec)
	if err != nil {
		return nil, err
	}
	if batchChange != nil {
		return nil, ErrNameNotUnique
	}

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	if err := tx.CreateBatchSpec(ctx, batchSpec); err != nil {
		return nil, err
	}

	batchChange = &btypes.BatchChange{
		Name:            opts.Name,
		NamespaceUserID: opts.NamespaceUserID,
		NamespaceOrgID:  opts.NamespaceOrgID,
		BatchSpecID:     batchSpec.ID,
		CreatorID:       actor.UID,
	}
	if err := tx.CreateBatchChange(ctx, batchChange); err != nil {
		return nil, err
	}

	return batchChange, nil
}

type UpsertEmptyBatchChangeOpts struct {
	NamespaceUserID int32
	NamespaceOrgID  int32

	Name string
}

// UpsertEmptyBatchChange creates a new batch change with an empty batch spec if a batch change with that name doesn't exist,
// otherwise it updates the existing batch change with an empty batch spec.
// It enforces namespace permissions of the caller and validates that the combination of name +
// namespace is unique.
func (s *Service) UpsertEmptyBatchChange(ctx context.Context, opts UpsertEmptyBatchChangeOpts) (*btypes.BatchChange, error) {
	// Check whether the current user has access to either one of the namespaces.
	err := s.CheckNamespaceAccess(ctx, opts.NamespaceUserID, opts.NamespaceOrgID)
	if err != nil {
		return nil, err
	}

	// Construct and parse the batch spec YAML of just the provided name to validate the
	// pattern of the name is okay
	rawSpec, err := yaml.Marshal(struct {
		Name string `yaml:"name"`
	}{Name: opts.Name})
	if err != nil {
		return nil, errors.Wrap(err, "marshalling name")
	}

	spec, err := batcheslib.ParseBatchSpec(rawSpec)
	if err != nil {
		return nil, err
	}

	_, err = template.ValidateBatchSpecTemplate(string(rawSpec))
	if err != nil {
		return nil, err
	}

	actor := sgactor.FromContext(ctx)
	// Actor is guaranteed to be set here, because CheckNamespaceAccess above enforces it.

	batchSpec := &btypes.BatchSpec{
		RawSpec:         string(rawSpec),
		Spec:            spec,
		NamespaceUserID: opts.NamespaceUserID,
		NamespaceOrgID:  opts.NamespaceOrgID,
		UserID:          actor.UID,
		CreatedFromRaw:  true,
	}

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	if err := tx.CreateBatchSpec(ctx, batchSpec); err != nil {
		return nil, err
	}

	batchChange := &btypes.BatchChange{
		Name:            opts.Name,
		NamespaceUserID: opts.NamespaceUserID,
		NamespaceOrgID:  opts.NamespaceOrgID,
		BatchSpecID:     batchSpec.ID,
		CreatorID:       actor.UID,
	}

	err = tx.UpsertBatchChange(ctx, batchChange)

	if err != nil {
		return nil, err
	}

	return batchChange, nil
}

type CreateBatchSpecOpts struct {
	RawSpec string `json:"raw_spec"`

	NamespaceUserID int32 `json:"namespace_user_id"`
	NamespaceOrgID  int32 `json:"namespace_org_id"`

	ChangesetSpecRandIDs []string `json:"changeset_spec_rand_ids"`
}

// CreateBatchSpec creates the BatchSpec.
func (s *Service) CreateBatchSpec(ctx context.Context, opts CreateBatchSpecOpts) (spec *btypes.BatchSpec, err error) {
	ctx, _, endObservation := s.operations.createBatchSpec.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("changesetSpecs", len(opts.ChangesetSpecRandIDs)),
	}})
	defer endObservation(1, observation.Args{})

	// TODO move license check logic from resolver to here

	spec, err = btypes.NewBatchSpecFromRaw(opts.RawSpec)
	if err != nil {
		return nil, err
	}

	// Check whether the current user has access to either one of the namespaces.
	err = s.CheckNamespaceAccess(ctx, opts.NamespaceUserID, opts.NamespaceOrgID)
	if err != nil {
		return nil, err
	}
	spec.NamespaceOrgID = opts.NamespaceOrgID
	spec.NamespaceUserID = opts.NamespaceUserID
	a := sgactor.FromContext(ctx)
	spec.UserID = a.UID

	if len(opts.ChangesetSpecRandIDs) == 0 {
		return spec, s.store.CreateBatchSpec(ctx, spec)
	}

	listOpts := store.ListChangesetSpecsOpts{RandIDs: opts.ChangesetSpecRandIDs}
	cs, _, err := s.store.ListChangesetSpecs(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: database.Repos.GetRepoIDsSet uses the authzFilter under the hood and
	// filters out repositories that the user doesn't have access to.
	accessibleReposByID, err := s.store.Repos().GetReposSetByIDs(ctx, cs.RepoIDs()...)
	if err != nil {
		return nil, err
	}

	byRandID := make(map[string]*btypes.ChangesetSpec, len(cs))
	for _, changesetSpec := range cs {
		// 🚨 SECURITY: We return an error if the user doesn't have access to one
		// of the repositories associated with a ChangesetSpec.
		if _, ok := accessibleReposByID[changesetSpec.BaseRepoID]; !ok {
			return nil, &database.RepoNotFoundErr{ID: changesetSpec.BaseRepoID}
		}
		byRandID[changesetSpec.RandID] = changesetSpec
	}

	// Check if a changesetSpec was not found
	for _, randID := range opts.ChangesetSpecRandIDs {
		if _, ok := byRandID[randID]; !ok {
			return nil, &changesetSpecNotFoundErr{RandID: randID}
		}
	}

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	if err := tx.CreateBatchSpec(ctx, spec); err != nil {
		return nil, err
	}

	csIDs := make([]int64, 0, len(cs))
	for _, c := range cs {
		csIDs = append(csIDs, c.ID)
	}
	if err := tx.UpdateChangesetSpecBatchSpecID(ctx, csIDs, spec.ID); err != nil {
		return nil, err
	}

	return spec, nil
}

type CreateBatchSpecFromRawOpts struct {
	RawSpec string

	NamespaceUserID int32
	NamespaceOrgID  int32

	AllowIgnored     bool
	AllowUnsupported bool
	NoCache          bool

	BatchChange int64
}

// CreateBatchSpecFromRaw creates the BatchSpec.
func (s *Service) CreateBatchSpecFromRaw(ctx context.Context, opts CreateBatchSpecFromRawOpts) (spec *btypes.BatchSpec, err error) {
	ctx, _, endObservation := s.operations.createBatchSpecFromRaw.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Bool("allowIgnored", opts.AllowIgnored),
		attribute.Bool("allowUnsupported", opts.AllowUnsupported),
	}})
	defer endObservation(1, observation.Args{})

	spec, err = btypes.NewBatchSpecFromRaw(opts.RawSpec)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Check whether the current user has access to either one of
	// the namespaces.
	err = s.CheckNamespaceAccess(ctx, opts.NamespaceUserID, opts.NamespaceOrgID)
	if err != nil {
		return nil, err
	}
	spec.NamespaceOrgID = opts.NamespaceOrgID
	spec.NamespaceUserID = opts.NamespaceUserID
	// Actor is guaranteed to be set here, because CheckNamespaceAccess above enforces it.
	a := sgactor.FromContext(ctx)
	spec.UserID = a.UID

	spec.BatchChangeID = opts.BatchChange

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	if opts.BatchChange != 0 {
		batchChange, err := tx.GetBatchChange(ctx, store.GetBatchChangeOpts{
			ID: opts.BatchChange,
		})
		if err != nil {
			return nil, err
		}

		// 🚨 SECURITY: Check whether the current user has access to the
		// namespace of the batch change. Note that this may be different to the
		// user-controlled namespace options — we need to check before changing
		// anything in the database!
		if err := s.CheckNamespaceAccess(ctx, batchChange.NamespaceUserID, batchChange.NamespaceOrgID); err != nil {
			return nil, err
		}
	}

	return spec, s.createBatchSpecForExecution(ctx, tx, createBatchSpecForExecutionOpts{
		spec:             spec,
		allowIgnored:     opts.AllowIgnored,
		allowUnsupported: opts.AllowUnsupported,
		noCache:          opts.NoCache,
	})
}

type createBatchSpecForExecutionOpts struct {
	spec             *btypes.BatchSpec
	allowUnsupported bool
	allowIgnored     bool
	noCache          bool
}

// createBatchSpecForExecution persists the given BatchSpec in the given
// transaction, possibly creating ChangesetSpecs if the spec contains
// importChangesets statements, and finally creating a BatchSpecResolutionJob.
func (s *Service) createBatchSpecForExecution(ctx context.Context, tx *store.Store, opts createBatchSpecForExecutionOpts) error {
	opts.spec.CreatedFromRaw = true
	opts.spec.AllowIgnored = opts.allowIgnored
	opts.spec.AllowUnsupported = opts.allowUnsupported
	opts.spec.NoCache = opts.noCache

	if err := tx.CreateBatchSpec(ctx, opts.spec); err != nil {
		return err
	}

	// Return spec and enqueue resolution
	return tx.CreateBatchSpecResolutionJob(ctx, &btypes.BatchSpecResolutionJob{
		State:       btypes.BatchSpecResolutionJobStateQueued,
		BatchSpecID: opts.spec.ID,
		InitiatorID: opts.spec.UserID,
	})
}

type ErrBatchSpecResolutionErrored struct {
	failureMessage *string
}

func (e ErrBatchSpecResolutionErrored) Error() string {
	if e.failureMessage != nil && *e.failureMessage != "" {
		return fmt.Sprintf("cannot execute batch spec, workspace resolution failed: %s", *e.failureMessage)
	}
	return "cannot execute batch spec, workspace resolution failed"
}

var ErrBatchSpecResolutionIncomplete = errors.New("cannot execute batch spec, workspaces still being resolved")

type ExecuteBatchSpecOpts struct {
	BatchSpecRandID string
	NoCache         *bool
}

// ExecuteBatchSpec creates BatchSpecWorkspaceExecutionJobs for every created
// BatchSpecWorkspace.
//
// It returns an error if the batchSpecWorkspaceResolutionJob didn't finish
// successfully.
func (s *Service) ExecuteBatchSpec(ctx context.Context, opts ExecuteBatchSpecOpts) (batchSpec *btypes.BatchSpec, err error) {
	ctx, _, endObservation := s.operations.executeBatchSpec.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("BatchSpecRandID", opts.BatchSpecRandID),
	}})
	defer endObservation(1, observation.Args{})

	batchSpec, err = s.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{RandID: opts.BatchSpecRandID})
	if err != nil {
		return nil, err
	}

	// Check whether the current user has access to either one of the namespaces.
	err = s.CheckNamespaceAccess(ctx, batchSpec.NamespaceUserID, batchSpec.NamespaceOrgID)
	if err != nil {
		return nil, err
	}

	// TODO: In the future we want to block here until the resolution is done
	// and only then check whether it failed or not.
	//
	// TODO: We also want to check that whether there was already an
	// execution.
	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	resolutionJob, err := tx.GetBatchSpecResolutionJob(ctx, store.GetBatchSpecResolutionJobOpts{BatchSpecID: batchSpec.ID})
	if err != nil {
		return nil, err
	}

	switch resolutionJob.State {
	case btypes.BatchSpecResolutionJobStateErrored, btypes.BatchSpecResolutionJobStateFailed:
		return nil, ErrBatchSpecResolutionErrored{resolutionJob.FailureMessage}

	case btypes.BatchSpecResolutionJobStateCompleted:
		// Continue below the switch statement.

	default:
		return nil, ErrBatchSpecResolutionIncomplete
	}

	// If the batch spec nocache flag doesn't match what's been provided in the API,
	// update the batch spec state in the db.
	if opts.NoCache != nil && batchSpec.NoCache != *opts.NoCache {
		batchSpec.NoCache = *opts.NoCache
		if err := tx.UpdateBatchSpec(ctx, batchSpec); err != nil {
			return nil, err
		}
	}

	// Disable caching if requested.
	if batchSpec.NoCache {
		err = tx.DisableBatchSpecWorkspaceExecutionCache(ctx, batchSpec.ID)
		if err != nil {
			return nil, err
		}
	}

	err = tx.CreateBatchSpecWorkspaceExecutionJobs(ctx, batchSpec.ID)
	if err != nil {
		return nil, err
	}

	err = tx.MarkSkippedBatchSpecWorkspaces(ctx, batchSpec.ID)
	if err != nil {
		return nil, err
	}

	return batchSpec, nil
}

var ErrBatchSpecNotCancelable = errors.New("batch spec is not in cancelable state")

type CancelBatchSpecOpts struct {
	BatchSpecRandID string
}

// CancelBatchSpec cancels all BatchSpecWorkspaceExecutionJobs associated with
// the BatchSpec.
func (s *Service) CancelBatchSpec(ctx context.Context, opts CancelBatchSpecOpts) (batchSpec *btypes.BatchSpec, err error) {
	ctx, _, endObservation := s.operations.cancelBatchSpec.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("BatchSpecRandID", opts.BatchSpecRandID),
	}})
	defer endObservation(1, observation.Args{})

	batchSpec, err = s.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{RandID: opts.BatchSpecRandID})
	if err != nil {
		return nil, err
	}

	// Check whether the current user has access to either one of the namespaces.
	err = s.CheckNamespaceAccess(ctx, batchSpec.NamespaceUserID, batchSpec.NamespaceOrgID)
	if err != nil {
		return nil, err
	}

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	state, err := computeBatchSpecState(ctx, tx, batchSpec)
	if err != nil {
		return nil, err
	}

	if !state.Cancelable() {
		return nil, ErrBatchSpecNotCancelable
	}

	cancelOpts := store.CancelBatchSpecWorkspaceExecutionJobsOpts{BatchSpecID: batchSpec.ID}
	_, err = tx.CancelBatchSpecWorkspaceExecutionJobs(ctx, cancelOpts)
	return batchSpec, err
}

type ReplaceBatchSpecInputOpts struct {
	BatchSpecRandID  string
	RawSpec          string
	AllowIgnored     bool
	AllowUnsupported bool
	NoCache          bool
}

// ReplaceBatchSpecInput creates BatchSpecWorkspaceExecutionJobs for every created
// BatchSpecWorkspace.
//
// It returns an error if the batchSpecWorkspaceResolutionJob didn't finish
// successfully.
func (s *Service) ReplaceBatchSpecInput(ctx context.Context, opts ReplaceBatchSpecInputOpts) (batchSpec *btypes.BatchSpec, err error) {
	ctx, _, endObservation := s.operations.replaceBatchSpecInput.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	// Before we hit the database, validate the new spec.
	newSpec, err := btypes.NewBatchSpecFromRaw(opts.RawSpec)
	if err != nil {
		return nil, err
	}

	// Also validate that the batch spec only uses known templating variables and
	// functions. If we get an error here that it's invalid, we also want to surface that
	// error to the UI.
	_, err = template.ValidateBatchSpecTemplate(opts.RawSpec)
	if err != nil {
		return nil, err
	}

	// Make sure the user has access.
	batchSpec, err = s.store.GetBatchSpec(ctx, store.GetBatchSpecOpts{RandID: opts.BatchSpecRandID})
	if err != nil {
		return nil, err
	}

	// Check whether the current user has access to either one of the namespaces.
	err = s.CheckNamespaceAccess(ctx, batchSpec.NamespaceUserID, batchSpec.NamespaceOrgID)
	if err != nil {
		return nil, err
	}

	// Start transaction.
	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	if err = replaceBatchSpec(ctx, tx, batchSpec, newSpec); err != nil {
		return nil, err
	}

	return newSpec, s.createBatchSpecForExecution(ctx, tx, createBatchSpecForExecutionOpts{
		spec:             newSpec,
		allowUnsupported: opts.AllowUnsupported,
		allowIgnored:     opts.AllowIgnored,
		noCache:          opts.NoCache,
	})
}

type UpsertBatchSpecInputOpts = CreateBatchSpecFromRawOpts

func (s *Service) UpsertBatchSpecInput(ctx context.Context, opts UpsertBatchSpecInputOpts) (spec *btypes.BatchSpec, err error) {
	ctx, _, endObservation := s.operations.upsertBatchSpecInput.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Bool("allowIgnored", opts.AllowIgnored),
		attribute.Bool("allowUnsupported", opts.AllowUnsupported),
	}})
	defer endObservation(1, observation.Args{})

	spec, err = btypes.NewBatchSpecFromRaw(opts.RawSpec)
	if err != nil {
		return nil, errors.Wrap(err, "parsing batch spec")
	}

	_, err = template.ValidateBatchSpecTemplate(opts.RawSpec)
	if err != nil {
		return nil, err
	}

	// Check whether the current user has access to either one of the namespaces.
	err = s.CheckNamespaceAccess(ctx, opts.NamespaceUserID, opts.NamespaceOrgID)
	if err != nil {
		return nil, errors.Wrap(err, "checking namespace access")
	}
	spec.NamespaceOrgID = opts.NamespaceOrgID
	spec.NamespaceUserID = opts.NamespaceUserID
	// Actor is guaranteed to be set here, because CheckNamespaceAccess above enforces it.
	a := sgactor.FromContext(ctx)
	spec.UserID = a.UID

	// Start transaction.
	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "starting transaction")
	}
	defer func() { err = tx.Done(err) }()

	// Figure out if there's a pre-existing batch spec to replace.
	old, err := s.store.GetNewestBatchSpec(ctx, store.GetNewestBatchSpecOpts{
		NamespaceUserID: opts.NamespaceUserID,
		NamespaceOrgID:  opts.NamespaceOrgID,
		UserID:          a.UID,
		Name:            spec.Spec.Name,
	})
	if err != nil && err != store.ErrNoResults {
		return nil, errors.Wrap(err, "checking for a previous batch spec")
	}

	if err == nil {
		// We're replacing an old batch spec.
		if err = replaceBatchSpec(ctx, tx, old, spec); err != nil {
			return nil, errors.Wrap(err, "replacing the previous batch spec")
		}
	}

	return spec, s.createBatchSpecForExecution(ctx, tx, createBatchSpecForExecutionOpts{
		spec:             spec,
		allowIgnored:     opts.AllowIgnored,
		allowUnsupported: opts.AllowUnsupported,
		noCache:          opts.NoCache,
	})
}

// replaceBatchSpec removes a previous batch spec and copies its random ID,
// namespace, and user IDs to the new spec.
//
// Callers are otherwise responsible for newSpec containing expected values,
// such as the name.
func replaceBatchSpec(ctx context.Context, tx *store.Store, oldSpec, newSpec *btypes.BatchSpec) error {
	// Delete the previous batch spec, which should delete
	// - batch_spec_resolution_jobs
	// - batch_spec_workspaces
	// - batch_spec_workspace_execution_jobs
	// - changeset_specs
	// associated with it
	if err := tx.DeleteBatchSpec(ctx, oldSpec.ID); err != nil {
		return err
	}

	// We keep the RandID so the user-visible GraphQL ID is stable
	newSpec.RandID = oldSpec.RandID

	newSpec.NamespaceOrgID = oldSpec.NamespaceOrgID
	newSpec.NamespaceUserID = oldSpec.NamespaceUserID
	newSpec.UserID = oldSpec.UserID
	newSpec.BatchChangeID = oldSpec.BatchChangeID

	return nil
}

// CreateChangesetSpec validates the given raw spec input and creates the ChangesetSpec.
func (s *Service) CreateChangesetSpec(ctx context.Context, rawSpec string, userID int32) (spec *btypes.ChangesetSpec, err error) {
	ctx, _, endObservation := s.operations.createChangesetSpec.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	spec, err = btypes.NewChangesetSpecFromRaw(rawSpec)
	if err != nil {
		return nil, err
	}
	spec.UserID = userID

	// 🚨 SECURITY: We use database.Repos.Get to check whether the user has access to
	// the repository or not.
	if _, err = s.store.Repos().Get(ctx, spec.BaseRepoID); err != nil {
		return nil, err
	}

	return spec, s.store.CreateChangesetSpec(ctx, spec)
}

// CreateChangesetSpecs validates the given raw spec inputs and creates the ChangesetSpecs.
func (s *Service) CreateChangesetSpecs(ctx context.Context, rawSpecs []string, userID int32) (specs []*btypes.ChangesetSpec, err error) {
	ctx, _, endObservation := s.operations.createChangesetSpec.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	specs = make([]*btypes.ChangesetSpec, len(rawSpecs))

	for i, rawSpec := range rawSpecs {
		spec, err := btypes.NewChangesetSpecFromRaw(rawSpec)
		if err != nil {
			return nil, err
		}

		// 🚨 SECURITY: We use database.Repos.Get to check whether the user has access to
		// the repository or not.
		if _, err = s.store.Repos().Get(ctx, spec.BaseRepoID); err != nil {
			return nil, err
		}

		spec.UserID = userID
		specs[i] = spec
	}

	return specs, s.store.CreateChangesetSpec(ctx, specs...)
}

// changesetSpecNotFoundErr is returned by CreateBatchSpec if a
// ChangesetSpec with the given RandID doesn't exist.
// It fulfills the interface required by errcode.IsNotFound.
type changesetSpecNotFoundErr struct {
	RandID string
}

func (e *changesetSpecNotFoundErr) Error() string {
	if e.RandID != "" {
		return fmt.Sprintf("changesetSpec not found: id=%s", e.RandID)
	}
	return "changesetSpec not found"
}

func (e *changesetSpecNotFoundErr) NotFound() bool { return true }

// GetBatchChangeMatchingBatchSpec returns the batch change that the BatchSpec
// applies to, if that BatchChange already exists.
// If it doesn't exist yet, both return values are nil.
// It accepts a *store.Store so that it can be used inside a transaction.
func (s *Service) GetBatchChangeMatchingBatchSpec(ctx context.Context, spec *btypes.BatchSpec) (_ *btypes.BatchChange, err error) {
	ctx, _, endObservation := s.operations.getBatchChangeMatchingBatchSpec.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	var opts store.GetBatchChangeOpts

	// if the batch spec is linked to a batch change, we want to take advantage of querying for the
	// batch change using the primary key as it's faster.
	if spec.BatchChangeID != 0 {
		opts = store.GetBatchChangeOpts{ID: spec.BatchChangeID}
	} else {
		opts = store.GetBatchChangeOpts{
			Name:            spec.Spec.Name,
			NamespaceUserID: spec.NamespaceUserID,
			NamespaceOrgID:  spec.NamespaceOrgID,
		}
	}

	batchChange, err := s.store.GetBatchChange(ctx, opts)
	if err != nil {
		if err != store.ErrNoResults {
			return nil, err
		}
		err = nil
	}
	return batchChange, err
}

// GetNewestBatchSpec returns the newest batch spec that matches the given
// spec's namespace and name and is owned by the given user, or nil if none is found.
func (s *Service) GetNewestBatchSpec(ctx context.Context, tx *store.Store, spec *btypes.BatchSpec, userID int32) (_ *btypes.BatchSpec, err error) {
	ctx, _, endObservation := s.operations.getNewestBatchSpec.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	opts := store.GetNewestBatchSpecOpts{
		UserID:          userID,
		NamespaceUserID: spec.NamespaceUserID,
		NamespaceOrgID:  spec.NamespaceOrgID,
		Name:            spec.Spec.Name,
	}

	newest, err := tx.GetNewestBatchSpec(ctx, opts)
	if err != nil {
		if err != store.ErrNoResults {
			return nil, err
		}
		return nil, nil
	}

	return newest, nil
}

type MoveBatchChangeOpts struct {
	BatchChangeID int64

	NewName string

	NewNamespaceUserID int32
	NewNamespaceOrgID  int32
}

func (o MoveBatchChangeOpts) String() string {
	return fmt.Sprintf(
		"BatchChangeID %d, NewName %q, NewNamespaceUserID %d, NewNamespaceOrgID %d",
		o.BatchChangeID,
		o.NewName,
		o.NewNamespaceUserID,
		o.NewNamespaceOrgID,
	)
}

// MoveBatchChange moves the batch change from one namespace to another and/or renames
// the batch change.
func (s *Service) MoveBatchChange(ctx context.Context, opts MoveBatchChangeOpts) (batchChange *btypes.BatchChange, err error) {
	ctx, _, endObservation := s.operations.moveBatchChange.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	batchChange, err = tx.GetBatchChange(ctx, store.GetBatchChangeOpts{ID: opts.BatchChangeID})
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Only the Author of the batch change can move it.
	// If the batch change belongs to an org namespace, org members will be able to access it if
	// the `orgs.allMembersBatchChangesAdmin` setting is true.
	if err := s.checkViewerCanAdminister(ctx, batchChange.NamespaceOrgID, batchChange.CreatorID, false); err != nil {
		return nil, err
	}
	// Check if current user has access to target namespace if set.
	if opts.NewNamespaceOrgID != 0 || opts.NewNamespaceUserID != 0 {
		err = s.CheckNamespaceAccess(ctx, opts.NewNamespaceUserID, opts.NewNamespaceOrgID)
		if err != nil {
			return nil, err
		}
	}

	if opts.NewNamespaceOrgID != 0 {
		batchChange.NamespaceOrgID = opts.NewNamespaceOrgID
		batchChange.NamespaceUserID = 0
	} else if opts.NewNamespaceUserID != 0 {
		batchChange.NamespaceUserID = opts.NewNamespaceUserID
		batchChange.NamespaceOrgID = 0
	}

	if opts.NewName != "" {
		batchChange.Name = opts.NewName
	}

	return batchChange, tx.UpdateBatchChange(ctx, batchChange)
}

// CloseBatchChange closes the BatchChange with the given ID if it has not been closed yet.
func (s *Service) CloseBatchChange(ctx context.Context, id int64, closeChangesets bool) (batchChange *btypes.BatchChange, err error) {
	ctx, _, endObservation := s.operations.closeBatchChange.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	batchChange, err = s.store.GetBatchChange(ctx, store.GetBatchChangeOpts{ID: id})
	if err != nil {
		return nil, errors.Wrap(err, "getting batch change")
	}

	if batchChange.Closed() {
		return batchChange, nil
	}

	if err := s.checkViewerCanAdminister(ctx, batchChange.NamespaceOrgID, batchChange.CreatorID, false); err != nil {
		return nil, err
	}

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Done(err)
		// We only enqueue the webhook after the transaction succeeds. If it fails and all
		// the DB changes are rolled back, the batch change will still be open. This
		// ensures we only send a webhook when the batch change is *actually* closed, and
		// ensures the batch change payload in the webhook is up-to-date as well.
		if err != nil {
			s.enqueueBatchChangeWebhook(ctx, webhooks.BatchChangeClose, bgql.MarshalBatchChangeID(batchChange.ID))
		}
	}()

	batchChange.ClosedAt = s.clock()
	if err := tx.UpdateBatchChange(ctx, batchChange); err != nil {
		return nil, err
	}

	if !closeChangesets {
		return batchChange, nil
	}

	// At this point we don't know which changesets have ExternalStateOpen,
	// since some might still be being processed in the background by the
	// reconciler.
	// So enqueue all, except the ones that are completed and closed/merged,
	// for closing. If after being processed they're not open, it'll be a noop.
	if err := tx.EnqueueChangesetsToClose(ctx, batchChange.ID); err != nil {
		return nil, err
	}

	return batchChange, nil
}

// DeleteBatchChange deletes the BatchChange with the given ID if it hasn't been
// deleted yet.
func (s *Service) DeleteBatchChange(ctx context.Context, id int64) (err error) {
	ctx, _, endObservation := s.operations.deleteBatchChange.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	batchChange, err := s.store.GetBatchChange(ctx, store.GetBatchChangeOpts{ID: id})
	if err != nil {
		return err
	}

	if err := s.checkViewerCanAdminister(ctx, batchChange.NamespaceOrgID, batchChange.CreatorID, false); err != nil {
		return err
	}

	// We enqueue this webhook before actually deleting the batch change, so that the
	// payload contains the last state of the batch change before it was hard-deleted.
	s.enqueueBatchChangeWebhook(ctx, webhooks.BatchChangeDelete, bgql.MarshalBatchChangeID(batchChange.ID))
	return s.store.DeleteBatchChange(ctx, id)
}

// EnqueueChangesetSync loads the given changeset from the database, checks
// whether the actor in the context has permission to enqueue a sync and then
// enqueues a sync by calling the repoupdater client.
func (s *Service) EnqueueChangesetSync(ctx context.Context, id int64) (err error) {
	ctx, _, endObservation := s.operations.enqueueChangesetSync.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	// Check for existence of changeset so we don't swallow that error.
	changeset, err := s.store.GetChangeset(ctx, store.GetChangesetOpts{ID: id})
	if err != nil {
		return err
	}

	// 🚨 SECURITY: We use database.Repos.Get to check whether the user has access to
	// the repository or not.
	if _, err = s.store.Repos().Get(ctx, changeset.RepoID); err != nil {
		return err
	}

	batchChanges, _, err := s.store.ListBatchChanges(ctx, store.ListBatchChangesOpts{ChangesetID: id})
	if err != nil {
		return err
	}

	// Check whether the user has admin rights for one of the batches.
	var (
		authErr        error
		hasAdminRights bool
	)

	for _, c := range batchChanges {
		err := s.checkViewerCanAdminister(ctx, c.NamespaceOrgID, c.CreatorID, false)
		if err != nil {
			authErr = err
		} else {
			hasAdminRights = true
			break
		}
	}

	if !hasAdminRights {
		return authErr
	}

	if err := repoupdater.DefaultClient.EnqueueChangesetSync(ctx, []int64{id}); err != nil {
		return err
	}

	return nil
}

// ReenqueueChangeset loads the given changeset from the database, checks
// whether the actor in the context has permission to enqueue a reconciler run and then
// enqueues it by calling ResetReconcilerState.
func (s *Service) ReenqueueChangeset(ctx context.Context, id int64) (changeset *btypes.Changeset, repo *types.Repo, err error) {
	ctx, _, endObservation := s.operations.reenqueueChangeset.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	changeset, err = s.store.GetChangeset(ctx, store.GetChangesetOpts{ID: id})
	if err != nil {
		return nil, nil, err
	}

	// 🚨 SECURITY: We use database.Repos.Get to check whether the user has access to
	// the repository or not.
	repo, err = s.store.Repos().Get(ctx, changeset.RepoID)
	if err != nil {
		return nil, nil, err
	}

	attachedBatchChanges, _, err := s.store.ListBatchChanges(ctx, store.ListBatchChangesOpts{ChangesetID: id})
	if err != nil {
		return nil, nil, err
	}

	// Check whether the user has admin rights for one of the batches.
	var (
		authErr        error
		hasAdminRights bool
	)

	for _, c := range attachedBatchChanges {
		err := s.checkViewerCanAdminister(ctx, c.NamespaceOrgID, c.CreatorID, false)
		if err != nil {
			authErr = err
		} else {
			hasAdminRights = true
			break
		}
	}

	if !hasAdminRights {
		return nil, nil, authErr
	}

	if err := s.store.EnqueueChangeset(ctx, changeset, global.DefaultReconcilerEnqueueState(), btypes.ReconcilerStateFailed); err != nil {
		return nil, nil, err
	}

	return changeset, repo, nil
}

// CheckNamespaceAccess checks whether the current user in the ctx has access
// to either the user ID or the org ID as a namespace.
// If the userID is non-zero that will be checked. Otherwise the org ID will be
// checked.
// If the current user is an admin, true will be returned.
// Otherwise it checks whether the current user _is_ the namespace user or has
// access to the namespace org.
// If both values are zero, an error is returned.
func (s *Service) CheckNamespaceAccess(ctx context.Context, namespaceUserID, namespaceOrgID int32) (err error) {
	return s.checkNamespaceAccessWithDB(ctx, s.store.DatabaseDB(), namespaceUserID, namespaceOrgID)
}

func (s *Service) checkNamespaceAccessWithDB(ctx context.Context, db database.DB, namespaceUserID, namespaceOrgID int32) (err error) {
	if namespaceOrgID != 0 {
		return auth.CheckOrgAccessOrSiteAdmin(ctx, db, namespaceOrgID)
	} else if namespaceUserID != 0 {
		return auth.CheckSiteAdminOrSameUser(ctx, db, namespaceUserID)
	} else {
		return ErrNoNamespace
	}
}

// ErrNoNamespace is returned by checkNamespaceAccess if no valid namespace ID is given.
var ErrNoNamespace = errors.New("no namespace given")

// FetchUsernameForBitbucketServerToken fetches the username associated with a
// Bitbucket server token.
//
// We need the username in order to use the token as the password in a HTTP
// BasicAuth username/password pair used by gitserver to push commits.
//
// In order to not require from users to type in their BitbucketServer username
// we only ask for a token and then use that token to talk to the
// BitbucketServer API and get their username.
//
// Since Bitbucket sends the username as a header in REST responses, we can
// take it from there and complete the UserCredential.
func (s *Service) FetchUsernameForBitbucketServerToken(ctx context.Context, externalServiceID, externalServiceType, token string) (_ string, err error) {
	ctx, _, endObservation := s.operations.fetchUsernameForBitbucketServerToken.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	// Get a changeset source for the external service and use the given authenticator.
	css, err := s.sourcer.ForExternalService(ctx, s.store, &extsvcauth.OAuthBearerToken{Token: token}, store.GetExternalServiceIDsOpts{
		ExternalServiceType: externalServiceType,
		ExternalServiceID:   externalServiceID,
	})
	if err != nil {
		return "", err
	}

	usernameSource, ok := css.(usernameSource)
	if !ok {
		return "", errors.New("external service source doesn't implement AuthenticatedUsername")
	}

	return usernameSource.AuthenticatedUsername(ctx)
}

// A usernameSource can fetch the username associated with the credentials used
// by the Source.
// It's only used by FetchUsernameForBitbucketServerToken.
type usernameSource interface {
	// AuthenticatedUsername makes a request to the code host to fetch the
	// username associated with the credentials.
	// If no username could be determined an error is returned.
	AuthenticatedUsername(ctx context.Context) (string, error)
}

var _ usernameSource = &sources.BitbucketServerSource{}

// ValidateAuthenticator creates a ChangesetSource, configures it with the given
// authenticator and validates it can correctly access the remote server.
func (s *Service) ValidateAuthenticator(ctx context.Context, externalServiceID, externalServiceType string, a extsvcauth.Authenticator) (err error) {
	ctx, _, endObservation := s.operations.validateAuthenticator.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	if Mocks.ValidateAuthenticator != nil {
		return Mocks.ValidateAuthenticator(ctx, externalServiceID, externalServiceType, a)
	}

	// Get a changeset source for the external service and use the given authenticator.
	css, err := s.sourcer.ForExternalService(ctx, s.store, a, store.GetExternalServiceIDsOpts{
		ExternalServiceType: externalServiceType,
		ExternalServiceID:   externalServiceID,
	})
	if err != nil {
		return err
	}

	if err := css.ValidateAuthenticator(ctx); err != nil {
		return err
	}
	return nil
}

// ErrChangesetsForJobNotFound can be returned by (*Service).CreateChangesetJobs
// if the number of changesets returned from the database doesn't match the
// number if IDs passed in. That can happen if some of the changesets are not
// published.
var ErrChangesetsForJobNotFound = errors.New("some changesets could not be found")

// CreateChangesetJobs creates one changeset job for each given Changeset in the
// given BatchChange, checking whether the actor in the context has permission to
// trigger a job, and enqueues it.
func (s *Service) CreateChangesetJobs(ctx context.Context, batchChangeID int64, ids []int64, jobType btypes.ChangesetJobType, payload any, listOpts store.ListChangesetsOpts) (bulkGroupID string, err error) {
	ctx, _, endObservation := s.operations.createChangesetJobs.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	// Load the BatchChange to check for write permissions.
	batchChange, err := s.store.GetBatchChange(ctx, store.GetBatchChangeOpts{ID: batchChangeID})
	if err != nil {
		return bulkGroupID, errors.Wrap(err, "loading batch change")
	}

	// If the batch change belongs to an org namespace, org members will be able to access it if
	// the `orgs.allMembersBatchChangesAdmin` setting is true.
	if err := s.checkViewerCanAdminister(ctx, batchChange.NamespaceOrgID, batchChange.CreatorID, false); err != nil {
		return bulkGroupID, err
	}

	// Construct list options.
	opts := listOpts
	opts.IDs = ids
	opts.BatchChangeID = batchChangeID
	// We only want to allow changesets the user has access to.
	opts.EnforceAuthz = true
	cs, _, err := s.store.ListChangesets(ctx, opts)
	if err != nil {
		return bulkGroupID, errors.Wrap(err, "listing changesets")
	}

	if len(cs) != len(ids) {
		return bulkGroupID, ErrChangesetsForJobNotFound
	}

	bulkGroupID, err = store.RandomID()
	if err != nil {
		return bulkGroupID, errors.Wrap(err, "creating bulkGroupID failed")
	}

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return bulkGroupID, errors.Wrap(err, "starting transaction")
	}
	defer func() { err = tx.Done(err) }()

	userID := sgactor.FromContext(ctx).UID
	changesetJobs := make([]*btypes.ChangesetJob, 0, len(cs))
	for _, changeset := range cs {
		changesetJobs = append(changesetJobs, &btypes.ChangesetJob{
			BulkGroup:     bulkGroupID,
			ChangesetID:   changeset.ID,
			BatchChangeID: batchChangeID,
			UserID:        userID,
			State:         btypes.ChangesetJobStateQueued,
			JobType:       jobType,
			Payload:       payload,
		})
	}

	// Bulk-insert all changeset jobs into the database.
	if err := tx.CreateChangesetJob(ctx, changesetJobs...); err != nil {
		return bulkGroupID, errors.Wrap(err, "creating changeset jobs")
	}

	return bulkGroupID, nil
}

// ValidateChangesetSpecs checks whether the given BachSpec has ChangesetSpecs
// that would publish to the same branch in the same repository.
// If the return value is nil, then the BatchSpec is valid.
func (s *Service) ValidateChangesetSpecs(ctx context.Context, batchSpecID int64) error {
	// We don't use `err` here to distinguish between errors we want to trace
	// as such and the validation errors that we want to return without logging
	// them as errors.
	var nonValidationErr error
	ctx, _, endObservation := s.operations.validateChangesetSpecs.With(ctx, &nonValidationErr, observation.Args{})
	defer endObservation(1, observation.Args{})

	conflicts, nonValidationErr := s.store.ListChangesetSpecsWithConflictingHeadRef(ctx, batchSpecID)
	if nonValidationErr != nil {
		return nonValidationErr
	}

	if len(conflicts) == 0 {
		return nil
	}

	repoIDs := make([]api.RepoID, 0, len(conflicts))
	for _, c := range conflicts {
		repoIDs = append(repoIDs, c.RepoID)
	}

	// 🚨 SECURITY: database.Repos.GetRepoIDsSet uses the authzFilter under the hood and
	// filters out repositories that the user doesn't have access to.
	accessibleReposByID, nonValidationErr := s.store.Repos().GetReposSetByIDs(ctx, repoIDs...)
	if nonValidationErr != nil {
		return nonValidationErr
	}

	var errs changesetSpecHeadRefConflictErrs
	for _, c := range conflicts {
		conflictErr := &changesetSpecHeadRefConflict{count: c.Count, headRef: c.HeadRef}

		// If the user has access to the repository, we can show the name
		if repo, ok := accessibleReposByID[c.RepoID]; ok {
			conflictErr.repo = repo
		}
		errs = append(errs, conflictErr)
	}
	return errs
}

type changesetSpecHeadRefConflict struct {
	repo    *types.Repo
	count   int
	headRef string
}

func (c changesetSpecHeadRefConflict) Error() string {
	if c.repo != nil {
		return fmt.Sprintf("%d changeset specs in %s use the same branch: %s", c.count, c.repo.Name, c.headRef)
	}
	return fmt.Sprintf("%d changeset specs in the same repository use the same branch: %s", c.count, c.headRef)
}

// changesetSpecHeadRefConflictErrs represents a set of changesetSpecHeadRefConflict and
// implements `Error` to render the errors nicely.
type changesetSpecHeadRefConflictErrs []*changesetSpecHeadRefConflict

func (es changesetSpecHeadRefConflictErrs) Error() string {
	if len(es) == 1 {
		return fmt.Sprintf("Validating changeset specs resulted in an error:\n* %s\n", es[0])
	}

	points := make([]string, len(es))
	for i, err := range es {
		points[i] = fmt.Sprintf("* %s", err)
	}

	return fmt.Sprintf(
		"%d errors when validating changeset specs:\n%s\n",
		len(es), strings.Join(points, "\n"))
}

func (s *Service) LoadBatchSpecStats(ctx context.Context, batchSpec *btypes.BatchSpec) (btypes.BatchSpecStats, error) {
	return loadBatchSpecStats(ctx, s.store, batchSpec)
}

func loadBatchSpecStats(ctx context.Context, bstore *store.Store, spec *btypes.BatchSpec) (btypes.BatchSpecStats, error) {
	statsMap, err := bstore.GetBatchSpecStats(ctx, []int64{spec.ID})
	if err != nil {
		return btypes.BatchSpecStats{}, err
	}

	stats, ok := statsMap[spec.ID]
	if !ok {
		return btypes.BatchSpecStats{}, store.ErrNoResults
	}
	return stats, nil
}

func computeBatchSpecState(ctx context.Context, s *store.Store, spec *btypes.BatchSpec) (btypes.BatchSpecState, error) {
	stats, err := loadBatchSpecStats(ctx, s, spec)
	if err != nil {
		return "", err
	}

	return btypes.ComputeBatchSpecState(spec, stats), nil
}

// RetryBatchSpecWorkspaces retries the BatchSpecWorkspaceExecutionJobs
// attached to the given BatchSpecWorkspaces.
// It only deletes changeset_specs created by workspaces. The imported changeset_specs
// will not be altered.
func (s *Service) RetryBatchSpecWorkspaces(ctx context.Context, workspaceIDs []int64) (err error) {
	ctx, _, endObservation := s.operations.retryBatchSpecWorkspaces.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	if len(workspaceIDs) == 0 {
		return errors.New("no workspaces specified")
	}

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// Load workspaces
	workspaces, _, err := tx.ListBatchSpecWorkspaces(ctx, store.ListBatchSpecWorkspacesOpts{IDs: workspaceIDs})
	if err != nil {
		return errors.Wrap(err, "loading batch spec workspaces")
	}

	var batchSpecID int64 = -1
	var changesetSpecIDs []int64

	for _, w := range workspaces {
		// Check that batch spec is the same
		if batchSpecID != -1 && w.BatchSpecID != batchSpecID {
			return errors.New("workspaces do not belong to the same batch spec")
		}

		batchSpecID = w.BatchSpecID
		changesetSpecIDs = append(changesetSpecIDs, w.ChangesetSpecIDs...)
	}

	// Make sure the user has access to retry it.
	batchSpec, err := tx.GetBatchSpec(ctx, store.GetBatchSpecOpts{ID: batchSpecID})
	if err != nil {
		return errors.Wrap(err, "loading batch spec")
	}

	// Check whether the current user has access to either one of the namespaces.
	err = s.checkNamespaceAccessWithDB(ctx, tx.DatabaseDB(), batchSpec.NamespaceUserID, batchSpec.NamespaceOrgID)
	if err != nil {
		return errors.Wrap(err, "checking whether user has access")
	}

	// Check that batch spec is not applied
	batchChange, err := tx.GetBatchChange(ctx, store.GetBatchChangeOpts{BatchSpecID: batchSpecID})
	if err != nil && err != store.ErrNoResults {
		return errors.Wrap(err, "checking whether batch spec has been applied")
	}
	if err == nil && !batchChange.IsDraft() {
		return errors.New("batch spec already applied")
	}

	// Load jobs and check their state
	jobs, err := tx.ListBatchSpecWorkspaceExecutionJobs(ctx, store.ListBatchSpecWorkspaceExecutionJobsOpts{
		BatchSpecWorkspaceIDs: workspaceIDs,
	})
	if err != nil {
		return errors.Wrap(err, "loading batch spec workspace execution jobs")
	}

	var errs error
	jobIDs := make([]int64, len(jobs))

	for i, j := range jobs {
		if !j.State.Retryable() {
			errs = errors.Append(errs, errors.Newf("job %d not retryable", j.ID))
		}
		jobIDs[i] = j.ID
	}

	if err := errs; err != nil {
		return err
	}

	// Delete the old execution jobs.
	if err := tx.DeleteBatchSpecWorkspaceExecutionJobs(ctx, store.DeleteBatchSpecWorkspaceExecutionJobsOpts{IDs: jobIDs}); err != nil {
		return errors.Wrap(err, "deleting batch spec workspace execution jobs")
	}

	// Delete the changeset specs they have created.
	if len(changesetSpecIDs) > 0 {
		if err := tx.DeleteChangesetSpecs(ctx, store.DeleteChangesetSpecsOpts{IDs: changesetSpecIDs}); err != nil {
			return errors.Wrap(err, "deleting batch spec workspace changeset specs")
		}
	}

	// Create new jobs
	if err := tx.CreateBatchSpecWorkspaceExecutionJobsForWorkspaces(ctx, workspaceIDs); err != nil {
		return errors.Wrap(err, "creating new batch spec workspace execution jobs")
	}

	return nil
}

// ErrRetryNonFinal is returned by RetryBatchSpecExecution if the batch spec is
// not in a final state.
var ErrRetryNonFinal = errors.New("batch spec execution has not finished; retry not possible")

type RetryBatchSpecExecutionOpts struct {
	BatchSpecRandID string

	IncludeCompleted bool
}

// RetryBatchSpecExecution retries all BatchSpecWorkspaceExecutionJobs
// attached to the given BatchSpec.
// It only deletes changeset_specs created by workspaces. The imported changeset_specs
// will not be altered.
func (s *Service) RetryBatchSpecExecution(ctx context.Context, opts RetryBatchSpecExecutionOpts) (err error) {
	ctx, _, endObservation := s.operations.retryBatchSpecExecution.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	tx, err := s.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// Make sure the user has access to retry it.
	batchSpec, err := tx.GetBatchSpec(ctx, store.GetBatchSpecOpts{RandID: opts.BatchSpecRandID})
	if err != nil {
		return errors.Wrap(err, "loading batch spec")
	}

	// Check whether the current user has access to either one of the namespaces.
	err = s.checkNamespaceAccessWithDB(ctx, tx.DatabaseDB(), batchSpec.NamespaceUserID, batchSpec.NamespaceOrgID)
	if err != nil {
		return errors.Wrap(err, "checking whether user has access")
	}

	// Check that batch spec is in final state
	state, err := computeBatchSpecState(ctx, tx, batchSpec)
	if err != nil {
		return errors.Wrap(err, "computing state of batch spec")
	}

	if !state.Finished() {
		return ErrRetryNonFinal
	}

	// Check that batch spec is not applied
	batchChange, err := tx.GetBatchChange(ctx, store.GetBatchChangeOpts{BatchSpecID: batchSpec.ID})
	if err != nil && err != store.ErrNoResults {
		return errors.Wrap(err, "checking whether batch spec has been applied")
	}
	if err == nil && !batchChange.IsDraft() {
		return errors.New("batch spec already applied")
	}

	workspaces, err := tx.ListRetryBatchSpecWorkspaces(ctx, store.ListRetryBatchSpecWorkspacesOpts{BatchSpecID: batchSpec.ID, IncludeCompleted: opts.IncludeCompleted})
	if err != nil {
		return errors.Wrap(err, "loading batch spec workspace execution jobs")
	}

	var changesetSpecsIDs []int64
	workspaceIDs := make([]int64, len(workspaces))

	for i, w := range workspaces {
		changesetSpecsIDs = append(changesetSpecsIDs, w.ChangesetSpecIDs...)
		workspaceIDs[i] = w.ID
	}

	// Delete the old execution jobs.
	if err := tx.DeleteBatchSpecWorkspaceExecutionJobs(ctx, store.DeleteBatchSpecWorkspaceExecutionJobsOpts{WorkspaceIDs: workspaceIDs}); err != nil {
		return errors.Wrap(err, "deleting batch spec workspace execution jobs")
	}

	// Delete the changeset specs they have created.
	if len(changesetSpecsIDs) > 0 {
		if err := tx.DeleteChangesetSpecs(ctx, store.DeleteChangesetSpecsOpts{IDs: changesetSpecsIDs}); err != nil {
			return errors.Wrap(err, "deleting batch spec workspace changeset specs")
		}
	}

	// Create new jobs
	if err := tx.CreateBatchSpecWorkspaceExecutionJobsForWorkspaces(ctx, workspaceIDs); err != nil {
		return errors.Wrap(err, "creating new batch spec workspace execution jobs")
	}

	return nil
}

type GetAvailableBulkOperationsOpts struct {
	BatchChange int64
	Changesets  []int64
}

// GetAvailableBulkOperations returns all bulk operations that can be carried out
// on an array of changesets.
func (s *Service) GetAvailableBulkOperations(ctx context.Context, opts GetAvailableBulkOperationsOpts) ([]string, error) {
	bulkOperationsCounter := map[btypes.ChangesetJobType]int{
		btypes.ChangesetJobTypeClose:     0,
		btypes.ChangesetJobTypeComment:   0,
		btypes.ChangesetJobTypeDetach:    0,
		btypes.ChangesetJobTypeMerge:     0,
		btypes.ChangesetJobTypePublish:   0,
		btypes.ChangesetJobTypeReenqueue: 0,
		btypes.ChangesetJobTypeExport:    0,
	}

	changesets, _, err := s.store.ListChangesets(ctx, store.ListChangesetsOpts{
		IDs:          opts.Changesets,
		EnforceAuthz: true,
	})
	if err != nil {
		return nil, err
	}

	for _, changeset := range changesets {
		// The assumption here is that all changesets should be exportable. That's why we always increment
		// this for all changesets. The other operations depend on the changeset state.
		bulkOperationsCounter[btypes.ChangesetJobTypeExport]++

		isChangesetArchived := changeset.ArchivedIn(opts.BatchChange)
		isChangesetDraft := changeset.ExternalState == btypes.ChangesetExternalStateDraft
		isChangesetOpen := changeset.ExternalState == btypes.ChangesetExternalStateOpen
		isChangesetClosed := changeset.ExternalState == btypes.ChangesetExternalStateClosed
		isChangesetMerged := changeset.ExternalState == btypes.ChangesetExternalStateMerged
		isChangesetReadOnly := changeset.ExternalState == btypes.ChangesetExternalStateReadOnly
		isChangesetJobFailed := changeset.ReconcilerState == btypes.ReconcilerStateFailed

		// can changeset be published
		isChangesetCommentable := isChangesetOpen || isChangesetDraft || isChangesetMerged || isChangesetClosed
		isChangesetClosable := isChangesetOpen || isChangesetDraft

		// check what operations this changeset supports, most likely from the state
		// so get the changeset then derive the operations from its state.

		// No operations are available for read-only changesets.
		if isChangesetReadOnly {
			continue
		}

		// DETACH
		if isChangesetArchived {
			bulkOperationsCounter[btypes.ChangesetJobTypeDetach] += 1
		}

		// REENQUEUE
		if !isChangesetArchived && isChangesetJobFailed {
			bulkOperationsCounter[btypes.ChangesetJobTypeReenqueue] += 1
		}

		// PUBLISH
		if !isChangesetArchived && !changeset.IsImported() {
			bulkOperationsCounter[btypes.ChangesetJobTypePublish] += 1
		}

		// CLOSE
		if !isChangesetArchived && isChangesetClosable {
			bulkOperationsCounter[btypes.ChangesetJobTypeClose] += 1
		}

		// MERGE
		if !isChangesetArchived && !isChangesetJobFailed && isChangesetOpen {
			bulkOperationsCounter[btypes.ChangesetJobTypeMerge] += 1
		}

		// COMMENT
		if isChangesetCommentable {
			bulkOperationsCounter[btypes.ChangesetJobTypeComment] += 1
		}
	}

	noOfChangesets := len(opts.Changesets)
	availableBulkOperations := make([]string, 0, len(bulkOperationsCounter))

	for jobType, count := range bulkOperationsCounter {
		// we only want to return bulkoperationType that can be applied
		// to all given changesets.
		if count == noOfChangesets {
			operation := strings.ToUpper(string(jobType))
			if operation == "COMMENTATORE" {
				operation = "COMMENT"
			}
			availableBulkOperations = append(availableBulkOperations, operation)
		}
	}

	return availableBulkOperations, nil
}

func (s *Service) GetChangesetsByIDs(ctx context.Context, batchChange int64, ids []int64) ([]*btypes.Changeset, error) {
	bc, err := s.store.GetBatchChange(ctx, store.GetBatchChangeOpts{ID: batchChange})
	if err != nil {
		return nil, errors.Wrap(err, "loading batch change")
	}

	if err := s.checkViewerCanAdminister(ctx, bc.NamespaceOrgID, bc.NamespaceUserID, true); err != nil {
		return nil, err
	}

	// We fetch all changesets with the given ID and check that the current user is authorized to access those changesets.
	changesets, _, err := s.store.ListChangesets(ctx, store.ListChangesetsOpts{
		IDs:           ids,
		BatchChangeID: batchChange,
		EnforceAuthz:  true,
	})
	if err != nil {
		return nil, err
	}

	return changesets, nil
}

func (s *Service) enqueueBatchChangeWebhook(ctx context.Context, eventType string, id graphql.ID) {
	webhooks.EnqueueBatchChange(ctx, s.logger, httpcli.InternalDoer, s.store, eventType, id)
}
