package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/tidwall/gjson"
	"github.com/xeipuuv/gojsonschema"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

type ExternalServiceStore interface {
	// Count counts all external services that satisfy the options (ignoring limit and offset).
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin or owner of the external service.
	Count(ctx context.Context, opt ExternalServicesListOptions) (int, error)

	// Create creates an external service.
	//
	// Since this method is used before the configuration server has started (search
	// for "EXTSVC_CONFIG_FILE") you must pass the conf.Get function in so that an
	// alternative can be used when the configuration server has not started,
	// otherwise a panic would occur once pkg/conf's deadlock detector determines a
	// deadlock occurred.
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin.
	//
	// 🚨 SECURITY: The value of `es.Unrestricted` is disregarded and will always be
	// recalculated based on whether "authorization" field is presented in
	// `es.Config`. For Sourcegraph Dotcom, the `es.Unrestricted` will always be
	// false (i.e. enforce permissions).
	Create(ctx context.Context, confGet func() *conf.Unified, es *types.ExternalService) error

	// Delete deletes an external service.
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin or owner of the external service.
	Delete(ctx context.Context, id int64) (err error)

	// DistinctKinds returns the distinct list of external services kinds that are stored in the database.
	DistinctKinds(ctx context.Context) ([]string, error)

	// GetLatestSyncErrors returns the most recent sync failure message for
	// each external service. If the latest sync did not have an error, the
	// string will be empty. We exclude cloud_default external services as they
	// are never synced.
	GetLatestSyncErrors(ctx context.Context) ([]*SyncError, error)

	// GetByID returns the external service for id.
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin or owner of the external service.
	GetByID(ctx context.Context, id int64) (*types.ExternalService, error)

	// GetLastSyncError returns the error associated with the latest sync of the
	// supplied external service.
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin or owner of the external service
	GetLastSyncError(ctx context.Context, id int64) (string, error)

	// GetSyncJobByID gets a sync job by its ID.
	GetSyncJobByID(ctx context.Context, id int64) (job *types.ExternalServiceSyncJob, err error)

	// GetSyncJobs gets all sync jobs.
	GetSyncJobs(ctx context.Context, opt ExternalServicesGetSyncJobsOptions) ([]*types.ExternalServiceSyncJob, error)

	// CountSyncJobs counts all sync jobs.
	CountSyncJobs(ctx context.Context, opt ExternalServicesGetSyncJobsOptions) (int64, error)

	// CancelSyncJob cancels a given sync job. It returns an error when the job was not
	// found or not in processing or queued state.
	CancelSyncJob(ctx context.Context, opts ExternalServicesCancelSyncJobOptions) error

	// CleanupSyncJobs removes sync jobs that have finished before the given threshold.
	// Additionally, only up to LimitPerService records are kept, deleting records
	// to only keep the newest N.
	CleanupSyncJobs(ctx context.Context, opts ExternalServicesCleanupSyncJobsOptions) error

	// UpdateSyncJobCounters persists only the sync job counters for the supplied job.
	UpdateSyncJobCounters(ctx context.Context, job *types.ExternalServiceSyncJob) error

	// List returns external services.
	//
	// 🚨 SECURITY: The caller must be a site admin
	List(ctx context.Context, opt ExternalServicesListOptions) ([]*types.ExternalService, error)

	// ListRepos returns external service repos for given externalServiceID.
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin or owner of the external service.
	ListRepos(ctx context.Context, opt ExternalServiceReposListOptions) ([]*types.ExternalServiceRepo, error)

	// RepoCount returns the number of repos synced by the external service with the
	// given id.
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin or owner of the external service.
	RepoCount(ctx context.Context, id int64) (int32, error)

	// SyncDue returns true if any of the supplied external services are due to sync
	// now or within given duration from now.
	SyncDue(ctx context.Context, intIDs []int64, d time.Duration) (bool, error)

	// Update updates an external service.
	//
	// 🚨 SECURITY: The caller must ensure that the actor is a site admin,
	// or has the legitimate access to the external service (i.e. the owner).
	Update(ctx context.Context, ps []schema.AuthProviders, id int64, update *ExternalServiceUpdate) (err error)

	// Upsert updates or inserts the given ExternalServices.
	//
	// NOTE: Deletion of an external service via Upsert is not allowed. Use Delete()
	// instead.
	//
	// 🚨 SECURITY: The value of `es.Unrestricted` is disregarded and will always be
	// recalculated based on whether "authorization" field is presented in
	// `es.Config`. For Sourcegraph Cloud, the `es.Unrestricted` will always be
	// false (i.e. enforce permissions).
	Upsert(ctx context.Context, svcs ...*types.ExternalService) (err error)

	WithEncryptionKey(key encryption.Key) ExternalServiceStore

	Transact(ctx context.Context) (ExternalServiceStore, error)
	With(other basestore.ShareableStore) ExternalServiceStore
	Done(err error) error
	basestore.ShareableStore
}

// An externalServiceStore stores external services and their configuration.
// Before updating or creating a new external service, validation is performed.
// The enterprise code registers additional validators at run-time and sets the
// global instance in stores.go
type externalServiceStore struct {
	logger log.Logger
	*basestore.Store

	key encryption.Key
}

func (e *externalServiceStore) copy() *externalServiceStore {
	return &externalServiceStore{
		Store: e.Store,
		key:   e.key,
	}
}

// ExternalServicesWith instantiates and returns a new ExternalServicesStore with prepared statements.
func ExternalServicesWith(logger log.Logger, other basestore.ShareableStore) ExternalServiceStore {
	return &externalServiceStore{
		logger: logger,
		Store:  basestore.NewWithHandle(other.Handle()),
	}
}

func (e *externalServiceStore) With(other basestore.ShareableStore) ExternalServiceStore {
	s := e.copy()
	s.Store = e.Store.With(other)
	return s
}

func (e *externalServiceStore) WithEncryptionKey(key encryption.Key) ExternalServiceStore {
	s := e.copy()
	s.key = key
	return s
}

func (e *externalServiceStore) Transact(ctx context.Context) (ExternalServiceStore, error) {
	return e.transact(ctx)
}

func (e *externalServiceStore) transact(ctx context.Context) (*externalServiceStore, error) {
	txBase, err := e.Store.Transact(ctx)
	s := e.copy()
	s.Store = txBase
	return s, err
}

func (e *externalServiceStore) Done(err error) error {
	return e.Store.Done(err)
}

