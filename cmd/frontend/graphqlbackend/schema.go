package graphqlbackend

import (
	"embed"
	_ "embed"
)

// mainSchema is the main raw graqhql schema.
//
//go:embed schema.graphql
var mainSchema string

// batchesSchema is the Batch Changes raw graqhql schema.
//
//go:embed batches.graphql
var batchesSchema string

// codeIntelSchema is the Code Intel raw graqhql schema.
//
//go:embed codeintel*.graphql
var codeIntelSchema embed.FS

// dotcomSchema is the Dotcom schema extension raw graqhql schema.
//
//go:embed dotcom.graphql
var dotcomSchema string

// licenseSchema is the Licensing raw graqhql schema.
//
//go:embed license.graphql
var licenseSchema string

// codeMonitorsSchema is the Code Monitoring raw graqhql schema.
//
//go:embed code_monitors.graphql
var codeMonitorsSchema string

// insightsSchema is the Code Insights raw graqhql schema.
//
//go:embed insights.graphql
var insightsSchema string

// authzSchema is the Authz raw graqhql schema.
//
//go:embed authz.graphql
var authzSchema string

// computeSchema is an experimental graphql endpoint for computing values from search results.
//
//go:embed compute.graphql
var computeSchema string

// searchContextsSchema is the Search Contexts raw graqhql schema.
//
//go:embed search_contexts.graphql
var searchContextsSchema string

// notebooksSchema is the Notebooks raw graqhql schema.
//
//go:embed notebooks.graphql
var notebooksSchema string

// insightsAggregationsSchema is the Code Insights Aggregations raw graqhql schema.
//
//go:embed insights_aggregations.graphql
var insightsAggregationsSchema string

// outboundWebhooksSchema is the outbound webhook raw GraphQL schema.
//
//go:embed outbound_webhooks.graphql
var outboundWebhooksSchema string

// embeddingsSchema is the Embeddings raw graqhql schema.
//
//go:embed embeddings.graphql
var embeddingsSchema string

// codyContextSchema is the Context raw graqhql schema.
//
//go:embed cody_context.graphql
var codyContextSchema string

// rbacSchema is the RBAC raw graphql schema.
//
//go:embed rbac.graphql
var rbacSchema string

// ownSchema is the Sourcegraph Own raw graqhql schema.
//
//go:embed own.graphql
var ownSchema string

// completionSchema is the Sourcegraph Completions raw graqhql schema.
//
//go:embed completions.graphql
var completionSchema string

// gitHubAppsSchema is the GitHub apps raw graqhql schema.
//
//go:embed githubapps.graphql
var gitHubAppsSchema string

// guardrailsSchema is the Sourcegraph Guardrails raw graphql schema.
//
//go:embed guardrails.graphql
var guardrailsSchema string

// contentLibrary is the Sourcegraph Content Library raw graphql schema.
//
//go:embed content_library.graphql
var contentLibrary string

// searchJobSchema is the Sourcegraph Search Job raw graphql schema.
//
//go:embed search_jobs.graphql
var searchJobSchema string

// telemetrySchema is the Sourcegraph Telemetry V2 raw graphql schema.
//
//go:embed telemetry.graphql
var telemetrySchema string

// oauthProviderSchema is the OAuth provider raw graqhql schema.
//
//go:embed oauthprovider.graphql
var oauthProviderSchema string
