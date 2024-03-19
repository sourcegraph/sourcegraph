package httpapi

import (
	"context"

	"net/http"

	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/completions/client/fireworks"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/telemetry/telemetryrecorder"
)

// chatAttributionTest always returns true, as chat attribution
// is performed on the client side (as opposed to code completions)
// which works on the server side.
func chatAttributionTest(context.Context, string) (bool, error) {
	return true, nil
}

// NewChatCompletionsStreamHandler is an http handler which streams back completions results.
func NewChatCompletionsStreamHandler(logger log.Logger, db database.DB) http.Handler {
	logger = logger.Scoped("chat")
	rl := NewRateLimiter(db, redispool.Store, types.CompletionsFeatureChat)

	return newCompletionsHandler(
		logger,
		db,
		db.Users(),
		db.AccessTokens(),
		telemetryrecorder.New(db),
		chatAttributionTest,
		types.CompletionsFeatureChat,
		rl,
		"chat",
		func(ctx context.Context, requestParams types.CodyCompletionRequestParameters, c *conftypes.CompletionsConfig) (string, error) {
			// Allow a number of additional models on Dotcom
			if dotcom.SourcegraphDotComMode() {
				actor := sgactor.FromContext(ctx)
				user, err := actor.User(ctx, db.Users())
				if err != nil {
					return "", err
				}

				subscription, err := cody.SubscriptionForUser(ctx, db, *user)
				if err != nil {
					return "", err
				}

				if isAllowedCustomChatModel(requestParams.Model, subscription.ApplyProRateLimits) {
					return requestParams.Model, nil
				}
			}
			// No user defined models for now.
			if requestParams.Fast {
				return c.FastChatModel, nil
			}
			return c.ChatModel, nil
		},
	)
}

// We only allow dotcom clients to select a custom chat model and maintain an allowlist for which
// custom values we support
func isAllowedCustomChatModel(model string, isProUser bool) bool {
	// When updating these two lists, make sure you also update `allowedModels` in codygateway_dotcom_user.go.
	if isProUser {
		switch model {
		case "anthropic/claude-3-sonnet-20240229",
			"anthropic/claude-3-opus-20240229",
			"fireworks/" + fireworks.Mixtral8x7bInstruct,
			"openai/gpt-3.5-turbo",
			"openai/gpt-4-1106-preview",

			// Remove after the Claude 3 rollout is complete
			"anthropic/claude-2",
			"anthropic/claude-2.0",
			"anthropic/claude-2.1",
			"anthropic/claude-instant-1.2-cyan",
			"anthropic/claude-instant-1.2",
			"anthropic/claude-instant-v1",
			"anthropic/claude-instant-1":
			return true
		}
	} else {
		switch model {
		case // Remove after the Claude 3 rollout is complete
			"anthropic/claude-2",
			"anthropic/claude-2.0",
			"anthropic/claude-instant-v1",
			"anthropic/claude-instant-1":
			return true
		}
	}

	return false
}
