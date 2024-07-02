package clientconfig

// This is the JSON object which all clients request after authentication to determine how
// they should behave, e.g. if a site admin has restricted chat/autocomplete/other functionality,
// if experimental features are available, etc.
//
// The configuration is always specific to a single authenticated user.
type ClientConfig struct {
	// Version 1 of the configuration schema.
	V1 *ClientConfigV1 `json:"v1"`

	// Version 2 of the configuration schema.
	V2 *ClientConfigV2 `json:"v2,omitempty"`
}

// Client configuration v1
//
// You can always add new fields to this struct, but you cannot delete fields or make breaking
// changes to the meaning of existing fields. Remember, old clients may exist in the wild
// and they need to understand this configuration. If you add a new field, old clients will ignore
// it.
//
// If you want to remove fields from this struct, or change the meaning of them, you should see the
// doc comment on ClientConfigV2 instead.
//
// After adding a field here, you can implement it in cmd/frontend/internal/clientconfig/service.go
// GetForActor method.
type ClientConfigV1 struct {
	// Whether the site admin allows this user to make use of Cody at all.
	CodyEnabled bool

	// Whether the site admin allows this user to make use of the Cody chat feature.
	ChatEnabled bool

	// Whether the site admin allows this user to make use of the Cody autocomplete feature.
	AutoCompleteEnabled bool

	// Whether the site admin allows this user to make use of the Cody commands feature.
	CommandsEnabled bool

	// Whether the site admin allows this user to make use of the Cody attribution feature.
	AttributionEnabled bool

	// Whether the 'smart context window' feature should be enabled, and whether the Sourcegraph
	// instance supports various new GraphQL APIs needed to make it work.
	SmartContextWindowEnabled bool

	// Whether the new Sourcegraph backend LLM models API endpoint should be used to query which
	// models are available.
	ModelsAPIEnabled bool
}

// Client configuration v2 (not yet released)
//
// If you can use ClientConfigV1 by adding a new field with a new meaning, do that instead.
//
// At some point, though, ClientConfigV1 will grow large with a number of 'old' fields we no longer
// care about - we'll want to remove some fields, or make breaking changes to the meaning of them.
// That is where config versioning comes in! Here's how you would make breaking changes:
//
//  1. Change this struct to represent "the new configuration shape", e.g. by copying all fields from
//     ClientConfigV1 and making whatever breaking changes you see fit.
//  2. Update cmd/frontend/internal/clientconfig/service.go GetForActor method to serve **both** the
//     ClientConfig.V1 and ClientConfig.V2. Old clients will expect "v1" in the response, while new
//     clients can look for "v2" configuration.
//  3. Update the clients to support **both** the "v1" configuration (connecting to an old Sourcegraph
//     server) and the "v2" configuration (connecting to a new Sourcegraph server.)
//  4. After some migratory period of time has passed, update the server to remove the "v1" response
//     field entirely. If response["v1"] does not exist, old clients will display an error during
//     authentication `Cody cannot connect to this server (client or server version too old.)`
//  5. When we are comfortable with the latest client versions not connecting to older Sourcegraph
//     servers, remove support from the client for reading "v1" configuration and have it display
//     `Cody cannot connect to this server (client or server version too old.)` when connecting
//     to an outdated server.
//  6. Copy this comment onto a new ClientConfigV3 type, and update the versions, so that it is clear
//     how the next person can do this the right way!
type ClientConfigV2 struct{}
