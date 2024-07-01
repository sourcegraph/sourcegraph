package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	gha "github.com/sourcegraph/sourcegraph/internal/github_apps/store"
)

// DB is an interface that embeds dbutil.DB, adding methods to
// return specialized stores on top of that interface. In time,
// the expectation is to replace uses of dbutil.DB with database.DB,
// and remove dbutil.DB altogether.
type DB interface {
	dbutil.DB
	basestore.ShareableStore

	AccessRequests() AccessRequestStore
	AccessTokens() AccessTokenStore
	Authz() AuthzStore
	BitbucketProjectPermissions() BitbucketProjectPermissionsStore
	CodeMonitors() CodeMonitorStore
	CodeHosts() CodeHostStore
	Codeowners() CodeownersStore
	Conf() ConfStore
	EventLogs() EventLogStore
	SecurityEventLogs() SecurityEventLogsStore
	ExternalServices() ExternalServiceStore
	FeatureFlags() FeatureFlagStore
	GitHubApps() gha.GitHubAppsStore
	GitserverRepos() GitserverRepoStore
	GlobalState() GlobalStateStore
	NamespacePermissions() NamespacePermissionStore
	Namespaces() NamespaceStore
	OrgInvitations() OrgInvitationStore
	OrgMembers() OrgMemberStore
	Orgs() OrgStore
	OutboundWebhooks(encryption.Key) OutboundWebhookStore
	OutboundWebhookJobs(encryption.Key) OutboundWebhookJobStore
	OutboundWebhookLogs(encryption.Key) OutboundWebhookLogStore
	OwnershipStats() OwnershipStatsStore
	RecentContributionSignals() RecentContributionSignalStore
	Perms() PermsStore
	Permissions() PermissionStore
	PermissionSyncJobs() PermissionSyncJobStore
	Phabricator() PhabricatorStore
	RedisKeyValue() RedisKeyValueStore
	Repos() RepoStore
	RepoCommitsChangelists() RepoCommitsChangelistsStore
	RepoKVPs() RepoKVPStore
	RepoPaths() RepoPathStore
	RolePermissions() RolePermissionStore
	Roles() RoleStore
	SavedSearches() SavedSearchStore
	SearchContexts() SearchContextsStore
	Settings() SettingsStore
	SubRepoPerms() SubRepoPermsStore
	TemporarySettings() TemporarySettingsStore
	TelemetryEventsExportQueue() TelemetryEventsExportQueueStore
	UserCredentials(encryption.Key) UserCredentialsStore
	UserEmails() UserEmailsStore
	UserExternalAccounts() UserExternalAccountsStore
	UserRoles() UserRoleStore
	Users() UserStore
	WebhookLogs(encryption.Key) WebhookLogStore
	Webhooks(encryption.Key) WebhookStore
	RepoStatistics() RepoStatisticsStore
	Executors() ExecutorStore
	ExecutorSecrets(encryption.Key) ExecutorSecretStore
	ExecutorSecretAccessLogs() ExecutorSecretAccessLogStore
	ZoektRepos() ZoektReposStore
	Teams() TeamStore
	EventLogsScrapeState() EventLogsScrapeStateStore
	RecentViewSignal() RecentViewSignalStore
	AssignedOwners() AssignedOwnersStore
	AssignedTeams() AssignedTeamsStore
	OwnSignalConfigurations() SignalConfigurationStore
	Workflows() WorkflowStore

	WithTransact(context.Context, func(tx DB) error) error
}

var _ DB = (*db)(nil)

// NewDB creates a new DB from a dbutil.DB, providing a thin wrapper
// that has constructor methods for the more specialized stores.
func NewDB(logger log.Logger, inner *sql.DB) DB {
	return &db{logger: logger, Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, inner, sql.TxOptions{}))}
}

func NewDBWith(logger log.Logger, other basestore.ShareableStore) DB {
	return &db{logger: logger, Store: basestore.NewWithHandle(other.Handle())}
}

type db struct {
	*basestore.Store
	logger log.Logger
}

func (d *db) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return d.Handle().QueryContext(dbconn.SkipFrameForQuerySource(ctx), q, args...)
}

