import fetch from 'isomorphic-fetch'

import type { ConfigurationWithAccessToken } from '../../configuration'
import { isError } from '../../utils'
import { DOTCOM_URL, isDotCom } from '../environments'

import {
    CURRENT_SITE_CODY_LLM_CONFIGURATION,
    CURRENT_SITE_CODY_LLM_PROVIDER,
    CURRENT_SITE_GRAPHQL_FIELDS_QUERY,
    CURRENT_SITE_HAS_CODY_ENABLED_QUERY,
    CURRENT_SITE_IDENTIFICATION,
    CURRENT_SITE_VERSION_QUERY,
    CURRENT_USER_ID_AND_VERIFIED_EMAIL_QUERY,
    CURRENT_USER_ID_QUERY,
    EVALUATE_FEATURE_FLAG_QUERY,
    GET_CODY_CONTEXT_QUERY,
    GET_FEATURE_FLAGS_QUERY,
    IS_CONTEXT_REQUIRED_QUERY,
    LEGACY_SEARCH_EMBEDDINGS_QUERY,
    LOG_EVENT_MUTATION,
    LOG_EVENT_MUTATION_DEPRECATED,
    REPOSITORY_EMBEDDING_EXISTS_QUERY,
    REPOSITORY_ID_QUERY,
    REPOSITORY_IDS_QUERY,
    REPOSITORY_NAMES_QUERY,
    SEARCH_ATTRIBUTION_QUERY,
    SEARCH_EMBEDDINGS_QUERY,
} from './queries'
import { buildGraphQLUrl } from './url'

interface APIResponse<T> {
    data?: T
    errors?: { message: string; path?: string[] }[]
}

interface SiteVersionResponse {
    site: { productVersion: string } | null
}

interface SiteIdentificationResponse {
    site: { siteID: string; productSubscription: { license: { hashedKey: string } } } | null
}

interface SiteGraphqlFieldsResponse {
    __type: { fields: { name: string }[] } | null
}

interface SiteHasCodyEnabledResponse {
    site: { isCodyEnabled: boolean } | null
}

interface CurrentUserIdResponse {
    currentUser: { id: string } | null
}

interface CurrentUserIdHasVerifiedEmailResponse {
    currentUser: { id: string; hasVerifiedEmail: boolean } | null
}

interface CodyLLMSiteConfigurationResponse {
    site: { codyLLMConfiguration: Omit<CodyLLMSiteConfiguration, 'provider'> | null } | null
}

interface CodyLLMSiteConfigurationProviderResponse {
    site: { codyLLMConfiguration: Pick<CodyLLMSiteConfiguration, 'provider'> | null } | null
}

interface RepositoryIdResponse {
    repository: { id: string } | null
}

interface RepositoryIdsResponse {
    repositories: { nodes: { id: string; name: string }[] }
}

interface RepositoryNamesResponse {
    repositories: { nodes: { id: string; name: string }[] }
}

interface RepositoryEmbeddingExistsResponse {
    repository: { id: string; embeddingExists: boolean } | null
}

interface EmbeddingsSearchResponse {
    embeddingsSearch: EmbeddingsSearchResults
}

interface EmbeddingsMultiSearchResponse {
    embeddingsMultiSearch: EmbeddingsSearchResults
}

interface CodyFileChunkContext {
    __typename: 'FileChunkContext'
    blob: {
        path: string
        repository: {
            id: string
            name: string
        }
        commit: {
            id: string
            oid: string
        }
    }
    startLine: number
    endLine: number
    chunkContent: string
}

type GetCodyContextResult = CodyFileChunkContext | null

interface GetCodyContextResponse {
    getCodyContext: GetCodyContextResult[]
}

interface SearchAttributionResponse {
    snippetAttribution: {
        limitHit: boolean
        nodes: { repositoryName: string }[]
    }
}

interface LogEventResponse {}

export interface EmbeddingsSearchResult {
    repoName?: string
    revision?: string
    fileName: string
    startLine: number
    endLine: number
    content: string
}

export interface EmbeddingsSearchResults {
    codeResults: EmbeddingsSearchResult[]
    textResults: EmbeddingsSearchResult[]
}

export interface SearchAttributionResults {
    limitHit: boolean
    nodes: { repositoryName: string }[]
}