// ExternalServiceKinds contains a map of all supported kinds of
// external services.
var ExternalServiceKinds = map[string]ExternalServiceKind{
	extsvc.KindAWSCodeCommit:   {CodeHost: true, JSONSchema: schema.AWSCodeCommitSchemaJSON},
	extsvc.KindAzureDevOps:     {CodeHost: true, JSONSchema: schema.AzureDevOpsSchemaJSON},
	extsvc.KindBitbucketCloud:  {CodeHost: true, JSONSchema: schema.BitbucketCloudSchemaJSON},
	extsvc.KindBitbucketServer: {CodeHost: true, JSONSchema: schema.BitbucketServerSchemaJSON},
	extsvc.KindGerrit:          {CodeHost: true, JSONSchema: schema.GerritSchemaJSON},
	extsvc.KindGitHub:          {CodeHost: true, JSONSchema: schema.GitHubSchemaJSON},
	extsvc.KindGitLab:          {CodeHost: true, JSONSchema: schema.GitLabSchemaJSON},
	extsvc.KindGitolite:        {CodeHost: true, JSONSchema: schema.GitoliteSchemaJSON},
	extsvc.KindGoPackages:      {CodeHost: true, JSONSchema: schema.GoModulesSchemaJSON},
	extsvc.KindJVMPackages:     {CodeHost: true, JSONSchema: schema.JVMPackagesSchemaJSON},
	extsvc.KindNpmPackages:     {CodeHost: true, JSONSchema: schema.NpmPackagesSchemaJSON},
	extsvc.KindOther:           {CodeHost: true, JSONSchema: schema.OtherExternalServiceSchemaJSON},
	extsvc.KindPagure:          {CodeHost: true, JSONSchema: schema.PagureSchemaJSON},
	extsvc.KindPerforce:        {CodeHost: true, JSONSchema: schema.PerforceSchemaJSON},
	extsvc.KindPhabricator:     {CodeHost: true, JSONSchema: schema.PhabricatorSchemaJSON},
	extsvc.KindPythonPackages:  {CodeHost: true, JSONSchema: schema.PythonPackagesSchemaJSON},
	extsvc.KindRustPackages:    {CodeHost: true, JSONSchema: schema.RustPackagesSchemaJSON},
	extsvc.KindRubyPackages:    {CodeHost: true, JSONSchema: schema.RubyPackagesSchemaJSON},
}

// ExternalServiceKind describes a kind of external service.
type ExternalServiceKind struct {
	// True if the external service can host repositories.
	CodeHost bool

	JSONSchema string // JSON Schema for the external service's configuration
}

type ExternalServiceReposListOptions ExternalServicesGetSyncJobsOptions

type ExternalServicesGetSyncJobsOptions struct {
	ExternalServiceID int64

	*LimitOffset
}

// ExternalServicesListOptions contains options for listing external services.
type ExternalServicesListOptions struct {
	// When specified, only include external services with the given IDs.
	IDs []int64
	// When specified, only include external services with given list of kinds.
	Kinds []string
	// When specified, only include external services with ID below this number
	// (because we're sorting results by ID in descending order).
	AfterID int64
	// When specified, only include external services with that were updated after
	// the specified time.
	UpdatedAfter time.Time
	// Possible values are ASC or DESC. Defaults to DESC.
	OrderByDirection string
	// When true, will only return services that have the cloud_default flag set to
	// true.
	OnlyCloudDefault bool
	// When specified, only include external services which contain repository with a given ID.
	RepoID api.RepoID

	// Only include external services that belong to the given CodeHost.
	CodeHostID int32

	*LimitOffset

	// When true, soft-deleted external services will also be included in the results.
	IncludeDeleted bool
}

func (o ExternalServicesListOptions) sqlConditions() []*sqlf.Query {
	conds := []*sqlf.Query{}
	if !o.IncludeDeleted {
		conds = append(conds, sqlf.Sprintf("deleted_at IS NULL"))
	}
	if len(o.IDs) > 0 {
		conds = append(conds, sqlf.Sprintf("id = ANY(%s)", pq.Array(o.IDs)))
	}
	if len(o.Kinds) > 0 {
		conds = append(conds, sqlf.Sprintf("kind = ANY(%s)", pq.Array(o.Kinds)))
	}
	if o.AfterID > 0 {
		conds = append(conds, sqlf.Sprintf(`id < %d`, o.AfterID))
	}
	if !o.UpdatedAfter.IsZero() {
		conds = append(conds, sqlf.Sprintf(`updated_at > %s`, o.UpdatedAfter))
	}
	if o.OnlyCloudDefault {
		conds = append(conds, sqlf.Sprintf("cloud_default = true"))
	}
	if o.CodeHostID != 0 {
		conds = append(conds, sqlf.Sprintf("code_host_id = %s", o.CodeHostID))
	}
	if o.RepoID > 0 {
		conds = append(conds, sqlf.Sprintf("id IN (SELECT external_service_id FROM external_service_repos WHERE repo_id = %s)", o.RepoID))
	}
	if len(conds) == 0 {
		conds = append(conds, sqlf.Sprintf("TRUE"))
	}
	return conds
}

type ValidateExternalServiceConfigOptions struct {
	// The ID of the external service, 0 is a valid value for not-yet-created external service.
	ExternalServiceID int64
	// The kind of external service.
	Kind string
	// The actual config of the external service.
	Config string
	// The list of authN providers configured on the instance.
	AuthProviders []schema.AuthProviders
}

type ValidateExternalServiceConfigFunc = func(ctx context.Context, db DB, opt ValidateExternalServiceConfigOptions) (normalized []byte, err error)

// ValidateExternalServiceConfig is the default non-enterprise version of our validation function
var ValidateExternalServiceConfig = MakeValidateExternalServiceConfigFunc(nil, nil, nil, nil, nil)

type (
	GitHubValidatorFunc          func(DB, *types.GitHubConnection) error
	GitLabValidatorFunc          func(*schema.GitLabConnection, []schema.AuthProviders) error
	BitbucketServerValidatorFunc func(*schema.BitbucketServerConnection) error
	PerforceValidatorFunc        func(*schema.PerforceConnection) error
	AzureDevOpsValidatorFunc     func(connection *schema.AzureDevOpsConnection) error
)

func MakeValidateExternalServiceConfigFunc(
	gitHubValidators []GitHubValidatorFunc,
	gitLabValidators []GitLabValidatorFunc,
	bitbucketServerValidators []BitbucketServerValidatorFunc,
	perforceValidators []PerforceValidatorFunc,
	azureDevOpsValidators []AzureDevOpsValidatorFunc,
) ValidateExternalServiceConfigFunc {
	return func(ctx context.Context, db DB, opt ValidateExternalServiceConfigOptions) (normalized []byte, err error) {
		ext, ok := ExternalServiceKinds[opt.Kind]
		if !ok {
			return nil, errors.Errorf("invalid external service kind: %s", opt.Kind)
		}

		// All configs must be valid JSON.
		// If this requirement is ever changed, you will need to update
		// serveExternalServiceConfigs to handle this case.

		sl := gojsonschema.NewSchemaLoader()
		sc, err := sl.Compile(gojsonschema.NewStringLoader(ext.JSONSchema))
		if err != nil {
			return nil, errors.Wrapf(err, "unable to compile schema for external service of kind %q", opt.Kind)
		}

		normalized, err = jsonc.Parse(opt.Config)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to normalize JSON")
		}

		// Check for any redacted secrets, in
		// graphqlbackend/external_service.go:externalServiceByID() we call
		// svc.RedactConfigSecrets() replacing any secret fields in the JSON with
		// types.RedactedSecret, this is to prevent us leaking tokens that users add.
		// Here we check that the config we've been passed doesn't contain any redacted
		// secrets in order to avoid breaking configs by writing the redacted version to
		// the database. we should have called svc.UnredactConfig(oldSvc) before this
		// point, e.g. in the Update method of the ExternalServiceStore.
		if bytes.Contains(normalized, []byte(types.RedactedSecret)) {
			return nil, errors.Errorf(
				"unable to write external service config as it contains redacted fields, this is likely a bug rather than a problem with your config",
			)
		}

		res, err := sc.Validate(gojsonschema.NewBytesLoader(normalized))
		if err != nil {
			return nil, errors.Wrap(err, "unable to validate config against schema")
		}

		var errs error
		for _, err := range res.Errors() {
			errString := err.String()
			// Remove `(root): ` from error formatting since these errors are
			// presented to users.
			errString = strings.TrimPrefix(errString, "(root): ")
			errs = errors.Append(errs, errors.New(errString))
		}

		// Extra validation not based on JSON Schema.
		switch opt.Kind {
		case extsvc.KindGitHub:
			var c schema.GitHubConnection
			if err = jsoniter.Unmarshal(normalized, &c); err != nil {
				return nil, err
			}
			err = validateGitHubConnection(db, gitHubValidators, opt.ExternalServiceID, &c)

		case extsvc.KindGitLab:
			var c schema.GitLabConnection
			if err = jsoniter.Unmarshal(normalized, &c); err != nil {
				return nil, err
			}
			err = validateGitLabConnection(gitLabValidators, opt.ExternalServiceID, &c, opt.AuthProviders)

		case extsvc.KindBitbucketServer:
			var c schema.BitbucketServerConnection
			if err = jsoniter.Unmarshal(normalized, &c); err != nil {
				return nil, err
			}
			err = validateBitbucketServerConnection(bitbucketServerValidators, opt.ExternalServiceID, &c)

		case extsvc.KindBitbucketCloud:
			var c schema.BitbucketCloudConnection
			if err = jsoniter.Unmarshal(normalized, &c); err != nil {
				return nil, err
			}

		case extsvc.KindPerforce:
			var c schema.PerforceConnection
			if err = jsoniter.Unmarshal(normalized, &c); err != nil {
				return nil, err
			}
			err = validatePerforceConnection(perforceValidators, opt.ExternalServiceID, &c)
		case extsvc.KindAzureDevOps:
			var c schema.AzureDevOpsConnection
			if err = jsoniter.Unmarshal(normalized, &c); err != nil {
				return nil, err
			}
			err = validateAzureDevOpsConnection(azureDevOpsValidators, opt.ExternalServiceID, &c)
		case extsvc.KindOther:
			var c schema.OtherExternalServiceConnection
			if err = jsoniter.Unmarshal(normalized, &c); err != nil {
				return nil, err
			}
			err = validateOtherExternalServiceConnection(&c)
		}

		return normalized, errors.Append(errs, err)
	}
}

