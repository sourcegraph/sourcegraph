package modelconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/embedded"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Tests for various corner cases and regressions.
func TestValidationMethods(t *testing.T) {
	t.Run("ValidateModelRef", func(t *testing.T) {
		tests := []struct {
			MRef string
			// WantErr is the text of the expected error message.
			// If empty, we expect no error.
			WantErr string
		}{
			// Valid MRefs.
			{"foo::bar::baz", ""},
			{"foo-dashed::bar-dashed::baz-dashed-twice", ""},
			{"foo.dotted::bar.dotted::baz.dotted.twice", ""},
			{"provider_id::api_id::model_id", ""},

			{"provider::api-version-can-totally/have-slashes::model", ""},

			{"anthropic::2023-06-01::claude-3.5-sonnet", ""},

			// Expected failure with older-style model references.
			{"claude-2", "modelRef syntax error"},
			{"anthropic/claude-2", "modelRef syntax error"},

			// Generic validation errors.
			{"a::b::c::d", "modelRef syntax error"},

			{"provider/v1::api-version::model", "invalid ProviderID"},
			{"CAPS_PROVIDER::v1::model", "invalid ProviderID"},
			{"g o o g l e::v1::gemini-1.5", "invalid ProviderID"},

			{"foo::name-with!exclamatnions-should::be-ok", "invalid APIVersionID"},
			{"google::version one::gemini-1.5", "invalid APIVersionID"},

			{"provider::api-version::model/v1", "invalid ModelID"},
			{"provider::apiver::CAPS_MODEL", "invalid ModelID"},
			{"anthropic::2023-01-01::claude instant", "invalid ModelID"},
			{"google::v1::Gemini 1.5", "invalid ModelID"},
		}

		for _, test := range tests {
			ref := types.ModelRef(test.MRef)
			gotErr := validateModelRef(ref)

			var gotErrText string
			if gotErr != nil {
				gotErrText = gotErr.Error()
			}

			assert.Equal(
				t, test.WantErr, gotErrText,
				"didn't get expected validation error for mref %q", test.MRef)
		}
	})
}

// Confirm that the model data currently in the repo is well-formed and valid.
func TestEmbeddedModelConfig(t *testing.T) {
	loadModelConfig := func(t *testing.T) types.ModelConfiguration {
		t.Helper()
		cfg, err := embedded.GetCodyGatewayModelConfig()
		require.NoError(t, err)
		return *cfg
	}

	t.Run("Loads", func(t *testing.T) {
		_ = loadModelConfig(t)
	})

	t.Run("IsValid", func(t *testing.T) {
		cfg := loadModelConfig(t)

		t.Run("Metadata", func(t *testing.T) {
			assert.Equal(t, types.CurrentModelSchemaVersion, cfg.SchemaVersion)
			assert.NotEmpty(t, cfg.Revision)
		})

		t.Run("Providers", func(t *testing.T) {
			for _, provider := range cfg.Providers {
				verr := validateProvider(provider)
				assert.NoError(t, verr, "Provider %q", provider.ID)
			}
		})

		t.Run("Models", func(t *testing.T) {
			for _, model := range cfg.Models {
				verr := validateModel(model)
				assert.NoError(t, verr, "Model %q", model.ModelRef)

				// Verify the model is referencing a known provider.
				var forKnownProvider bool
				for _, provider := range cfg.Providers {
					if strings.HasPrefix(string(model.ModelRef), string(provider.ID)+"::") {
						forKnownProvider = true
						break
					}
				}
				assert.True(t, forKnownProvider, "Model %q", model.ModelRef)
			}
		})

		t.Run("DefaultModels", func(t *testing.T) {
			isKnownModel := func(mref types.ModelRef) bool {
				for _, model := range cfg.Models {
					if model.ModelRef == mref {
						return true
					}
				}
				return false
			}

			assert.True(t, isKnownModel(cfg.DefaultModels.Chat))
			assert.True(t, isKnownModel(cfg.DefaultModels.CodeCompletion))
			assert.True(t, isKnownModel(cfg.DefaultModels.FastChat))
		})
	})

	t.Run("ValidationFn", func(t *testing.T) {
		cfg := loadModelConfig(t)

		// All of the validation checks in this testsuite should be redundant with
		// the "official" validation function exported from the package.
		err := ValidateModelConfig(&cfg)
		assert.NoError(t, err)
	})
}

