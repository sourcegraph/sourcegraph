package completions

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/internal/completions/client/anthropic"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/fireworks"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/google"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

// getModelFn is the thunk used to return the LLM model we should use for processing
// the supplied completion request. Depending on the incomming request, site config,
// feature used, etc. it could be any number of things.
type getModelFn func(ctx context.Context, requestParams types.CodyCompletionRequestParameters, c *conftypes.CompletionsConfig) (string, error)

func getCodeCompletionModelFn() getModelFn {
	return func(_ context.Context, requestParams types.CodyCompletionRequestParameters, c *conftypes.CompletionsConfig) (string, error) {
		// For code completions, we only allow certain models to be used.
		// (Regardless of if the user is on Cody Free, Pro, or Enterprise.)
		if requestParams.Model != "" {
			if isAllowedCodeCompletionModel(requestParams.Model) {
				return requestParams.Model, nil
			}
			return "", errors.Newf("unsupported code completion model %q", requestParams.Model)
		}
		// The caller will probably return a 4xx if Cody isn't available on the Sourcegraph
		// instance before calling getModel.
		if c == nil {
			return "", errors.New("no completions config available")
		}
		return c.CompletionModel, nil
	}
}

func getChatModelFn(db database.DB) getModelFn {
	return func(ctx context.Context, requestParams types.CodyCompletionRequestParameters, c *conftypes.CompletionsConfig) (string, error) {
		// If running on dotcom, i.e. using Cody Free/Cody Pro, then a number
		// of models are available depending on the caller's subscription status.
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

		// For any other Sourcegraph instance, i.e. using Cody Enterprise,
		// we just use the configured "chat" or "fastChat" model.
		// TODO(PRIME-283): Enable LLM model selection Cody Enterprise users.
		if requestParams.Fast {
			return c.FastChatModel, nil
		}
		return c.ChatModel, nil
	}
}

func isAllowedCodeCompletionModel(model string) bool {
	switch model {
	case "fireworks/starcoder",
		"fireworks/starcoder-16b",
		"fireworks/starcoder-7b",
		"fireworks/starcoder2-15b",
		"fireworks/starcoder2-7b",
		"fireworks/" + fireworks.Starcoder16b,
		"fireworks/" + fireworks.Starcoder7b,
		"fireworks/" + fireworks.Llama27bCode,
		"fireworks/" + fireworks.Llama213bCode,
		"fireworks/" + fireworks.Llama213bCodeInstruct,
		"fireworks/" + fireworks.Llama234bCodeInstruct,
		"fireworks/" + fireworks.Mistral7bInstruct,
		"fireworks/" + fireworks.FineTunedFIMVariant1,
		"fireworks/" + fireworks.FineTunedFIMVariant2,
		"fireworks/" + fireworks.FineTunedFIMVariant3,
		"fireworks/" + fireworks.FineTunedFIMVariant4,
		"fireworks/" + fireworks.FineTunedFIMLangSpecificMixtral,
		"fireworks/" + fireworks.DeepseekCoder1p3b,
		"fireworks/" + fireworks.DeepseekCoder7b,
		"anthropic/claude-instant-1.2",
		"anthropic/claude-3-haiku-20240307",
		// Deprecated model identifiers
		"anthropic/claude-instant-v1",
		"anthropic/claude-instant-1",
		"anthropic/claude-instant-1.2-cyan",
		"google/" + google.Gemini15Flash,
		"google/" + google.Gemini15FlashLatest,
		"google/" + google.Gemini15Flash001,
		"google/" + google.GeminiPro,
		"google/" + google.GeminiProLatest,
		"fireworks/accounts/sourcegraph/models/starcoder-7b",
		"fireworks/accounts/sourcegraph/models/starcoder-16b",
		"fireworks/accounts/fireworks/models/starcoder-3b-w8a16",
		"fireworks/accounts/fireworks/models/starcoder-1b-w8a16":
		return true
	}

	return false
}

// We only allow dotcom clients to select a custom chat model and maintain an allowlist for which
// custom values we support
func isAllowedCustomChatModel(model string, isProUser bool) bool {
	// When updating these two lists, make sure you also update `allowedModels` in codygateway_dotcom_user.go.
	if isProUser {
		switch model {
		case
			"anthropic/" + anthropic.Claude3Haiku,
			"anthropic/" + anthropic.Claude3Sonnet,
			"anthropic/" + anthropic.Claude35Sonnet,
			"anthropic/" + anthropic.Claude3Opus,
			"fireworks/" + fireworks.Mixtral8x7bInstruct,
			"fireworks/" + fireworks.Mixtral8x22Instruct,
			"openai/gpt-3.5-turbo",
			"openai/gpt-4o",
			"openai/gpt-4-turbo",
			"openai/gpt-4-turbo-preview",
			"google/" + google.Gemini15FlashLatest,
			"google/" + google.Gemini15ProLatest,
			"google/" + google.GeminiProLatest,
			"google/" + google.Gemini15Flash001,
			"google/" + google.Gemini15Pro001,
			"google/" + google.Gemini15Flash,
			"google/" + google.Gemini15Pro,
			"google/" + google.GeminiPro,

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
		case
			"anthropic/" + anthropic.Claude3Haiku,
			"anthropic/" + anthropic.Claude3Sonnet,
			"anthropic/" + anthropic.Claude35Sonnet,
			"fireworks/" + fireworks.Mixtral8x7bInstruct,
			"fireworks/" + fireworks.Mixtral8x22Instruct,
			"openai/gpt-3.5-turbo",
			"google/" + google.Gemini15FlashLatest,
			"google/" + google.Gemini15ProLatest,
			"google/" + google.GeminiProLatest,
			"google/" + google.Gemini15Flash,
			"google/" + google.Gemini15Pro,
			"google/" + google.GeminiPro,
			// Remove after the Claude 3 rollout is complete
			"anthropic/claude-2",
			"anthropic/claude-2.0",
			"anthropic/claude-instant-v1",
			"anthropic/claude-instant-1":
			return true
		}
	}

	return false
}
