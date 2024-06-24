package conftypes

type CompletionsConfig struct {
	ChatModel          string
	ChatModelMaxTokens int

	SmartContextWindow string

	FastChatModel          string
	FastChatModelMaxTokens int

	CompletionModel          string
	CompletionModelMaxTokens int

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