export interface CodyLLMSiteConfiguration {
    chatModel?: string
    chatModelMaxTokens?: number
    fastChatModel?: string
    fastChatModelMaxTokens?: number
    completionModel?: string
    completionModelMaxTokens?: number
    provider?: string
    smartContextWindow?: boolean
    disableClientConfigAPI?: boolean
}

interface IsContextRequiredForChatQueryResponse {
    isContextRequiredForChatQuery: boolean
}

interface EvaluatedFeatureFlag {
    name: string
    value: boolean
}

interface EvaluatedFeatureFlagsResponse {
    evaluatedFeatureFlags: EvaluatedFeatureFlag[]
}

interface EvaluateFeatureFlagResponse {
    evaluateFeatureFlag: boolean
}

function extractDataOrError<T, R>(response: APIResponse<T> | Error, extract: (data: T) => R): R | Error {
    if (isError(response)) {
        return response
    }
    if (response.errors && response.errors.length > 0) {
        return new Error(response.errors.map(({ message }) => message).join(', '))
    }
    if (!response.data) {
        return new Error('response is missing data')
    }
    return extract(response.data)
}

export interface event {
    event: string
    userCookieID: string
    url: string
    source: string
    argument?: string | {}
    publicArgument?: string | {}
    client: string
    connectedSiteID?: string
    hashedLicenseKey?: string
}

type GraphQLAPIClientConfig = Pick<ConfigurationWithAccessToken, 'serverEndpoint' | 'accessToken' | 'customHeaders'> &
    Pick<Partial<ConfigurationWithAccessToken>, 'telemetryLevel'>

export let customUserAgent: string | undefined

export function addCustomUserAgent(headers: Headers): void {
    if (customUserAgent) {
        headers.set('User-Agent', customUserAgent)
    }
}

export function setUserAgent(newUseragent: string): void {
    customUserAgent = newUseragent
}

export class SourcegraphGraphQLAPIClient {
    private dotcomUrl = DOTCOM_URL

    constructor(private config: GraphQLAPIClientConfig) {}

    public onConfigurationChange(newConfig: GraphQLAPIClientConfig): void {
        this.config = newConfig
    }

    public isDotCom(): boolean {
        return isDotCom(this.config.serverEndpoint)
    }

    public async getSiteVersion(): Promise<string | Error> {
        return this.fetchSourcegraphAPI<APIResponse<SiteVersionResponse>>(CURRENT_SITE_VERSION_QUERY, {}).then(
            response =>
                extractDataOrError(
                    response,
                    data =>
                        // Example values: "5.1.0" or "222587_2023-05-30_5.0-39cbcf1a50f0" for insider builds
                        data.site?.productVersion ?? new Error('site version not found')
                )
        )
    }

    public async getSiteIdentification(): Promise<{ siteid: string; hashedLicenseKey: string } | Error> {
        const response = await this.fetchSourcegraphAPI<APIResponse<SiteIdentificationResponse>>(
            CURRENT_SITE_IDENTIFICATION,
            {}
        )
        return extractDataOrError(response, data =>
            data.site?.siteID
                ? data.site?.productSubscription?.license?.hashedKey
                    ? {
                          siteid: data.site?.siteID,
                          hashedLicenseKey: data.site?.productSubscription?.license?.hashedKey,
                      }
                    : new Error('site hashed license key not found')
                : new Error('site ID not found')
        )
    }

    public async getSiteHasIsCodyEnabledField(): Promise<boolean | Error> {
        return this.fetchSourcegraphAPI<APIResponse<SiteGraphqlFieldsResponse>>(
            CURRENT_SITE_GRAPHQL_FIELDS_QUERY,
            {}
        ).then(response =>
            extractDataOrError(response, data => !!data.__type?.fields?.find(field => field.name === 'isCodyEnabled'))
        )
    }

    public async getSiteHasCodyEnabled(): Promise<boolean | Error> {
        return this.fetchSourcegraphAPI<APIResponse<SiteHasCodyEnabledResponse>>(
            CURRENT_SITE_HAS_CODY_ENABLED_QUERY,
            {}
        ).then(response => extractDataOrError(response, data => data.site?.isCodyEnabled ?? false))
    }

    public async getCurrentUserId(): Promise<string | Error> {
        return this.fetchSourcegraphAPI<APIResponse<CurrentUserIdResponse>>(CURRENT_USER_ID_QUERY, {}).then(response =>
            extractDataOrError(response, data =>
                data.currentUser ? data.currentUser.id : new Error('current user not found')
            )
        )
    }