func (d *db) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return d.Handle().ExecContext(dbconn.SkipFrameForQuerySource(ctx), q, args...)
}

func (d *db) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	return d.Handle().QueryRowContext(dbconn.SkipFrameForQuerySource(ctx), q, args...)
}

func (d *db) Transact(ctx context.Context) (DB, error) {
	tx, err := d.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}
	return &db{logger: d.logger, Store: tx}, nil
}

func (d *db) WithTransact(ctx context.Context, f func(tx DB) error) error {
	return d.Store.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&db{logger: d.logger, Store: tx})
	})
}

func (d *db) Done(err error) error {
	return d.Store.Done(err)
}

func (d *db) AccessTokens() AccessTokenStore {
	return AccessTokensWith(d.Store, d.logger.Scoped("AccessTokenStore"))
}

func (d *db) AccessRequests() AccessRequestStore {
	return AccessRequestsWith(d.Store, d.logger.Scoped("AccessRequestStore"))
}

func (d *db) BitbucketProjectPermissions() BitbucketProjectPermissionsStore {
	return BitbucketProjectPermissionsStoreWith(d.Store)
}

func (d *db) Authz() AuthzStore {
	return AuthzWith(d.Store)
}

func (d *db) CodeMonitors() CodeMonitorStore {
	return CodeMonitorsWith(d.Store)
}

func (d *db) CodeHosts() CodeHostStore {
	return CodeHostsWith(d.Store)
}

func (d *db) Codeowners() CodeownersStore {
	return CodeownersWith(basestore.NewWithHandle(d.Handle()))
}

func (d *db) Conf() ConfStore {
	return ConfStoreWith(d.Store)
}

func (d *db) EventLogs() EventLogStore {
	return EventLogsWith(d.Store)
}

func (d *db) SecurityEventLogs() SecurityEventLogsStore {
	return SecurityEventLogsWith(d.logger, d.Store)
}

func (d *db) ExternalServices() ExternalServiceStore {
	return ExternalServicesWith(d.logger, d.Store)
}

func (d *db) FeatureFlags() FeatureFlagStore {
	return FeatureFlagsWith(d.Store)
}

func (d *db) GitHubApps() gha.GitHubAppsStore {
	return gha.GitHubAppsWith(d.Store)
}

func (d *db) GitserverRepos() GitserverRepoStore {
	return GitserverReposWith(d.Store)
}

func (d *db) GlobalState() GlobalStateStore {
	return GlobalStateWith(d.Store)
}

func (d *db) NamespacePermissions() NamespacePermissionStore {
	return NamespacePermissionsWith(d.Store)
}

func (d *db) Namespaces() NamespaceStore {
	return NamespacesWith(d.Store)
}

func (d *db) OrgInvitations() OrgInvitationStore {
	return OrgInvitationsWith(d.Store)
}

func (d *db) OrgMembers() OrgMemberStore {
	return OrgMembersWith(d.Store)
}

func (d *db) Orgs() OrgStore {
	return OrgsWith(d.Store)
}

func (d *db) OutboundWebhooks(key encryption.Key) OutboundWebhookStore {
	return OutboundWebhooksWith(d.Store, key)
}

func (d *db) OutboundWebhookJobs(key encryption.Key) OutboundWebhookJobStore {
	return OutboundWebhookJobsWith(d.Store, key)
}

func (d *db) OutboundWebhookLogs(key encryption.Key) OutboundWebhookLogStore {
	return OutboundWebhookLogsWith(d.Store, key)
}

func (d *db) OwnershipStats() OwnershipStatsStore {
	return &ownershipStats{d.Store}
}

func (d *db) RecentContributionSignals() RecentContributionSignalStore {
	return RecentContributionSignalStoreWith(d.Store)
}

func (d *db) Permissions() PermissionStore {
	return PermissionsWith(d.Store)
}

func (d *db) Perms() PermsStore {
	return PermsWith(d.logger, d.Store, time.Now)
}

func (d *db) PermissionSyncJobs() PermissionSyncJobStore {
	return PermissionSyncJobsWith(d.logger, d.Store)
}

