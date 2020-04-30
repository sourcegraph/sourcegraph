package graphqlbackend

import (
	"context"
	"fmt"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/inconshreveable/log15"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/db"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/types"
	"github.com/sourcegraph/sourcegraph/internal/campaigns"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

type externalServiceResolver struct {
	externalService *types.ExternalService
	warning         string

	webhookURLOnce sync.Once
	webhookURL     string
}

const externalServiceIDKind = "ExternalService"

func externalServiceByID(ctx context.Context, id graphql.ID) (*externalServiceResolver, error) {
	// 🚨 SECURITY: Only site admins are allowed to read external services.
	if err := backend.CheckCurrentUserIsSiteAdmin(ctx); err != nil {
		return nil, err
	}

	externalServiceID, err := unmarshalExternalServiceID(id)
	if err != nil {
		return nil, err
	}

	externalService, err := db.ExternalServices.GetByID(ctx, externalServiceID)
	if err != nil {
		return nil, err
	}

	return &externalServiceResolver{externalService: externalService}, nil
}

func marshalExternalServiceID(id int64) graphql.ID {
	return relay.MarshalID(externalServiceIDKind, id)
}

func unmarshalExternalServiceID(id graphql.ID) (externalServiceID int64, err error) {
	if kind := relay.UnmarshalKind(id); kind != externalServiceIDKind {
		err = fmt.Errorf("expected graphql ID to have kind %q; got %q", externalServiceIDKind, kind)
		return
	}
	err = relay.UnmarshalSpec(id, &externalServiceID)
	return
}

func (r *externalServiceResolver) ID() graphql.ID {
	return marshalExternalServiceID(r.externalService.ID)
}

func (r *externalServiceResolver) Kind() string {
	return r.externalService.Kind
}

func (r *externalServiceResolver) DisplayName() string {
	return r.externalService.DisplayName
}

func (r *externalServiceResolver) Config() JSONCString {
	return JSONCString(r.externalService.Config)
}

func (r *externalServiceResolver) CreatedAt() DateTime {
	return DateTime{Time: r.externalService.CreatedAt}
}

func (r *externalServiceResolver) UpdatedAt() DateTime {
	return DateTime{Time: r.externalService.UpdatedAt}
}

func (r *externalServiceResolver) CampaignWebhookURL() *string {
	r.webhookURLOnce.Do(func() {
		parsed, err := extsvc.ParseConfig(r.externalService.Kind, r.externalService.Config)
		if err != nil {
			log15.Error("parsing external service config", "err", err)
		}
		u := campaigns.WebhookURL(r.externalService.ID)
		switch c := parsed.(type) {
		case *schema.BitbucketServerConnection:
			if c.Webhooks != nil && c.Webhooks.Secret != "" {
				r.webhookURL = u
			}
			if c.Plugin != nil && c.Plugin.Webhooks != nil && c.Plugin.Webhooks.Secret != "" {
				r.webhookURL = u
			}
		case *schema.GitHubConnection:
			if len(c.Webhooks) > 0 {
				r.webhookURL = u
			}
		}
	})
	if r.webhookURL == "" {
		return nil
	}
	return &r.webhookURL
}

func (r *externalServiceResolver) Warning() *string {
	if r.warning == "" {
		return nil
	}
	return &r.warning
}
