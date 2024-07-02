package config

import (
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/embeddings"
	"github.com/sourcegraph/sourcegraph/internal/codygateway/codygatewayactor"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/anthropic"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/fireworks"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/google"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Config struct {
	env.BaseConfig

	InsecureDev bool

	Port int

	DiagnosticsSecret string

	Dotcom struct {
		URL                          string
		AccessToken                  string
		ActorRefreshCoolDownInterval time.Duration

		// Prompts that get flagged are stored in Redis for a short-time, for
		// better understanding the nature of any ongoing spam/abuse waves.
		FlaggedPromptRecorderTTL time.Duration

		ClientID string
	}

	EnterprisePortal struct {
		URL *url.URL
	}

	Anthropic AnthropicConfig

	OpenAI OpenAIConfig

	Fireworks FireworksConfig

	// Prefixed model names
	AllowedEmbeddingsModels []string

	AllowAnonymous bool

	SourcesSyncInterval time.Duration
	SourcesCacheTTL     time.Duration

	BigQuery struct {
		ProjectID string
		Dataset   string
		Table     string

		EventBufferSize    int
		EventBufferWorkers int
	}

	OpenTelemetry OpenTelemetryConfig

	ActorConcurrencyLimit       codygatewayactor.ActorConcurrencyLimitConfig
	ActorRateLimitNotify        codygatewayactor.ActorRateLimitNotifyConfig
	AutoFlushStreamingResponses bool
	IdentifiersToLogFor         collections.Set[string]

	Attribution struct {
		Enabled bool
	}

	Sourcegraph SourcegraphConfig

	// SAMSClientConfig for verifying and generating SAMS access tokens.
	SAMSClientConfig SAMSClientConfig
	// one of "production", "staging" or "dev" (all 3 can connect to sourcegraph.com)
	Environment string

	Google GoogleConfig
}

type OpenTelemetryConfig struct {
	TracePolicy  policy.TracePolicy
	GCPProjectID string
}

type AnthropicConfig struct {
	// Non-prefixed model names
	AllowedModels  []string
	AccessToken    string
	FlaggingConfig FlaggingConfig
}

type FireworksConfig struct {
	// Non-prefixed model names
	AllowedModels                          []string
	AccessToken                            string
	StarcoderCommunitySingleTenantPercent  int
	StarcoderEnterpriseSingleTenantPercent int
	FlaggingConfig                         FlaggingConfig
}

type OpenAIConfig struct {
	// Non-prefixed model names
	AllowedModels  []string
	AccessToken    string
	OrgID          string
	FlaggingConfig FlaggingConfig
}

type SourcegraphConfig struct {
	EmbeddingsAPIURL   string
	EmbeddingsAPIToken string
}

type GoogleConfig struct {
	AccessToken    string
	AllowedModels  []string
	FlaggingConfig FlaggingConfig
}

// FlaggingConfig defines common parameters for filtering and flagging requests,
// in an LLM-provider agnostic manner.
type FlaggingConfig struct {
	// Phrases we look for in the prompt to consider it valid.
	// Each phrase is lower case.
	AllowedPromptPatterns []string

	// Phrases we look for in a flagged request to consider blocking the response.
	// Each phrase is lower case. Can be empty (to disable blocking).
	BlockedPromptPatterns []string

	// Identifiers (of actors) for which we will log all prompts
	IdentifiersToLogFor []string

	// RequestBlockingEnabled controls whether or not requests can be blocked.
	// A possible escape hatch if there is a sudden spike in false-positives.
	RequestBlockingEnabled bool

	PromptTokenFlaggingLimit int
	PromptTokenBlockingLimit int

	// MaxTokensToSample is the hard-cap, used to block requests that are too long.
	MaxTokensToSample int
	// MaxTokensToSampleFlaggingLimit is a soft-cap, used to flag requests. (But not necessarily block.)
	// So MaxTokensToSampleFlaggingLimit should be <= MaxTokensToSample.
	MaxTokensToSampleFlaggingLimit int

	// FlaggedModelNames is a list of provider-specific model names (e.g. "gtp-3.5")
	// that if used will lead to the request being flagged.
	FlaggedModelNames []string

	// ResponseTokenBlockingLimit is the maximum number of tokens we allow before outright blocking
	// a response. e.g. the client sends a request desiring a response with 100 max tokens, we will
	// block it IFF the ResponseTokenBlockingLimit is less than 100.
	ResponseTokenBlockingLimit int
}

type SAMSClientConfig struct {
	ConnConfig   sams.ConnConfig
	ClientID     string
	ClientSecret string
}

