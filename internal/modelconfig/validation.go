package modelconfig

import (
	"net/url"
	"strings"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// IMPORTANT: Validation MUST be backwards-compatible.
//
// We can NEVER "relax" any validation checks we perform. Because that would lead to older
// Sourcegraph instances failing to accept newer versions of the configuration data.

const maxDisplayNameLength = 128

// resourceIDRE is a regular expression for verifying resource IDs are
// of a simple format.
var resourceIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_\-\.]*[a-z0-9]$`)

func validateProvider(p types.Provider) error {
	// Display name is optional, but if it is set ensure it is under 128 chars.
	if l := len(p.DisplayName); l > 0 && l > maxDisplayNameLength {
		return errors.Errorf("display name length: %d", l)
	}
	if !resourceIDRE.MatchString(string(p.ID)) {
		return errors.New("id format")
	}

	// We intentionally don't validate any provider configuration
	// data here, as it isn't clear if that is even a good idea
	// in the first place. (e.g. we may not know the exact format
	// of some 3rd party provider's configuration knob.)
	return nil
}

func validateModelRef(ref types.ModelRef) error {
	if ref == "" {
		return errors.New("modelRef is blank")
	}

	parts := strings.Split(string(ref), "::")
	if len(parts) != 3 {
		return errors.New("modelRef syntax error")
	}
	if !resourceIDRE.MatchString(parts[0]) {
		return errors.New("invalid ProviderID")
	}
	if !resourceIDRE.MatchString(parts[2]) {
		return errors.New("invalid ModelID")
	}
	// We don't impose any constraints on the API Version ID, because
	// while it's something Sourcegraph manages, there are lots of exotic
	// but reasonable forms it could take. e.g. "2024-06-01" or
	// "v1+beta2/with-git-lfs-context-support".
	//
	// But we still want to impose some basic standards, defined here.
	apiVersion := parts[1]
	if strings.ContainsAny(apiVersion, `:;*% $#\"',!@`) {
		return errors.New("invalid APIVersionID")
	}

	return nil
}

func validateModel(m types.Model) error {
	if l := len(m.DisplayName); l > 0 && l > maxDisplayNameLength {
		return errors.Errorf("display name length: %d", l)
	}
	if err := validateModelRef(m.ModelRef); err != nil {
		return errors.Wrap(err, "modelref")
	}

	ctxWin := m.ContextWindow
	if ctxWin.MaxInputTokens <= 0 {
		return errors.New("invalid max input tokens")
	}
	if ctxWin.MaxOutputTokens <= 0 {
		return errors.New("invalid max output tokens")
	}

	// We intentionally do not validate any of the enum metadata fields, because
	// older Sourcegraph instances wouldn't be able to recognize any newer values.

	return nil
}

// ValidateModelConfig validates that the model configuration data expressed is valid.
func ValidateModelConfig(cfg *types.ModelConfiguration) error {
	if cfg == nil {
		return errors.New("no config provided")
	}

	for _, provider := range cfg.Providers {
		if err := validateProvider(provider); err != nil {
			return errors.Wrapf(err, "validating provider %q", provider.ID)
		}
	}

	for _, model := range cfg.Models {
		if err := validateModel(model); err != nil {
			return errors.Wrapf(err, "validating model %q", model.ModelRef)
		}

		// Verify the model is referencing a known provider.
		var forKnownProvider bool
		for _, provider := range cfg.Providers {
			if strings.HasPrefix(string(model.ModelRef), string(provider.ID)+"::") {
				forKnownProvider = true
				break
			}
		}
		if !forKnownProvider {
			return errors.Errorf("model %q does not match a known provider", model.ModelRef)
		}
	}

	isKnownModel := func(mref types.ModelRef) bool {
		for _, model := range cfg.Models {
			if model.ModelRef == mref {
				return true
			}
		}
		return false
	}

	if !isKnownModel(cfg.DefaultModels.Chat) {
		return errors.Errorf("unknown chat model %q", cfg.DefaultModels.Chat)
	}
	if !isKnownModel(cfg.DefaultModels.CodeCompletion) {
		return errors.Errorf("unknown chat model %q", cfg.DefaultModels.CodeCompletion)
	}
	if !isKnownModel(cfg.DefaultModels.FastChat) {
		return errors.Errorf("unknown chat model %q", cfg.DefaultModels.FastChat)
	}

	return nil
}

