package types

type ContextWindow struct {
	MaxInputTokens  int `json:"maxInputTokens"`
	MaxOutputTokens int `json:"maxOutputTokens"`
	// TODO(chrsmith): Provide "Smart Context", and allocating
	// context window space to "user", etc.
}

type ModelCapability string

const (
	ModelCapabilityAutocomplete ModelCapability = "autocomplete"
	ModelCapabilityChat         ModelCapability = "chat"
)

type ModelStatus string

const (
	ModelStatusExperimental ModelStatus = "experimental"
	ModelStatusBeta         ModelStatus = "beta"
	ModelStatusStable       ModelStatus = "stable"
	ModelStatusDeprecated   ModelStatus = "deprecated"
)

type ModelTier string

const (
	ModelTierFree       ModelTier = "free"
	ModelTierPro        ModelTier = "pro"
	ModelTierEnterprise ModelTier = "enterprise"
)

type ModelCategory string

const (
	ModelCategoryAccuracy ModelCategory = "accuracy"
	ModelCategoryBalanced ModelCategory = "balanced"
	ModelCategorySpeed    ModelCategory = "speed"
)

type Model struct {
	ModelRef ModelRef `json:"modelRef"`

	// DisplayName is an optional user-friendly name (max 128 chars).
	// If unset, clients should just display the ModelID portion of the ModelRef.
	// (And NOT the ModelName!)
	DisplayName string `json:"displayName"`

	// ModelName is the opaque name of the model to identify it within the API Provider.
	// This will usually be identical to the ModelRef.ModelID(), but can differ.
	// e.g. the ModelRef may be "openai::...::gpt-4o", but the ModeName may be more
	// specific like "gpt-4o-2024-05-13".
	ModelName string `json:"modelName"`

	Capabilities []ModelCapability `json:"capabilities"`
	Category     ModelCategory     `json:"category"`
	Status       ModelStatus       `json:"status"`
	Tier         ModelTier         `json:"tier"`

	ContextWindow ContextWindow `json:"contextWindow"`

	ClientSideConfig *ClientSideModelConfig `json:"clientSideConfig,omitempty"`
	ServerSideConfig *ServerSideModelConfig `json:"serverSideConfig,omitempty"`
}
