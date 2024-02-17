package webhooks

import (
	"context"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/webhooks/outbound"
)

var service struct {
	once sync.Once
	key  encryption.Key
}

func getService(db basestore.ShareableStore) outbound.OutboundWebhookService {
	service.once.Do(func() {
		service.key = keyring.Default().OutboundWebhookKey
	})
	return outbound.NewOutboundWebhookService(db, service.key)
}

// Enqueue creates an outbound webhook job that will dispatch a webhook of the
// given type with a payload marshalled by the given marshaller.
//
// Note the typed helpers below — if you're sending a webhook for a type that is
// already handled, you may as well use them and enjoy a slightly simpler
// function call.
func Enqueue(
	ctx context.Context, logger log.Logger, db basestore.ShareableStore,
	eventType string,
	marshaller func(context.Context, httpcli.Doer, graphql.ID) ([]byte, error),
	id graphql.ID,
	client httpcli.Doer,
) {
	svc := getService(db)

	// Webhooks are generally intended to be fire and forget from the point of
	// view of calling code, so we'll simply log on error and carry on.
	logger = logger.With(
		log.String("id", string(id)),
		log.String("event_type", eventType),
	)

	payload, err := marshaller(ctx, client, id)
	if err != nil {
		logger.Error("error marshalling webhook payload", log.Error(err))
		return
	}

	if err := svc.Enqueue(ctx, eventType, nil, payload); err != nil {
		logger.Error("error enqueuing webhook job", log.Error(err))
		return
	}
}

func EnqueueBatchChange(
	ctx context.Context,
	logger log.Logger,
	cli httpcli.Doer,
	db basestore.ShareableStore,
	eventType string,
	id graphql.ID,
) {
	Enqueue(ctx, logger, db, eventType, marshalBatchChange, id, cli)
}

func EnqueueChangeset(
	ctx context.Context,
	logger log.Logger,
	cli httpcli.Doer,
	db basestore.ShareableStore,
	eventType string,
	id graphql.ID,
) {
	Enqueue(ctx, logger, db, eventType, marshalChangeset, id, cli)
}