    public async getCurrentUserIdAndVerifiedEmail(): Promise<{ id: string; hasVerifiedEmail: boolean } | Error> {
        return this.fetchSourcegraphAPI<APIResponse<CurrentUserIdHasVerifiedEmailResponse>>(
            CURRENT_USER_ID_AND_VERIFIED_EMAIL_QUERY,
            {}
        ).then(response =>
            extractDataOrError(response, data =>
                data.currentUser ? { ...data.currentUser } : new Error('current user not found with verified email')
            )
        )
    }

    public async getCodyLLMConfiguration(): Promise<undefined | CodyLLMSiteConfiguration | Error> {
        // fetch Cody LLM provider separately for backward compatability
        const [configResponse, providerResponse] = await Promise.all([
            this.fetchSourcegraphAPI<APIResponse<CodyLLMSiteConfigurationResponse>>(
                CURRENT_SITE_CODY_LLM_CONFIGURATION
            ),
            this.fetchSourcegraphAPI<APIResponse<CodyLLMSiteConfigurationProviderResponse>>(
                CURRENT_SITE_CODY_LLM_PROVIDER
            ),
        ])

        const config = extractDataOrError(configResponse, data => data.site?.codyLLMConfiguration || undefined)
        if (!config || isError(config)) {
            return config
        }

        let provider: string | undefined
        const llmProvider = extractDataOrError(providerResponse, data => data.site?.codyLLMConfiguration?.provider)
        if (llmProvider && !isError(llmProvider)) {
            provider = llmProvider
        }

        return { ...config, provider }
    }

    public async getRepoIds(names: string[]): Promise<{ id: string; name: string }[] | Error> {
        return this.fetchSourcegraphAPI<APIResponse<RepositoryIdsResponse>>(REPOSITORY_IDS_QUERY, {
            names,
            first: names.length,
        }).then(response => extractDataOrError(response, data => data.repositories?.nodes))
    }

    public async getRepoId(repoName: string): Promise<string | Error> {
        return this.fetchSourcegraphAPI<APIResponse<RepositoryIdResponse>>(REPOSITORY_ID_QUERY, {
            name: repoName,
        }).then(response =>
            extractDataOrError(response, data =>
                data.repository ? data.repository.id : new RepoNotFoundError(`repository ${repoName} not found`)
            )
        )
    }

    public async getRepoNames(first: number): Promise<string[] | Error> {
        return this.fetchSourcegraphAPI<APIResponse<RepositoryNamesResponse>>(REPOSITORY_NAMES_QUERY, { first }).then(
            response =>
                extractDataOrError(
                    response,
                    data => data?.repositories?.nodes?.map((node: { id: string; name: string }) => node?.name) || []
                )
        )
    }

    public async getRepoIdIfEmbeddingExists(repoName: string): Promise<string | null | Error> {
        return this.fetchSourcegraphAPI<APIResponse<RepositoryEmbeddingExistsResponse>>(
            REPOSITORY_EMBEDDING_EXISTS_QUERY,
            {
                name: repoName,
            }
        ).then(response =>
            extractDataOrError(response, data => (data.repository?.embeddingExists ? data.repository.id : null))
        )
    }

    /**
     * Checks if Cody is enabled on the current Sourcegraph instance.
     * @returns
     * enabled: Whether Cody is enabled.
     * version: The Sourcegraph version.
     *
     * This method first checks the Sourcegraph version using `getSiteVersion()`.
     * If the version is before 5.0.0, Cody is disabled.
     * If the version is 5.0.0 or newer, it checks for the existence of the `isCodyEnabled` field using `getSiteHasIsCodyEnabledField()`.
     * If the field exists, it calls `getSiteHasCodyEnabled()` to check its value.
     * If the field does not exist, Cody is assumed to be enabled for versions between 5.0.0 - 5.1.0.
     */
    public async isCodyEnabled(): Promise<{ enabled: boolean; version: string }> {
        // Check site version.
        const siteVersion = await this.getSiteVersion()
        if (isError(siteVersion)) {
            return { enabled: false, version: 'unknown' }
        }
        const insiderBuild = siteVersion.length > 12 || siteVersion.includes('dev')
        if (insiderBuild) {
            return { enabled: true, version: siteVersion }
        }
        // NOTE: Cody does not work on versions older than 5.0
        const versionBeforeCody = siteVersion < '5.0.0'
        if (versionBeforeCody) {
            return { enabled: false, version: siteVersion }
        }
        // Beta version is betwewen 5.0.0 - 5.1.0 and does not have isCodyEnabled field
        const betaVersion = siteVersion >= '5.0.0' && siteVersion < '5.1.0'
        const hasIsCodyEnabledField = await this.getSiteHasIsCodyEnabledField()
        // The isCodyEnabled field does not exist before version 5.1.0
        if (!betaVersion && !isError(hasIsCodyEnabledField) && hasIsCodyEnabledField) {
            const siteHasCodyEnabled = await this.getSiteHasCodyEnabled()
            return { enabled: !isError(siteHasCodyEnabled) && siteHasCodyEnabled, version: siteVersion }
        }
        return { enabled: insiderBuild || betaVersion, version: siteVersion }
    }

