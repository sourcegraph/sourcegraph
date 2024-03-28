import { trimEnd } from 'lodash'

const GRAPHQL_URI = '/.api/graphql'

interface BuildGraphQLUrlOptions {
    request?: string
    baseUrl: string
}

/**
 * Constructs GraphQL Request URL
 */
export const buildGraphQLUrl = ({ request, baseUrl }: BuildGraphQLUrlOptions): string => {
    const nameMatch = request ? request.match(/^\s*(?:query|mutation)\s+(\w+)/) : ''
    const apiURL = `${GRAPHQL_URI}${nameMatch ? '?' + nameMatch[1] : ''}`
    return baseUrl ? new URL(trimEnd(baseUrl, '/') + apiURL).href : apiURL
}
