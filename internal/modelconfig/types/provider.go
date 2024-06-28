package types

type Provider struct {
	ID ProviderID `json:"id"`

	// DisplayName is a user-friendly name for the provider. Optional.
	// If unset, clients should fall back to just displaying the ID.
	// Restricted to be < 128 characters.
	DisplayName string `json:"displayName"`

	ClientSideConfig *ClientSideProviderConfig `json:"clientSideConfig,omitempty"`
	ServerSideConfig *ServerSideProviderConfig `json:"serverSideConfig,omitempty"`
}
