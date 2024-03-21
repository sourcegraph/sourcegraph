import { authentication } from 'vscode'

import { asError } from '@sourcegraph/common'
import { checkOk, GRAPHQL_URI, type GraphQLResult, isHTTPAuthError } from '@sourcegraph/http-client'

import { handleAccessTokenError } from '../settings/accessTokenSetting'
import { endpointRequestHeadersSetting, endpointSetting } from '../settings/endpointSetting'

import { fetch, getProxyAgent, Headers, type HeadersInit } from './fetch'

let invalidated = false

/**
 * To be called when Sourcegraph URL changes.
 */
export function invalidateClient(): void {
    invalidated = true
}

export const requestGraphQLFromVSCode = async <R, V = object>(
    request: string,
    variables: V,
    overrideAccessToken?: string,
    overrideSourcegraphURL?: string
): Promise<GraphQLResult<R>> => {
    if (invalidated) {
        throw new Error(
            'Sourcegraph GraphQL Client has been invalidated due to instance URL change. Restart VS Code to fix.'
        )
    }
    const session = await authentication.getSession(endpointSetting(), [], { createIfNone: false })
    const sourcegraphURL = overrideSourcegraphURL || endpointSetting()
    const accessToken = overrideAccessToken || session?.accessToken
    const nameMatch = request.match(/^\s*(?:query|mutation)\s+(\w+)/)
    const apiURL = `${GRAPHQL_URI}${nameMatch ? '?' + nameMatch[1] : ''}`
    const customHeaders = endpointRequestHeadersSetting()
    // create a headers container based on the custom headers configuration if there are any
    // then add Access Token to request header, contributed by @ptxmac!
    const headers = new Headers(customHeaders as HeadersInit)
    headers.set('Content-Type', 'application/json')
    if (accessToken) {
        headers.set('Authorization', `token ${accessToken}`)
    }
    try {
        const url = new URL(apiURL, sourcegraphURL).href
        // Debt: intercepted requests in integration tests
        // have 0 status codes, so don't check in test environment.
        const checkFunction = process.env.IS_TEST ? <T>(value: T): T => value : checkOk
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const options: any = {
            agent: getProxyAgent(),
            body: JSON.stringify({
                query: request,
                variables,
            }),
            method: 'POST',
            headers,
        }

        const response = checkFunction(await fetch(url, options))
        // TODO request cancellation w/ VS Code cancellation tokens.
        return (await response.json()) as GraphQLResult<R>
    } catch (error) {
        // If `overrideAccessToken` is set, we're validating the token
        // and errors will be displayed in the UI.
        if (isHTTPAuthError(error) && !overrideAccessToken) {
            handleAccessTokenError(accessToken || '', sourcegraphURL).then(
                () => {},
                () => {}
            )
        }
        throw asError(error)
    }
}
