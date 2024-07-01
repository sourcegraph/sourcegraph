package modelconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
)

func TestRedactServerSideConfig(t *testing.T) {
	testCfg := types.ModelConfiguration{
		Providers: []types.Provider{
			{
				ID: "test-provider-1",
				ServerSideConfig: &types.ServerSideProviderConfig{
					AWSBedrock: &types.AWSBedrockProviderConfig{
						AccessToken: "top-secret",
					},
				},
			},
			{
				ID: "test-provider-2",
				ServerSideConfig: &types.ServerSideProviderConfig{
					AzureOpenAI: &types.AzureOpenAIProviderConfig{
						AccessToken: "top-secret",
					},
				},
			},
		},
		Models: []types.Model{
			{
				ServerSideConfig: &types.ServerSideModelConfig{
					AWSBedrockProvisionedCapacity: &types.AwsBedrockProvisionedCapacity{
						ARN: "secret-arn",
					},
				},
			},
		},
	}

	RedactServerSideConfig(&testCfg)

	for _, provider := range testCfg.Providers {
		assert.Nil(t, provider.ServerSideConfig)
	}
	for _, model := range testCfg.Models {
		assert.Nil(t, model.ServerSideConfig)
	}
}
