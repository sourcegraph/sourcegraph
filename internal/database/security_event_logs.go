package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log"

	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/audit"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type SecurityEventName string

const (
	SecurityEventNameSignOutAttempted SecurityEventName = "SignOutAttempted"
	SecurityEventNameSignOutFailed    SecurityEventName = "SignOutFailed"
	SecurityEventNameSignOutSucceeded SecurityEventName = "SignOutSucceeded"

	SecurityEventNameSignInAttempted SecurityEventName = "SignInAttempted"
	SecurityEventNameSignInFailed    SecurityEventName = "SignInFailed"
	SecurityEventNameSignInSucceeded SecurityEventName = "SignInSucceeded"

	SecurityEventNameAccountCreated  SecurityEventName = "AccountCreated"
	SecurityEventNameAccountDeleted  SecurityEventName = "AccountDeleted"
	SecurityEventNameAccountModified SecurityEventName = "AccountModified"
	SecurityEventNameAccountNuked    SecurityEventName = "AccountNuked"

	SecurityEventNamPasswordResetRequested SecurityEventName = "PasswordResetRequested"
	SecurityEventNamPasswordRandomized     SecurityEventName = "PasswordRandomized"
	SecurityEventNamePasswordChanged       SecurityEventName = "PasswordChanged"

	SecurityEventNameEmailVerified       SecurityEventName = "EmailVerified"
	SecurityEventNameEmailVerifiedToggle SecurityEventName = "EmailVerificationChanged"
	SecurityEventNameEmailAdded          SecurityEventName = "EmailAdded"
	SecurityEventNameEmailRemoved        SecurityEventName = "EmailRemoved"

	SecurityEventNameRoleChangeDenied  SecurityEventName = "RoleChangeDenied"
	SecurityEventNameRoleChangeGranted SecurityEventName = "RoleChangeGranted"

	SecurityEventNameRBACRoleAdded     SecurityEventName = "RBACRoleAdded"
	SecurityEventNameRBACRoleRemoved   SecurityEventName = "RBACRoleRemoved"
	SecurityEventNameRBACRoleSet       SecurityEventName = "RBACRoleSet"
	SecurityEventNameRBACPermissionSet SecurityEventName = "RBACPermissionSet"

	SecurityEventNameAccessGranted SecurityEventName = "AccessGranted"

	SecurityEventAccessTokenCreated             SecurityEventName = "AccessTokenCreated"
	SecurityEventAccessTokenDeleted             SecurityEventName = "AccessTokenDeleted"
	SecurityEventAccessTokenHardDeleted         SecurityEventName = "AccessTokenHardDeleted"
	SecurityEventAccessTokenImpersonated        SecurityEventName = "AccessTokenImpersonated"
	SecurityEventAccessTokenInvalid             SecurityEventName = "AccessTokenInvalid"
	SecurityEventAccessTokenSubjectNotSiteAdmin SecurityEventName = "AccessTokenSubjectNotSiteAdmin"

	SecurityEventGitHubAuthSucceeded SecurityEventName = "GitHubAuthSucceeded"
	SecurityEventGitHubAuthFailed    SecurityEventName = "GitHubAuthFailed"

	SecurityEventGitLabAuthSucceeded SecurityEventName = "GitLabAuthSucceeded"
	SecurityEventGitLabAuthFailed    SecurityEventName = "GitLabAuthFailed"

	SecurityEventBitbucketCloudAuthSucceeded SecurityEventName = "BitbucketCloudAuthSucceeded"
	SecurityEventBitbucketCloudAuthFailed    SecurityEventName = "BitbucketCloudAuthFailed"

	SecurityEventBitbucketServerAuthSucceeded SecurityEventName = "BitbucketServerAuthSucceeded"
	SecurityEventBitbucketServerAuthFailed    SecurityEventName = "BitbucketServerAuthFailed"

	SecurityEventAzureDevOpsAuthSucceeded SecurityEventName = "AzureDevOpsAuthSucceeded"
	SecurityEventAzureDevOpsAuthFailed    SecurityEventName = "AzureDevOpsAuthFailed"

	SecurityEventOIDCLoginSucceeded SecurityEventName = "SecurityEventOIDCLoginSucceeded"
	SecurityEventOIDCLoginFailed    SecurityEventName = "SecurityEventOIDCLoginFailed"

	SecurityEventNameSiteConfigUpdated        SecurityEventName = "SiteConfigUpdated"
	SecurityEventNameSiteConfigRedactedViewed SecurityEventName = "SiteConfigRedactedViewed"
	SecurityEventNameSiteConfigViewed         SecurityEventName = "SiteConfigViewed"

	SecurityEventNameDotComLicenseCreated       SecurityEventName = "DotComLicenseCreated"
	SecurityEventNameDotComLicenseViewed        SecurityEventName = "DotComLicenseViewed"
	SecurityEventNameDotComSubscriptionViewed   SecurityEventName = "DotComSubscriptionViewed"
	SecurityEventNameDotComSubscriptionCreated  SecurityEventName = "DotComSubscriptionCreated"
	SecurityEventNameDotComSubscriptionArchived SecurityEventName = "DotComSubscriptionArchived"
	SecurityEventNameDotComSubscriptionsListed  SecurityEventName = "DotComSubscriptionsListed"
	SecurityEventNameDotComSubscriptionUpdated  SecurityEventName = "DotComSubscriptionUpdated"

	SecurityEventNameOrgViewed         SecurityEventName = "OrganizationViewed"
	SecurityEventNameOrgListViewed     SecurityEventName = "OrganizationListViewed"
	SecurityEventNameOrgCreated        SecurityEventName = "OrganizationCreated"
	SecurityEventNameOrgUpdated        SecurityEventName = "OrganizationUpdated"
	SecurityEventNameOrgSettingsViewed SecurityEventName = "OrganizationSettingsViewed"

	SecurityEventNameOutboundReqViewed SecurityEventName = "OutboundRequestViewed"

	SecurityEventNameUserCompletionQuotaUpdated     SecurityEventName = "UserCompletionQuotaUpdated"
	SecurityEventNameUserCodeCompletionQuotaUpdated SecurityEventName = "UserCodeCompletionQuotaUpdated"

	SecurityEventNameCodeHostConnectionsViewed SecurityEventName = "CodeHostConnectionsViewed"
	SecurityEventNameCodeHostConnectionDeleted SecurityEventName = "CodeHostConnectionDeleted"
	SecurityEventNameCodeHostConnectionAdded   SecurityEventName = "CodeHostConnectionAdded"
	SecurityEventNameCodeHostConnectionUpdated SecurityEventName = "CodeHostConnectionUpdated"
)

