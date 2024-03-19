import { lastValueFrom } from 'rxjs'

import { gql, requestGraphQLCommon } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { CurrentAuthStateResult, CurrentAuthStateVariables } from '@sourcegraph/shared/src/graphql-operations'

export type SiteVersionAndCurrentAuthStateResult = CurrentAuthStateResult & {
    site: {
        productVersion: string
    }
}

// TODO: Could be deduplicated with `currentAuthStateQuery` in `shared/src/auth.ts`, using fragments
export const siteVersionAndUserQuery = gql`
    query SiteVersionAndCurrentUser {
        site {
            productVersion
        }
        currentUser {
            __typename
            id
            databaseID
            username
            avatarURL
            email
            displayName
            siteAdmin
            url
            settingsURL
            hasVerifiedEmail
            organizations {
                nodes {
                    id
                    name
                    displayName
                    url
                    settingsURL
                }
            }
            session {
                canSignOut
            }
            viewerCanAdminister
        }
    }
`

export interface SiteVersionAndCurrentUser {
    site: { productVersion: string } | null
    currentUser: AuthenticatedUser | null
}

export async function getSiteVersionAndAuthenticatedUser(
    instanceURL: string,
    accessToken: string | null,
    customRequestHeaders: { [name: string]: string } | null
): Promise<SiteVersionAndCurrentUser> {
    if (!instanceURL) {
        return { site: null, currentUser: null }
    }

    const result = await lastValueFrom(
        requestGraphQLCommon<SiteVersionAndCurrentAuthStateResult, CurrentAuthStateVariables>({
            request: siteVersionAndUserQuery,
            variables: {},
            baseUrl: instanceURL,
            headers: {
                Accept: 'application/json',
                'Content-Type': 'application/json',
                'X-Sourcegraph-Should-Trace': new URLSearchParams(window.location.search).get('trace') || 'false',
                ...(accessToken && { Authorization: `token ${accessToken}` }),
                ...customRequestHeaders,
            },
        })
    )

    return result.data ?? { site: null, currentUser: null }
}