func (sams SAMSClientConfig) Validate() error {
	if err := sams.ConnConfig.Validate(); err != nil {
		return errors.Wrap(err, "invalid ConnConfig")
	}
	if sams.ClientID == "" || sams.ClientSecret == "" {
		return errors.New("ClientID and ClientSecret must be set")
	}
	return nil
}

func (c *Config) Load() {
	c.InsecureDev = env.InsecureDev
	c.Port = c.GetInt("PORT", "9992", "Port to serve Cody Gateway on, generally injected by Cloud Run.")
	// TODO: Eventually migrate to MSP standard (no prefix)
	c.DiagnosticsSecret = c.Get("CODY_GATEWAY_DIAGNOSTICS_SECRET", "", "Secret for accessing diagnostics - "+
		"should be used as 'Authorization: Bearer $secret' header when accessing diagnostics endpoints.")

	c.Dotcom.AccessToken = c.GetOptional("CODY_GATEWAY_DOTCOM_ACCESS_TOKEN",
		"The Sourcegraph.com access token to be used. If not provided, the dotcom-user actor source will be disabled.")
	c.Dotcom.ClientID = c.GetOptional("CODY_GATEWAY_DOTCOM_CLIENT_ID",
		"Value of X-Sourcegraph-Client-Id header to be passed to sourcegraph.com.")
	c.Dotcom.URL = c.Get("CODY_GATEWAY_DOTCOM_API_URL", "https://sourcegraph.com/.api/graphql", "Custom override for the dotcom API endpoint")
	if _, err := url.Parse(c.Dotcom.URL); err != nil {
		c.AddError(errors.Wrap(err, "invalid CODY_GATEWAY_DOTCOM_API_URL"))
	}
	c.Dotcom.ActorRefreshCoolDownInterval = c.GetInterval("CODY_GATEWAY_DOTCOM_ACTOR_COOLDOWN_INTERVAL", "300s",
		"Cooldown period for refreshing the actor info from dotcom.")
	c.Dotcom.FlaggedPromptRecorderTTL = c.GetInterval("CODY_GATEWAY_DOTCOM_FLAGGED_PROMPT_RECORDER_TTL", "1h",
		"Period to retain prompts in Redis.")

	enterprisePortalURL := c.GetOptional("CODY_GATEWAY_ENTERPRISE_PORTAL_URL",
		"The Enterprise Portal instance to target. If not provided, the product subscriptions actor source will be disabled.")
	if enterprisePortalURL != "" {
		var err error
		c.EnterprisePortal.URL, err = url.Parse(enterprisePortalURL)
		if err != nil {
			c.AddError(errors.Wrap(err, "invalid CODY_GATEWAY_ENTERPRISE_PORTAL_URL"))
		}
	}

	c.Anthropic.AccessToken = c.Get("CODY_GATEWAY_ANTHROPIC_ACCESS_TOKEN", "", "The Anthropic access token to be used.")
	c.Anthropic.AllowedModels = splitMaybe(c.Get("CODY_GATEWAY_ANTHROPIC_ALLOWED_MODELS",
		strings.Join([]string{
			"claude-v1",
			"claude-v1-100k",
			"claude-v1.0",
			"claude-v1.2",
			"claude-v1.3",
			"claude-v1.3-100k",
			"claude-2",
			"claude-2.0",
			"claude-2.1",
			"claude-2-100k",
			"claude-instant-v1",
			"claude-instant-1",
			"claude-instant-v1-100k",
			"claude-instant-v1.0",
			"claude-instant-v1.1",
			"claude-instant-v1.1-100k",
			"claude-instant-v1.2",
			"claude-instant-1.2",
			"claude-instant-1.2-cyan",
			anthropic.Claude3Haiku,
			anthropic.Claude3Sonnet,
			anthropic.Claude35Sonnet,
			anthropic.Claude3Opus,
		}, ","),
		"Anthropic models that can be used."))
	if c.Anthropic.AccessToken != "" && len(c.Anthropic.AllowedModels) == 0 {
		c.AddError(errors.New("must provide allowed models for Anthropic"))
	}

	// Load configuration settings specific to how we flag Anthropic requests.
	c.loadFlaggingConfig(&c.Anthropic.FlaggingConfig, "CODY_GATEWAY_ANTHROPIC")

	c.OpenAI.AccessToken = c.GetOptional("CODY_GATEWAY_OPENAI_ACCESS_TOKEN", "The OpenAI access token to be used.")
	c.OpenAI.OrgID = c.GetOptional("CODY_GATEWAY_OPENAI_ORG_ID", "The OpenAI organization to count billing towards. Setting this ensures we always use the correct negotiated terms.")
	c.OpenAI.AllowedModels = splitMaybe(c.Get("CODY_GATEWAY_OPENAI_ALLOWED_MODELS",
		strings.Join([]string{
			"gpt-4",
			"gpt-3.5-turbo",
			"gpt-4o",
			"gpt-4-turbo",
			"gpt-4-turbo-preview",
		}, ","),
		"OpenAI models that can to be used."),
	)
	if c.OpenAI.AccessToken != "" && len(c.OpenAI.AllowedModels) == 0 {
		c.AddError(errors.New("must provide allowed models for OpenAI"))
	}

	// Load configuration settings specific to how we flag OpenAI requests.
	//
	// HACK: Rather than duplicate all of the env vars to independently control how we flag
	// Anthropic and OpenAI models, we are just reusing the same settings for Anthropic.
	// If we follow this better, we should just rename the env vars to something general, e.g.
	// "CODY_GATEWAY_FLAGGING_" and just load a single, shared flagging config.
	c.loadFlaggingConfig(&c.OpenAI.FlaggingConfig, "CODY_GATEWAY_ANTHROPIC")

	c.Fireworks.AccessToken = c.GetOptional("CODY_GATEWAY_FIREWORKS_ACCESS_TOKEN", "The Fireworks access token to be used.")
	c.Fireworks.AllowedModels = splitMaybe(c.Get("CODY_GATEWAY_FIREWORKS_ALLOWED_MODELS",
		strings.Join(slices.Concat([]string{
			// Virtual model strings. Setting these will allow one or more of the specific models
			// and allows Cody Gateway to decide which specific model to route the request to.
			"starcoder",
			// Fireworks multi-tenant models:
			fireworks.StarcoderTwo15b,
			fireworks.StarcoderTwo7b,
			fireworks.Starcoder16b,
			fireworks.Starcoder7b,
			fireworks.Starcoder16b8bit,
			fireworks.Starcoder7b8bit,
			fireworks.Starcoder16bSingleTenant,
			"accounts/fireworks/models/llama-v2-7b-code",
			"accounts/fireworks/models/llama-v2-13b-code",
			"accounts/fireworks/models/llama-v2-13b-code-instruct",
			"accounts/fireworks/models/llama-v2-34b-code-instruct",
			"accounts/fireworks/models/mistral-7b-instruct-4k",
			"accounts/fireworks/models/mixtral-8x7b-instruct",
			"accounts/fireworks/models/mixtral-8x22b-instruct",
			// Deprecated model strings
			"accounts/fireworks/models/starcoder-3b-w8a16",
			"accounts/fireworks/models/starcoder-1b-w8a16",
			fireworks.DeepseekCoder1p3b,
			fireworks.DeepseekCoder7b,
		}, fireworks.FineTunedLlamaModelVariants, fireworks.FineTunedMixtralModelVariants), ","),
		"Fireworks models that can be used."))
	if c.Fireworks.AccessToken != "" && len(c.Fireworks.AllowedModels) == 0 {
		c.AddError(errors.New("must provide allowed models for Fireworks"))
	}
	c.Fireworks.StarcoderCommunitySingleTenantPercent = c.GetPercent("CODY_GATEWAY_FIREWORKS_STARCODER_COMMUNITY_SINGLE_TENANT_PERCENT", "0", "The percentage of community traffic for Starcoder to be redirected to the single-tenant deployment.")
	c.Fireworks.StarcoderEnterpriseSingleTenantPercent = c.GetPercent("CODY_GATEWAY_FIREWORKS_STARCODER_ENTERPRISE_SINGLE_TENANT_PERCENT", "100", "The percentage of Enterprise traffic for Starcoder to be redirected to the single-tenant deployment.")

	// Configurations for Google Gemini models.
	c.Google.AccessToken = c.GetOptional("CODY_GATEWAY_GOOGLE_ACCESS_TOKEN", "The Google AI Studio access token to be used.")
	c.Google.AllowedModels = splitMaybe(c.Get("CODY_GATEWAY_GOOGLE_ALLOWED_MODELS",
		strings.Join([]string{
			google.Gemini15FlashLatest,
			google.Gemini15ProLatest,
			google.GeminiProLatest,
			google.Gemini15Flash001,
			google.Gemini15Pro001,
			google.Gemini15Flash,
			google.Gemini15Pro,
			google.GeminiPro,
		}, ","),
		"Google models that can to be used."),
	)
	if c.Google.AccessToken != "" && len(c.Google.AllowedModels) == 0 {
		c.AddError(errors.New("must provide allowed models for Google"))
	}

	// Load configuration settings specific to how we flag Google-routed requests.
	// HACK: Same as the comment on OpenAI or Fireworks, re: only using one env var prefix.
	c.loadFlaggingConfig(&c.Google.FlaggingConfig, "CODY_GATEWAY_ANTHROPIC")

	defaultEmbeddingModels := strings.Join([]string{
		string(embeddings.ModelNameOpenAIAda),
		string(embeddings.ModelNameSourcegraphSTMultiQA),
		string(embeddings.ModelNameSourcegraphMetadataGen),
	}, ",")
	c.AllowedEmbeddingsModels = splitMaybe(c.Get(
		"CODY_GATEWAY_ALLOWED_EMBEDDINGS_MODELS",
		defaultEmbeddingModels,
		"The models allowed for embeddings generation."))
	if len(c.AllowedEmbeddingsModels) == 0 {
		c.AddError(errors.New("must provide allowed models for embeddings generation"))
	}

	c.AllowAnonymous = c.GetBool("CODY_GATEWAY_ALLOW_ANONYMOUS", "false", "Allow anonymous access to Cody Gateway.")
	c.SourcesSyncInterval = c.GetInterval("CODY_GATEWAY_SOURCES_SYNC_INTERVAL", "2m", "The interval at which to sync actor sources.")
	c.SourcesCacheTTL = c.GetInterval("CODY_GATEWAY_SOURCES_CACHE_TTL", "24h", "The TTL for caches used by actor sources.")

	c.BigQuery.ProjectID = c.GetOptional("CODY_GATEWAY_BIGQUERY_PROJECT_ID", "The project ID for the BigQuery events.")
	if c.BigQuery.ProjectID == "" {
		c.BigQuery.ProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	c.BigQuery.Dataset = c.Get("CODY_GATEWAY_BIGQUERY_DATASET", "cody_gateway", "The dataset for the BigQuery events.")
	c.BigQuery.Table = c.Get("CODY_GATEWAY_BIGQUERY_TABLE", "events", "The table for the BigQuery events.")
	c.BigQuery.EventBufferSize = c.GetInt("CODY_GATEWAY_BIGQUERY_EVENT_BUFFER_SIZE", "100",
		"The number of events allowed to buffer when submitting BigQuery events - set to 0 to disable.")
	c.BigQuery.EventBufferWorkers = c.GetInt("CODY_GATEWAY_BIGQUERY_EVENT_BUFFER_WORKERS", "0",
		"The number of workers to process events - set to 0 to use a default that scales off buffer size.")

	c.OpenTelemetry.TracePolicy = policy.TracePolicy(c.Get("CODY_GATEWAY_TRACE_POLICY", "all", "Trace policy, one of 'all', 'selective', 'none'."))
	c.OpenTelemetry.GCPProjectID = c.GetOptional("CODY_GATEWAY_OTEL_GCP_PROJECT_ID", "Google Cloud Traces project ID.")
	if c.OpenTelemetry.GCPProjectID == "" {
		c.OpenTelemetry.GCPProjectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}

	c.ActorConcurrencyLimit.Percentage = float32(c.GetPercent("CODY_GATEWAY_ACTOR_CONCURRENCY_LIMIT_PERCENTAGE", "50", "The percentage of daily rate limit to be allowed as concurrent requests limit from an actor.")) / 100
	c.ActorConcurrencyLimit.Interval = c.GetInterval("CODY_GATEWAY_ACTOR_CONCURRENCY_LIMIT_INTERVAL", "10s", "The interval at which to check the concurrent requests limit from an actor.")

	c.ActorRateLimitNotify.SlackWebhookURL = c.GetOptional("CODY_GATEWAY_ACTOR_RATE_LIMIT_NOTIFY_SLACK_WEBHOOK_URL", "The Slack webhook URL to send notifications to.")
	c.AutoFlushStreamingResponses = c.GetBool("CODY_GATEWAY_AUTO_FLUSH_STREAMING_RESPONSES", "false", "Whether we should flush streaming responses after every write.")
	c.IdentifiersToLogFor = collections.NewSet(splitMaybe(c.GetOptional("CODY_GATEWAY_IDENTIFIERS_TO_LOG_FOR", "Identifiers of actors that have all their prompts logged."))...)

	c.Attribution.Enabled = c.GetBool("CODY_GATEWAY_ENABLE_ATTRIBUTION_SEARCH", "false", "Whether attribution search endpoint is available.")

	c.Sourcegraph.EmbeddingsAPIURL = c.Get("CODY_GATEWAY_SOURCEGRAPH_EMBEDDINGS_API_URL", "https://embeddings.sourcegraph.com/v2/models/st-multi-qa-mpnet-base-dot-v1/infer", "URL of the SMEGA API.")
	c.Sourcegraph.EmbeddingsAPIToken = c.Get("CODY_GATEWAY_SOURCEGRAPH_EMBEDDINGS_API_TOKEN", "", "Token to use for the SMEGA API.")

	// SAMS_URL, SAMS_API_URL are same keys used for sams.NewConnConfigFromEnv
	c.SAMSClientConfig.ConnConfig.ExternalURL = c.Get("SAMS_URL", "https://accounts.sourcegraph.com",
		"SAMS external service endpoint")
	if apiurl := c.GetOptional("SAMS_API_URL", "SAMS API endpoint"); apiurl != "" {
		c.SAMSClientConfig.ConnConfig.APIURL = &apiurl
	}
	c.SAMSClientConfig.ClientID = c.GetOptional("SAMS_CLIENT_ID", "SAMS OAuth client ID")
	c.SAMSClientConfig.ClientSecret = c.GetOptional("SAMS_CLIENT_SECRET", "SAMS OAuth client secret")

	c.Environment = c.Get("CODY_GATEWAY_ENVIRONMENT", "dev", "Environment name.")
}

// loadFlaggingConfig loads the common set of flagging-related environment variables for
// an LLM provider. The expectation is that the env vars all share the provider-specific
// prefix, and a flagging config-specific suffix.
//
// IMPORTANT: Some of the env vars loaded are _required_. So be sure that they are all
// set before calling loadFlaggingConfig for a new LLM provider.
func (c *Config) loadFlaggingConfig(cfg *FlaggingConfig, envVarPrefix string) {
	// Ensure the prefix ends with a _, so we require
	// "ACME_CORP_MAX_TOKENS" and not "ACME_CORPMAX_TOKENS".
	if !strings.HasSuffix(envVarPrefix, "_") {
		envVarPrefix += "_"
	}

	// Loads a comma-separated env var, and converts it to lower-case.
	maybeLoadLowercaseSlice := func(envVar, description string) []string {
		value := c.GetOptional(envVarPrefix+envVar, description)
		values := splitMaybe(value)
		return toLower(values)
	}

	cfg.MaxTokensToSample = c.GetInt(envVarPrefix+"MAX_TOKENS_TO_SAMPLE", "4000", "Maximum permitted value of maxTokensToSample")
	cfg.MaxTokensToSampleFlaggingLimit = c.GetInt(envVarPrefix+"MAX_TOKENS_TO_SAMPLE_FLAGGING_LIMIT", "4000", "Maximum value of max_tokens_to_sample to allow without flagging.")

	cfg.AllowedPromptPatterns = maybeLoadLowercaseSlice("ALLOWED_PROMPT_PATTERNS", "Allowed prompt patterns")
	cfg.BlockedPromptPatterns = maybeLoadLowercaseSlice("BLOCKED_PROMPT_PATTERNS", "Patterns to block in prompt.")
	cfg.RequestBlockingEnabled = c.GetBool(envVarPrefix+"REQUEST_BLOCKING_ENABLED", "false", "Whether we should block requests that match our blocking criteria.")

	cfg.PromptTokenBlockingLimit = c.GetInt(envVarPrefix+"PROMPT_TOKEN_BLOCKING_LIMIT", "20000", "Maximum number of prompt tokens to allow without blocking.")
	cfg.PromptTokenFlaggingLimit = c.GetInt(envVarPrefix+"PROMPT_TOKEN_FLAGGING_LIMIT", "18000", "Maximum number of prompt tokens to allow without flagging.")
	cfg.ResponseTokenBlockingLimit = c.GetInt(envVarPrefix+"RESPONSE_TOKEN_BLOCKING_LIMIT", "4000", "Maximum number of completion tokens to allow without blocking.")

	cfg.FlaggedModelNames = maybeLoadLowercaseSlice("FLAGGED_MODEL_NAMES", "LLM models that will always lead to the request getting flagged.")
}

// splitMaybe splits the provided string on commas, but returns nil if given the empty string.
func splitMaybe(input string) []string {
	if input == "" {
		return nil
	}
	return strings.Split(input, ",")
}

func toLower(input []string) []string {
	var res []string
	for _, s := range input {
		res = append(res, strings.ToLower(s))
	}
	return res
}
