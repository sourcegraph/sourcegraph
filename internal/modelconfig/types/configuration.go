package types

// ========================================================
// Client-side Provider Configuration Data
// ========================================================

type ClientSideProviderConfig struct {
	// We currently do not have any known client-side provider configuration.
	// But later, if anything needs to be provided to Cody clients at the
	// provider-level it will go here.
}

// ========================================================
// Server-side Provider Configuration Data
// ========================================================

// https://sourcegraph.com/docs/cody/clients/enable-cody-enterprise#use-amazon-bedrock-aws
type AWSBedrockProviderConfig struct {
	// Access token encodes your AWS credentials one one of several ways:
	// - Leave it empty and rely on instance role bindings or other AWS configurations in the frontend service
	// - Set it to <ACCESS_KEY_ID>:<SECRET_ACCESS_KEY> if directly configuring the credentials
	// - Set it to <ACCESS_KEY_ID>:<SECRET_ACCESS_KEY>:<SESSION_TOKEN> if a session token is also required.
	AccessToken string `json:"accessToken"`
	// - For Pay-as-you-go, set it to an AWS region code (e.g., us-west-2) when using a public Amazon Bedrock endpoint
	// - For Provisioned Throughput, set it to the provisioned VPC endpoint for the bedrock-runtime API (e.g., "https://vpce-0a10b2345cd67e89f-abc0defg.bedrock-runtime.us-west-2.vpce.amazonaws.com")
	Endpoint string `json:"endpoint"`
	// Region to use when configuring API clients. (Since the `frontend` binary's container won't
	// be able to pick this up from the host OS's environment variables.)
	Region string `json:"region"`
}

// https://sourcegraph.com/docs/cody/clients/enable-cody-enterprise#use-azure-openai-service
type AzureOpenAIProviderConfig struct {
	// - As of 5.2.4 the access token can be left empty and it will rely on Environmental, Workload Identity or Managed Identity credentials configured for the frontend and worker services
	// - Set it to <API_KEY> if directly configuring the credentials using the API key specified in the Azure portal
	AccessToken string `json:"accessToken"`
	// From the Azure OpenAI Service portal.
	Endpoint string `json:"endpoint"`
}

// GenericProvider is the generic format that older Sourcegraph instances used.
//
// Try to avoid using this where possible, and instead rely on the more specific
// types like `AzureOpenAIProviderConfig`. Even if they only contain the same fields.
// This allows us to more gracefully migrate customers forward as the API providers
// and associated API clients evolve. e.g. giving us the possibility of providing
// defaults for any missing configuration settings that get added later.
type GenericProviderConfig struct {
	AccessToken string `json:"accessToken"`
	Endpoint    string `json:"endpoint"`
}

// SourcegraphProviderConfig is the configuration blog for configuring a provider
// to be use Sourcegraph's Cody Gateway for requests.
type SourcegraphProviderConfig struct {
	AccessToken string `json:"accessToken"`
	Endpoint    string `json:"endpoint"`
}

// The "Provider" is conceptually a namespace for models. The server-side provider configuration
// is needed to describe the API endpoint needed to serve its models.
type ServerSideProviderConfig struct {
	AWSBedrock          *AWSBedrockProviderConfig  `json:"awsBedrock,omitempty"`
	AzureOpenAI         *AzureOpenAIProviderConfig `json:"azureOpenAi,omitempty"`
	GenericProvider     *GenericProviderConfig     `json:"genericProvider,omitempty"`
	SourcegraphProvider *SourcegraphProviderConfig `json:"sourcegraphProvider,omitempty"`
}

// ========================================================
// Client-side Model Configuration Data
// ========================================================

type ClientSideModelConfig struct {
	// We currently do not have any known client-side model configuration.
	// But later, if anything needs to be provided to Cody clients at the
	// model-level it will go here.
	//
	// For example, allowing the server to customize/override the LLM
	// prompt used. Or describe how clients should upload context to
	// remote servers, etc. Or "hints", like "this model is great when
	// working with 'C' code.".
}

// ========================================================
// Server-side Model Configuration Data
// ========================================================

// https://sourcegraph.com/docs/cody/clients/enable-cody-enterprise#use-amazon-bedrock-aws
// https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html#prov-throughput-models
// https://docs.aws.amazon.com/bedrock/latest/userguide/prov-throughput.html
type AWSBedrockProvisionedThroughput struct {
	// ARN is the "provisioned throughput ARN" to use when sending requests to AWS Bedrock
	// for a given model.
	ARN string `json:"arn"`
}

type ServerSideModelConfig struct {
	AWSBedrockProvisionedThroughput *AWSBedrockProvisionedThroughput `json:"awsBedrockProvisionedThroughput"`
}
