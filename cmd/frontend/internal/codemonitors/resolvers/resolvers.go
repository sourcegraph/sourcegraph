package resolvers

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/codemonitors"
	"github.com/sourcegraph/sourcegraph/internal/codemonitors/background"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// NewResolver returns a new Resolver that uses the given database
func NewResolver(logger log.Logger, db database.DB) graphqlbackend.CodeMonitorsResolver {
	return &Resolver{logger: logger, db: db}
}

type Resolver struct {
	logger log.Logger
	db     database.DB
}

func (r *Resolver) Now() time.Time {
	return r.db.CodeMonitors().Now()
}

func (r *Resolver) NodeResolvers() map[string]graphqlbackend.NodeByIDFunc {
	return map[string]graphqlbackend.NodeByIDFunc{
		MonitorKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.MonitorByID(ctx, id)
		},
		// TODO: These kinds are currently not implemented, but need a node resolver.
		// monitorTriggerQueryKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
		// 	return r.MonitorTriggerQueryByID(ctx, id)
		// },
		// monitorTriggerEventKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
		// 	return r.MonitorTriggerEventByID(ctx, id)
		// },
		// monitorActionEmailKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
		// 	return r.MonitorActionEmailByID(ctx, id)
		// },
		// monitorActionEventKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
		// 	return r.MonitorActionEventByID(ctx, id)
		// },
	}
}