    public async logEvent(event: event): Promise<LogEventResponse | Error> {
        if (process.env.CODY_TESTING === 'true') {
            return this.sendEventLogRequestToTestingAPI(event)
        }
        if (this.config?.telemetryLevel === 'off') {
            return {}
        }
        if (this.isDotCom()) {
            return this.sendEventLogRequestToAPI(event)
        }
        const responses = await Promise.all([
            this.sendEventLogRequestToAPI(event),
            this.sendEventLogRequestToDotComAPI(event),
        ])
        if (isError(responses[0]) && isError(responses[1])) {
            return new Error('Errors logging events: ' + responses[0].toString() + ', ' + responses[1].toString())
        }
        if (isError(responses[0])) {
            return responses[0]
        }
        if (isError(responses[1])) {
            return responses[1]
        }
        return {}
    }

    private async sendEventLogRequestToDotComAPI(event: event): Promise<LogEventResponse | Error> {
        const response = await this.fetchSourcegraphDotcomAPI<APIResponse<LogEventResponse>>(LOG_EVENT_MUTATION, event)
        return extractDataOrError(response, data => data)
    }

    private async sendEventLogRequestToAPI(event: event): Promise<LogEventResponse | Error> {
        const initialResponse = await this.fetchSourcegraphAPI<APIResponse<LogEventResponse>>(LOG_EVENT_MUTATION, event)
        const initialDataOrError = extractDataOrError(initialResponse, data => data)

        if (isError(initialDataOrError)) {
            const secondResponse = await this.fetchSourcegraphAPI<APIResponse<LogEventResponse>>(
                LOG_EVENT_MUTATION_DEPRECATED,
                event
            )
            return extractDataOrError(secondResponse, data => data)
        }

        return initialDataOrError
    }

    private async sendEventLogRequestToTestingAPI(event: event): Promise<LogEventResponse | Error> {
        const initialResponse = await this.fetchSourcegraphTestingAPI<APIResponse<LogEventResponse>>(event)
        const initialDataOrError = extractDataOrError(initialResponse, data => data)

        if (isError(initialDataOrError)) {
            const secondResponse = await this.fetchSourcegraphTestingAPI<APIResponse<LogEventResponse>>(event)
            return extractDataOrError(secondResponse, data => data)
        }

        return initialDataOrError
    }

    public async getCodyContext(
        repos: string[],
        query: string,
        codeResultsCount: number,
        textResultsCount: number
    ): Promise<GetCodyContextResult[] | Error> {
        return this.fetchSourcegraphAPI<APIResponse<GetCodyContextResponse>>(GET_CODY_CONTEXT_QUERY, {
            repos,
            query,
            codeResultsCount,
            textResultsCount,
        }).then(response => extractDataOrError(response, data => data.getCodyContext))
    }

    public async searchEmbeddings(
        repos: string[],
        query: string,
        codeResultsCount: number,
        textResultsCount: number
    ): Promise<EmbeddingsSearchResults | Error> {
        return this.fetchSourcegraphAPI<APIResponse<EmbeddingsMultiSearchResponse>>(SEARCH_EMBEDDINGS_QUERY, {
            repos,
            query,
            codeResultsCount,
            textResultsCount,
        }).then(response => extractDataOrError(response, data => data.embeddingsMultiSearch))
    }