func TestValidateSiteConfig(t *testing.T) {
	// getValidSiteConfiguration returns a sophisticated SiteModelConfiguration object
	// that is valid. So tests can start with something and introduce problems as needed.
	getValidSiteConfiguration := func() *types.SiteModelConfiguration {
		return &types.SiteModelConfiguration{
			SourcegraphModelConfig: &types.SourcegraphModelConfig{
				PollingInterval: pointers.Ptr("8h"),
				Endpoint:        pointers.Ptr("https://cody-gateway.sourcegraph.com/current-models.json"),
				ModelFilters: &types.ModelFilters{
					StatusFilter: []string{string(types.ModelStatusStable)},
					Allow:        []string{"openai::*", "anthropic::*"},
					Deny:         []string{"*-latest"},
				},
			},

			ProviderOverrides: []types.ProviderOverride{
				// BYOK. Supply server-side configuration data for the "openai" provider.
				{
					ID: types.ProviderID("opoenai"),
					ServerSideConfig: &types.ServerSideProviderConfig{
						AzureOpenAI: &types.AzureOpenAIProviderConfig{
							AccessToken: "secret",
							Endpoint:    "https://acmeco-inc-llc.openai.azure.com/ ",
						},
					},
					// Change the defaults for all provider models, those with a ModelRef prefixed
					// with "anthropic::", to have an extended context window.
					DefaultModelConfig: &types.DefaultModelConfig{
						ContextWindow: types.ContextWindow{
							MaxInputTokens:  100_000,
							MaxOutputTokens: 10_000,
						},
					},
				},
				// BYOLLM. Introduce an entirely new ProviderID. None of the Cody Gateway
				// supplied models will reference this provider ID, but models from this
				// site config object can.
				{
					// Create an "AWS" provider, for serving AWS Titan models.
					ID: types.ProviderID("aws"),
					ServerSideConfig: &types.ServerSideProviderConfig{
						AWSBedrock: &types.AWSBedrockProviderConfig{
							AccessToken: "AK...",
							Region:      "us-west-2",
							Endpoint:    "https://vpce-0000000000-00000000.bedrock-runtime.us-west-2.vpce.amazonaws.com",
						},
					},
					DefaultModelConfig: &types.DefaultModelConfig{
						Status: types.ModelStatusStable,
					},
				},
			},

			ModelOverrides: []types.ModelOverride{
				// Add a new LLM model. This will get routed to the overridden "openai" provider.
				// It uses the same ModelName as GPT 3.5 turbo, but overrides the context window
				// to have an even larger value than the admin-supplied model default.
				{
					ModelRef:     "openai::2024-02-01::gpt-3.5-turbo_extra-turbo",
					ModelName:    "gpt-3.5-turbo",
					DisplayName:  "GPT 3.5 Turbo (With Extra Turbo)",
					Capabilities: []types.ModelCapability{types.ModelCapabilityChat},
					ContextWindow: types.ContextWindow{
						MaxInputTokens:  200_000,
						MaxOutputTokens: 20_000,
					},
				},
				// As an example, this will just replace the DisplayName of an existing
				// LLM model that we expect to have been provided by Sourcegraph.
				{
					ModelRef:    "openai::2024-02-01::gpt-3.5-turbo",
					DisplayName: "GPT 3.5 Turbo (Not much Turbo)",
				},
				// Using BYOLLM, we are introducing a model that will be routed to the
				// "aws" LLM provider, defined in this site configuration.
				{
					ModelRef: "aws::2023-04-20::titan-text-express-v1",

					DisplayName: "Titan Text Express v1",
					ModelName:   "amazon.titan-text-express-v1",

					Capabilities: []types.ModelCapability{types.ModelCapabilityChat, types.ModelCapabilityAutocomplete},
					Category:     types.ModelCategoryBalanced,
					Status:       types.ModelStatusExperimental,
					Tier:         types.ModelTierEnterprise,

					ContextWindow: types.ContextWindow{
						MaxInputTokens:  200_000,
						MaxOutputTokens: 10_000,
					},

					ServerSideConfig: &types.ServerSideModelConfig{
						AWSBedrockProvisionedThroughput: &types.AWSBedrockProvisionedThroughput{
							ARN: "arn:aws:bedrock:us-west-2:012345678901:provisioned-model/xxxxxxxx",
						},
					},
				},
			},
		}
	}

	t.Run("SourcegraphModelConfig", func(t *testing.T) {
		t.Run("Endpoint", func(t *testing.T) {
			siteConfig := getValidSiteConfiguration()
			siteConfig.SourcegraphModelConfig.Endpoint = pointers.Ptr("not a valid URL")
			err := ValidateSiteConfig(siteConfig)
			assert.ErrorContains(t, err, "sourcegraph config: invalid endpoint URL")
		})
		t.Run("AllowDenyList", func(t *testing.T) {
			// Add a bogus value into the Allow list.
			{
				siteConfig := getValidSiteConfiguration()
				siteConfig.SourcegraphModelConfig.ModelFilters.Allow = []string{
					"valid", "invalid * because asterisks must be on ends", "valid",
				}
				err := ValidateSiteConfig(siteConfig)
				assert.ErrorContains(t, err, `sourcegraph config: invalid allow list rule: "invalid * because`)
			}
			// Add a bogus value into the Deny list.
			{
				siteConfig := getValidSiteConfiguration()
				siteConfig.SourcegraphModelConfig.ModelFilters.Deny = []string{
					"valid", "invalid * because asterisks must be on ends", "valid",
				}
				err := ValidateSiteConfig(siteConfig)
				assert.ErrorContains(t, err, `sourcegraph config: invalid deny list rule: "invalid * because`)
			}
		})
	})

	t.Run("ProviderOverrides", func(t *testing.T) {
		t.Run("InvalidProviderID", func(t *testing.T) {
			siteConfig := getValidSiteConfiguration()
			siteConfig.ProviderOverrides[0].ID = "invalid id"

			err := ValidateSiteConfig(siteConfig)
			assert.ErrorContains(t, err, `provider overrides: invalid provider ID "invalid id"`)
		})
	})
	t.Run("ModelOverrides", func(t *testing.T) {
		t.Run("InvalidModelID", func(t *testing.T) {
			siteConfig := getValidSiteConfiguration()
			siteConfig.ModelOverrides[0].ModelRef = types.ModelRef("foo/bar")

			err := ValidateSiteConfig(siteConfig)
			assert.ErrorContains(t, err, `model overrides: validating model ref "foo/bar": modelRef syntax error`)
		})
	})

	t.Run("DefaultModels", func(t *testing.T) {
		{
			// Valid DefaultModels object.
			siteConfig := getValidSiteConfiguration()
			siteConfig.DefaultModels = &types.DefaultModels{
				// Just valid ModelRef values.
				Chat:           types.ModelRef("foo::bar::baz"),
				FastChat:       types.ModelRef("foo::bar::baz"),
				CodeCompletion: types.ModelRef("foo::bar::baz"),
			}
			err := ValidateSiteConfig(siteConfig)
			assert.Nil(t, err)
		}

		{
			siteConfig := getValidSiteConfiguration()
			siteConfig.DefaultModels = &types.DefaultModels{
				// Invalid ModelRefs, Chat not specified.
				FastChat:       types.ModelRef("foo::bar::baz"),
				CodeCompletion: types.ModelRef("foo::bar::baz"),
			}
			err := ValidateSiteConfig(siteConfig)
			assert.ErrorContains(t, err, "default chat model: modelRef is blank")
		}
	})
}
