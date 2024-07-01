package modelconfig

import (
	"strings"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// IMPORTANT: Validation MUST be backwards-compatible.
//
// We can NEVER "relax" any validation checks we perform. Because that would lead to older
// Sourcegraph instances failing to accept newer versions of the configuration data.

// resourceIDRE is a regular expression for verifying resource IDs are
// of a simple format.
var resourceIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_\-\.]*[a-z0-9]$`)

func validateProvider(p types.Provider) error {
	if l := len(p.DisplayName); l < 5 || l > 40 {
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
	if l := len(m.DisplayName); l < 5 || l > 40 {
		return errors.Errorf("display name length: %d", l)
	}
	// We don't do any validation of the ModelName, as that the
	// values needed by LLM providers is outside of our control.
	if !resourceIDRE.MatchString(string(m.ID)) {
		return errors.New("id format")
	}

	// Require that the ModelID matches the supplied ModelRef.
	if !strings.HasSuffix(string(m.ModelRef), "::"+string(m.ID)) {
		return errors.New("id does not match modelref")
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

// ValidateSiteConfig validates that the site configuration data expressed is valid.
func ValidateSiteConfig(doc *types.SiteModelConfiguration) error {
	// TODO: Verify Sourcegraph
	// - Endpoint is a valid URL.
	// - PollingInterval is a valid duration, or "never".
	// - ModelFilters.CategoryFilter only contains valid categories.
	// - The strings in the Allow/Deny lists are reasonable, e.g. do not
	//   contain any whitespaces, more than six colons, etc.
	// - The Allow/Deny list wildcard strings are well-formed: they may
	//   only contain an asterisk as the first and/or last character.

	// TODO: Verify ProviderOverrides
	// - No ProviderIDs are duplicated.

	// TODO: Verify ModelOverrides
	// - Every model must have a ModelRef. All other fields are optional.
	// - No ModelRefs are duplicated.
	// - Every ModelOverride for a ProviderOverrided must have a ModelRef that is
	//   prefixed by the "${ProviderID}::".
	return nil
}