// Neither our JSON schema library nor the Monaco editor we use supports
// object dependencies well, so we must validate here that repo items
// match the uri-reference format when url is set, instead of uri when
// it isn't.
func validateOtherExternalServiceConnection(c *schema.OtherExternalServiceConnection) error {
	parseRepo := url.Parse
	if c.Url != "" {
		// We ignore the error because this already validated by JSON Schema.
		baseURL, _ := url.Parse(c.Url)
		parseRepo = baseURL.Parse
	}

	if !dotcom.SourcegraphDotComMode() && c.MakeReposPublicOnDotCom {
		return errors.Errorf(`"makeReposPublicOnDotCom" can only be set when running on Sourcegraph.com`)
	}

	for i, repo := range c.Repos {
		cloneURL, err := parseRepo(repo)
		if err != nil {
			return errors.Errorf(`repos.%d: %s`, i, err)
		}

		switch cloneURL.Scheme {
		case "git", "http", "https", "ssh":
			continue
		default:
			return errors.Errorf("repos.%d: scheme %q not one of git, http, https or ssh", i, cloneURL.Scheme)
		}
	}

	return nil
}

func validateGitHubConnection(db DB, githubValidators []GitHubValidatorFunc, id int64, c *schema.GitHubConnection) error {
	var err error
	for _, validate := range githubValidators {
		err = errors.Append(err,
			validate(db, &types.GitHubConnection{
				URN:              extsvc.URN(extsvc.KindGitHub, id),
				GitHubConnection: c,
			}),
		)
	}

	if c.Token == "" && c.GitHubAppDetails == nil {
		err = errors.Append(err, errors.New("either token or GitHub App Details must be set"))
	}
	if c.Repos == nil && c.RepositoryQuery == nil && c.Orgs == nil && (c.GitHubAppDetails == nil || !c.GitHubAppDetails.CloneAllRepositories) {
		err = errors.Append(err, errors.New("at least one of repositoryQuery, repos, orgs, or gitHubAppDetails.cloneAllRepositories must be set"))
	}
	return err
}

func validateGitLabConnection(gitLabValidators []GitLabValidatorFunc, _ int64, c *schema.GitLabConnection, ps []schema.AuthProviders) error {
	var err error
	for _, validate := range gitLabValidators {
		err = errors.Append(err, validate(c, ps))
	}
	return err
}

func validateAzureDevOpsConnection(azureDevOpsValidators []AzureDevOpsValidatorFunc, _ int64, c *schema.AzureDevOpsConnection) error {
	var err error
	for _, validate := range azureDevOpsValidators {
		err = errors.Append(err, validate(c))
	}
	if c.Projects == nil && c.Orgs == nil {
		err = errors.Append(err, errors.New("either 'projects' or 'orgs' must be set"))
	}
	return err
}

func validateBitbucketServerConnection(bitbucketServerValidators []BitbucketServerValidatorFunc, _ int64, c *schema.BitbucketServerConnection) error {
	var err error
	for _, validate := range bitbucketServerValidators {
		err = errors.Append(err, validate(c))
	}

	if c.Repos == nil && c.RepositoryQuery == nil && c.ProjectKeys == nil {
		err = errors.Append(err, errors.New("at least one of: repositoryQuery, projectKeys, or repos must be set"))
	}
	return err
}

func validatePerforceConnection(perforceValidators []PerforceValidatorFunc, _ int64, c *schema.PerforceConnection) error {
	var err error
	for _, validate := range perforceValidators {
		err = errors.Append(err, validate(c))
	}

	if c.Depots == nil {
		err = errors.Append(err, errors.New("depots must be set"))
	}

	if strings.Contains(c.P4Passwd, ":") {
		err = errors.Append(err, errors.New("p4.passwd must not contain a colon. It must be the ticket generated by `p4 login -p`, not a full ticket from the `.p4tickets` file."))
	}

	return err
}

// disablePermsSyncingForExternalService removes "authorization" or
// "enforcePermissions" fields from the external service config
// when present on the external service config.
func disablePermsSyncingForExternalService(config string) (string, error) {
	withoutEnforcePermissions, err := jsonc.Remove(config, "enforcePermissions")
	// in case removing "enforcePermissions" fails, we try to remove "authorization" anyway
	if err != nil {
		withoutEnforcePermissions = config
	}
	return jsonc.Remove(withoutEnforcePermissions, "authorization")
}

func (e *externalServiceStore) Create(ctx context.Context, confGet func() *conf.Unified, es *types.ExternalService) (err error) {
	rawConfig, err := es.Config.Decrypt(ctx)
	if err != nil {
		return err
	}

	db := NewDBWith(e.logger, e)
	normalized, err := ValidateExternalServiceConfig(ctx, db, ValidateExternalServiceConfigOptions{
		Kind:          es.Kind,
		Config:        rawConfig,
		AuthProviders: confGet().AuthProviders,
	})
	if err != nil {
		return err
	}

	// 🚨 SECURITY: For all code host connections on Sourcegraph.com,
	// we always want to disable repository permissions to prevent
	// permission syncing from trying to sync permissions from public code.
	if dotcom.SourcegraphDotComMode() {
		rawConfig, err = disablePermsSyncingForExternalService(rawConfig)
		if err != nil {
			return err
		}

		es.Config.Set(rawConfig)
	}

	es.CreatedAt = timeutil.Now()
	es.UpdatedAt = es.CreatedAt

	// Ensure the calculated fields in the external service are up to date.
	e.recalculateFields(es, string(normalized))

	encryptedConfig, keyID, err := es.Config.Encrypt(ctx, e.getEncryptionKey())
	if err != nil {
		return err
	}

	tx, err := e.transact(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Done(err)
	}()

	chID, err := ensureCodeHost(ctx, tx, es.Kind, string(normalized))
	if err != nil {
		return err
	}
	es.CodeHostID = &chID

	return tx.QueryRow(
		ctx,
		sqlf.Sprintf(
			createExternalServiceQueryFmtstr,
			es.Kind,
			es.DisplayName,
			encryptedConfig,
			keyID,
			es.CreatedAt,
			es.UpdatedAt,
			es.Unrestricted,
			es.CloudDefault,
			es.HasWebhooks,
			es.CodeHostID,
			es.CreatorID,
			es.LastUpdaterID,
		),
	).Scan(&es.ID)
}

