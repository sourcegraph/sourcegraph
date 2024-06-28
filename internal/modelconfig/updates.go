package modelconfig

import (
	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// updateIfNonZero replaces the value pointed to by `out` if the supplied value is
// different from the zero-state for type T.
func updateIfNonZero[T comparable](out *T, value T) {
	var zero T
	if value != zero {
		*out = value
	}
}

// ApplyDefaultModelConfiguration updates the supplied model in-place, overriding any fields
// in which the providerDefaults are non-zero.
func ApplyDefaultModelConfiguration(mod *types.Model, providerDefaults *types.DefaultModelConfig) error {
	if mod == nil {
		return errors.New("no model provided")
	}
	// If we don't have any defaults to apply, then all is well.
	if providerDefaults == nil {
		return nil
	}

	if len(providerDefaults.Capabilities) > 0 {
		mod.Capabilities = providerDefaults.Capabilities
	}
	updateIfNonZero(&mod.Category, providerDefaults.Category)
	updateIfNonZero(&mod.Status, providerDefaults.Status)
	updateIfNonZero(&mod.Tier, providerDefaults.Tier)

	modCtxWin := &mod.ContextWindow
	provDefCtxWin := providerDefaults.ContextWindow
	updateIfNonZero(&modCtxWin.MaxInputTokens, provDefCtxWin.MaxInputTokens)
	updateIfNonZero(&modCtxWin.MaxOutputTokens, provDefCtxWin.MaxOutputTokens)

	if mod.ClientSideConfig != nil {
		mod.ClientSideConfig = providerDefaults.ClientSideConfig
	}
	if mod.ServerSideConfig != nil {
		mod.ServerSideConfig = providerDefaults.ServerSideConfig
	}

	if err := validateModel(*mod); err != nil {
		return errors.Wrap(err, "model is now in an invalid state")
	}

	return nil
}

// ApplyModelOverride updates the supplied model in-place, so that any non-zero fields from
// `modelOverrides` are applied to `mod`.
func ApplyModelOverride(mod *types.Model, modelOverrides types.ModelOverride) error {
	if mod == nil {
		return errors.New("no model provided")
	}

	// It doesn't make sense to apply an override that would change the identity of the
	// model. So we return an error in this case.
	if modelOverrides.ModelRef != "" && mod.ModelRef != modelOverrides.ModelRef {
		return errors.New("cannot change the model's identity")
	}

	// NOTE: These two fields are on ModelOverride but not DefaultModelConfig.
	updateIfNonZero(&mod.ModelName, modelOverrides.ModelName)
	updateIfNonZero(&mod.DisplayName, modelOverrides.DisplayName)

	// The following code overlaps with ApplyDefaultModelConfig, since both allow
	// a Sourcegraph admin to replace a Model's metadata.
	if len(modelOverrides.Capabilities) > 0 {
		mod.Capabilities = modelOverrides.Capabilities
	}
	updateIfNonZero(&mod.Category, modelOverrides.Category)
	updateIfNonZero(&mod.Status, modelOverrides.Status)
	updateIfNonZero(&mod.Tier, modelOverrides.Tier)

	modCtxWin := &mod.ContextWindow
	overrideCtxWin := modelOverrides.ContextWindow
	updateIfNonZero(&modCtxWin.MaxInputTokens, overrideCtxWin.MaxInputTokens)
	updateIfNonZero(&modCtxWin.MaxOutputTokens, overrideCtxWin.MaxOutputTokens)

	if mod.ClientSideConfig != nil {
		mod.ClientSideConfig = modelOverrides.ClientSideConfig
	}
	if mod.ServerSideConfig != nil {
		mod.ServerSideConfig = modelOverrides.ServerSideConfig
	}

	if err := validateModel(*mod); err != nil {
		return errors.Wrap(err, "model is now in an invalid state")
	}

	return nil
}