func (r *Resolver) Monitors(ctx context.Context, userID *int32, args *graphqlbackend.ListMonitorsArgs) (graphqlbackend.MonitorConnectionResolver, error) {
	// Request one extra to determine if there are more pages
	newArgs := *args
	newArgs.First += 1

	after, err := unmarshalAfter(args.After)
	if err != nil {
		return nil, err
	}

	listOpts := database.ListMonitorsOpts{
		UserID:       userID,
		First:        pointers.Ptr(int(newArgs.First)),
		After:        intPtrToInt64Ptr(after),
		SkipOrphaned: true,
	}

	ms, err := r.db.CodeMonitors().ListMonitors(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	totalCount, err := r.db.CodeMonitors().CountMonitors(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	hasNextPage := false
	if len(ms) == int(args.First)+1 {
		hasNextPage = true
		ms = ms[:len(ms)-1]
	}

	mrs := make([]graphqlbackend.MonitorResolver, 0, len(ms))
	for _, m := range ms {
		mrs = append(mrs, &monitor{
			Resolver: r,
			Monitor:  m,
		})
	}

	return &monitorConnection{Resolver: r, monitors: mrs, totalCount: totalCount, hasNextPage: hasNextPage}, nil
}

func (r *Resolver) MonitorByID(ctx context.Context, id graphql.ID) (graphqlbackend.MonitorResolver, error) {
	err := r.isAllowedToEdit(ctx, id)
	if err != nil {
		return nil, err
	}
	monitorID, err := unmarshalMonitorID(id)
	if err != nil {
		return nil, err
	}
	mo, err := r.db.CodeMonitors().GetMonitor(ctx, monitorID)
	if err != nil {
		return nil, err
	}
	return &monitor{
		Resolver: r,
		Monitor:  mo,
	}, nil
}

func (r *Resolver) CreateCodeMonitor(ctx context.Context, args *graphqlbackend.CreateCodeMonitorArgs) (_ graphqlbackend.MonitorResolver, err error) {
	if err := r.isAllowedToCreate(ctx, args.Monitor.Namespace); err != nil {
		return nil, err
	}

	userID, orgID, err := graphqlbackend.UnmarshalNamespaceToIDs(args.Monitor.Namespace)
	if err != nil {
		return nil, err
	}

	// Snapshot the state of the searched repos when the monitor is created so that
	// we can distinguish new repos. We run the snapshot outside the transaction because
	// search requires that the DB handle is not a transaction.
	resolvedRevisions, err := codemonitors.Snapshot(ctx, r.logger, r.db, args.Trigger.Query)
	if err != nil {
		return nil, err
	}

	// Start transaction.
	var newMonitor *database.Monitor
	err = r.withTransact(ctx, func(tx *Resolver) error {
		// Create monitor.
		m, err := tx.db.CodeMonitors().CreateMonitor(ctx, database.MonitorArgs{
			Description:     args.Monitor.Description,
			Enabled:         args.Monitor.Enabled,
			NamespaceUserID: userID,
			NamespaceOrgID:  orgID,
		})
		if err != nil {
			return err
		}

		// Create trigger.
		_, err = tx.db.CodeMonitors().CreateQueryTrigger(ctx, m.ID, args.Trigger.Query)
		if err != nil {
			return err
		}

		// Save the snapshotted commit IDs
		for repoID, commitIDs := range resolvedRevisions {
			err = tx.db.CodeMonitors().UpsertLastSearched(ctx, m.ID, repoID, commitIDs)
			if err != nil {
				return err
			}
		}

		// Create actions.
		err = tx.createActions(ctx, m.ID, args.Actions)
		if err != nil {
			return err
		}

		newMonitor = m
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &monitor{
		Resolver: r,
		Monitor:  newMonitor,
	}, nil
}

func (r *Resolver) ToggleCodeMonitor(ctx context.Context, args *graphqlbackend.ToggleCodeMonitorArgs) (graphqlbackend.MonitorResolver, error) {
	err := r.isAllowedToEdit(ctx, args.Id)
	if err != nil {
		return nil, errors.Errorf("UpdateMonitorEnabled: %w", err)
	}
	monitorID, err := unmarshalMonitorID(args.Id)
	if err != nil {
		return nil, err
	}

	mo, err := r.db.CodeMonitors().UpdateMonitorEnabled(ctx, monitorID, args.Enabled)
	if err != nil {
		return nil, err
	}
	return &monitor{r, mo}, nil
}

func (r *Resolver) DeleteCodeMonitor(ctx context.Context, args *graphqlbackend.DeleteCodeMonitorArgs) (*graphqlbackend.EmptyResponse, error) {
	err := r.isAllowedToEdit(ctx, args.Id)
	if err != nil {
		return nil, errors.Errorf("DeleteCodeMonitor: %w", err)
	}

	monitorID, err := unmarshalMonitorID(args.Id)
	if err != nil {
		return nil, err
	}

	if err := r.db.CodeMonitors().DeleteMonitor(ctx, monitorID); err != nil {
		return nil, err
	}
	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) UpdateCodeMonitor(ctx context.Context, args *graphqlbackend.UpdateCodeMonitorArgs) (graphqlbackend.MonitorResolver, error) {
	err := r.isAllowedToEdit(ctx, args.Monitor.Id)
	if err != nil {
		return nil, errors.Errorf("UpdateCodeMonitor: %w", err)
	}

	err = r.isAllowedToCreate(ctx, args.Monitor.Update.Namespace)
	if err != nil {
		return nil, errors.Errorf("update namespace: %w", err)
	}

	monitorID, err := unmarshalMonitorID(args.Monitor.Id)
	if err != nil {
		return nil, err
	}

	// Get all action IDs of the monitor.
	actionIDs, err := r.actionIDsForMonitorIDInt64(ctx, monitorID)
	if err != nil {
		return nil, err
	}

	toCreate, toDelete, err := splitActionIDs(args, actionIDs)
	if len(toDelete) == len(actionIDs) && len(toCreate) == 0 {
		return nil, errors.Errorf("you tried to delete all actions, but every monitor must be connected to at least 1 action")
	}

	// Run all queries within a transaction.
	var updatedMonitor *monitor
	err = r.withTransact(ctx, func(tx *Resolver) error {
		if err = tx.deleteActions(ctx, monitorID, toDelete); err != nil {
			return err
		}
		if err = tx.createActions(ctx, monitorID, toCreate); err != nil {
			return err
		}
		m, err := tx.updateCodeMonitor(ctx, r.db, args)
		if err != nil {
			return err
		}

		updatedMonitor = m
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Hydrate monitor with Resolver.
	updatedMonitor.Resolver = r
	return updatedMonitor, nil
}

func (r *Resolver) createActions(ctx context.Context, monitorID int64, args []*graphqlbackend.CreateActionArgs) error {
	for _, a := range args {
		switch {
		case a.Email != nil:
			e, err := r.db.CodeMonitors().CreateEmailAction(ctx, monitorID, &database.EmailActionArgs{
				Enabled:        a.Email.Enabled,
				IncludeResults: a.Email.IncludeResults,
				Priority:       a.Email.Priority,
				Header:         a.Email.Header,
			})
			if err != nil {
				return err
			}

			if err := r.createRecipients(ctx, e.ID, a.Email.Recipients); err != nil {
				return err
			}
		case a.Webhook != nil:
			_, err := r.db.CodeMonitors().CreateWebhookAction(ctx, monitorID, a.Webhook.Enabled, a.Webhook.IncludeResults, a.Webhook.URL)
			if err != nil {
				return err
			}
		case a.SlackWebhook != nil:
			if err := validateSlackURL(a.SlackWebhook.URL); err != nil {
				return err
			}
			_, err := r.db.CodeMonitors().CreateSlackWebhookAction(ctx, monitorID, a.SlackWebhook.Enabled, a.SlackWebhook.IncludeResults, a.SlackWebhook.URL)
			if err != nil {
				return err
			}
		default:
			return errors.New("exactly one of Email, Webhook, or SlackWebhook must be set")
		}
	}
	return nil
}

func (r *Resolver) deleteActions(ctx context.Context, monitorID int64, ids []graphql.ID) error {
	var email, webhook, slackWebhook []int64
	for _, id := range ids {
		var intID int64
		err := relay.UnmarshalSpec(id, &intID)
		if err != nil {
			return err
		}

		switch relay.UnmarshalKind(id) {
		case monitorActionEmailKind:
			email = append(email, intID)
		case monitorActionWebhookKind:
			webhook = append(webhook, intID)
		case monitorActionSlackWebhookKind:
			slackWebhook = append(slackWebhook, intID)
		default:
			return errors.New("action IDs must be exactly one of email, webhook, or slack webhook")
		}
	}

	if err := r.db.CodeMonitors().DeleteEmailActions(ctx, email, monitorID); err != nil {
		return err
	}

	if err := r.db.CodeMonitors().DeleteWebhookActions(ctx, monitorID, webhook...); err != nil {
		return err
	}

	if err := r.db.CodeMonitors().DeleteSlackWebhookActions(ctx, monitorID, slackWebhook...); err != nil {
		return err
	}

	return nil
}

func (r *Resolver) createRecipients(ctx context.Context, emailID int64, recipients []graphql.ID) error {
	for _, recipient := range recipients {
		userID, orgID, err := graphqlbackend.UnmarshalNamespaceToIDs(recipient)
		if err != nil {
			return errors.Wrap(err, "UnmarshalNamespaceID")
		}

		_, err = r.db.CodeMonitors().CreateRecipient(ctx, emailID, userID, orgID)
		if err != nil {
			return err
		}
	}
	return nil
}

// ResetTriggerQueryTimestamps is a convenience function which resets the
// timestamps `next_run` and `last_result` with the purpose to trigger associated
// actions (emails, webhooks) immediately. This is useful during development and
// troubleshooting. Only site admins can call this functions.
func (r *Resolver) ResetTriggerQueryTimestamps(ctx context.Context, args *graphqlbackend.ResetTriggerQueryTimestampsArgs) (*graphqlbackend.EmptyResponse, error) {
	err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var queryIDInt64 int64
	err = relay.UnmarshalSpec(args.Id, &queryIDInt64)
	if err != nil {
		return nil, err
	}
	err = r.db.CodeMonitors().ResetQueryTriggerTimestamps(ctx, queryIDInt64)
	if err != nil {
		return nil, err
	}
	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) TriggerTestEmailAction(ctx context.Context, args *graphqlbackend.TriggerTestEmailActionArgs) (*graphqlbackend.EmptyResponse, error) {
	err := r.isAllowedToCreate(ctx, args.Namespace)
	if err != nil {
		return nil, err
	}

	for _, recipient := range args.Email.Recipients {
		if err := sendTestEmail(ctx, r.db, recipient, args.Description); err != nil {
			return nil, err
		}
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) TriggerTestWebhookAction(ctx context.Context, args *graphqlbackend.TriggerTestWebhookActionArgs) (*graphqlbackend.EmptyResponse, error) {
	err := r.isAllowedToCreate(ctx, args.Namespace)
	if err != nil {
		return nil, err
	}

	if err := background.SendTestWebhook(ctx, httpcli.ExternalDoer, args.Description, args.Webhook.URL); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) TriggerTestSlackWebhookAction(ctx context.Context, args *graphqlbackend.TriggerTestSlackWebhookActionArgs) (*graphqlbackend.EmptyResponse, error) {
	err := r.isAllowedToCreate(ctx, args.Namespace)
	if err != nil {
		return nil, err
	}

	if err := background.SendTestSlackWebhook(ctx, httpcli.ExternalDoer, args.Description, args.SlackWebhook.URL); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func sendTestEmail(ctx context.Context, db database.DB, recipient graphql.ID, description string) error {
	var (
		userID int32
		orgID  int32
	)
	err := graphqlbackend.UnmarshalNamespaceID(recipient, &userID, &orgID)
	if err != nil {
		return err
	}
	// TODO: Send test email to org members.
	if orgID != 0 {
		return nil
	}
	data := background.NewTestTemplateDataForNewSearchResults(description)
	return background.SendEmailForNewSearchResult(ctx, db, userID, data)
}

func (r *Resolver) actionIDsForMonitorIDInt64(ctx context.Context, monitorID int64) ([]graphql.ID, error) {
	opts := database.ListActionsOpts{MonitorID: &monitorID}
	emailActions, err := r.db.CodeMonitors().ListEmailActions(ctx, opts)
	if err != nil {
		return nil, err
	}
	webhookActions, err := r.db.CodeMonitors().ListWebhookActions(ctx, opts)
	if err != nil {
		return nil, err
	}
	slackWebhookActions, err := r.db.CodeMonitors().ListSlackWebhookActions(ctx, opts)
	if err != nil {
		return nil, err
	}
	ids := make([]graphql.ID, 0, len(emailActions)+len(webhookActions)+len(slackWebhookActions))
	for _, emailAction := range emailActions {
		ids = append(ids, (&monitorEmail{EmailAction: emailAction}).ID())
	}
	for _, webhookAction := range webhookActions {
		ids = append(ids, (&monitorWebhook{WebhookAction: webhookAction}).ID())
	}
	for _, slackWebhookAction := range slackWebhookActions {
		ids = append(ids, (&monitorSlackWebhook{SlackWebhookAction: slackWebhookAction}).ID())
	}
	return ids, nil
}

// splitActionIDs splits actions into three buckets: create, delete and update.
// Note: args is mutated. After splitActionIDs, args only contains actions to be updated.
func splitActionIDs(args *graphqlbackend.UpdateCodeMonitorArgs, actionIDs []graphql.ID) (toCreate []*graphqlbackend.CreateActionArgs, toDelete []graphql.ID, err error) {
	aMap := make(map[graphql.ID]struct{}, len(actionIDs))
	for _, id := range actionIDs {
		aMap[id] = struct{}{}
	}

	var toUpdateActions []*graphqlbackend.EditActionArgs
	for _, a := range args.Actions {
		switch {
		case a.Email != nil:
			if a.Email.Id == nil {
				toCreate = append(toCreate, &graphqlbackend.CreateActionArgs{Email: a.Email.Update})
				continue
			}
			if _, ok := aMap[*a.Email.Id]; !ok {
				return nil, nil, errors.Errorf("unknown ID=%s for action", *a.Email.Id)
			}
			toUpdateActions = append(toUpdateActions, a)
			delete(aMap, *a.Email.Id)
		case a.Webhook != nil:
			if a.Webhook.Id == nil {
				toCreate = append(toCreate, &graphqlbackend.CreateActionArgs{Webhook: a.Webhook.Update})
				continue
			}
			if _, ok := aMap[*a.Webhook.Id]; !ok {
				return nil, nil, errors.Errorf("unknown ID=%s for action", *a.Webhook.Id)
			}
			toUpdateActions = append(toUpdateActions, a)
			delete(aMap, *a.Webhook.Id)
		case a.SlackWebhook != nil:
			if a.SlackWebhook.Id == nil {
				toCreate = append(toCreate, &graphqlbackend.CreateActionArgs{SlackWebhook: a.SlackWebhook.Update})
				continue
			}
			if _, ok := aMap[*a.SlackWebhook.Id]; !ok {
				return nil, nil, errors.Errorf("unknown ID=%s for action", *a.SlackWebhook.Id)
			}
			toUpdateActions = append(toUpdateActions, a)
			delete(aMap, *a.SlackWebhook.Id)
		}
	}

	args.Actions = toUpdateActions
	for id := range aMap {
		toDelete = append(toDelete, id)
	}
	return toCreate, toDelete, nil
}

// updateCodeMonitor updates the code monitor in the database. We pass in "rawDB" because Snapshot requires that the
// database being used is not in a transaction, and updateCodeMonitor is run with a transacted resolver.
func (r *Resolver) updateCodeMonitor(ctx context.Context, rawDB database.DB, args *graphqlbackend.UpdateCodeMonitorArgs) (*monitor, error) {
	// Update monitor.
	monitorID, err := unmarshalMonitorID(args.Monitor.Id)
	if err != nil {
		return nil, err
	}

	userID, orgID, err := graphqlbackend.UnmarshalNamespaceToIDs(args.Monitor.Update.Namespace)
	if err != nil {
		return nil, err
	}

	mo, err := r.db.CodeMonitors().UpdateMonitor(ctx, monitorID, database.MonitorArgs{
		Description:     args.Monitor.Update.Description,
		Enabled:         args.Monitor.Update.Enabled,
		NamespaceUserID: userID,
		NamespaceOrgID:  orgID,
	})
	if err != nil {
		return nil, err
	}

	var triggerID int64
	if err := relay.UnmarshalSpec(args.Trigger.Id, &triggerID); err != nil {
		return nil, err
	}

	currentTrigger, err := r.db.CodeMonitors().GetQueryTriggerForMonitor(ctx, monitorID)
	if err != nil {
		return nil, err
	}

	// When the query is changed, take a new snapshot of the commits that currently
	// exist so we know where to start.
	if currentTrigger.QueryString != args.Trigger.Update.Query {
		// Snapshot the state of the searched repos when the monitor is created so that
		// we can distinguish new repos.
		// NOTE: we use rawDB here because Snapshot requires that the db conn is not a transaction.
		resolvedRevisions, err := codemonitors.Snapshot(ctx, r.logger, rawDB, args.Trigger.Update.Query)
		if err != nil {
			return nil, err
		}
		for repoID, commitIDs := range resolvedRevisions {
			err = r.db.CodeMonitors().UpsertLastSearched(ctx, monitorID, repoID, commitIDs)
			if err != nil {
				return nil, err
			}
		}
	}

	// Update trigger.
	err = r.db.CodeMonitors().UpdateQueryTrigger(ctx, triggerID, args.Trigger.Update.Query)
	if err != nil {
		return nil, err
	}

	// Update actions.
	if len(args.Actions) == 0 {
		return &monitor{
			Resolver: r,
			Monitor:  mo,
		}, nil
	}
	for _, action := range args.Actions {
		switch {
		case action.Email != nil:
			err = r.updateEmailAction(ctx, *action.Email)
		case action.Webhook != nil:
			err = r.updateWebhookAction(ctx, *action.Webhook)
		case action.SlackWebhook != nil:
			if err := validateSlackURL(action.SlackWebhook.Update.URL); err != nil {
				return nil, err
			}
			err = r.updateSlackWebhookAction(ctx, *action.SlackWebhook)
		default:
			err = errors.New("action must be one of email, webhook, or slack webhook")
		}
		if err != nil {
			return nil, err
		}
	}
	return &monitor{
		Resolver: r,
		Monitor:  mo,
	}, nil
}

func (r *Resolver) updateEmailAction(ctx context.Context, args graphqlbackend.EditActionEmailArgs) error {
	emailID, err := unmarshalEmailID(*args.Id)
	if err != nil {
		return err
	}
	err = r.db.CodeMonitors().DeleteRecipients(ctx, emailID)
	if err != nil {
		return err
	}

	e, err := r.db.CodeMonitors().UpdateEmailAction(ctx, emailID, &database.EmailActionArgs{
		Enabled:        args.Update.Enabled,
		IncludeResults: args.Update.IncludeResults,
		Priority:       args.Update.Priority,
		Header:         args.Update.Header,
	})
	if err != nil {
		return err
	}
	return r.createRecipients(ctx, e.ID, args.Update.Recipients)
}

func (r *Resolver) updateWebhookAction(ctx context.Context, args graphqlbackend.EditActionWebhookArgs) error {
	var id int64
	err := relay.UnmarshalSpec(*args.Id, &id)
	if err != nil {
		return err
	}

	_, err = r.db.CodeMonitors().UpdateWebhookAction(ctx, id, args.Update.Enabled, args.Update.IncludeResults, args.Update.URL)
	return err
}

func (r *Resolver) updateSlackWebhookAction(ctx context.Context, args graphqlbackend.EditActionSlackWebhookArgs) error {
	var id int64
	err := relay.UnmarshalSpec(*args.Id, &id)
	if err != nil {
		return err
	}

	_, err = r.db.CodeMonitors().UpdateSlackWebhookAction(ctx, id, args.Update.Enabled, args.Update.IncludeResults, args.Update.URL)
	return err
}

func (r *Resolver) withTransact(ctx context.Context, f func(*Resolver) error) error {
	return r.db.WithTransact(ctx, func(tx database.DB) error {
		return f(&Resolver{
			logger: r.logger,
			db:     tx,
		})
	})
}

// isAllowedToEdit checks whether an actor is allowed to edit a given monitor.
func (r *Resolver) isAllowedToEdit(ctx context.Context, id graphql.ID) error {
	if dotcom.SourcegraphDotComMode() {
		return errors.New("Code Monitors are disabled on sourcegraph.com")
	}
	monitorID, err := unmarshalMonitorID(id)
	if err != nil {
		return err
	}
	owner, err := r.ownerForID64(ctx, monitorID)
	if err != nil {
		return err
	}
	return r.isAllowedToCreate(ctx, owner)
}

// isAllowedToCreate compares the owner of a monitor (user or org) to the actor of
// the request. A user can create a monitor if either of the following statements
// is true:
// - she is the owner
// - she is a member of the organization which is the owner of the monitor
// - she is a site-admin
func (r *Resolver) isAllowedToCreate(ctx context.Context, owner graphql.ID) error {
	if dotcom.SourcegraphDotComMode() {
		return errors.New("Code Monitors are disabled on sourcegraph.com")
	}
	var ownerInt32 int32
	err := relay.UnmarshalSpec(owner, &ownerInt32)
	if err != nil {
		return err
	}
	switch kind := relay.UnmarshalKind(owner); kind {
	case "User":
		return auth.CheckSiteAdminOrSameUser(ctx, r.db, ownerInt32)
	case "Org":
		return errors.Errorf("creating a code monitor with an org namespace is no longer supported")
	default:
		return errors.Errorf("provided ID is not a namespace")
	}
}

func (r *Resolver) ownerForID64(ctx context.Context, monitorID int64) (graphql.ID, error) {
	monitor, err := r.db.CodeMonitors().GetMonitor(ctx, monitorID)
	if err != nil {
		return "", err
	}

	return graphqlbackend.MarshalUserID(monitor.UserID), nil
}

// MonitorConnection
type monitorConnection struct {
	*Resolver
	monitors    []graphqlbackend.MonitorResolver
	totalCount  int32
	hasNextPage bool
}

func (m *monitorConnection) Nodes() []graphqlbackend.MonitorResolver {
	return m.monitors
}

func (m *monitorConnection) TotalCount() int32 {
	return m.totalCount
}

func (m *monitorConnection) PageInfo() *graphqlutil.PageInfo {
	if len(m.monitors) == 0 || !m.hasNextPage {
		return graphqlutil.HasNextPage(false)
	}
	return graphqlutil.NextPageCursor(string(m.monitors[len(m.monitors)-1].ID()))
}

const (
	MonitorKind                        = "CodeMonitor"
	monitorTriggerQueryKind            = "CodeMonitorTriggerQuery"
	monitorTriggerEventKind            = "CodeMonitorTriggerEvent"
	monitorActionEmailKind             = "CodeMonitorActionEmail"
	monitorActionWebhookKind           = "CodeMonitorActionWebhook"
	monitorActionSlackWebhookKind      = "CodeMonitorActionSlackWebhook"
	monitorActionEmailEventKind        = "CodeMonitorActionEmailEvent"
	monitorActionWebhookEventKind      = "CodeMonitorActionWebhookEvent"
	monitorActionSlackWebhookEventKind = "CodeMonitorActionSlackWebhookEvent"
	monitorActionEmailRecipientKind    = "CodeMonitorActionEmailRecipient"
)

func unmarshalMonitorID(id graphql.ID) (int64, error) {
	if kind := relay.UnmarshalKind(id); kind != MonitorKind {
		return 0, errors.Errorf("expected graphql ID kind %s, got %s", MonitorKind, kind)
	}
	var i int64
	err := relay.UnmarshalSpec(id, &i)
	return i, err
}

func unmarshalEmailID(id graphql.ID) (int64, error) {
	if kind := relay.UnmarshalKind(id); kind != monitorActionEmailKind {
		return 0, errors.Errorf("expected graphql ID kind %s, got %s", monitorActionEmailKind, kind)
	}
	var i int64
	err := relay.UnmarshalSpec(id, &i)
	return i, err
}

func unmarshalAfter(after *string) (*int, error) {
	if after == nil {
		return nil, nil
	}

	var a int
	err := relay.UnmarshalSpec(graphql.ID(*after), &a)
	return &a, err
}

// Monitor
type monitor struct {
	*Resolver
	*database.Monitor
}

func (m *monitor) ID() graphql.ID {
	return relay.MarshalID(MonitorKind, m.Monitor.ID)
}

func (m *monitor) CreatedBy(ctx context.Context) (*graphqlbackend.UserResolver, error) {
	return graphqlbackend.UserByIDInt32(ctx, m.db, m.Monitor.CreatedBy)
}

func (m *monitor) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: m.Monitor.CreatedAt}
}

func (m *monitor) Description() string {
	return m.Monitor.Description
}

func (m *monitor) Enabled() bool {
	return m.Monitor.Enabled
}

func (m *monitor) Owner(ctx context.Context) (graphqlbackend.NamespaceResolver, error) {
	n, err := graphqlbackend.UserByIDInt32(ctx, m.db, m.UserID)
	return graphqlbackend.NamespaceResolver{Namespace: n}, err
}

func (m *monitor) Trigger(ctx context.Context) (graphqlbackend.MonitorTrigger, error) {
	t, err := m.db.CodeMonitors().GetQueryTriggerForMonitor(ctx, m.Monitor.ID)
	if err != nil {
		return nil, err
	}
	return &monitorTrigger{&monitorQuery{m.Resolver, t}}, nil
}

func (m *monitor) Actions(ctx context.Context, args *graphqlbackend.ListActionArgs) (graphqlbackend.MonitorActionConnectionResolver, error) {
	return m.actionConnectionResolverWithTriggerID(ctx, nil, m.Monitor.ID, args)
}

func (r *Resolver) actionConnectionResolverWithTriggerID(ctx context.Context, triggerEventID *int32, monitorID int64, args *graphqlbackend.ListActionArgs) (graphqlbackend.MonitorActionConnectionResolver, error) {
	opts := database.ListActionsOpts{MonitorID: &monitorID}

	es, err := r.db.CodeMonitors().ListEmailActions(ctx, opts)
	if err != nil {
		return nil, err
	}

	ws, err := r.db.CodeMonitors().ListWebhookActions(ctx, opts)
	if err != nil {
		return nil, err
	}

	sws, err := r.db.CodeMonitors().ListSlackWebhookActions(ctx, opts)
	if err != nil {
		return nil, err
	}

	actions := make([]graphqlbackend.MonitorAction, 0, len(es)+len(ws)+len(sws))
	for _, e := range es {
		actions = append(actions, &action{
			email: &monitorEmail{
				Resolver:       r,
				EmailAction:    e,
				triggerEventID: triggerEventID,
			},
		})
	}
	for _, w := range ws {
		actions = append(actions, &action{
			webhook: &monitorWebhook{
				Resolver:       r,
				WebhookAction:  w,
				triggerEventID: triggerEventID,
			},
		})
	}
	for _, sw := range sws {
		actions = append(actions, &action{
			slackWebhook: &monitorSlackWebhook{
				Resolver:           r,
				SlackWebhookAction: sw,
				triggerEventID:     triggerEventID,
			},
		})
	}

	totalCount := len(actions)
	if args.After != nil {
		for i, action := range actions {
			if action.ID() == graphql.ID(*args.After) {
				actions = actions[i+1:]
				break
			}
		}
	}

	if args.First > 0 && len(actions) > int(args.First) {
		actions = actions[:args.First]
	}

	return &monitorActionConnection{actions: actions, totalCount: int32(totalCount)}, nil
}

// MonitorTrigger <<UNION>>
type monitorTrigger struct {
	query graphqlbackend.MonitorQueryResolver
}

func (t *monitorTrigger) ToMonitorQuery() (graphqlbackend.MonitorQueryResolver, bool) {
	return t.query, t.query != nil
}

// Query
type monitorQuery struct {
	*Resolver
	*database.QueryTrigger
}

func (q *monitorQuery) ID() graphql.ID {
	return relay.MarshalID(monitorTriggerQueryKind, q.QueryTrigger.ID)
}

func (q *monitorQuery) Query() string {
	return q.QueryString
}

func (q *monitorQuery) Events(ctx context.Context, args *graphqlbackend.ListEventsArgs) (graphqlbackend.MonitorTriggerEventConnectionResolver, error) {
	after, err := unmarshalAfter(args.After)
	if err != nil {
		return nil, err
	}
	es, err := q.db.CodeMonitors().ListQueryTriggerJobs(ctx, database.ListTriggerJobsOpts{
		QueryID: &q.QueryTrigger.ID,
		First:   pointers.Ptr(int(args.First)),
		After:   intPtrToInt64Ptr(after),
	})
	if err != nil {
		return nil, err
	}
	totalCount, err := q.db.CodeMonitors().CountQueryTriggerJobs(ctx, q.QueryTrigger.ID)
	if err != nil {
		return nil, err
	}
	events := make([]graphqlbackend.MonitorTriggerEventResolver, 0, len(es))
	for _, e := range es {
		events = append(events, graphqlbackend.MonitorTriggerEventResolver(&monitorTriggerEvent{
			Resolver:   q.Resolver,
			monitorID:  q.Monitor,
			TriggerJob: e,
		}))
	}
	return &monitorTriggerEventConnection{Resolver: q.Resolver, events: events, totalCount: totalCount}, nil
}

// MonitorTriggerEventConnection
type monitorTriggerEventConnection struct {
	*Resolver
	events     []graphqlbackend.MonitorTriggerEventResolver
	totalCount int32
}

func (a *monitorTriggerEventConnection) Nodes() []graphqlbackend.MonitorTriggerEventResolver {
	return a.events
}

func (a *monitorTriggerEventConnection) TotalCount() int32 {
	return a.totalCount
}

func (a *monitorTriggerEventConnection) PageInfo() *graphqlutil.PageInfo {
	if len(a.events) == 0 {
		return graphqlutil.HasNextPage(false)
	}
	return graphqlutil.NextPageCursor(string(a.events[len(a.events)-1].ID()))
}

// MonitorTriggerEvent
type monitorTriggerEvent struct {
	*Resolver
	*database.TriggerJob
	monitorID int64
}

func (m *monitorTriggerEvent) ID() graphql.ID {
	return relay.MarshalID(monitorTriggerEventKind, m.TriggerJob.ID)
}

// stateToStatus maps the state of the dbworker job to the public GraphQL status of
// events.
var stateToStatus = map[string]string{
	"completed":  "SUCCESS",
	"queued":     "PENDING",
	"processing": "PENDING",
	"errored":    "ERROR",
	"failed":     "ERROR",
}

func (m *monitorTriggerEvent) Status() (string, error) {
	if v, ok := stateToStatus[m.State]; ok {
		return v, nil
	}
	return "", errors.Errorf("unknown status: %s", m.State)
}

func (m *monitorTriggerEvent) Query() *string {
	return m.TriggerJob.QueryString
}

func (m *monitorTriggerEvent) ResultCount() int32 {
	count := 0
	for _, cm := range m.TriggerJob.SearchResults {
		count += cm.ResultCount()
	}
	return int32(count)
}

func (m *monitorTriggerEvent) Message() *string {
	// Print failure message first
	var msg string
	if m.FailureMessage != nil {
		msg = *m.FailureMessage + "\n"
	}
	for _, log := range m.Logs {
		msg += log.Message + "\n"
	}
	msg = strings.TrimSpace(msg)
	return &msg
}

func (m *monitorTriggerEvent) Timestamp() (gqlutil.DateTime, error) {
	if m.FinishedAt == nil {
		return gqlutil.DateTime{Time: m.db.CodeMonitors().Now()}, nil
	}
	return gqlutil.DateTime{Time: *m.FinishedAt}, nil
}

func (m *monitorTriggerEvent) Actions(ctx context.Context, args *graphqlbackend.ListActionArgs) (graphqlbackend.MonitorActionConnectionResolver, error) {
	return m.actionConnectionResolverWithTriggerID(ctx, &m.TriggerJob.ID, m.monitorID, args)
}

// ActionConnection
type monitorActionConnection struct {
	actions    []graphqlbackend.MonitorAction
	totalCount int32
}

func (a *monitorActionConnection) Nodes() []graphqlbackend.MonitorAction {
	return a.actions
}

func (a *monitorActionConnection) TotalCount() int32 {
	return a.totalCount
}

func (a *monitorActionConnection) PageInfo() *graphqlutil.PageInfo {
	if len(a.actions) == 0 {
		return graphqlutil.HasNextPage(false)
	}
	last := a.actions[len(a.actions)-1]
	if email, ok := last.ToMonitorEmail(); ok {
		return graphqlutil.NextPageCursor(string(email.ID()))
	}
	panic("found non-email monitor action")
}

// Action <<UNION>>
type action struct {
	email        graphqlbackend.MonitorEmailResolver
	webhook      graphqlbackend.MonitorWebhookResolver
	slackWebhook graphqlbackend.MonitorSlackWebhookResolver
}

func (a *action) ID() graphql.ID {
	switch {
	case a.email != nil:
		return a.email.ID()
	case a.webhook != nil:
		return a.webhook.ID()
	case a.slackWebhook != nil:
		return a.slackWebhook.ID()
	default:
		panic("action must have a type")
	}
}

func (a *action) ToMonitorEmail() (graphqlbackend.MonitorEmailResolver, bool) {
	return a.email, a.email != nil
}

func (a *action) ToMonitorWebhook() (graphqlbackend.MonitorWebhookResolver, bool) {
	return a.webhook, a.webhook != nil
}

func (a *action) ToMonitorSlackWebhook() (graphqlbackend.MonitorSlackWebhookResolver, bool) {
	return a.slackWebhook, a.slackWebhook != nil
}

// Email
type monitorEmail struct {
	*Resolver
	*database.EmailAction

	// If triggerEventID == nil, all events of this action will be returned.
	// Otherwise, only those events of this action which are related to the specified
	// trigger event will be returned.
	triggerEventID *int32
}

func (m *monitorEmail) Recipients(ctx context.Context, args *graphqlbackend.ListRecipientsArgs) (graphqlbackend.MonitorActionEmailRecipientsConnectionResolver, error) {
	after, err := unmarshalAfter(args.After)
	if err != nil {
		return nil, err
	}
	ms, err := m.db.CodeMonitors().ListRecipients(ctx, database.ListRecipientsOpts{
		EmailID: &m.EmailAction.ID,
		First:   pointers.Ptr(int(args.First)),
		After:   intPtrToInt64Ptr(after),
	})
	if err != nil {
		return nil, err
	}
	ns := make([]graphqlbackend.NamespaceResolver, 0, len(ms))
	for _, r := range ms {
		n := graphqlbackend.NamespaceResolver{}
		if r.NamespaceOrgID == nil {
			n.Namespace, err = graphqlbackend.UserByIDInt32(ctx, m.db, *r.NamespaceUserID)
		} else {
			n.Namespace, err = graphqlbackend.OrgByIDInt32(ctx, m.db, *r.NamespaceOrgID)
		}
		if err != nil {
			return nil, err
		}
		ns = append(ns, n)
	}

	// Since recipients can either be a user or an org it would be very tedious to
	// use the user-id or org-id of the last entry as a cursor for the next page. It
	// is easier to just use the id of the recipients table.
	var nextPageCursor string
	if len(ms) > 0 {
		nextPageCursor = string(relay.MarshalID(monitorActionEmailRecipientKind, ms[len(ms)-1].ID))
	}

	total, err := m.db.CodeMonitors().CountRecipients(ctx, m.EmailAction.ID)
	if err != nil {
		return nil, err
	}
	return &monitorActionEmailRecipientsConnection{ns, nextPageCursor, total}, nil
}

func (m *monitorEmail) Enabled() bool {
	return m.EmailAction.Enabled
}

func (m *monitorEmail) IncludeResults() bool {
	return m.EmailAction.IncludeResults
}

func (m *monitorEmail) Priority() string {
	return m.EmailAction.Priority
}

func (m *monitorEmail) Header() string {
	return m.EmailAction.Header
}

func (m *monitorEmail) ID() graphql.ID {
	return relay.MarshalID(monitorActionEmailKind, m.EmailAction.ID)
}

func (m *monitorEmail) Events(ctx context.Context, args *graphqlbackend.ListEventsArgs) (graphqlbackend.MonitorActionEventConnectionResolver, error) {
	after, err := unmarshalAfter(args.After)
	if err != nil {
		return nil, err
	}

	ajs, err := m.db.CodeMonitors().ListActionJobs(ctx, database.ListActionJobsOpts{
		EmailID:        pointers.Ptr(int(m.EmailAction.ID)),
		TriggerEventID: m.triggerEventID,
		First:          pointers.Ptr(int(args.First)),
		After:          after,
	})
	if err != nil {
		return nil, err
	}

	totalCount, err := m.db.CodeMonitors().CountActionJobs(ctx, database.ListActionJobsOpts{
		EmailID:        pointers.Ptr(int(m.EmailAction.ID)),
		TriggerEventID: m.triggerEventID,
	})
	if err != nil {
		return nil, err
	}
	events := make([]graphqlbackend.MonitorActionEventResolver, len(ajs))
	for i, aj := range ajs {
		events[i] = &monitorActionEvent{Resolver: m.Resolver, ActionJob: aj}
	}
	return &monitorActionEventConnection{events: events, totalCount: int32(totalCount)}, nil
}

type monitorWebhook struct {
	*Resolver
	*database.WebhookAction

	// If triggerEventID == nil, all events of this action will be returned.
	// Otherwise, only those events of this action which are related to the specified
	// trigger event will be returned.
	triggerEventID *int32
}

func (m *monitorWebhook) ID() graphql.ID {
	return relay.MarshalID(monitorActionWebhookKind, m.WebhookAction.ID)
}

func (m *monitorWebhook) Enabled() bool {
	return m.WebhookAction.Enabled
}

func (m *monitorWebhook) IncludeResults() bool {
	return m.WebhookAction.IncludeResults
}

func (m *monitorWebhook) URL() string {
	return m.WebhookAction.URL
}

func (m *monitorWebhook) Events(ctx context.Context, args *graphqlbackend.ListEventsArgs) (graphqlbackend.MonitorActionEventConnectionResolver, error) {
	after, err := unmarshalAfter(args.After)
	if err != nil {
		return nil, err
	}

	ajs, err := m.db.CodeMonitors().ListActionJobs(ctx, database.ListActionJobsOpts{
		WebhookID:      pointers.Ptr(int(m.WebhookAction.ID)),
		TriggerEventID: m.triggerEventID,
		First:          pointers.Ptr(int(args.First)),
		After:          after,
	})
	if err != nil {
		return nil, err
	}

	totalCount, err := m.db.CodeMonitors().CountActionJobs(ctx, database.ListActionJobsOpts{
		WebhookID:      pointers.Ptr(int(m.WebhookAction.ID)),
		TriggerEventID: m.triggerEventID,
	})
	if err != nil {
		return nil, err
	}
	events := make([]graphqlbackend.MonitorActionEventResolver, len(ajs))
	for i, aj := range ajs {
		events[i] = &monitorActionEvent{Resolver: m.Resolver, ActionJob: aj}
	}
	return &monitorActionEventConnection{events: events, totalCount: int32(totalCount)}, nil
}

type monitorSlackWebhook struct {
	*Resolver
	*database.SlackWebhookAction

	// If triggerEventID == nil, all events of this action will be returned.
	// Otherwise, only those events of this action which are related to the specified
	// trigger event will be returned.
	triggerEventID *int32
}

func (m *monitorSlackWebhook) ID() graphql.ID {
	return relay.MarshalID(monitorActionSlackWebhookKind, m.SlackWebhookAction.ID)
}

func (m *monitorSlackWebhook) Enabled() bool {
	return m.SlackWebhookAction.Enabled
}

func (m *monitorSlackWebhook) IncludeResults() bool {
	return m.SlackWebhookAction.IncludeResults
}

func (m *monitorSlackWebhook) URL() string {
	return m.SlackWebhookAction.URL
}

func (m *monitorSlackWebhook) Events(ctx context.Context, args *graphqlbackend.ListEventsArgs) (graphqlbackend.MonitorActionEventConnectionResolver, error) {
	after, err := unmarshalAfter(args.After)
	if err != nil {
		return nil, err
	}

	ajs, err := m.db.CodeMonitors().ListActionJobs(ctx, database.ListActionJobsOpts{
		SlackWebhookID: pointers.Ptr(int(m.SlackWebhookAction.ID)),
		TriggerEventID: m.triggerEventID,
		First:          pointers.Ptr(int(args.First)),
		After:          after,
	})
	if err != nil {
		return nil, err
	}

	totalCount, err := m.db.CodeMonitors().CountActionJobs(ctx, database.ListActionJobsOpts{
		SlackWebhookID: pointers.Ptr(int(m.SlackWebhookAction.ID)),
		TriggerEventID: m.triggerEventID,
	})
	if err != nil {
		return nil, err
	}
	events := make([]graphqlbackend.MonitorActionEventResolver, len(ajs))
	for i, aj := range ajs {
		events[i] = &monitorActionEvent{Resolver: m.Resolver, ActionJob: aj}
	}
	return &monitorActionEventConnection{events: events, totalCount: int32(totalCount)}, nil
}

func intPtrToInt64Ptr(i *int) *int64 {
	if i == nil {
		return nil
	}
	j := int64(*i)
	return &j
}

// MonitorActionEmailRecipientConnection
type monitorActionEmailRecipientsConnection struct {
	recipients     []graphqlbackend.NamespaceResolver
	nextPageCursor string
	totalCount     int32
}

func (a *monitorActionEmailRecipientsConnection) Nodes() []graphqlbackend.NamespaceResolver {
	return a.recipients
}

func (a *monitorActionEmailRecipientsConnection) TotalCount() int32 {
	return a.totalCount
}

func (a *monitorActionEmailRecipientsConnection) PageInfo() *graphqlutil.PageInfo {
	if len(a.recipients) == 0 {
		return graphqlutil.HasNextPage(false)
	}
	return graphqlutil.NextPageCursor(a.nextPageCursor)
}

// MonitorActionEventConnection
type monitorActionEventConnection struct {
	events     []graphqlbackend.MonitorActionEventResolver
	totalCount int32
}

func (a *monitorActionEventConnection) Nodes() []graphqlbackend.MonitorActionEventResolver {
	return a.events
}

func (a *monitorActionEventConnection) TotalCount() int32 {
	return a.totalCount
}

func (a *monitorActionEventConnection) PageInfo() *graphqlutil.PageInfo {
	if len(a.events) == 0 {
		return graphqlutil.HasNextPage(false)
	}
	return graphqlutil.NextPageCursor(string(a.events[len(a.events)-1].ID()))
}

// MonitorEvent
type monitorActionEvent struct {
	*Resolver
	*database.ActionJob
}

func (m *monitorActionEvent) ID() graphql.ID {
	return relay.MarshalID(monitorActionEmailEventKind, m.ActionJob.ID)
}

func (m *monitorActionEvent) Status() (string, error) {
	status, ok := stateToStatus[m.State]
	if !ok {
		return "", errors.Errorf("unknown state: %s", m.State)
	}
	return status, nil
}

func (m *monitorActionEvent) Message() *string {
	return m.FailureMessage
}

func (m *monitorActionEvent) Timestamp() gqlutil.DateTime {
	if m.FinishedAt == nil {
		return gqlutil.DateTime{Time: m.db.CodeMonitors().Now()}
	}
	return gqlutil.DateTime{Time: *m.FinishedAt}
}

func validateSlackURL(urlString string) error {
	u, err := url.Parse(urlString)
	if err != nil {
		return err
	}

	// Restrict slack webhooks to only canonical host and HTTPS
	if u.Host != "hooks.slack.com" || u.Scheme != "https" {
		return errors.New("slack webhook URL must begin with 'https://hooks.slack.com/")
	}
	return nil
}