    // (Naman): This is a temporary workaround for supporting vscode cody integrated with older version of sourcegraph which do not support the latest searchEmbeddings query.
    public async legacySearchEmbeddings(
        repo: string,
        query: string,
        codeResultsCount: number,
        textResultsCount: number
    ): Promise<EmbeddingsSearchResults | Error> {
        return this.fetchSourcegraphAPI<APIResponse<EmbeddingsSearchResponse>>(LEGACY_SEARCH_EMBEDDINGS_QUERY, {
            repo,
            query,
            codeResultsCount,
            textResultsCount,
        }).then(response => extractDataOrError(response, data => data.embeddingsSearch))
    }

    public async searchAttribution(snippet: string): Promise<SearchAttributionResults | Error> {
        return this.fetchSourcegraphAPI<APIResponse<SearchAttributionResponse>>(SEARCH_ATTRIBUTION_QUERY, {
            snippet,
        }).then(response => extractDataOrError(response, data => data.snippetAttribution))
    }

    public async isContextRequiredForQuery(query: string): Promise<boolean | Error> {
        return this.fetchSourcegraphAPI<APIResponse<IsContextRequiredForChatQueryResponse>>(IS_CONTEXT_REQUIRED_QUERY, {
            query,
        }).then(response => extractDataOrError(response, data => data.isContextRequiredForChatQuery))
    }

    public async getEvaluatedFeatureFlags(): Promise<Record<string, boolean> | Error> {
        return this.fetchSourcegraphAPI<APIResponse<EvaluatedFeatureFlagsResponse>>(GET_FEATURE_FLAGS_QUERY, {}).then(
            response =>
                extractDataOrError(response, data =>
                    data.evaluatedFeatureFlags.reduce((acc, { name, value }) => {
                        acc[name] = value
                        return acc
                    }, {} as Record<string, boolean>)
                )
        )
    }

    public async evaluateFeatureFlag(flagName: string): Promise<boolean | null | Error> {
        return this.fetchSourcegraphAPI<APIResponse<EvaluateFeatureFlagResponse>>(EVALUATE_FEATURE_FLAG_QUERY, {
            flagName,
        }).then(response => extractDataOrError(response, data => data.evaluateFeatureFlag))
    }

    private fetchSourcegraphAPI<T>(query: string, variables: Record<string, any> = {}): Promise<T | Error> {
        const headers = new Headers(this.config.customHeaders as HeadersInit)
        headers.set('Content-Type', 'application/json; charset=utf-8')
        if (this.config.accessToken) {
            headers.set('Authorization', `token ${this.config.accessToken}`)
        }
        addCustomUserAgent(headers)

        const url = buildGraphQLUrl({ request: query, baseUrl: this.config.serverEndpoint })
        return fetch(url, {
            method: 'POST',
            body: JSON.stringify({ query, variables }),
            headers,
        })
            .then(verifyResponseCode)
            .then(response => response.json() as T)
            .catch(error => new Error(`accessing Sourcegraph GraphQL API: ${error} (${url})`))
    }

    // make an anonymous request to the dotcom API
    private fetchSourcegraphDotcomAPI<T>(query: string, variables: Record<string, any>): Promise<T | Error> {
        const url = buildGraphQLUrl({ request: query, baseUrl: this.dotcomUrl.href })
        const headers = new Headers()
        addCustomUserAgent(headers)
        return fetch(url, {
            method: 'POST',
            body: JSON.stringify({ query, variables }),
            headers,
        })
            .then(verifyResponseCode)
            .then(response => response.json() as T)
            .catch(error => new Error(`error fetching Sourcegraph GraphQL API: ${error} (${url})`))
    }

    // make an anonymous request to the Testing API
    private fetchSourcegraphTestingAPI<T>(body: Record<string, any>): Promise<T | Error> {
        const url = 'http://localhost:49300/.api/testLogging'
        const headers = new Headers({
            'Content-Type': 'application/json',
        })
        addCustomUserAgent(headers)

        return fetch(url, {
            method: 'POST',
            headers,
            body: JSON.stringify(body),
        })
            .then(verifyResponseCode)
            .then(response => response.json() as T)
            .catch(error => new Error(`error fetching Testing Sourcegraph API: ${error} (${url})`))
    }
}

function verifyResponseCode(response: Response): Response {
    if (!response.ok) {
        throw new Error(`HTTP status code: ${response.status}`)
    }
    return response
}

class RepoNotFoundError extends Error {}

export const isRepoNotFoundError = (value: unknown): value is RepoNotFoundError => value instanceof RepoNotFoundError