// SecurityEvent contains information needed for logging a security-relevant event.
type SecurityEvent struct {
	Name            SecurityEventName
	URL             string
	UserID          uint32
	AnonymousUserID string
	Argument        json.RawMessage
	Source          string
	Timestamp       time.Time
}

func (e *SecurityEvent) argumentToJSONString() string {
	if e.Argument == nil || string(e.Argument) == "null" {
		return "{}"
	}
	return string(e.Argument)
}

// SecurityEventLogsStore provides persistence for security events.
type SecurityEventLogsStore interface {
	basestore.ShareableStore

	// Insert adds a new security event to the store.
	Insert(ctx context.Context, e *SecurityEvent) error
	// Bulk "Insert" action.
	InsertList(ctx context.Context, events []*SecurityEvent) error
	// LogEvent logs the given security events.
	//
	// It logs errors directly instead of returning to callers.
	LogEvent(ctx context.Context, e *SecurityEvent)
	// Bulk "LogEvent" action.
	LogEventList(ctx context.Context, events []*SecurityEvent)
	// LogSecurityEvent creates an event and logs it.
	LogSecurityEvent(ctx context.Context, eventName SecurityEventName, url string, userID uint32, anonymousUserID string, source string, arguments any) error
}

type securityEventLogsStore struct {
	logger log.Logger
	*basestore.Store
}

// SecurityEventLogsWith instantiates and returns a new SecurityEventLogsStore
// using the other store handle, and a scoped sub-logger of the passed base logger.
func SecurityEventLogsWith(baseLogger log.Logger, other basestore.ShareableStore) SecurityEventLogsStore {
	logger := baseLogger.Scoped("SecurityEvents")
	return &securityEventLogsStore{logger: logger, Store: basestore.NewWithHandle(other.Handle())}
}

func (s *securityEventLogsStore) Insert(ctx context.Context, event *SecurityEvent) error {
	return s.InsertList(ctx, []*SecurityEvent{event})
}

