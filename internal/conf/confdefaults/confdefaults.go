// Package confdefaults contains default configuration files for various
// deployment types.
//
// It is a separate package so that users of pkg/conf do not indirectly import
// pkg/database/confdb, which we have a linter to protect against.
package confdefaults

import (
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
)

// TODO(slimsag): consider moving these into actual json files for improved
// editing.

// DevAndTesting is the default configuration applied to dev instances of
// Sourcegraph, as well as what is used by default during Go testing.
//
// Tests that wish to use a specific configuration should use conf.Mock.
//
// Note: This actually generally only applies to 'go test' because we always
// override this configuration via *_CONFIG_FILE environment variables.
var DevAndTesting = conftypes.RawUnified{
	Site: `{
	"auth.providers": [
		{
			"type": "builtin",
			"allowSignup": true
		}
	],
}`,
}

// DockerContainer is the default configuration applied to Docker
// single-container instances of Sourcegraph.
var DockerContainer = conftypes.RawUnified{
	Site: `{
	// The externally accessible URL for this Sourcegraph instance.
	// i.e. the address you enter in your browser's address bar.
	// This is required to be configured for Sourcegraph to work correctly.
	// Only root URLs are allowed.
	// Note: When configured to the recommended https:// users will not be able to log in using http://
	// "externalURL": "https://sourcegraph.example.com",
	//
	//
	// The authentication providers to use for identifying and signing in users.
	"auth.providers": [
		{
			"type": "builtin",
			"allowSignup": true
		}
	]
}`,
}

// KubernetesOrDockerComposeOrPureDocker is the default configuration
// applied to Kubernetes, Docker Compose, and pure Docker instances of Sourcegraph.
var KubernetesOrDockerComposeOrPureDocker = conftypes.RawUnified{
	Site: `{
	// The externally accessible URL for this Sourcegraph instance.
	// i.e. the address you enter in your browser's address bar.
	// This is required to be configured for Sourcegraph to work correctly.
	// Only root URLs are allowed.
	// Note: When configured to the recommended https:// users will not be able to log in using http://
	// "externalURL": "https://sourcegraph.example.com",
	//
	//
	// The authentication providers to use for identifying and signing in users.
	// The builtin auth provider with signup disallowed (default) means that
	// after the initial site admin signs in, all other users must be invited.
	// Other providers are documented at https://sourcegraph.com/docs/admin/auth.
	"auth.providers": [
		{
			"type": "builtin",
			"allowSignup": false
		}
	],
}`,
}

// Default is the default for *this* deployment type. It is populated by
// pkg/conf at init time.
var Default conftypes.RawUnified
