import { type Observable, of, throwError } from 'rxjs'

import type { GraphQLResult, SuccessGraphQLResult } from '@sourcegraph/http-client'
import type { CurrentAuthStateResult } from '@sourcegraph/shared/src/graphql-operations'
import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'

export interface GraphQLResponseMap {
    [requestName: string]: (
        variables: { [k: string]: any },
        mightContainPrivateInfo?: boolean
    ) => Observable<GraphQLResult<any>>
}

export const DEFAULT_GRAPHQL_RESPONSES: GraphQLResponseMap = {
    SiteProductVersion: () =>
        of({
            data: {
                site: {
                    productVersion: 'dev',
                    buildVersion: 'dev',
                    hasCodeIntelligence: true,
                },
            },
            errors: undefined,
        }),
    CurrentUSer: () =>
        of({
            data: {
                currentUser: {
                    id: 'u1',
                    displayName: 'Alice',
                    username: 'alice',
                    avatarURL: null,
                    url: 'https://example.com/alice',
                    settingsURL: 'https://example.com/alice/settings',
                    emails: [{ email: 'alice@example.com' }],
                    siteAdmin: false,
                },
            },
        } as SuccessGraphQLResult<CurrentAuthStateResult>),

    ResolveRev: () =>
        of({
            data: {
                repository: {
                    mirrorInfo: {
                        cloned: true,
                    },
                    commit: {
                        oid: 'foo',
                    },
                },
            },
            errors: undefined,
        }),
    BlobContent: () =>
        of({
            data: {
                repository: {
                    commit: {
                        file: {
                            content: 'Hello World',
                        },
                    },
                },
            },
            errors: undefined,
        }),
    ResolveRepo: variables =>
        of({
            data: {
                repository: {
                    name: variables.rawRepoName,
                },
            },
            errors: undefined,
        }),
}

/**
 * @param responseMap a {@link GraphQLResponseMap} of request names (eg. `ResolveRev`) to response builders.
 *
 * @returns a mock implementation of {@link PlatformContext#requestGraphQL}
 */
export const mockRequestGraphQL =
    (responseMap: GraphQLResponseMap = DEFAULT_GRAPHQL_RESPONSES): PlatformContext['requestGraphQL'] =>
    <R, V extends {} = object>({
        request,
        variables,
        mightContainPrivateInfo,
    }: {
        request: string
        variables: V
        mightContainPrivateInfo?: boolean
    }) => {
        const nameMatch = request.match(/^\s*(?:query|mutation)\s+(\w+)/)
        const requestName = nameMatch?.[1]
        if (!requestName || !responseMap[requestName]) {
            return throwError(() => new Error(`No mock for GraphQL request ${String(requestName)}`))
        }
        return responseMap[requestName](variables, mightContainPrivateInfo)
    }
