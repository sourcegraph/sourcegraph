package conftypes

import "time"

type CompletionsConfig struct {
	ChatModel          string
	ChatModelMaxTokens int

	SmartContextWindow     string
	DisableClientConfigAPI bool

	FastChatModel          string
	FastChatModelMaxTokens int

	CompletionModel          string
	CompletionModelMaxTokens int

	AzureCompletionModel                         string
	AzureChatModel                               string
	AzureUseDeprecatedCompletionsAPIForOldModels bool

	AccessToken                                            string
	Provider                                               CompletionsProviderName
	Endpoint                                               string
	PerUserDailyLimit                                      int
	PerUserCodeCompletionsDailyLimit                       int
	PerCommunityUserChatMonthlyLLMRequestLimit             int
	PerCommunityUserCodeCompletionsMonthlyLLMRequestLimit  int
	PerProUserChatDailyLLMRequestLimit                     int
	PerProUserCodeCompletionsDailyLLMRequestLimit          int
	PerCommunityUserChatMonthlyInteractionLimit            int
	PerCommunityUserCodeCompletionsMonthlyInteractionLimit int
	PerProUserChatDailyInteractionLimit                    int
	PerProUserCodeCompletionsDailyInteractionLimit         int
	User                                                   string
}

type ConfigFeatures struct {
	Chat         bool
	AutoComplete bool
	Commands     bool
	Attribution  bool
}

type CompletionsProviderName string

const (
	CompletionsProviderNameAnthropic   CompletionsProviderName = "anthropic"
	CompletionsProviderNameOpenAI      CompletionsProviderName = "openai"
	CompletionsProviderNameGoogle      CompletionsProviderName = "google"
	CompletionsProviderNameAzureOpenAI CompletionsProviderName = "azure-openai"
	CompletionsProviderNameSourcegraph CompletionsProviderName = "sourcegraph"
	CompletionsProviderNameFireworks   CompletionsProviderName = "fireworks"
	CompletionsProviderNameAWSBedrock  CompletionsProviderName = "aws-bedrock"
)

type EmbeddingsConfig struct {
	Provider                               EmbeddingsProviderName
	AccessToken                            string
	Model                                  string
	Endpoint                               string
	Dimensions                             int
	Incremental                            bool
	MinimumInterval                        time.Duration
	FileFilters                            EmbeddingsFileFilters
	MaxCodeEmbeddingsPerRepo               int
	MaxTextEmbeddingsPerRepo               int
	PolicyRepositoryMatchLimit             *int
	ExcludeChunkOnError                    bool
	PerCommunityUserEmbeddingsMonthlyLimit int
	PerProUserEmbeddingsMonthlyLimit       int
}

type EmbeddingsProviderName string

const (
	EmbeddingsProviderNameOpenAI      EmbeddingsProviderName = "openai"
	EmbeddingsProviderNameAzureOpenAI EmbeddingsProviderName = "azure-openai"
	EmbeddingsProviderNameSourcegraph EmbeddingsProviderName = "sourcegraph"
)

type EmbeddingsFileFilters struct {
	IncludedFilePathPatterns []string
	ExcludedFilePathPatterns []string
	MaxFileSizeBytes         int
}