func (s *securityEventLogsStore) InsertList(ctx context.Context, events []*SecurityEvent) error {
	cfg := conf.SiteConfig()
	loc := audit.SecurityEventLocation(cfg)
	if loc == audit.None {
		return nil
	}

	actor := sgactor.FromContext(ctx)
	vals := make([]*sqlf.Query, len(events))
	for index, event := range events {
		// Add an attribution for Sourcegraph operator to be distinguished in our analytics pipelines
		if actor.SourcegraphOperator {
			result, err := jsonc.Edit(
				event.argumentToJSONString(),
				true,
				EventLogsSourcegraphOperatorKey,
			)
			event.Argument = json.RawMessage(result)
			if err != nil {
				return errors.Wrap(err, `edit "argument" for Sourcegraph operator`)
			}
		}

		// If actor is internal, we may violate the security_event_logs_check_has_user
		// constraint, since internal actors do not have either an anonymous UID or a
		// user ID - at many callsites, we already set anonymous UID as "internal" in
		// these scenarios, so as a workaround, we assign the event the anonymous UID
		// "internal".
		noUser := event.UserID == 0 && event.AnonymousUserID == ""
		if actor.IsInternal() && noUser {
			// only log internal access if we are explicitly configured to do so
			if !audit.IsEnabled(cfg, audit.InternalTraffic) {
				return nil
			}
			event.AnonymousUserID = "internal"
		}

		// Set values corresponding to this event.
		vals[index] = sqlf.Sprintf(`(%s, %s, %s, %s, %s, %s, %s, %s)`,
			event.Name,
			event.URL,
			event.UserID,
			event.AnonymousUserID,
			event.Source,
			event.argumentToJSONString(),
			version.Version(),
			event.Timestamp.UTC(),
		)
	}

	if loc == audit.Database || loc == audit.All {
		query := sqlf.Sprintf("INSERT INTO security_event_logs(name, url, user_id, anonymous_user_id, source, argument, version, timestamp) VALUES %s", sqlf.Join(vals, ","))

		if _, err := s.Handle().ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
			return errors.Wrap(err, "INSERT")
		}
	}
	if loc == audit.AuditLog || loc == audit.All {
		for _, event := range events {
			audit.Log(ctx, s.logger, audit.Record{
				Entity: "security events",
				Action: string(event.Name),
				Fields: []log.Field{
					log.Object("event",
						log.String("URL", event.URL),
						log.Uint32("UserID", event.UserID),
						log.String("AnonymousUserID", event.AnonymousUserID),
						log.String("source", event.Source),
						log.String("argument", event.argumentToJSONString()),
						log.String("version", version.Version()),
						log.String("timestamp", event.Timestamp.UTC().String()),
					),
				},
			})
		}
	}
	return nil
}

func (s *securityEventLogsStore) LogEvent(ctx context.Context, e *SecurityEvent) {
	s.LogEventList(ctx, []*SecurityEvent{e})
}

func (s *securityEventLogsStore) LogEventList(ctx context.Context, events []*SecurityEvent) {
	if err := s.InsertList(ctx, events); err != nil {
		names := make([]string, len(events))
		for i, e := range events {
			names[i] = string(e.Name)
		}
		j, _ := json.Marshal(&events)
		if errors.Is(err, context.Canceled) {
			trace.Logger(ctx, s.logger).Warn(strings.Join(names, ","), log.String("events", string(j)), log.Error(err))
		} else {
			trace.Logger(ctx, s.logger).Error(strings.Join(names, ","), log.String("events", string(j)), log.Error(err))
		}
	}
}

func (s *securityEventLogsStore) LogSecurityEvent(ctx context.Context, eventName SecurityEventName, url string, userID uint32, anonymousUserID string, source string, arguments any) error {
	event := SecurityEvent{
		Name:            eventName,
		URL:             url,
		UserID:          userID,
		AnonymousUserID: anonymousUserID,
		Source:          source,
		Timestamp:       time.Now(),
	}
	argsJSON, err := json.Marshal(arguments)
	if err != nil {
		event.Argument = nil
		s.LogEvent(ctx, &event)
		return errors.Wrap(err, "error marshalling arguments")
	} else {
		event.Argument = argsJSON
		s.LogEvent(ctx, &event)
		return nil
	}
}
