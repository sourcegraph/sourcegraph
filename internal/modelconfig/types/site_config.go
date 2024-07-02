package types

type ModelFilters struct {
	// StatusFilter will constrain LLM models to just those supplied in this slice.
	// e.g. "stable", "beta" will mean that all "experimental" models will be omitted.
	StatusFilter []string `json:"statusFilter"`

	// If provided, only the models matching the supplied ModelReferences will be made
	// available. Wildcards are allowed, but this does not support arbitrary regular
	// expressions.
	//
	// e.g. [ "anthropic::*", "openai::2024-02-01::*" ] would include all Anthropic-
	// supplied models and models using a particular API Version of OpenAI.
	Allow []string `json:"allow"`
	// Deny will remove any models whose ModelReference matches the supplied string.
	// e.g. [ "*gpt*" ] will remove any LLMs that have "gpt" anywhere in the mref.
	Deny []string `json:"deny"`
}

// SourcegraphModelConfig is how we represent the configuration of Sourcegraph-supplied
// LLM models to the Sourcegraph instance.
type SourcegraphModelConfig struct {

	// PollingInterval is the frequency by which this instance should poll Cody Gateway
	// for an updated list of LLM models. e.g. "6h" or "1d". Or "never" to disable this
	// capability entirely.
	PollingInterval *string `json:"pollingInterval"`
	// Endpoint is the Cody Gateway URL that will be polled in order to pick up new
	// LLM models as they are released..
	Endpoint *string `json:"endpoint"`

	// ModelFilters provide a way for the Sourcegraph admin to constrain the set of
	// LLM models made available, e.g. to only "stable" models. Or those from
	// particular providers.
	ModelFilters *ModelFilters `json:"modelFilters"`
}

// DefaultModelConfig is the model configuration that is applied to every LLM model
// for a given provider. This allows Sourcegraph admins to set common configuration
// settings once.
type DefaultModelConfig struct {
	// The fields here are a subset of those defined on `Model`

	Capabilities []ModelCapability `json:"capabilities"`
	Category     ModelCategory     `json:"category"`
	Status       ModelStatus       `json:"status"`
	Tier         ModelTier         `json:"tier"`

	ContextWindow ContextWindow `json:"contextWindow"`

	ClientSideConfig *ClientSideModelConfig `json:"clientSideConfig,omitempty"`
	ServerSideConfig *ServerSideModelConfig `json:"serverSideConfig,omitempty"`
}

// ProviderOverride is how a Sourcegraph admin would describe a `Provider` within
// the site-configuration.
type ProviderOverride struct {
	ID          ProviderID `json:"id"`
	DisplayName string     `json:"displayName"`

	ClientSideConfig *ClientSideProviderConfig `json:"clientSideConfig,omitempty"`
	ServerSideConfig *ServerSideProviderConfig `json:"serverSideConfig,omitempty"`

	DefaultModelConfig *DefaultModelConfig `json:"defaultModelConfig"`
}

// ModelOverride is how a Sourcegraph admin would describe a `Model` with the
// the site-configuration. This will either overwrite the existing set of model
// fields, or add an entirely new model.
type ModelOverride struct {
	ModelRef ModelRef `json:"modelRef"`

	DisplayName string `json:"displayName"`
	ModelName   string `json:"string"`

	Capabilities []ModelCapability `json:"capabilities"`
	Category     ModelCategory     `json:"category"`
	Status       ModelStatus       `json:"status"`
	Tier         ModelTier         `json:"tier"`

	ContextWindow ContextWindow `json:"contextWindow"`

	ClientSideConfig *ClientSideModelConfig `json:"clientSideConfig,omitempty"`
	ServerSideConfig *ServerSideModelConfig `json:"serverSideConfig,omitempty"`
}
