package resolvers

import (
	"context"
	"strings"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/httpapi/completions"
	"github.com/sourcegraph/sourcegraph/internal/completions/client"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/telemetry/telemetryrecorder"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var _ graphqlbackend.CompletionsResolver = &completionsResolver{}

// completionsResolver provides chat completions
type completionsResolver struct {
	rl     completions.RateLimiter
	db     database.DB
	logger log.Logger
}

func NewCompletionsResolver(db database.DB, logger log.Logger) graphqlbackend.CompletionsResolver {
	rl := completions.NewRateLimiter(db, redispool.Store, types.CompletionsFeatureChat)
	return &completionsResolver{rl: rl, db: db, logger: logger}
}

func (c *completionsResolver) Completions(ctx context.Context, args graphqlbackend.CompletionsArgs) (_ string, err error) {
	if isEnabled, reason := cody.IsCodyEnabled(ctx, c.db); !isEnabled {
		return "", errors.Newf("cody is not enabled: %s", reason)
	}

	if err := cody.CheckVerifiedEmailRequirement(ctx, c.db, c.logger); err != nil {
		return "", err
	}

	completionsConfig := conf.GetCompletionsConfig(conf.Get().SiteConfig())
	if completionsConfig == nil {
		return "", errors.New("completions are not configured")
	}

	var chatModel string
	if args.Fast {
		chatModel = completionsConfig.FastChatModel
	} else {
		chatModel = completionsConfig.ChatModel
	}

	ctx, done := completions.Trace(ctx, "resolver", chatModel, int(args.Input.MaxTokensToSample)).
		WithErrorP(&err).
		Build()
	defer done()

	client, err := client.Get(
		c.logger,
		telemetryrecorder.New(c.db),
		completionsConfig.Endpoint,
		completionsConfig.Provider,
		completionsConfig.AccessToken,
		completionsConfig.TokenRetrievalEndpoint,
		completionsConfig.ClientId,
		completionsConfig.ClientSecret,
	)
	if err != nil {
		return "", errors.Wrap(err, "GetCompletionStreamClient")
	}

	// Check rate limit.
	if err := c.rl.TryAcquire(ctx); err != nil {
		return "", err
	}

	// GraphQL API is considered a legacy API
	version := types.CompletionsVersionLegacy

	params := convertParams(args)
	// No way to configure the model through the request, we hard code to chat.
	params.Model = chatModel
	resp, err := client.Complete(ctx, types.CompletionsFeatureChat, version, params, c.logger)
	if err != nil {
		return "", errors.Wrap(err, "client.Complete")
	}
	return resp.Completion, nil
}

func convertParams(args graphqlbackend.CompletionsArgs) types.CompletionRequestParameters {
	return types.CompletionRequestParameters{
		Messages:          convertMessages(args.Input.Messages),
		Temperature:       float32(args.Input.Temperature),
		MaxTokensToSample: int(args.Input.MaxTokensToSample),
		TopK:              int(args.Input.TopK),
		TopP:              float32(args.Input.TopP),
	}
}

func convertMessages(messages []graphqlbackend.Message) (result []types.Message) {
	for _, message := range messages {
		result = append(result, types.Message{
			Speaker: strings.ToLower(message.Speaker),
			Text:    message.Text,
		})
	}
	return result
}
