package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
)

// getValidModel returns a valid types.Model.
func getValidModel() types.Model {
	return types.Model{
		ModelRef:    types.ModelRef("alpha::beta::gamma"),
		DisplayName: "Test model for reerence",
		ModelName:   "Test model name",

		Capabilities: []types.ModelCapability{types.ModelCapabilityAutocomplete},
		Category:     types.ModelCategoryAccuracy,
		Status:       types.ModelStatusBeta,
		Tier:         types.ModelTierEnterprise,

		ContextWindow: types.ContextWindow{
			MaxInputTokens:  1111,
			MaxOutputTokens: 2222,
		},

		ClientSideConfig: &types.ClientSideModelConfig{},
		ServerSideConfig: &types.ServerSideModelConfig{
			AWSBedrockProvisionedThroughput: &types.AWSBedrockProvisionedThroughput{
				ARN: "arn from the test model config",
			},
		},
	}
}

func TestApplyModelOverrides(t *testing.T) {
	// Keep a reference of the stock "valid model" to compare
	// against, so we know if fields weren't changed.
	unchangedModel := getValidModel()

	t.Run("Basic", func(t *testing.T) {
		mod := getValidModel()
		err := ApplyModelOverride(&mod, types.ModelOverride{
			DisplayName: "override-display-name",
			ModelName:   "override-model-name",

			ContextWindow: types.ContextWindow{
				MaxInputTokens: 100_000,
				// Do not set MaxOutputTokens, ensure that value won't be changed.
			},
		})
		require.NoError(t, err)

		assert.EqualValues(t, "override-display-name", mod.DisplayName)
		assert.EqualValues(t, "override-model-name", mod.ModelName)

		assert.Equal(t, 100_000, mod.ContextWindow.MaxInputTokens)
		assert.Equal(t, unchangedModel.ContextWindow.MaxOutputTokens, mod.ContextWindow.MaxOutputTokens)
	})

	t.Run("Errors", func(t *testing.T) {
		noModErr := ApplyModelOverride(nil, types.ModelOverride{})
		assert.ErrorContains(t, noModErr, "no model provided")

		startingMod := getValidModel()
		diffModRefErr := ApplyModelOverride(&startingMod, types.ModelOverride{
			ModelRef: types.ModelRef("anything else"),
		})
		assert.ErrorContains(t, diffModRefErr, "cannot change the model's identity")
	})
}

func TestApplyDefaultModelConfig(t *testing.T) {
	// Keep a reference of the stock "valid model" to compare
	// against, so we know if fields weren't changed.
	unchangedModel := getValidModel()

	t.Run("Basic", func(t *testing.T) {
		mod := getValidModel()
		err := ApplyDefaultModelConfiguration(&mod, &types.DefaultModelConfig{
			Tier:   types.ModelTierEnterprise,
			Status: types.ModelStatusDeprecated,
			ContextWindow: types.ContextWindow{
				MaxInputTokens: 100_000,
				// Do not set MaxOutputTokens, ensure that value won't be changed.
			},
		})
		require.NoError(t, err)

		assert.EqualValues(t, types.ModelTierEnterprise, mod.Tier)
		assert.EqualValues(t, types.ModelStatusDeprecated, mod.Status)

		assert.Equal(t, 100_000, mod.ContextWindow.MaxInputTokens)
		assert.Equal(t, unchangedModel.ContextWindow.MaxOutputTokens, mod.ContextWindow.MaxOutputTokens)
	})
}