const createExternalServiceQueryFmtstr = `
INSERT INTO external_services
	(kind, display_name, config, encryption_key_id, created_at, updated_at, unrestricted, cloud_default, has_webhooks, code_host_id, creator_id, last_updater_id)
	VALUES(%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
RETURNING id
`

func (e *externalServiceStore) getEncryptionKey() encryption.Key {
	if e.key != nil {
		return e.key
	}

	return keyring.Default().ExternalServiceKey
}

func (e *externalServiceStore) Upsert(ctx context.Context, svcs ...*types.ExternalService) (err error) {
	if len(svcs) == 0 {
		return nil
	}

	tx, err := e.transact(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Done(err)
	}()

	authProviders := conf.Get().AuthProviders
	for _, s := range svcs {
		rawConfig, err := s.Config.Decrypt(ctx)
		if err != nil {
			return err
		}

		normalized, err := ValidateExternalServiceConfig(ctx, NewDBWith(e.logger, e), ValidateExternalServiceConfigOptions{
			Kind:          s.Kind,
			Config:        rawConfig,
			AuthProviders: authProviders,
		})
		if err != nil {
			return errors.Wrapf(err, "validating service of kind %q", s.Kind)
		}

		// 🚨 SECURITY: For all code host connections on Sourcegraph.com,
		// we always want to disable repository permissions to prevent
		// permission syncing from trying to sync permissions from public code.
		if dotcom.SourcegraphDotComMode() {
			rawConfig, err = disablePermsSyncingForExternalService(rawConfig)
			if err != nil {
				return err
			}

			s.Config.Set(rawConfig)
		}

		e.recalculateFields(s, string(normalized))

		chID, err := ensureCodeHost(ctx, tx, s.Kind, string(normalized))
		if err != nil {
			return err
		}
		s.CodeHostID = &chID

		// Ensure CreatedAt is set.
		if s.CreatedAt.IsZero() {
			s.CreatedAt = timeutil.Now()
		}
	}

	// Get the list services that are marked as deleted. We don't know at this point
	// whether they are marked as deleted in the DB too.
	var deleted []int64
	for _, es := range svcs {
		if es.ID != 0 && es.IsDeleted() {
			deleted = append(deleted, es.ID)
		}
	}

	// Fetch any services marked for deletion. list() only fetches non deleted
	// services so if we find anything here it indicates that we are marking a
	// service as deleted that is NOT deleted in the DB
	if len(deleted) > 0 {
		existing, err := tx.List(ctx, ExternalServicesListOptions{IDs: deleted})
		if err != nil {
			return errors.Wrap(err, "fetching services marked for deletion")
		}
		if len(existing) > 0 {
			// We found services marked for deletion that are currently not deleted in the
			// DB.
			return errors.New("deletion via Upsert() not allowed, use Delete()")
		}
	}

	q, err := tx.upsertExternalServicesQuery(ctx, svcs)
	if err != nil {
		return err
	}

	rows, err := tx.Query(ctx, q)
	if err != nil {
		return err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	i := 0
	for rows.Next() {
		var encryptedConfig, keyID string
		err = rows.Scan(
			&svcs[i].ID,
			&svcs[i].Kind,
			&svcs[i].DisplayName,
			&encryptedConfig,
			&svcs[i].CreatedAt,
			&dbutil.NullTime{Time: &svcs[i].UpdatedAt},
			&dbutil.NullTime{Time: &svcs[i].DeletedAt},
			&dbutil.NullTime{Time: &svcs[i].LastSyncAt},
			&dbutil.NullTime{Time: &svcs[i].NextSyncAt},
			&svcs[i].Unrestricted,
			&svcs[i].CloudDefault,
			&keyID,
			&dbutil.NullBool{B: svcs[i].HasWebhooks},
			&svcs[i].CodeHostID,
			&svcs[i].CreatorID,
			&svcs[i].LastUpdaterID,
		)
		if err != nil {
			return err
		}

		svcs[i].Config = extsvc.NewEncryptedConfig(encryptedConfig, keyID, e.getEncryptionKey())
		i++
	}

	return nil
}

func (e *externalServiceStore) upsertExternalServicesQuery(ctx context.Context, svcs []*types.ExternalService) (*sqlf.Query, error) {
	vals := make([]*sqlf.Query, 0, len(svcs))
	for _, s := range svcs {
		encryptedConfig, keyID, err := s.Config.Encrypt(ctx, e.getEncryptionKey())
		if err != nil {
			return nil, err
		}
		vals = append(vals, sqlf.Sprintf(
			upsertExternalServicesQueryValueFmtstr,
			s.ID,
			s.Kind,
			s.DisplayName,
			encryptedConfig,
			keyID,
			s.CreatedAt.UTC(),
			s.UpdatedAt.UTC(),
			dbutil.NullTimeColumn(s.DeletedAt),
			dbutil.NullTimeColumn(s.LastSyncAt),
			dbutil.NullTimeColumn(s.NextSyncAt),
			s.Unrestricted,
			s.CloudDefault,
			s.HasWebhooks,
			s.CodeHostID,
			s.CreatorID,
			s.LastUpdaterID,
		))
	}

	return sqlf.Sprintf(
		upsertExternalServicesQueryFmtstr,
		sqlf.Join(vals, ",\n"),
	), nil
}

const upsertExternalServicesQueryValueFmtstr = `
  (COALESCE(NULLIF(%s, 0), (SELECT nextval('external_services_id_seq'))), UPPER(%s), %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
`

const upsertExternalServicesQueryFmtstr = `
INSERT INTO external_services (
  id,
  kind,
  display_name,
  config,
  encryption_key_id,
  created_at,
  updated_at,
  deleted_at,
  last_sync_at,
  next_sync_at,
  unrestricted,
  cloud_default,
  has_webhooks,
  code_host_id,
  creator_id,
  last_updater_id
)
VALUES %s
ON CONFLICT(id) DO UPDATE
SET
  kind               = UPPER(excluded.kind),
  display_name       = excluded.display_name,
  config             = excluded.config,
  encryption_key_id  = excluded.encryption_key_id,
  created_at         = excluded.created_at,
  updated_at         = excluded.updated_at,
  deleted_at         = excluded.deleted_at,
  last_sync_at       = excluded.last_sync_at,
  next_sync_at       = excluded.next_sync_at,
  unrestricted       = excluded.unrestricted,
  cloud_default      = excluded.cloud_default,
  has_webhooks       = excluded.has_webhooks,
  code_host_id       = excluded.code_host_id,
  last_updater_id    = excluded.last_updater_id
RETURNING
	id,
	kind,
	display_name,
	config,
	created_at,
	updated_at,
	deleted_at,
	last_sync_at,
	next_sync_at,
	unrestricted,
	cloud_default,
	encryption_key_id,
	has_webhooks,
	code_host_id,
	creator_id,
	last_updater_id
`

// ExternalServiceUpdate contains optional fields to update.
type ExternalServiceUpdate struct {
	DisplayName    *string
	Config         *string
	CloudDefault   *bool
	TokenExpiresAt *time.Time
	LastSyncAt     *time.Time
	NextSyncAt     *time.Time
	LastUpdaterID  *int32
}

func (e *externalServiceStore) Update(ctx context.Context, ps []schema.AuthProviders, id int64, update *ExternalServiceUpdate) (err error) {
	tx, err := e.transact(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Done(err)
	}()

	var (
		normalized      []byte
		encryptedConfig string
		keyID           string
		hasWebhooks     bool
	)

	// 5 is the number of fields of the ExternalServiceUpdate plus 1 for code_host_id
	updates := make([]*sqlf.Query, 0, 5)

	if update.Config != nil {
		rawConfig := *update.Config

		// Query to get the kind (which is immutable) so we can validate the new config.
		externalService, err := tx.GetByID(ctx, id)
		if err != nil {
			return err
		}
		newSvc := types.ExternalService{
			Kind:   externalService.Kind,
			Config: extsvc.NewUnencryptedConfig(rawConfig),
		}

		if err := newSvc.UnredactConfig(ctx, externalService); err != nil {
			return errors.Wrapf(err, "error unredacting config")
		}
		unredactedConfig, err := newSvc.Config.Decrypt(ctx)
		if err != nil {
			return err
		}

		cfg, err := newSvc.Configuration(ctx)
		if err == nil {
			hasWebhooks = configurationHasWebhooks(cfg)
		} else {
			// Legacy configurations might not be valid JSON; in that case, they
			// also can't have webhooks, so we'll just log the issue and move
			// on.
			e.logger.Warn("cannot parse external service configuration as JSON", log.Error(err), log.Int64("id", id))
			hasWebhooks = false
		}

		normalized, err = ValidateExternalServiceConfig(ctx, NewDBWith(e.logger, tx), ValidateExternalServiceConfigOptions{
			ExternalServiceID: id,
			Kind:              externalService.Kind,
			Config:            unredactedConfig,
			AuthProviders:     ps,
		})
		if err != nil {
			return err
		}

		// 🚨 SECURITY: For all code host connections on Sourcegraph.com,
		// we always want to disable repository permissions to prevent
		// permission syncing from trying to sync permissions from public code.
		if dotcom.SourcegraphDotComMode() {
			unredactedConfig, err = disablePermsSyncingForExternalService(unredactedConfig)
			if err != nil {
				return err
			}
			newSvc.Config.Set(unredactedConfig)
		}

		chID, err := ensureCodeHost(ctx, tx, externalService.Kind, string(normalized))
		if err != nil {
			return err
		}
		updates = append(updates, sqlf.Sprintf("code_host_id = %s", chID))

		encryptedConfig, keyID, err = newSvc.Config.Encrypt(ctx, e.getEncryptionKey())
		if err != nil {
			return err
		}
	}

	if update.DisplayName != nil {
		updates = append(updates, sqlf.Sprintf("display_name = %s", update.DisplayName))
	}

	if update.Config != nil {
		unrestricted := calcUnrestricted(string(normalized), dotcom.SourcegraphDotComMode(), conf.PermissionsUserMapping().Enabled)

		updates = append(updates,
			sqlf.Sprintf(
				"config = %s, encryption_key_id = %s, unrestricted = %s, has_webhooks = %s, last_updater_id = %s",
				encryptedConfig, keyID, unrestricted, hasWebhooks, update.LastUpdaterID,
			))
	}

	if update.CloudDefault != nil {
		updates = append(updates, sqlf.Sprintf("cloud_default = %s", update.CloudDefault))
	}

	if update.TokenExpiresAt != nil {
		updates = append(updates, sqlf.Sprintf("token_expires_at = %s", update.TokenExpiresAt))
	}

	if update.LastSyncAt != nil {
		updates = append(updates, sqlf.Sprintf("last_sync_at = %s", dbutil.NullTimeColumn(*update.LastSyncAt)))
	}

	if update.NextSyncAt != nil {
		updates = append(updates, sqlf.Sprintf("next_sync_at = %s", dbutil.NullTimeColumn(*update.NextSyncAt)))
	} else if update.Config != nil {
		// If the config changed, trigger a new sync immediately.
		updates = append(updates, sqlf.Sprintf("next_sync_at = NOW()"))
	}

	if len(updates) == 0 {
		return nil
	}

	q := sqlf.Sprintf("UPDATE external_services SET %s, updated_at = NOW() WHERE id = %d AND deleted_at IS NULL", sqlf.Join(updates, ","), id)
	res, err := tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return externalServiceNotFoundError{id: id}
	}
	return nil
}

