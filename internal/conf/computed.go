package conf

import (
	"encoding/hex"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/cronexpr"

	"github.com/sourcegraph/sourcegraph/internal/conf/confdefaults"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/hashutil"
	"github.com/sourcegraph/sourcegraph/internal/license"
	srccli "github.com/sourcegraph/sourcegraph/internal/src-cli"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

func init() {
	deployType := deploy.Type()
	if !deploy.IsValidDeployType(deployType) {
		log.Fatalf("The 'DEPLOY_TYPE' environment variable is invalid. Expected one of: %q, %q, %q, %q, %q, %q. Got: %q", deploy.Kubernetes, deploy.DockerCompose, deploy.PureDocker, deploy.SingleDocker, deploy.Dev, deploy.Helm, deployType)
	}

	confdefaults.Default = defaultConfigForDeployment()
}

func defaultConfigForDeployment() conftypes.RawUnified {
	deployType := deploy.Type()
	switch {
	case deploy.IsDev(deployType):
		return confdefaults.DevAndTesting
	case deploy.IsDeployTypeSingleDockerContainer(deployType):
		return confdefaults.DockerContainer
	case deploy.IsDeployTypeKubernetes(deployType), deploy.IsDeployTypeDockerCompose(deployType), deploy.IsDeployTypePureDocker(deployType):
		return confdefaults.KubernetesOrDockerComposeOrPureDocker
	default:
		panic("deploy type did not register default configuration: " + deployType)
	}
}

func ExecutorsAccessToken() string {
	return Get().ExecutorsAccessToken
}

type AccessTokenAllow string

const (
	AccessTokensNone  AccessTokenAllow = "none"
	AccessTokensAll   AccessTokenAllow = "all-users-create"
	AccessTokensAdmin AccessTokenAllow = "site-admin-create"
)

// AccessTokensAllow returns whether access tokens are enabled, disabled, or
// restricted creation to only site admins.
func AccessTokensAllow() AccessTokenAllow {
	cfg := Get().AuthAccessTokens
	if cfg == nil || cfg.Allow == "" {
		return AccessTokensAll
	}
	v := AccessTokenAllow(cfg.Allow)
	switch v {
	case AccessTokensAll, AccessTokensAdmin:
		return v
	default:
		return AccessTokensNone
	}
}

func AccessTokensMaxPerUser() int {
	defaultValue := 25
	cfg := Get().AuthAccessTokens
	if cfg == nil || cfg.MaxTokensPerUser == nil {
		return defaultValue
	}
	return *cfg.MaxTokensPerUser
}

// AccessTokensAllowNoExpiration returns whether access tokens can be created without expiration.
func AccessTokensAllowNoExpiration() bool {
	cfg := Get().AuthAccessTokens
	if cfg == nil || cfg.AllowNoExpiration == nil {
		return true
	}
	return *cfg.AllowNoExpiration
}

// AccessTokensExpirationOptions returns the default access token expiration days
// and the available expiration options (in days). It first checks if any defaults
// or options are configured, and falls back to the hardcoded defaults if not.
// Options will be in ascending order.
func AccessTokensExpirationOptions() (defaultDays int, options []int) {
	defaultOptions := []int{7, 14, 30, 60, 90}
	defaultExpiryDays := 90
	cfg := Get().AuthAccessTokens
	if cfg == nil {
		return defaultExpiryDays, defaultOptions
	}

	// If there is a default specified, use that.
	if cfg.DefaultExpirationDays != nil {
		defaultExpiryDays = *cfg.DefaultExpirationDays
	}

	// use the default options if there are none specified
	expiryOptions := cfg.ExpirationOptionDays
	if len(expiryOptions) == 0 {
		expiryOptions = defaultOptions
	}

	// add the default option if it wasn't in the list already
	foundDefault := false
	for _, days := range expiryOptions {
		foundDefault = foundDefault || days == defaultExpiryDays
	}
	if !foundDefault {
		expiryOptions = append(expiryOptions, defaultExpiryDays)
	}

	slices.Sort(expiryOptions)

	return defaultExpiryDays, expiryOptions
}

// EmailVerificationRequired returns whether users must verify an email address before they
// can perform most actions on this site.
//
// It's false for sites that do not have an email sending API key set up.
func EmailVerificationRequired() bool {
	return CanSendEmail()
}

// CanSendEmail returns whether the site can send emails (e.g., to reset a password or
// invite a user to an org).
//
// It's false for sites that do not have an email sending API key set up.
func CanSendEmail() bool {
	return Get().EmailSmtp != nil
}

// EmailSenderName returns `email.senderName`. If that's not set, it returns
// the default value "Sourcegraph".
func EmailSenderName() string {
	sender := Get().EmailSenderName
	if sender != "" {
		return sender
	}
	return "Sourcegraph"
}

// UpdateChannel tells the update channel. Default is "release".
func UpdateChannel() string {
	channel := Get().UpdateChannel
	if channel == "" {
		return "release"
	}
	return channel
}

func BatchChangesEnabled() bool {
	if enabled := Get().BatchChangesEnabled; enabled != nil {
		return *enabled
	}
	return true
}

func BatchChangesRestrictedToAdmins() bool {
	if restricted := Get().BatchChangesRestrictToAdmins; restricted != nil {
		return *restricted
	}
	return false
}

// CodyEnabled returns whether Cody is enabled on this instance.
//
// If `cody.enabled` is not set or set to false, it's not enabled.
//
// Legacy-support for `completions.enabled`:
// If `cody.enabled` is NOT set, but `completions.enabled` is true, then cody is enabled.
// If `cody.enabled` is set, and `completions.enabled` is set to false, cody is disabled.
func CodyEnabled() bool {
	return codyEnabled(Get().SiteConfig())
}

func codyEnabled(siteConfig schema.SiteConfiguration) bool {
	enabled := siteConfig.CodyEnabled
	completions := siteConfig.Completions

	// If the cody.enabled flag is explicitly false, disable all cody features.
	if enabled != nil && !*enabled {
		return false
	}

	// Support for Legacy configurations in which `completions` is set to
	// `enabled`, but `cody.enabled` is not set.
	if enabled == nil && completions != nil {
		// Unset means false.
		return completions.Enabled != nil && *completions.Enabled
	}

	if enabled == nil {
		return false
	}

	return *enabled
}

// newCodyEnabled checks only for the new CodyEnabled flag. If you need back
// compat, use codyEnabled instead.
func newCodyEnabled(siteConfig schema.SiteConfiguration) bool {
	return siteConfig.CodyEnabled != nil && *siteConfig.CodyEnabled
}

func CodyRestrictUsersFeatureFlag() bool {
	if restrict := Get().CodyRestrictUsersFeatureFlag; restrict != nil {
		return *restrict
	}
	return false
}

func CodyPermissionsEnabled() bool {
	// CodyPermissions is never used if the deprecated CodyRestrictUsersFeatureFlag is set,
	// as that implies the site admin has not upgraded to the new RBAC model yet.
	if CodyRestrictUsersFeatureFlag() {
		return false
	}

	if enabled := Get().CodyPermissions; enabled != nil {
		return *enabled
	}
	return true // default to enabled
}

func ExecutorsEnabled() bool {
	return Get().ExecutorsAccessToken != ""
}

func ExecutorsFrontendURL() string {
	current := Get()
	if current.ExecutorsFrontendURL != "" {
		return current.ExecutorsFrontendURL
	}

	return current.ExternalURL
}

func ExecutorsSrcCLIImage() string {
	current := Get()
	if current.ExecutorsSrcCLIImage != "" {
		return current.ExecutorsSrcCLIImage
	}

	return "sourcegraph/src-cli"
}

func ExecutorsSrcCLIImageTag() string {
	current := Get()
	if current.ExecutorsSrcCLIImageTag != "" {
		return current.ExecutorsSrcCLIImageTag
	}

	return srccli.MinimumVersion
}

func ExecutorsLsifGoImage() string {
	current := Get()
	if current.ExecutorsLsifGoImage != "" {
		return current.ExecutorsLsifGoImage
	}
	return "sourcegraph/lsif-go"
}

func ExecutorsBatcheshelperImage() string {
	current := Get()
	if current.ExecutorsBatcheshelperImage != "" {
		return current.ExecutorsBatcheshelperImage
	}

	return "sourcegraph/batcheshelper"
}

func ExecutorsBatcheshelperImageTag() string {
	current := Get()
	if current.ExecutorsBatcheshelperImageTag != "" {
		return current.ExecutorsBatcheshelperImageTag
	}

	if version.IsDev(version.Version()) {
		return "insiders"
	}

	return version.Version()
}

func CodeIntelAutoIndexingEnabled() bool {
	if enabled := Get().CodeIntelAutoIndexingEnabled; enabled != nil {
		return *enabled
	}
	return false
}

func CodeIntelAutoIndexingAllowGlobalPolicies() bool {
	if enabled := Get().CodeIntelAutoIndexingAllowGlobalPolicies; enabled != nil {
		return *enabled
	}
	return false
}

func CodeIntelAutoIndexingPolicyRepositoryMatchLimit() int {
	val := Get().CodeIntelAutoIndexingPolicyRepositoryMatchLimit
	if val == nil || *val < -1 {
		return -1
	}

	return *val
}

func CodeIntelRankingDocumentReferenceCountsEnabled() bool {
	if enabled := Get().CodeIntelRankingDocumentReferenceCountsEnabled; enabled != nil {
		return *enabled
	}
	return false
}

func CodeIntelRankingDocumentReferenceCountsCronExpression() (*cronexpr.Expression, error) {
	if cronExpression := Get().CodeIntelRankingDocumentReferenceCountsCronExpression; cronExpression != nil {
		return cronexpr.Parse(*cronExpression)
	}

	return cronexpr.Parse("@weekly")
}

func CodeIntelRankingDocumentReferenceCountsGraphKey() string {
	if val := Get().CodeIntelRankingDocumentReferenceCountsGraphKey; val != "" {
		return val
	}
	return "dev"
}

func EmbeddingsEnabled() bool {
	return GetEmbeddingsConfig(Get().SiteConfiguration) != nil
}

func ProductResearchPageEnabled() bool {
	if enabled := Get().ProductResearchPageEnabled; enabled != nil {
		return *enabled
	}
	return true
}

func ExternalURL() string {
	return Get().ExternalURL
}

func UsingExternalURL() bool {
	url := Get().ExternalURL
	return !(url == "" || strings.HasPrefix(url, "http://localhost") || strings.HasPrefix(url, "https://localhost") || strings.HasPrefix(url, "http://127.0.0.1") || strings.HasPrefix(url, "https://127.0.0.1")) // CI:LOCALHOST_OK
}

func IsExternalURLSecure() bool {
	return strings.HasPrefix(Get().ExternalURL, "https:")
}

func IsBuiltinSignupAllowed() bool {
	provs := Get().AuthProviders
	for _, prov := range provs {
		if prov.Builtin != nil {
			return prov.Builtin.AllowSignup
		}
	}
	return false
}

// IsAccessRequestEnabled returns whether request access experimental feature is enabled or not.
func IsAccessRequestEnabled() bool {
	authAccessRequest := Get().AuthAccessRequest
	return authAccessRequest == nil || authAccessRequest.Enabled == nil || *authAccessRequest.Enabled
}

// AuthPrimaryLoginProvidersCount returns the number of primary login providers
// configured, or 3 (the default) if not explicitly configured.
// This is only used for the UI
func AuthPrimaryLoginProvidersCount() int {
	c := Get().AuthPrimaryLoginProvidersCount
	if c == 0 {
		return 3 // default to 3
	}
	return c
}

// SearchSymbolsParallelism returns 20, or the site config
// "debug.search.symbolsParallelism" value if configured.
func SearchSymbolsParallelism() int {
	val := Get().DebugSearchSymbolsParallelism
	if val == 0 {
		return 20
	}
	return val
}

func EventLoggingEnabled() bool {
	val := ExperimentalFeatures().EventLogging
	if val == "" {
		return true
	}
	return val == "enabled"
}

// SearchDocumentRanksWeight controls the impact of document ranks on the final ranking when
// SearchOptions.UseDocumentRanks is enabled. The default is 0.5 * 9000 (half the zoekt default),
// to match existing behavior where ranks are given half the priority as existing scoring signals.
// We plan to eventually remove this, once we experiment on real data to find a good default.
func SearchDocumentRanksWeight() float64 {
	ranking := ExperimentalFeatures().Ranking
	if ranking != nil && ranking.DocumentRanksWeight != nil {
		return *ranking.DocumentRanksWeight
	} else {
		return 4500
	}
}

func RankingMaxQueueSizeBytes() int {
	ranking := ExperimentalFeatures().Ranking
	if ranking == nil || ranking.MaxQueueSizeBytes == nil {
		return -1
	}
	return *ranking.MaxQueueSizeBytes
}

// SearchFlushWallTime controls the amount of time that Zoekt shards collect and rank results. For
// larger codebases, it can be helpful to increase this to improve the ranking stability and quality.
func SearchFlushWallTime(keywordScoring bool) time.Duration {
	ranking := ExperimentalFeatures().Ranking
	if ranking != nil && ranking.FlushWallTimeMS > 0 {
		return time.Duration(ranking.FlushWallTimeMS) * time.Millisecond
	} else {
		if keywordScoring {
			// Keyword scoring takes longer than standard searches, so use a higher FlushWallTime
			// to help ensure ranking is stable
			return 2 * time.Second
		} else {
			return 500 * time.Millisecond
		}
	}
}

func ExperimentalFeatures() schema.ExperimentalFeatures {
	val := Get().ExperimentalFeatures
	if val == nil {
		return schema.ExperimentalFeatures{}
	}
	return *val
}

// AuthMinPasswordLength returns the value of minimum password length requirement.
// If not set, it returns the default value 12.
func AuthMinPasswordLength() int {
	val := Get().AuthMinPasswordLength
	if val <= 0 {
		return 12
	}
	return val
}

// GenericPasswordPolicy is a generic password policy that defines password requirements.
type GenericPasswordPolicy struct {
	Enabled                   bool
	MinimumLength             int
	NumberOfSpecialCharacters int
	RequireAtLeastOneNumber   bool
	RequireUpperandLowerCase  bool
}

// AuthPasswordPolicy returns a GenericPasswordPolicy for password validation
func AuthPasswordPolicy() GenericPasswordPolicy {
	ml := Get().AuthMinPasswordLength

	if p := Get().AuthPasswordPolicy; p != nil {
		return GenericPasswordPolicy{
			Enabled:                   p.Enabled,
			MinimumLength:             ml,
			NumberOfSpecialCharacters: p.NumberOfSpecialCharacters,
			RequireAtLeastOneNumber:   p.RequireAtLeastOneNumber,
			RequireUpperandLowerCase:  p.RequireUpperandLowerCase,
		}
	}

	if ep := ExperimentalFeatures().PasswordPolicy; ep != nil {
		return GenericPasswordPolicy{
			Enabled:                   ep.Enabled,
			MinimumLength:             ml,
			NumberOfSpecialCharacters: ep.NumberOfSpecialCharacters,
			RequireAtLeastOneNumber:   ep.RequireAtLeastOneNumber,
			RequireUpperandLowerCase:  ep.RequireUpperandLowerCase,
		}
	}

	return GenericPasswordPolicy{
		Enabled:                   false,
		MinimumLength:             0,
		NumberOfSpecialCharacters: 0,
		RequireAtLeastOneNumber:   false,
		RequireUpperandLowerCase:  false,
	}
}

func PasswordPolicyEnabled() bool {
	pc := AuthPasswordPolicy()
	return pc.Enabled
}

func RateLimits() schema.RateLimits {
	rl := schema.RateLimits{
		GraphQLMaxAliases:             500,
		GraphQLMaxFieldCount:          500_000,
		GraphQLMaxDepth:               30,
		GraphQLMaxDuplicateFieldCount: 500,
		GraphQLMaxUniqueFieldCount:    500,
	}

	configured := Get().RateLimits

	if configured != nil {
		if configured.GraphQLMaxAliases <= 0 {
			rl.GraphQLMaxAliases = configured.GraphQLMaxAliases
		}
		if configured.GraphQLMaxFieldCount <= 0 {
			rl.GraphQLMaxFieldCount = configured.GraphQLMaxFieldCount
		}
		if configured.GraphQLMaxDepth <= 0 {
			rl.GraphQLMaxDepth = configured.GraphQLMaxDepth
		}
	}
	return rl
}

// By default, password reset links are valid for 4 hours.
const defaultPasswordLinkExpiry = 14400

// AuthPasswordResetLinkExpiry returns the time (in seconds) indicating how long password
// reset links are considered valid. If not set, it returns the default value.
func AuthPasswordResetLinkExpiry() int {
	val := Get().AuthPasswordResetLinkExpiry
	if val <= 0 {
		return defaultPasswordLinkExpiry
	}
	return val
}

// AuthLockout populates and returns the *schema.AuthLockout with default values
// for fields that are not initialized.
func AuthLockout() *schema.AuthLockout {
	val := Get().AuthLockout
	if val == nil {
		return &schema.AuthLockout{
			ConsecutivePeriod:      3600,
			FailedAttemptThreshold: 5,
			LockoutPeriod:          1800,
		}
	}

	if val.ConsecutivePeriod <= 0 {
		val.ConsecutivePeriod = 3600
	}
	if val.FailedAttemptThreshold <= 0 {
		val.FailedAttemptThreshold = 5
	}
	if val.LockoutPeriod <= 0 {
		val.LockoutPeriod = 1800
	}
	return val
}

const defaultGitLongCommandTimeout = time.Hour

// GitLongCommandTimeout returns the maximum amount of time in seconds that a
// long Git command (e.g. clone or remote update) is allowed to execute. If not
// set, it returns the default value.
//
// In general, Git commands that are expected to take a long time should be
// executed in the background in a non-blocking fashion.
func GitLongCommandTimeout() time.Duration {
	val := Get().GitLongCommandTimeout
	if val < 1 {
		return defaultGitLongCommandTimeout
	}
	return time.Duration(val) * time.Second
}

// GitMaxCodehostRequestsPerSecond returns maximum number of remote code host
// git operations to be run per second per gitserver. If not set, it returns the
// default value -1.
func GitMaxCodehostRequestsPerSecond() int {
	val := Get().GitMaxCodehostRequestsPerSecond
	if val == nil || *val < -1 {
		return -1
	}
	return *val
}

func GitMaxConcurrentClones() int {
	v := Get().GitMaxConcurrentClones
	if v <= 0 {
		return 5
	}
	return v
}

// HashedCurrentLicenseKeyForAnalytics provides the current site license key, hashed using sha256, for anaytics purposes.
func HashedCurrentLicenseKeyForAnalytics() string {
	return HashedLicenseKeyForAnalytics(Get().LicenseKey)
}

// HashedCurrentLicenseKeyForAnalytics provides a license key, hashed using sha256, for anaytics purposes.
func HashedLicenseKeyForAnalytics(licenseKey string) string {
	return HashedLicenseKeyWithPrefix(licenseKey, "event-logging-telemetry-prefix")
}

// HashedLicenseKeyWithPrefix provides a sha256 hashed license key with a prefix (to ensure unique hashed values by use case).
func HashedLicenseKeyWithPrefix(licenseKey string, prefix string) string {
	return hex.EncodeToString(hashutil.ToSHA256Bytes([]byte(prefix + licenseKey)))
}

// GetCompletionsConfig evaluates a complete completions configuration based on
// site configuration. The configuration may be nil if completions is disabled.
func GetCompletionsConfig(siteConfig schema.SiteConfiguration) (c *conftypes.CompletionsConfig) {
	// If cody is disabled, don't use completions.
	if !codyEnabled(siteConfig) {
		return nil
	}

	completionsConfig := siteConfig.Completions
	// If no completions configuration is set at all, but cody is enabled, assume
	// a default configuration.
	if completionsConfig == nil {
		completionsConfig = &schema.Completions{
			Provider:        string(conftypes.CompletionsProviderNameSourcegraph),
			ChatModel:       "anthropic/claude-2.0",
			FastChatModel:   "anthropic/claude-instant-1",
			CompletionModel: "anthropic/claude-instant-1",
		}
	}

	// If no provider is configured, we assume the Sourcegraph provider. Prior
	// to provider becoming an optional field, provider was required, so unset
	// provider is definitely with the understanding that the Sourcegraph
	// provider is the default. Since this is new, we also enforce that the new
	// CodyEnabled config is set (instead of relying on backcompat)
	if completionsConfig.Provider == "" {
		if !newCodyEnabled(siteConfig) {
			return nil
		}
		completionsConfig.Provider = string(conftypes.CompletionsProviderNameSourcegraph)
	}

	// If ChatModel is not set, fall back to the deprecated Model field.
	// Note: It might also be empty.
	if completionsConfig.ChatModel == "" {
		completionsConfig.ChatModel = completionsConfig.Model
	}

	if completionsConfig.Provider == string(conftypes.CompletionsProviderNameSourcegraph) {
		// If no endpoint is configured, use a default value.
		if completionsConfig.Endpoint == "" {
			completionsConfig.Endpoint = "https://cody-gateway.sourcegraph.com"
		}

		// Set the access token, either use the configured one, or generate one for the platform.
		completionsConfig.AccessToken = getSourcegraphProviderAccessToken(completionsConfig.AccessToken, siteConfig)
		// If we weren't able to generate an access token of some sort, authing with
		// Cody Gateway is not possible and we cannot use completions.
		if completionsConfig.AccessToken == "" {
			return nil
		}

		// Set a default chat model.
		if completionsConfig.ChatModel == "" {
			completionsConfig.ChatModel = "anthropic/claude-2.0"
		}

		// Set a default fast chat model.
		if completionsConfig.FastChatModel == "" {
			completionsConfig.FastChatModel = "anthropic/claude-instant-1"
		}

		// Set a default completions model.
		if completionsConfig.CompletionModel == "" {
			completionsConfig.CompletionModel = "anthropic/claude-instant-1"
		}
	} else if completionsConfig.Provider == string(conftypes.CompletionsProviderNameOpenAI) {
		// If no endpoint is configured, use a default value.
		if completionsConfig.Endpoint == "" {
			completionsConfig.Endpoint = "https://api.openai.com"
		}

		// If not access token is set, we cannot talk to OpenAI. Bail.
		if completionsConfig.AccessToken == "" {
			return nil
		}

		// Set a default chat model.
		if completionsConfig.ChatModel == "" {
			completionsConfig.ChatModel = "gpt-4"
		}

		// Set a default fast chat model.
		if completionsConfig.FastChatModel == "" {
			completionsConfig.FastChatModel = "gpt-3.5-turbo"
		}

		// Set a default completions model.
		if completionsConfig.CompletionModel == "" {
			completionsConfig.CompletionModel = "gpt-3.5-turbo-instruct"
		}
	} else if completionsConfig.Provider == string(conftypes.CompletionsProviderNameAnthropic) {
		// If no endpoint is configured, use a default value.
		if completionsConfig.Endpoint == "" {
			completionsConfig.Endpoint = "https://api.anthropic.com/v1/complete"
		}

		// If not access token is set, we cannot talk to Anthropic. Bail.
		if completionsConfig.AccessToken == "" {
			return nil
		}

		// Set a default chat model.
		if completionsConfig.ChatModel == "" {
			completionsConfig.ChatModel = "claude-2.0"
		}

		// Set a default fast chat model.
		if completionsConfig.FastChatModel == "" {
			completionsConfig.FastChatModel = "claude-instant-1"
		}

		// Set a default completions model.
		if completionsConfig.CompletionModel == "" {
			completionsConfig.CompletionModel = "claude-instant-1"
		}
	} else if completionsConfig.Provider == string(conftypes.CompletionsProviderNameAzureOpenAI) {
		// If no endpoint is configured, this provider is misconfigured.
		if completionsConfig.Endpoint == "" {
			return nil
		}

		// If not chat model is set, we cannot talk to Azure OpenAI. Bail.
		if completionsConfig.ChatModel == "" {
			return nil
		}

		// If not fast chat model is set, we fall back to the Chat Model.
		if completionsConfig.FastChatModel == "" {
			completionsConfig.FastChatModel = completionsConfig.ChatModel
		}

		// If not completions model is set, we cannot talk to Azure OpenAI. Bail.
		if completionsConfig.CompletionModel == "" {
			return nil
		}
	} else if completionsConfig.Provider == string(conftypes.CompletionsProviderNameFireworks) {
		// If no endpoint is configured, use a default value.
		if completionsConfig.Endpoint == "" {
			completionsConfig.Endpoint = "https://api.fireworks.ai/inference/v1/completions"
		}

		// If not access token is set, we cannot talk to Fireworks. Bail.
		if completionsConfig.AccessToken == "" {
			return nil
		}

		// Set a default chat model.
		if completionsConfig.ChatModel == "" {
			completionsConfig.ChatModel = "accounts/fireworks/models/llama-v2-7b"
		}

		// Set a default fast chat model.
		if completionsConfig.FastChatModel == "" {
			completionsConfig.FastChatModel = "accounts/fireworks/models/llama-v2-7b"
		}

		// Set a default completions model.
		if completionsConfig.CompletionModel == "" {
			// Use the virtual fireworks/starcoder model name as the default
			completionsConfig.CompletionModel = "starcoder"
		}
	} else if completionsConfig.Provider == string(conftypes.CompletionsProviderNameAWSBedrock) {
		// If no endpoint is configured, no default available.
		if completionsConfig.Endpoint == "" {
			return nil
		}

		// Set a default chat model.
		if completionsConfig.ChatModel == "" {
			completionsConfig.ChatModel = "anthropic.claude-v2"
		}

		// Set a default fast chat model.
		if completionsConfig.FastChatModel == "" {
			completionsConfig.FastChatModel = "anthropic.claude-instant-v1"
		}

		// Set a default completions model.
		if completionsConfig.CompletionModel == "" {
			completionsConfig.CompletionModel = "anthropic.claude-instant-v1"
		}
	}

	// Make sure models are always treated case-insensitive.
	completionsConfig.ChatModel = strings.ToLower(completionsConfig.ChatModel)
	completionsConfig.FastChatModel = strings.ToLower(completionsConfig.FastChatModel)
	completionsConfig.CompletionModel = strings.ToLower(completionsConfig.CompletionModel)

	// If after trying to set default we still have not all models configured, completions are
	// not available.
	if completionsConfig.ChatModel == "" || completionsConfig.FastChatModel == "" || completionsConfig.CompletionModel == "" {
		return nil
	}

	if completionsConfig.ChatModelMaxTokens == 0 {
		completionsConfig.ChatModelMaxTokens = defaultMaxPromptTokens(conftypes.CompletionsProviderName(completionsConfig.Provider), completionsConfig.ChatModel)
	}

	if completionsConfig.FastChatModelMaxTokens == 0 {
		completionsConfig.FastChatModelMaxTokens = defaultMaxPromptTokens(conftypes.CompletionsProviderName(completionsConfig.Provider), completionsConfig.FastChatModel)
	}

	if completionsConfig.CompletionModelMaxTokens == 0 {
		completionsConfig.CompletionModelMaxTokens = defaultMaxPromptTokens(conftypes.CompletionsProviderName(completionsConfig.Provider), completionsConfig.CompletionModel)
	}

	computedConfig := &conftypes.CompletionsConfig{
		Provider:                         conftypes.CompletionsProviderName(completionsConfig.Provider),
		AccessToken:                      completionsConfig.AccessToken,
		ChatModel:                        completionsConfig.ChatModel,
		ChatModelMaxTokens:               completionsConfig.ChatModelMaxTokens,
		FastChatModel:                    completionsConfig.FastChatModel,
		FastChatModelMaxTokens:           completionsConfig.FastChatModelMaxTokens,
		CompletionModel:                  completionsConfig.CompletionModel,
		CompletionModelMaxTokens:         completionsConfig.CompletionModelMaxTokens,
		Endpoint:                         completionsConfig.Endpoint,
		PerUserDailyLimit:                completionsConfig.PerUserDailyLimit,
		PerUserCodeCompletionsDailyLimit: completionsConfig.PerUserCodeCompletionsDailyLimit,
		PerCommunityUserChatMonthlyLLMRequestLimit:             completionsConfig.PerCommunityUserChatMonthlyLLMRequestLimit,
		PerCommunityUserCodeCompletionsMonthlyLLMRequestLimit:  completionsConfig.PerCommunityUserCodeCompletionsMonthlyLLMRequestLimit,
		PerProUserChatDailyLLMRequestLimit:                     completionsConfig.PerProUserChatDailyLLMRequestLimit,
		PerProUserCodeCompletionsDailyLLMRequestLimit:          completionsConfig.PerProUserCodeCompletionsDailyLLMRequestLimit,
		PerCommunityUserChatMonthlyInteractionLimit:            completionsConfig.PerCommunityUserChatMonthlyInteractionLimit,
		PerCommunityUserCodeCompletionsMonthlyInteractionLimit: completionsConfig.PerCommunityUserCodeCompletionsMonthlyInteractionLimit,
		PerProUserChatDailyInteractionLimit:                    completionsConfig.PerProUserChatDailyInteractionLimit,
		PerProUserCodeCompletionsDailyInteractionLimit:         completionsConfig.PerProUserCodeCompletionsDailyInteractionLimit,
	}

	return computedConfig
}

func GetConfigFeatures(siteConfig schema.SiteConfiguration) (c *conftypes.ConfigFeatures) {
	// If cody is disabled, don't use any of the other features.
	if !codyEnabled(siteConfig) {
		return nil
	}
	configFeatures := siteConfig.ConfigFeatures
	var attributionEnabled bool
	if enabled := siteConfig.AttributionEnabled; enabled != nil {
		attributionEnabled = *enabled
	}
	// If no features configuration is set at all, but cody is enabled, assume a default configuration
	// where all the features are enabled this is to handle edge cases where no config is set etc
	if configFeatures == nil {
		return &conftypes.ConfigFeatures{
			Chat:         true,
			AutoComplete: true,
			Commands:     true,
			Attribution:  attributionEnabled,
		}
	}

	computedConfig := &conftypes.ConfigFeatures{
		Chat:         configFeatures.Chat,
		AutoComplete: configFeatures.AutoComplete,
		Commands:     configFeatures.Commands,
		Attribution:  attributionEnabled,
	}
	return computedConfig
}

func GetAttributionGateway(siteConfig schema.SiteConfiguration) (string, string) {
	if !codyEnabled(siteConfig) {
		return "", ""
	}
	// Explicit attribution gateway config overrides autocomplete config (if used).
	if g := siteConfig.AttributionGateway; g != nil {
		return g.Endpoint, getSourcegraphProviderAccessToken(g.AccessToken, siteConfig)
	}
	// Fall back to autocomplete config if no explicit gateway config.
	cc := GetCompletionsConfig(siteConfig)
	ccUsingGateway := cc != nil && cc.Provider == conftypes.CompletionsProviderNameSourcegraph
	if ccUsingGateway {
		return cc.Endpoint, getSourcegraphProviderAccessToken(cc.AccessToken, siteConfig)
	}
	return "", ""
}

const embeddingsMaxFileSizeBytes = 1000000

// GetEmbeddingsConfig evaluates a complete embeddings configuration based on
// site configuration. The configuration may be nil if completions is disabled.
func GetEmbeddingsConfig(siteConfig schema.SiteConfiguration) *conftypes.EmbeddingsConfig {
	// If cody is disabled, don't use embeddings.
	if !codyEnabled(siteConfig) {
		return nil
	}

	// Only allow embeddings on dotcom
	if !dotcom.SourcegraphDotComMode() {
		return nil
	}

	// If embeddings are explicitly disabled (legacy flag, TODO: remove after 5.1),
	// don't use embeddings either.
	if siteConfig.Embeddings != nil && siteConfig.Embeddings.Enabled != nil && !*siteConfig.Embeddings.Enabled {
		return nil
	}

	embeddingsConfig := siteConfig.Embeddings
	// If no embeddings configuration is set at all, but cody is enabled, assume
	// a default configuration.
	if embeddingsConfig == nil {
		embeddingsConfig = &schema.Embeddings{
			Provider: string(conftypes.EmbeddingsProviderNameSourcegraph),
		}
	}

	// If after setting defaults for no `embeddings` config given there still is no
	// provider configured.
	// Before, this meant "use OpenAI", but it's easy to accidentally send Cody Gateway
	// auth tokens to OpenAI by that, so if an access token is explicitly set we
	// are careful and require the provider to be explicit. This lets us have good
	// support for optional Provider in most cases (token is generated for
	// default provider Sourcegraph)
	if embeddingsConfig.Provider == "" {
		if embeddingsConfig.AccessToken != "" {
			return nil
		}

		// Otherwise, assume Provider, since it is optional
		embeddingsConfig.Provider = string(conftypes.EmbeddingsProviderNameSourcegraph)
	}

	// The default value for incremental is true.
	if embeddingsConfig.Incremental == nil {
		embeddingsConfig.Incremental = pointers.Ptr(true)
	}

	// Set default values for max embeddings counts.
	embeddingsConfig.MaxCodeEmbeddingsPerRepo = defaultTo(embeddingsConfig.MaxCodeEmbeddingsPerRepo, defaultMaxCodeEmbeddingsPerRepo)
	embeddingsConfig.MaxTextEmbeddingsPerRepo = defaultTo(embeddingsConfig.MaxTextEmbeddingsPerRepo, defaultMaxTextEmbeddingsPerRepo)

	// The default value for MinimumInterval is 24h.
	if embeddingsConfig.MinimumInterval == "" {
		embeddingsConfig.MinimumInterval = defaultMinimumInterval.String()
	}

	// Set the default for PolicyRepositoryMatchLimit.
	if embeddingsConfig.PolicyRepositoryMatchLimit == nil {
		v := defaultPolicyRepositoryMatchLimit
		embeddingsConfig.PolicyRepositoryMatchLimit = &v
	}

	// If endpoint is not set, fall back to URL, it's the previous name of the setting.
	// Note: It might also be empty.
	if embeddingsConfig.Endpoint == "" {
		embeddingsConfig.Endpoint = embeddingsConfig.Url
	}

	if embeddingsConfig.Provider == string(conftypes.EmbeddingsProviderNameSourcegraph) {
		// If no endpoint is configured, use a default value.
		if embeddingsConfig.Endpoint == "" {
			embeddingsConfig.Endpoint = "https://cody-gateway.sourcegraph.com/v1/embeddings"
		}

		// Set the access token, either use the configured one, or generate one for the platform.
		embeddingsConfig.AccessToken = getSourcegraphProviderAccessToken(embeddingsConfig.AccessToken, siteConfig)
		// If we weren't able to generate an access token of some sort, authing with
		// Cody Gateway is not possible and we cannot use embeddings.
		if embeddingsConfig.AccessToken == "" {
			return nil
		}

		// Set a default model.
		if embeddingsConfig.Model == "" {
			embeddingsConfig.Model = "openai/text-embedding-ada-002"
		}
		// Make sure models are always treated case-insensitive.
		embeddingsConfig.Model = strings.ToLower(embeddingsConfig.Model)

		// Set a default for model dimensions if using the default model.
		if embeddingsConfig.Dimensions <= 0 && embeddingsConfig.Model == "openai/text-embedding-ada-002" {
			embeddingsConfig.Dimensions = 1536
		}
	} else if embeddingsConfig.Provider == string(conftypes.EmbeddingsProviderNameOpenAI) {
		// If no endpoint is configured, use a default value.
		if embeddingsConfig.Endpoint == "" {
			embeddingsConfig.Endpoint = "https://api.openai.com/v1/embeddings"
		}

		// If not access token is set, we cannot talk to OpenAI. Bail.
		if embeddingsConfig.AccessToken == "" {
			return nil
		}

		// Set a default model.
		if embeddingsConfig.Model == "" {
			embeddingsConfig.Model = "text-embedding-ada-002"
		}
		// Make sure models are always treated case-insensitive.
		embeddingsConfig.Model = strings.ToLower(embeddingsConfig.Model)

		// Set a default for model dimensions if using the default model.
		if embeddingsConfig.Dimensions <= 0 && embeddingsConfig.Model == "text-embedding-ada-002" {
			embeddingsConfig.Dimensions = 1536
		}
	} else if embeddingsConfig.Provider == string(conftypes.EmbeddingsProviderNameAzureOpenAI) {
		// If no endpoint is configured, we cannot talk to Azure OpenAI.
		if embeddingsConfig.Endpoint == "" {
			return nil
		}

		// If no model is set, we cannot do anything here.
		if embeddingsConfig.Model == "" {
			return nil
		}
		// Make sure models are always treated case-insensitive.
		// TODO: Are model names on azure case insensitive?
		embeddingsConfig.Model = strings.ToLower(embeddingsConfig.Model)
	} else {
		// Unknown provider value.
		return nil
	}

	// While its not removed, use both options
	var includedFilePathPatterns []string
	excludedFilePathPatterns := embeddingsConfig.ExcludedFilePathPatterns
	maxFileSizeLimit := embeddingsMaxFileSizeBytes
	if embeddingsConfig.FileFilters != nil {
		includedFilePathPatterns = embeddingsConfig.FileFilters.IncludedFilePathPatterns
		excludedFilePathPatterns = append(excludedFilePathPatterns, embeddingsConfig.FileFilters.ExcludedFilePathPatterns...)
		if embeddingsConfig.FileFilters.MaxFileSizeBytes > 0 && embeddingsConfig.FileFilters.MaxFileSizeBytes <= embeddingsMaxFileSizeBytes {
			maxFileSizeLimit = embeddingsConfig.FileFilters.MaxFileSizeBytes
		}
	}
	fileFilters := conftypes.EmbeddingsFileFilters{
		IncludedFilePathPatterns: includedFilePathPatterns,
		ExcludedFilePathPatterns: excludedFilePathPatterns,
		MaxFileSizeBytes:         maxFileSizeLimit,
	}

	// Default values should match the documented defaults in site.schema.json.
	computedQdrantConfig := conftypes.QdrantConfig{
		Enabled: false,
		QdrantHNSWConfig: conftypes.QdrantHNSWConfig{
			EfConstruct:       nil,
			FullScanThreshold: nil,
			M:                 nil,
			OnDisk:            true,
			PayloadM:          nil,
		},
		QdrantOptimizersConfig: conftypes.QdrantOptimizersConfig{
			IndexingThreshold: 0,
			MemmapThreshold:   100,
		},
		QdrantQuantizationConfig: conftypes.QdrantQuantizationConfig{
			Enabled:  true,
			Quantile: 0.98,
		},
	}
	if embeddingsConfig.Qdrant != nil {
		qc := embeddingsConfig.Qdrant
		computedQdrantConfig.Enabled = qc.Enabled

		if qc.Hnsw != nil {
			computedQdrantConfig.QdrantHNSWConfig.EfConstruct = toUint64(qc.Hnsw.EfConstruct)
			computedQdrantConfig.QdrantHNSWConfig.FullScanThreshold = toUint64(qc.Hnsw.FullScanThreshold)
			computedQdrantConfig.QdrantHNSWConfig.M = toUint64(qc.Hnsw.M)
			computedQdrantConfig.QdrantHNSWConfig.PayloadM = toUint64(qc.Hnsw.PayloadM)
			if qc.Hnsw.OnDisk != nil {
				computedQdrantConfig.QdrantHNSWConfig.OnDisk = *qc.Hnsw.OnDisk
			}
		}

		if qc.Optimizers != nil {
			if qc.Optimizers.IndexingThreshold != nil {
				computedQdrantConfig.QdrantOptimizersConfig.IndexingThreshold = uint64(*qc.Optimizers.IndexingThreshold)
			}
			if qc.Optimizers.MemmapThreshold != nil {
				computedQdrantConfig.QdrantOptimizersConfig.MemmapThreshold = uint64(*qc.Optimizers.MemmapThreshold)
			}
		}

		if qc.Quantization != nil {
			if qc.Quantization.Enabled != nil {
				computedQdrantConfig.QdrantQuantizationConfig.Enabled = *qc.Quantization.Enabled
			}

			if qc.Quantization.Quantile != nil {
				computedQdrantConfig.QdrantQuantizationConfig.Quantile = float32(*qc.Quantization.Quantile)
			}
		}
	}

	computedConfig := &conftypes.EmbeddingsConfig{
		Provider:    conftypes.EmbeddingsProviderName(embeddingsConfig.Provider),
		AccessToken: embeddingsConfig.AccessToken,
		Model:       embeddingsConfig.Model,
		Endpoint:    embeddingsConfig.Endpoint,
		Dimensions:  embeddingsConfig.Dimensions,
		// This is definitely set at this point.
		Incremental:                            *embeddingsConfig.Incremental,
		FileFilters:                            fileFilters,
		MaxCodeEmbeddingsPerRepo:               embeddingsConfig.MaxCodeEmbeddingsPerRepo,
		MaxTextEmbeddingsPerRepo:               embeddingsConfig.MaxTextEmbeddingsPerRepo,
		PolicyRepositoryMatchLimit:             embeddingsConfig.PolicyRepositoryMatchLimit,
		ExcludeChunkOnError:                    pointers.Deref(embeddingsConfig.ExcludeChunkOnError, true),
		Qdrant:                                 computedQdrantConfig,
		PerCommunityUserEmbeddingsMonthlyLimit: embeddingsConfig.PerCommunityUserEmbeddingsMonthlyLimit,
		PerProUserEmbeddingsMonthlyLimit:       embeddingsConfig.PerProUserEmbeddingsMonthlyLimit,
	}
	d, err := time.ParseDuration(embeddingsConfig.MinimumInterval)
	if err != nil {
		computedConfig.MinimumInterval = defaultMinimumInterval
	} else {
		computedConfig.MinimumInterval = d
	}

	return computedConfig
}

func toUint64(input *int) *uint64 {
	if input == nil {
		return nil
	}
	u := uint64(*input)
	return &u
}

func getSourcegraphProviderAccessToken(accessToken string, config schema.SiteConfiguration) string {
	// If an access token is configured, use it.
	if accessToken != "" {
		return accessToken
	}

	// Otherwise, use the current license key to compute an access token.
	if config.LicenseKey == "" {
		return ""
	}

	return license.GenerateLicenseKeyBasedAccessToken(config.LicenseKey)
}

const (
	defaultPolicyRepositoryMatchLimit = 5000
	defaultMinimumInterval            = 24 * time.Hour
	defaultMaxCodeEmbeddingsPerRepo   = 3_072_000
	defaultMaxTextEmbeddingsPerRepo   = 512_000
)

func defaultTo(val, def int) int {
	if val == 0 {
		return def
	}
	return val
}

func defaultMaxPromptTokens(provider conftypes.CompletionsProviderName, model string) int {
	switch provider {
	case conftypes.CompletionsProviderNameSourcegraph:
		if strings.HasPrefix(model, "openai/") {
			return openaiDefaultMaxPromptTokens(strings.TrimPrefix(model, "openai/"))
		}
		if strings.HasPrefix(model, "anthropic/") {
			return anthropicDefaultMaxPromptTokens(strings.TrimPrefix(model, "anthropic/"))
		}
		// Fallback for weird values.
		return 9_000
	case conftypes.CompletionsProviderNameAnthropic:
		return anthropicDefaultMaxPromptTokens(model)
	case conftypes.CompletionsProviderNameOpenAI:
		return openaiDefaultMaxPromptTokens(model)
	case conftypes.CompletionsProviderNameFireworks:
		return fireworksDefaultMaxPromptTokens(model)
	case conftypes.CompletionsProviderNameAzureOpenAI:
		// We cannot know based on the model name what model is actually used,
		// this is a sane default for GPT in general.
		return 7_000
	case conftypes.CompletionsProviderNameAWSBedrock:
		if strings.HasPrefix(model, "anthropic.") {
			return anthropicDefaultMaxPromptTokens(strings.TrimPrefix(model, "anthropic."))
		}
		// Fallback for weird values.
		return 9_000
	}

	// Should be unreachable.
	return 9_000
}

func anthropicDefaultMaxPromptTokens(model string) int {
	if strings.HasSuffix(model, "-100k") {
		return 100_000

	}
	if model == "claude-2" || model == "claude-2.0" || model == "claude-2.1" || model == "claude-v2" {
		// TODO: Technically, v2 also uses a 100k window, but we should validate
		// that returning 100k here is the right thing to do.
		return 12_000
	}
	// For now, all other claude models have a 9k token window.
	return 9_000
}

func openaiDefaultMaxPromptTokens(model string) int {
	switch model {
	case "gpt-4":
		return 7_000
	case "gpt-4-32k":
		return 32_000
	case "gpt-3.5-turbo", "gpt-3.5-turbo-instruct", "gpt-4-1106-preview":
		return 4_000
	case "gpt-3.5-turbo-16k":
		return 16_000
	default:
		return 4_000
	}
}

func fireworksDefaultMaxPromptTokens(model string) int {
	if strings.HasPrefix(model, "accounts/fireworks/models/llama-v2") {
		// Llama 2 has a context window of 4000 tokens
		return 3_000
	}

	if strings.HasPrefix(model, "accounts/fireworks/models/starcoder-") || strings.HasPrefix(model, "starcoder") {
		// StarCoder has a context window of 8192 tokens
		return 6_000
	}

	return 4_000
}

// RepoListUpdateInterval returns the repository list update interval.
//
// If the RepoListUpdateInterval site configuration setting is 0, it defaults to 1 minute.
func RepoListUpdateInterval() time.Duration {
	v := Get().RepoListUpdateInterval
	if v == 0 { //  default to 1 minute
		v = 1
	}
	return time.Duration(v) * time.Minute
}

// RepoConcurrentExternalServiceSyncers returns the number of concurrent external service syncers.
//
// If the RepoConcurrentExternalServiceSyncers site configuration setting is 0, it defaults to 3.
func RepoConcurrentExternalServiceSyncers() int {
	v := Get().RepoConcurrentExternalServiceSyncers
	if v <= 0 {
		return 3
	}
	return v
}

// PermissionsUserMapping returns the last valid value of permissions user mapping in the site configuration.
// Callers must not mutate the returned pointer.
func PermissionsUserMapping() *schema.PermissionsUserMapping {
	c := Get().PermissionsUserMapping
	if c == nil {
		return &schema.PermissionsUserMapping{Enabled: false, BindID: "email"}
	}
	// Invalid config.
	if c.BindID != "email" && c.BindID != "username" {
		return &schema.PermissionsUserMapping{Enabled: false, BindID: "email"}
	}
	return c
}
