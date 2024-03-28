export const CURRENT_USER_ID_QUERY = `
query CurrentUser {
    currentUser {
        id
    }
}`

export const CURRENT_SITE_VERSION_QUERY = `
query SiteProductVersion {
    site {
        productVersion
    }
}`

export const CURRENT_SITE_HAS_CODY_ENABLED_QUERY = `
query SiteHasCodyEnabled {
    site {
        isCodyEnabled
    }
}`

export const CURRENT_SITE_GRAPHQL_FIELDS_QUERY = `
query SiteGraphQLFields {
    __type(name: "Site") {
        fields {
            name
        }
    }
}`

export const CURRENT_USER_ID_AND_VERIFIED_EMAIL_QUERY = `
query CurrentUser {
    currentUser {
        id
        hasVerifiedEmail
    }
}`

export const CURRENT_SITE_CODY_LLM_PROVIDER = `
query CurrentSiteCodyLlmConfiguration {
    site {
        codyLLMConfiguration {
            provider
        }
    }
}`

export const CURRENT_SITE_CODY_LLM_CONFIGURATION = `
query CurrentSiteCodyLlmConfiguration {
    site {
        codyLLMConfiguration {
            chatModel
            chatModelMaxTokens
            fastChatModel
            fastChatModelMaxTokens
            completionModel
            completionModelMaxTokens
        }
    }
}`

export const REPOSITORY_ID_QUERY = `
query Repository($name: String!) {
	repository(name: $name) {
		id
	}
}`

export const REPOSITORY_IDS_QUERY = `
query Repositories($names: [String!]!, $first: Int!) {
	repositories(names: $names, first: $first) {
                nodes {
		        id
                        name
                }
	}
}`

export const REPOSITORY_NAMES_QUERY = `
query Repositories($first: Int!) {
	repositories(first: $first) {
                nodes {
		        id
                        name
                }
	}
}`

export const REPOSITORY_EMBEDDING_EXISTS_QUERY = `
query Repository($name: String!) {
	repository(name: $name) {
                id
                embeddingExists
	}
}`

export const GET_CODY_CONTEXT_QUERY = `
query GetCodyContext($repos: [ID!]!, $query: String!, $codeResultsCount: Int!, $textResultsCount: Int!) {
	getCodyContext(repos: $repos, query: $query, codeResultsCount: $codeResultsCount, textResultsCount: $textResultsCount) {
                __typename
		... on FileChunkContext {
                        blob {
                                path
                                repository {
                                        id
                                        name
                                }
                                commit {
                                        id
                                        oid
                                }
                        }
			startLine
			endLine
                        chunkContent
		}
	}
}`

export const SEARCH_EMBEDDINGS_QUERY = `
query EmbeddingsSearch($repos: [ID!]!, $query: String!, $codeResultsCount: Int!, $textResultsCount: Int!) {
	embeddingsMultiSearch(repos: $repos, query: $query, codeResultsCount: $codeResultsCount, textResultsCount: $textResultsCount) {
		codeResults {
                        repoName
                        revision
			fileName
			startLine
			endLine
			content
		}
		textResults {
                        repoName
                        revision
			fileName
			startLine
			endLine
			content
		}
	}
}`

export const LEGACY_SEARCH_EMBEDDINGS_QUERY = `
query LegacyEmbeddingsSearch($repo: ID!, $query: String!, $codeResultsCount: Int!, $textResultsCount: Int!) {
	embeddingsSearch(repo: $repo, query: $query, codeResultsCount: $codeResultsCount, textResultsCount: $textResultsCount) {
		codeResults {
			fileName
			startLine
			endLine
			content
		}
		textResults {
			fileName
			startLine
			endLine
			content
		}
	}
}`

export const SEARCH_ATTRIBUTION_QUERY = `
query SnippetAttribution($snippet: String!) {
    snippetAttribution(snippet: $snippet) {
        limitHit
        nodes {
            repositoryName
        }
    }
}`

export const IS_CONTEXT_REQUIRED_QUERY = `
query IsContextRequiredForChatQuery($query: String!) {
	isContextRequiredForChatQuery(query: $query)
}`

/**
 * Deprecated following new event structure: https://github.com/sourcegraph/sourcegraph/pull/55126.
 */
export const LOG_EVENT_MUTATION_DEPRECATED = `
mutation LogEventMutation($event: String!, $userCookieID: String!, $url: String!, $source: EventSource!, $argument: String, $publicArgument: String) {
    logEvent(
		event: $event
		userCookieID: $userCookieID
		url: $url
		source: $source
		argument: $argument
		publicArgument: $publicArgument
    ) {
		alwaysNil
	}
}`

export const LOG_EVENT_MUTATION = `
mutation LogEventMutation($event: String!, $userCookieID: String!, $url: String!, $source: EventSource!, $argument: String, $publicArgument: String, $client: String, $connectedSiteID: String, $hashedLicenseKey: String) {
    logEvent(
		event: $event
		userCookieID: $userCookieID
		url: $url
		source: $source
		argument: $argument
		publicArgument: $publicArgument
		client: $client
		connectedSiteID: $connectedSiteID
		hashedLicenseKey: $hashedLicenseKey
    ) {
		alwaysNil
	}
}`

export const CURRENT_SITE_IDENTIFICATION = `
query SiteIdentification {
	site {
		siteID
		productSubscription {
			license {
				hashedKey
			}
		}
	}
}`

export const GET_FEATURE_FLAGS_QUERY = `
    query FeatureFlags {
        evaluatedFeatureFlags() {
            name
            value
          }
    }
`

export const EVALUATE_FEATURE_FLAG_QUERY = `
    query EvaluateFeatureFlag($flagName: String!) {
        evaluateFeatureFlag(flagName: $flagName)
    }
`