type externalServiceNotFoundError struct {
	id int64
}

func (e externalServiceNotFoundError) Error() string {
	return fmt.Sprintf("external service not found: %v", e.id)
}

func (e externalServiceNotFoundError) NotFound() bool {
	return true
}

func (e *externalServiceStore) Delete(ctx context.Context, id int64) (err error) {
	tx, err := e.transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// Load the external service *for update* so that no sync job can be created
	if err := tx.selectForUpdate(ctx, id); err != nil {
		return err
	}

	// Cancel all currently running sync jobs, *outside* the transaction.
	err = e.CancelSyncJob(ctx, ExternalServicesCancelSyncJobOptions{ExternalServiceID: id})
	if err != nil {
		return err
	}

	// Wait until all the sync jobs we just canceled are done executing to
	// ensure that we delete all repositories and no new ones are inserted.
	runningJobsCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	for {
		if err := runningJobsCtx.Err(); err != nil {
			return err
		}

		runningJobsExist, err := e.hasRunningSyncJobs(runningJobsCtx, id)
		if err != nil {
			return err
		}

		if !runningJobsExist {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Create a temporary table where we'll store repos affected by the deletion of
	// the external service
	if err := tx.Exec(ctx, sqlf.Sprintf(`
CREATE TEMPORARY TABLE IF NOT EXISTS
    deleted_repos_temp(
    repo_id int
) ON COMMIT DROP`)); err != nil {
		return errors.Wrap(err, "creating temporary table")
	}

	// Delete external service <-> repo relationships, storing the affected repos
	if err := tx.Exec(ctx, sqlf.Sprintf(`
	WITH deleted AS (
	   DELETE FROM external_service_repos
	       WHERE external_service_id = %s
	       RETURNING repo_id
	)
	INSERT INTO deleted_repos_temp
	SELECT repo_id from deleted
`, id)); err != nil {
		return errors.Wrap(err, "populating temporary table")
	}

	// Soft delete orphaned repos
	if err := tx.Exec(ctx, sqlf.Sprintf(`
	UPDATE repo
	SET name       = soft_deleted_repository_name(name),
	   deleted_at = TRANSACTION_TIMESTAMP()
	WHERE deleted_at IS NULL
	 AND EXISTS (SELECT FROM deleted_repos_temp WHERE repo.id = deleted_repos_temp.repo_id)
	 AND NOT EXISTS (
	       SELECT FROM external_service_repos
	       WHERE repo_id = repo.id
	   );
`)); err != nil {
		return errors.Wrap(err, "cleaning up potentially orphaned repos")
	}

	// Clear temporary table in case delete is called multiple times within the same
	// transaction
	if err := tx.Exec(ctx, sqlf.Sprintf(`
    DELETE FROM deleted_repos_temp;
`)); err != nil {
		return errors.Wrap(err, "clearing temporary table")
	}

	// Soft delete external service
	res, err := tx.ExecResult(ctx, sqlf.Sprintf(`
	-- Soft delete external service
	UPDATE external_services
	SET deleted_at=TRANSACTION_TIMESTAMP()
	WHERE id = %s
	 AND deleted_at IS NULL;
	`, id))
	if err != nil {
		return err
	}
	nrows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if nrows == 0 {
		return externalServiceNotFoundError{id: id}
	}
	return nil
}

// selectForUpdate loads an external service with FOR UPDATE with the given ID
// and that is not deleted. It's used by Delete.
func (e *externalServiceStore) selectForUpdate(ctx context.Context, id int64) error {
	q := sqlf.Sprintf(
		`SELECT id FROM external_services WHERE id = %s AND deleted_at IS NULL FOR UPDATE`,
		id,
	)
	_, ok, err := basestore.ScanFirstInt(e.Query(ctx, q))
	if err != nil {
		return err
	}
	if !ok {
		return &externalServiceNotFoundError{id: id}
	}
	return nil
}

func (e *externalServiceStore) GetByID(ctx context.Context, id int64) (*types.ExternalService, error) {
	opt := ExternalServicesListOptions{
		IDs: []int64{id},
	}

	ess, err := e.List(ctx, opt)
	if err != nil {
		return nil, err
	}
	if len(ess) == 0 {
		return nil, externalServiceNotFoundError{id: id}
	}
	return ess[0], nil
}

const getSyncJobsQueryFmtstr = `
SELECT
	id,
	state,
	failure_message,
	queued_at,
	started_at,
	finished_at,
	process_after,
	num_resets,
	external_service_id,
	num_failures,
	cancel,
	repos_synced,
	repo_sync_errors,
	repos_added,
	repos_modified,
	repos_unmodified,
	repos_deleted
FROM
	external_service_sync_jobs
WHERE %s
ORDER BY
	started_at DESC
%s
`

func (e *externalServiceStore) GetSyncJobs(ctx context.Context, opt ExternalServicesGetSyncJobsOptions) (_ []*types.ExternalServiceSyncJob, err error) {
	var preds []*sqlf.Query

	if opt.ExternalServiceID != 0 {
		preds = append(preds, sqlf.Sprintf("external_service_id = %s", opt.ExternalServiceID))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	q := sqlf.Sprintf(getSyncJobsQueryFmtstr, sqlf.Join(preds, "AND"), opt.LimitOffset.SQL())

	return scanExternalServiceSyncJobs(e.Query(ctx, q))
}

const countSyncJobsQueryFmtstr = `
SELECT
	COUNT(*)
FROM
	external_service_sync_jobs
WHERE %s
`

func (e *externalServiceStore) CountSyncJobs(ctx context.Context, opt ExternalServicesGetSyncJobsOptions) (int64, error) {
	var preds []*sqlf.Query

	if opt.ExternalServiceID != 0 {
		preds = append(preds, sqlf.Sprintf("external_service_id = %s", opt.ExternalServiceID))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	q := sqlf.Sprintf(countSyncJobsQueryFmtstr, sqlf.Join(preds, "AND"))

	count, _, err := basestore.ScanFirstInt64(e.Query(ctx, q))
	return count, err
}

type errSyncJobNotFound struct {
	id, externalServiceID int64
}

func (e errSyncJobNotFound) Error() string {
	if e.id != 0 {
		return fmt.Sprintf("sync job with id %d not found", e.id)
	} else if e.externalServiceID != 0 {
		return fmt.Sprintf("sync job with external service id %d not found", e.externalServiceID)
	}
	return "sync job not found"
}

func (errSyncJobNotFound) NotFound() bool {
	return true
}

func (e *externalServiceStore) GetSyncJobByID(ctx context.Context, id int64) (*types.ExternalServiceSyncJob, error) {
	q := sqlf.Sprintf(getSyncJobsQueryFmtstr, sqlf.Sprintf("id = %s", id), (&LimitOffset{Limit: 1}).SQL())

	job, err := scanExternalServiceSyncJob(e.QueryRow(ctx, q))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &errSyncJobNotFound{id: id}
		}
		return nil, errors.Wrap(err, "scanning external service job row")
	}

	return job, nil
}

// UpdateSyncJobCounters persists only the sync job counters for the supplied job.
func (e *externalServiceStore) UpdateSyncJobCounters(ctx context.Context, job *types.ExternalServiceSyncJob) error {
	q := sqlf.Sprintf(updateSyncJobQueryFmtstr, job.ReposSynced, job.RepoSyncErrors, job.ReposAdded, job.ReposModified, job.ReposUnmodified, job.ReposDeleted, job.ID)
	result, err := e.ExecResult(ctx, q)
	if err != nil {
		return errors.Wrap(err, "updating sync job counters")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "checking affected rows")
	}
	if affected == 0 {
		return &errSyncJobNotFound{id: job.ID}
	}
	return nil
}

const updateSyncJobQueryFmtstr = `
UPDATE external_service_sync_jobs
SET
	repos_synced = %d,
	repo_sync_errors = %d,
	repos_added = %d,
	repos_modified = %d,
	repos_unmodified = %d,
	repos_deleted = %d
WHERE
    id = %d
`

var scanExternalServiceSyncJobs = basestore.NewSliceScanner(scanExternalServiceSyncJob)

func scanExternalServiceSyncJob(sc dbutil.Scanner) (*types.ExternalServiceSyncJob, error) {
	var job types.ExternalServiceSyncJob
	err := sc.Scan(
		&job.ID,
		&job.State,
		&dbutil.NullString{S: &job.FailureMessage},
		&job.QueuedAt,
		&dbutil.NullTime{Time: &job.StartedAt},
		&dbutil.NullTime{Time: &job.FinishedAt},
		&dbutil.NullTime{Time: &job.ProcessAfter},
		&job.NumResets,
		&dbutil.NullInt64{N: &job.ExternalServiceID},
		&job.NumFailures,
		&job.Cancel,
		&job.ReposSynced,
		&job.RepoSyncErrors,
		&job.ReposAdded,
		&job.ReposModified,
		&job.ReposUnmodified,
		&job.ReposDeleted,
	)
	return &job, err
}

func (e *externalServiceStore) GetLastSyncError(ctx context.Context, id int64) (string, error) {
	q := sqlf.Sprintf(`
SELECT failure_message from external_service_sync_jobs
WHERE external_service_id = %d
AND state IN ('completed','errored','failed')
ORDER BY finished_at DESC
LIMIT 1
`, id)

	lastError, _, err := basestore.ScanFirstNullString(e.Query(ctx, q))
	return lastError, err
}

type ExternalServicesCancelSyncJobOptions struct {
	ID                int64
	ExternalServiceID int64
}

func buildCancelSyncJobQuery(opts ExternalServicesCancelSyncJobOptions) (*sqlf.Query, error) {
	var conds []*sqlf.Query
	if opts.ID != 0 {
		conds = append(conds, sqlf.Sprintf("id = %s", opts.ID))
	}
	if opts.ExternalServiceID != 0 {
		conds = append(conds, sqlf.Sprintf("external_service_id = %s", opts.ExternalServiceID))
	}

	if len(conds) == 0 {
		return nil, errors.New("not enough conditions given to build query to cancel external service sync job")
	}

	now := timeutil.Now()
	q := sqlf.Sprintf(`
UPDATE
	external_service_sync_jobs
SET
	cancel = TRUE,
	-- If the sync job is still queued, we directly abort, otherwise we keep the
	-- state, so the worker can do teardown and, at some point, mark it failed itself.
	state = CASE WHEN external_service_sync_jobs.state = 'processing' THEN external_service_sync_jobs.state ELSE 'canceled' END,
	finished_at = CASE WHEN external_service_sync_jobs.state = 'processing' THEN external_service_sync_jobs.finished_at ELSE %s END
WHERE
	%s
	AND
	state IN ('queued', 'processing')
	AND
	cancel IS FALSE
`, now, sqlf.Join(conds, " AND "))

	return q, nil
}

func (e *externalServiceStore) CancelSyncJob(ctx context.Context, opts ExternalServicesCancelSyncJobOptions) error {
	q, err := buildCancelSyncJobQuery(opts)
	if err != nil {
		return err
	}

	res, err := e.ExecResult(ctx, q)
	if err != nil {
		return err
	}
	af, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if opts.ID != 0 && af != 1 {
		return &errSyncJobNotFound{id: opts.ID, externalServiceID: opts.ExternalServiceID}
	}

	// If opts.ExternalServiceID is set and affected rows are 0 we don't treat
	// it as an error, because we want to be able to use this method to cancel
	// jobs *if there are any*.
	// Just like a `DeleteUserByID(1234)` function should fail if there is no
	// user with that ID, but a `DeleteUsersWithUsernameStartingWith("foo")`
	// shouldn't fail if there are no users with that prefix in the name.

	return nil
}

type ExternalServicesCleanupSyncJobsOptions struct {
	// Remove jobs that are older than the given duration from NOW().
	OlderThan time.Duration
	// Removes the oldest jobs until only MaxPerExternalService jobs remain for
	// each service.
	MaxPerExternalService int
}

func (e *externalServiceStore) CleanupSyncJobs(ctx context.Context, opts ExternalServicesCleanupSyncJobsOptions) error {
	if opts.MaxPerExternalService < 1 {
		return errors.New("MaxPerExternalService must be greater than 0")
	}

	q := buildCleanupSyncJobsQuery(opts)
	return e.Exec(ctx, q)
}

const cleanupSyncJobsQueryFmtstr = `
WITH ranked_jobs AS (
    SELECT
        id,
        external_service_id,
        state,
        finished_at,
        ROW_NUMBER() OVER (PARTITION BY external_service_id ORDER BY finished_at DESC) as rn
    FROM
        external_service_sync_jobs
    WHERE
        state IN ('completed', 'failed', 'canceled')
		AND
		finished_at IS NOT NULL
)
DELETE FROM
    external_service_sync_jobs
WHERE
    id IN (
        SELECT id FROM ranked_jobs
        WHERE
			rn > %s
			OR
			finished_at < %s
    )
`

func buildCleanupSyncJobsQuery(opts ExternalServicesCleanupSyncJobsOptions) *sqlf.Query {
	return sqlf.Sprintf(cleanupSyncJobsQueryFmtstr, opts.MaxPerExternalService, time.Now().Add(-opts.OlderThan))
}

func (e *externalServiceStore) hasRunningSyncJobs(ctx context.Context, id int64) (bool, error) {
	q := sqlf.Sprintf(`
SELECT 1
FROM external_service_sync_jobs
WHERE
	external_service_id = %s
	AND
	state IN ('queued', 'processing')
LIMIT 1
`, id)

	_, ok, err := basestore.ScanFirstInt(e.Query(ctx, q))
	return ok, err
}

type SyncError struct {
	ServiceID int64
	Message   string
}

var scanSyncErrors = basestore.NewSliceScanner(scanExternalServiceSyncErrorRow)

func scanExternalServiceSyncErrorRow(scanner dbutil.Scanner) (*SyncError, error) {
	var s SyncError
	err := scanner.Scan(
		&s.ServiceID,
		&dbutil.NullString{S: &s.Message},
	)
	return &s, err
}

func (e *externalServiceStore) GetLatestSyncErrors(ctx context.Context) ([]*SyncError, error) {
	q := sqlf.Sprintf(`
SELECT DISTINCT ON (es.id) es.id, essj.failure_message
FROM external_services es
         LEFT JOIN external_service_sync_jobs essj
                   ON es.id = essj.external_service_id
                       AND essj.state IN ('completed', 'errored', 'failed')
                       AND essj.finished_at IS NOT NULL
WHERE es.deleted_at IS NULL AND NOT es.cloud_default
ORDER BY es.id, essj.finished_at DESC
`)

	return scanSyncErrors(e.Query(ctx, q))
}

func (e *externalServiceStore) List(ctx context.Context, opt ExternalServicesListOptions) (_ []*types.ExternalService, err error) {
	tr, ctx := trace.New(ctx, "externalServiceStore.List")
	defer tr.EndWithErr(&err)

	if opt.OrderByDirection != "ASC" {
		opt.OrderByDirection = "DESC"
	}

	q := sqlf.Sprintf(`
		SELECT
			id,
			kind,
			display_name,
			config,
			encryption_key_id,
			created_at,
			updated_at,
			deleted_at,
			last_sync_at,
			next_sync_at,
			unrestricted,
			cloud_default,
			has_webhooks,
			token_expires_at,
			code_host_id,
			creator_id,
			last_updater_id
		FROM external_services
		WHERE (%s)
		ORDER BY id `+opt.OrderByDirection+`
		%s`,
		sqlf.Join(opt.sqlConditions(), ") AND ("),
		opt.LimitOffset.SQL(),
	)

	rows, err := e.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*types.ExternalService
	for rows.Next() {
		var (
			h               types.ExternalService
			deletedAt       sql.NullTime
			lastSyncAt      sql.NullTime
			nextSyncAt      sql.NullTime
			encryptedConfig string
			keyID           string
			hasWebhooks     sql.NullBool
			tokenExpiresAt  sql.NullTime
		)
		if err := rows.Scan(
			&h.ID,
			&h.Kind,
			&h.DisplayName,
			&encryptedConfig,
			&keyID,
			&h.CreatedAt,
			&h.UpdatedAt,
			&deletedAt,
			&lastSyncAt,
			&nextSyncAt,
			&h.Unrestricted,
			&h.CloudDefault,
			&hasWebhooks,
			&tokenExpiresAt,
			&h.CodeHostID,
			&h.CreatorID,
			&h.LastUpdaterID,
		); err != nil {
			return nil, err
		}

		if deletedAt.Valid {
			h.DeletedAt = deletedAt.Time
		}
		if lastSyncAt.Valid {
			h.LastSyncAt = lastSyncAt.Time
		}
		if nextSyncAt.Valid {
			h.NextSyncAt = nextSyncAt.Time
		}
		if hasWebhooks.Valid {
			h.HasWebhooks = &hasWebhooks.Bool
		}
		if tokenExpiresAt.Valid {
			h.TokenExpiresAt = &tokenExpiresAt.Time
		}
		h.Config = extsvc.NewEncryptedConfig(encryptedConfig, keyID, e.getEncryptionKey())

		results = append(results, &h)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (e *externalServiceStore) ListRepos(ctx context.Context, opt ExternalServiceReposListOptions) (_ []*types.ExternalServiceRepo, err error) {
	tr, ctx := trace.New(ctx, "externalServiceStore.ListRepos")
	defer tr.EndWithErr(&err)

	predicate := sqlf.Sprintf("TRUE")

	if opt.ExternalServiceID != 0 {
		predicate = sqlf.Sprintf("external_service_id = %s", opt.ExternalServiceID)
	}

	q := sqlf.Sprintf(`
SELECT
	external_service_id,
	repo_id,
	clone_url,
	created_at
FROM external_service_repos
WHERE %s
%s`,
		predicate,
		opt.LimitOffset.SQL(),
	)

	return scanExternalServiceRepos(e.Query(ctx, q))
}

var scanExternalServiceRepos = basestore.NewSliceScanner(scanExternalServiceRepo)

func scanExternalServiceRepo(s dbutil.Scanner) (*types.ExternalServiceRepo, error) {
	var repo types.ExternalServiceRepo

	if err := s.Scan(
		&repo.ExternalServiceID,
		&repo.RepoID,
		&repo.CloneURL,
		&repo.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &repo, nil
}

func (e *externalServiceStore) DistinctKinds(ctx context.Context) ([]string, error) {
	q := sqlf.Sprintf(`
SELECT ARRAY_AGG(DISTINCT(kind)::TEXT)
FROM external_services
WHERE deleted_at IS NULL
`)

	var kinds []string
	err := e.QueryRow(ctx, q).Scan(pq.Array(&kinds))
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, nil
		}
		return nil, err
	}

	return kinds, nil
}

func (e *externalServiceStore) Count(ctx context.Context, opt ExternalServicesListOptions) (int, error) {
	q := sqlf.Sprintf("SELECT COUNT(*) FROM external_services WHERE (%s)", sqlf.Join(opt.sqlConditions(), ") AND ("))
	var count int
	if err := e.QueryRow(ctx, q).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (e *externalServiceStore) RepoCount(ctx context.Context, id int64) (int32, error) {
	q := sqlf.Sprintf("SELECT COUNT(*) FROM external_service_repos WHERE external_service_id = %s", id)
	var count int32

	if err := e.QueryRow(ctx, q).Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (e *externalServiceStore) SyncDue(ctx context.Context, intIDs []int64, d time.Duration) (bool, error) {
	if len(intIDs) == 0 {
		return false, nil
	}
	ids := make([]*sqlf.Query, 0, len(intIDs))
	for _, id := range intIDs {
		ids = append(ids, sqlf.Sprintf("%s", id))
	}
	idFilter := sqlf.Sprintf("IN (%s)", sqlf.Join(ids, ","))
	deadline := time.Now().Add(d)

	q := sqlf.Sprintf(`
SELECT TRUE
WHERE EXISTS(
        SELECT
        FROM external_services
        WHERE id %s
          AND (
                next_sync_at IS NULL
                OR next_sync_at <= %s)
    )
   OR EXISTS(
        SELECT
        FROM external_service_sync_jobs
        WHERE external_service_id %s
          AND state IN ('queued', 'processing')
    );
`, idFilter, deadline, idFilter)

	v, exists, err := basestore.ScanFirstBool(e.Query(ctx, q))
	if err != nil {
		return false, err
	}
	return v && exists, nil
}

// calcUnrestricted determines whether or not permissions should be enforced
// on an external service.
//
// isDotComMode and permissionsUserMappingEnabled can be passed via
//
//	dotcom.SourcegraphDotComMode() and globals.PermissionsUserMapping().Enabled
//
// respectively.
func calcUnrestricted(config string, isDotComMode bool, permissionsUserMappingEnabled bool) bool {
	if isDotComMode {
		return false
	}

	// If PermissionsUserMapping is enabled, we return false since permissions
	// will be managed by the explicit permissions API.
	if permissionsUserMappingEnabled {
		return false
	}

	// Only override the value of es.Unrestricted if `enforcePermissions` is set.
	//
	// All code hosts apart from Azure DevOps use the `authorization` pattern for enforcing
	// permissions. Instead of continuing to use this pattern for Azure DevOps, it is simpler to add
	// a boolean which has an explicit name and describes what it does better.
	//
	// The end result: we start to break away from the `authorization` pattern with an additional
	// check for this new field - `enforcePermissions`.
	//
	// For existing auth providers, this is forwards compatible. While at the same time if they also
	// wanted to get on the `enforcePermissions` pattern, this change is backwards compatible.
	enforcePermissions := gjson.Get(config, "enforcePermissions")
	if enforcePermissions.Exists() {
		return !enforcePermissions.Bool()
	}

	return !gjson.Get(config, "authorization").Exists()
}

// recalculateFields updates the value of the external service fields that are
// calculated depending on the external service configuration, namely
// `Unrestricted` and `HasWebhooks`.
func (e *externalServiceStore) recalculateFields(es *types.ExternalService, rawConfig string) {
	es.Unrestricted = calcUnrestricted(rawConfig, dotcom.SourcegraphDotComMode(), conf.PermissionsUserMapping().Enabled)

	hasWebhooks := false
	cfg, err := extsvc.ParseConfig(es.Kind, rawConfig)
	if err == nil {
		hasWebhooks = configurationHasWebhooks(cfg)
	} else {
		// Legacy configurations might not be valid JSON; in that case, they
		// also can't have webhooks, so we'll just log the issue and move on.
		e.logger.Warn("cannot parse external service configuration as JSON", log.Error(err), log.Int64("id", es.ID))
	}
	es.HasWebhooks = &hasWebhooks
}

func ensureCodeHost(ctx context.Context, tx *externalServiceStore, kind string, config string) (codeHostID int32, _ error) {
	// Ensure a code host for this external service exists.
	// TODO: Use this method for the OOB migrator as well.
	codeHostIdentifier, err := extsvc.UniqueCodeHostIdentifier(kind, config)
	if err != nil {
		return 0, err
	}
	// TODO: Use this method for the OOB migrator as well.
	rateLimit, isDefaultRateLimit, err := extsvc.ExtractRateLimit(config, kind)
	if err != nil && !errors.HasType(err, extsvc.ErrRateLimitUnsupported{}) {
		return 0, err
	}
	ch := &types.CodeHost{
		Kind:      kind,
		URL:       codeHostIdentifier,
		CreatedAt: timeutil.Now(),
	}
	if rateLimit != rate.Inf && rateLimit != 0. && !isDefaultRateLimit {
		ch.APIRateLimitQuota = pointers.Ptr(int32(rateLimit * 3600.0))
		ch.APIRateLimitIntervalSeconds = pointers.Ptr(int32(3600))
	}
	siteCfg := conf.Get()
	if siteCfg.GitMaxCodehostRequestsPerSecond != nil {
		ch.GitRateLimitQuota = pointers.Ptr(int32(*siteCfg.GitMaxCodehostRequestsPerSecond))
		ch.GitRateLimitIntervalSeconds = pointers.Ptr(int32(1))
	}
	chstore := CodeHostsWith(tx)
	if err := chstore.Create(ctx, ch); err != nil {
		return 0, errors.Wrap(err, "failed to create code host")
	}
	return ch.ID, nil
}

func configurationHasWebhooks(config any) bool {
	switch v := config.(type) {
	case *schema.GitHubConnection:
		return len(v.Webhooks) > 0
	case *schema.GitLabConnection:
		return len(v.Webhooks) > 0
	case *schema.BitbucketServerConnection:
		return v.WebhookSecret() != ""
	}

	return false
}