func (d *db) Phabricator() PhabricatorStore {
	return PhabricatorWith(d.Store)
}

func (d *db) RedisKeyValue() RedisKeyValueStore {
	return &redisKeyValueStore{d.Store}
}

func (d *db) Repos() RepoStore {
	return ReposWith(d.logger, d.Store)
}

func (d *db) RepoCommitsChangelists() RepoCommitsChangelistsStore {
	return RepoCommitsChangelistsWith(d.logger, d.Store)
}

func (d *db) RepoKVPs() RepoKVPStore {
	return &repoKVPStore{d.Store}
}

func (d *db) RepoPaths() RepoPathStore {
	return &repoPathStore{d.Store}
}

func (d *db) RolePermissions() RolePermissionStore {
	return RolePermissionsWith(d.Store)
}

func (d *db) Roles() RoleStore {
	return RolesWith(d.Store)
}

func (d *db) SavedSearches() SavedSearchStore {
	return SavedSearchesWith(d.Store)
}

func (d *db) SearchContexts() SearchContextsStore {
	return SearchContextsWith(d.logger, d.Store)
}

func (d *db) Settings() SettingsStore {
	return SettingsWith(d.Store)
}

func (d *db) SubRepoPerms() SubRepoPermsStore {
	return SubRepoPermsWith(basestore.NewWithHandle(d.Handle()))
}

func (d *db) TemporarySettings() TemporarySettingsStore {
	return TemporarySettingsWith(d.Store)
}

func (d *db) TelemetryEventsExportQueue() TelemetryEventsExportQueueStore {
	return TelemetryEventsExportQueueWith(
		d.logger.Scoped("telemetry_events"),
		d.Store,
	)
}

func (d *db) UserCredentials(key encryption.Key) UserCredentialsStore {
	return UserCredentialsWith(d.logger, d.Store, key)
}

func (d *db) UserEmails() UserEmailsStore {
	return UserEmailsWith(d.Store)
}

func (d *db) UserExternalAccounts() UserExternalAccountsStore {
	return ExternalAccountsWith(d.logger, d.Store)
}

func (d *db) UserRoles() UserRoleStore {
	return UserRolesWith(d.Store)
}

func (d *db) Users() UserStore {
	return UsersWith(d.logger, d.Store)
}

func (d *db) WebhookLogs(key encryption.Key) WebhookLogStore {
	return WebhookLogsWith(d.Store, key)
}

func (d *db) Webhooks(key encryption.Key) WebhookStore {
	return WebhooksWith(d.Store, key)
}

func (d *db) RepoStatistics() RepoStatisticsStore {
	return RepoStatisticsWith(d.Store)
}

func (d *db) Executors() ExecutorStore {
	return ExecutorsWith(d.Store)
}

func (d *db) ExecutorSecrets(key encryption.Key) ExecutorSecretStore {
	return ExecutorSecretsWith(d.logger, d.Store, key)
}

func (d *db) ExecutorSecretAccessLogs() ExecutorSecretAccessLogStore {
	return ExecutorSecretAccessLogsWith(d.Store)
}

func (d *db) ZoektRepos() ZoektReposStore {
	return ZoektReposWith(d.Store)
}

func (d *db) Teams() TeamStore {
	return TeamsWith(d.Store)
}

func (d *db) EventLogsScrapeState() EventLogsScrapeStateStore {
	return EventLogsScrapeStateStoreWith(d.Store)
}

func (d *db) RecentViewSignal() RecentViewSignalStore {
	return RecentViewSignalStoreWith(d.Store, d.logger)
}

func (d *db) AssignedOwners() AssignedOwnersStore {
	return AssignedOwnersStoreWith(d.Store, d.logger)
}

func (d *db) AssignedTeams() AssignedTeamsStore {
	return AssignedTeamsStoreWith(d.Store, d.logger)
}

func (d *db) OwnSignalConfigurations() SignalConfigurationStore {
	return SignalConfigurationStoreWith(d.Store)
}

func (d *db) Workflows() WorkflowStore {
	return WorkflowsWith(d.Store)
}
