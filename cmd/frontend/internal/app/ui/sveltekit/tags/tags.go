package tags

type Tag int

const (
	RepoRoot Tag = 1 << iota
	// EnableAlways always renders the SvelteKit app for this route
	EnableAlways
	// EnableRollout renders the SvelteKit app for this route when the "web-next-rollout" feature flag is enabled
	EnableRollout
	// EnableOptin renders the SvelteKit app for this route when the "web-next" feature flag is enabled.
	EnableOptIn
	// Dotcom marks a route as being only available on Sourcegraph.com
	Dotcom
)

// IsTagValid returns true if the tag is a valid tag for a route
// this is used by the code generator to validate the tags
func IsTagValid(tag string) bool {
	switch tag {
	case "RepoRoot", "EnableAlways", "EnableRollout", "EnableOptIn", "Dotcom":
		return true
	}
	return false
}

func AvailableTags() []string {
	return []string{"RepoRoot", "EnableAlways", "EnableRollout", "EnableOptIn", "Dotcom"}
}