// isValidRule returns whether the ModelRef allow/deny list rule is well-formed.
func isValidRule(rule string) bool {
	if rule == "" {
		return false
	}
	// Aserisks can only be the first or last character of the rule.
	for i := 1; i < len(rule)-1; i++ {
		if rule[i] == '*' {
			return false
		}
	}
	return true
}

func verifySourcegraphSiteConfig(sgConfig *types.SourcegraphModelConfig) error {
	if sgConfig == nil {
		return nil
	}

	if endpoint := sgConfig.Endpoint; endpoint != nil {
		u, err := url.Parse(*endpoint)
		if err != nil || u.Scheme == "" {
			return errors.New("invalid endpoint URL")
		}
	}

	if modelFilters := sgConfig.ModelFilters; modelFilters != nil {
		for _, allowRule := range modelFilters.Allow {
			if !isValidRule(allowRule) {
				return errors.Errorf("invalid allow list rule: %q", allowRule)
			}
		}
		for _, denyRule := range modelFilters.Deny {
			if !isValidRule(denyRule) {
				return errors.Errorf("invalid deny list rule: %q", denyRule)
			}
		}
	}

	return nil
}

func validateProviderOverrides(overrides []types.ProviderOverride) error {
	seenProviderIDs := map[types.ProviderID]bool{}
	for _, override := range overrides {
		// All provider IDs are unique.
		if seenProviderIDs[override.ID] {
			return errors.Errorf("provider %q specified twice", override.ID)
		}
		seenProviderIDs[override.ID] = true

		// All provider IDs are valid.
		if !resourceIDRE.MatchString(string(override.ID)) {
			return errors.Errorf("invalid provider ID %q", override.ID)
		}
	}
	return nil
}

func validateModelOverrides(overrides []types.ModelOverride) error {
	seenModelRefs := map[types.ModelRef]bool{}
	for _, override := range overrides {
		// All models have a valid ModelRef.
		if err := validateModelRef(override.ModelRef); err != nil {
			return errors.Wrapf(err, "validating model ref %q", override.ModelRef)
		}

		// All model ID overrides are unique.
		if seenModelRefs[override.ModelRef] {
			return errors.Errorf("model %q specified twice", override.ModelRef)
		}
		seenModelRefs[override.ModelRef] = true
	}
	return nil
}

// ValidateSiteConfig validates that the site configuration data expressed is valid.
func ValidateSiteConfig(doc *types.SiteModelConfiguration) error {
	if err := verifySourcegraphSiteConfig(doc.SourcegraphModelConfig); err != nil {
		return errors.Wrap(err, "sourcegraph config")
	}
	if err := validateProviderOverrides(doc.ProviderOverrides); err != nil {
		return errors.Wrap(err, "provider overrides")
	}
	if err := validateModelOverrides(doc.ModelOverrides); err != nil {
		return errors.Wrap(err, "model overrides")
	}

	// When verifying default models, we expect it to be OK to default to
	// a model NOT explicitly defined in the site config. e.g. using something
	// that we expect to be supplied by Sourcegraph. So we just check if they
	// are valid ModelRefs.
	if defModels := doc.DefaultModels; defModels != nil {
		if err := validateModelRef(defModels.Chat); err != nil {
			return errors.Wrap(err, "default chat model")
		}
		if err := validateModelRef(defModels.CodeCompletion); err != nil {
			return errors.Wrap(err, "default completion model")
		}
		if err := validateModelRef(defModels.FastChat); err != nil {
			return errors.Wrap(err, "default fast chat model")
		}
	}

	return nil
}
