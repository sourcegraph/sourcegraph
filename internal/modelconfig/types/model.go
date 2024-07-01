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
	// BUG? Why don't we just remove this since ID is already encoded in the ModelRef
	// So it's redundant?
	ID       ModelID  `json:"modelId"`
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
