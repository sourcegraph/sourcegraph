package main

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
)

func mRef(providerApiVerRef, modelID string) types.ModelRef {
	raw := fmt.Sprintf("%s::%s", providerApiVerRef, modelID)
	return types.ModelRef(raw)
}

func newProvider(id, name string) types.Provider {
	return types.Provider{
		ID:          types.ProviderID(id),
		DisplayName: name,
	}
}

type modelIdentity struct {
	MRef        types.ModelRef
	Name        string
	DisplayName string
}

type modelMetadata struct {
	Capabilities []types.ModelCapability
	Category     types.ModelCategory
	Status       types.ModelStatus
	Tier         types.ModelTier
}

func newModel(identity modelIdentity, metadata modelMetadata, ctxWindow types.ContextWindow) types.Model {
	return types.Model{
		ModelRef:    identity.MRef,
		DisplayName: identity.DisplayName,
		ModelName:   identity.Name,

		Capabilities: metadata.Capabilities,
		Category:     metadata.Category,
		Status:       metadata.Status,
		Tier:         metadata.Tier,

		ContextWindow: ctxWindow,
	}
}
