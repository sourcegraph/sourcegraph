package graphqlbackend

import (
	"context"
	"fmt"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/ssc"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type externalAccountResolver struct {
	db      database.DB
	account extsvc.Account
}

func externalAccountByID(ctx context.Context, db database.DB, id graphql.ID) (*externalAccountResolver, error) {
	externalAccountID, err := unmarshalExternalAccountID(id)
	if err != nil {
		return nil, err
	}
	account, err := db.UserExternalAccounts().Get(ctx, externalAccountID)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Only the user and site admins should be able to see a user's external accounts.
	if err := auth.CheckSiteAdminOrSameUser(ctx, db, account.UserID); err != nil {
		return nil, err
	}

	return &externalAccountResolver{db: db, account: *account}, nil
}

func marshalExternalAccountID(repo int32) graphql.ID { return relay.MarshalID("ExternalAccount", repo) }

func unmarshalExternalAccountID(id graphql.ID) (externalAccountID int32, err error) {
	err = relay.UnmarshalSpec(id, &externalAccountID)
	return
}

func (r *externalAccountResolver) ID() graphql.ID { return marshalExternalAccountID(r.account.ID) }
func (r *externalAccountResolver) User(ctx context.Context) (*UserResolver, error) {
	return UserByIDInt32(ctx, r.db, r.account.UserID)
}
func (r *externalAccountResolver) ServiceType() string { return r.account.ServiceType }
func (r *externalAccountResolver) ServiceID() string   { return r.account.ServiceID }
func (r *externalAccountResolver) ClientID() string    { return r.account.ClientID }
func (r *externalAccountResolver) AccountID() string   { return r.account.AccountID }

// TEMPORARY: This resolver is temporary to help us debug the #inc-284-plg-users-paying-for-and-being-billed-for-pro-without-being-upgrade.
func (r *externalAccountResolver) CodySubscription(ctx context.Context) (*CodySubscriptionResolver, error) {
	if !dotcom.SourcegraphDotComMode() {
		return nil, errors.New("this feature is only available on sourcegraph.com")
	}

	if r.account.ServiceType != "openidconnect" || r.account.ServiceID != fmt.Sprintf("https://%s", ssc.GetSAMSHostName()) {
		return nil, nil
	}

	userResolver, err := r.User(ctx)
	if err != nil {
		return nil, err
	}

	subscription, err := cody.SubscriptionForSAMSAccountID(ctx, r.db, *userResolver.user, r.account.AccountID)
	if err != nil {
		return nil, err
	}

	return &CodySubscriptionResolver{subscription: subscription}, nil
}

func (r *externalAccountResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.account.CreatedAt}
}
func (r *externalAccountResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.account.UpdatedAt}
}

func (r *externalAccountResolver) RefreshURL() *string {
	// TODO(sqs): Not supported.
	return nil
}

func (r *externalAccountResolver) AccountData(ctx context.Context) (*JSONValue, error) {
	// 🚨 SECURITY: It is only safe to assume account data of GitHub and GitLab do
	// not contain sensitive information that is not known to the user (which is
	// accessible via APIs by users themselves). We cannot take the same assumption
	// for other types of external accounts.
	//
	// Therefore, the site admins and the user can view account data of GitHub and
	// GitLab, but only site admins can view account data for all other types.
	var err error
	if r.account.ServiceType == extsvc.TypeGitHub || r.account.ServiceType == extsvc.TypeGitLab {
		err = auth.CheckSiteAdminOrSameUser(ctx, r.db, actor.FromContext(ctx).UID)
	} else {
		err = auth.CheckUserIsSiteAdmin(ctx, r.db, actor.FromContext(ctx).UID)
	}
	if err != nil {
		return nil, err
	}

	if r.account.Data != nil {
		raw, err := r.account.Data.Decrypt(ctx)
		if err != nil {
			return nil, err
		}

		return &JSONValue{raw}, nil
	}
	return nil, nil
}

func (r *externalAccountResolver) PublicAccountData(ctx context.Context) (*externalAccountDataResolver, error) {
	// 🚨 SECURITY: We only return this data to site admin or user who is linked to the external account
	// This method differs from the one above - here we only return specific attributes
	// from the account that are public info, e.g. username, email, etc.
	err := auth.CheckSiteAdminOrSameUser(ctx, r.db, actor.FromContext(ctx).UID)
	if err != nil {
		return nil, err
	}

	if r.account.Data != nil {
		res, err := NewExternalAccountDataResolver(ctx, r.account)
		if err != nil {
			return nil, nil
		}
		return res, nil
	}

	return nil, nil
}
