package httpapi

import (
	"context"
	"net/http"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/completions/client/fireworks"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/guardrails"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/telemetry/telemetryrecorder"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewCodeCompletionsHandler is an http handler which sends back code completion results.
func NewCodeCompletionsHandler(logger log.Logger, db database.DB, test guardrails.AttributionTest) http.Handler {
	logger = logger.Scoped("code")
	rl := NewRateLimiter(db, redispool.Store, types.CompletionsFeatureCode)
	return newCompletionsHandler(
		logger,
		db,
		db.Users(),
		db.AccessTokens(),
		telemetryrecorder.New(db),
		test,
		types.CompletionsFeatureCode,
		rl,
		"code",
		func(_ context.Context, requestParams types.CodyCompletionRequestParameters, c *conftypes.CompletionsConfig) (string, error) {
			customModel := allowedCustomModel(requestParams.Model)
			if customModel != "" {
				return customModel, nil
			}
			if requestParams.Model != "" {
				return "", errors.Newf("Unsupported code completion model %q", requestParams.Model)
			}
			return c.CompletionModel, nil
		},
	)
}

func allowedCustomModel(model string) string {
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
		"anthropic/claude-instant-1.2",
		// Deprecated model identifiers
		"anthropic/claude-instant-v1",
		"anthropic/claude-instant-1",
		"anthropic/claude-instant-1.2-cyan",
		"fireworks/accounts/sourcegraph/models/starcoder-7b",
		"fireworks/accounts/sourcegraph/models/starcoder-16b",
		"fireworks/accounts/fireworks/models/starcoder-3b-w8a16",
		"fireworks/accounts/fireworks/models/starcoder-1b-w8a16":
		return model
	}

	return ""
}
