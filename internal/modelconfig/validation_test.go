package modelconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/embedded"
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
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

	t.Run("ValidateModel", func(t *testing.T) {
		// TODO: Verify error if the model ID doesn't match the model ref.
		// "id does not match modelref"
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
}
