import { type Observable, ReplaySubject } from 'rxjs'
import type * as vscode from 'vscode'

import { gql } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import type { CurrentAuthStateResult, CurrentAuthStateVariables } from '@sourcegraph/shared/src/graphql-operations'

import { scretTokenKey } from '../webview/platform/AuthProvider'

import { requestGraphQLFromVSCode } from './requestGraphQl'

// Minimal auth state for the VS Code extension.
// Uses only old fields for backwards compatibility with old GraphQL API versions.
const currentAuthStateQuery = gql`
    query CurrentAuthState {
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

// Update authenticatedUser on accessToken changes
export function observeAuthenticatedUser(secretStorage: vscode.SecretStorage): Observable<AuthenticatedUser | null> {
    const authenticatedUsers = new ReplaySubject<AuthenticatedUser | null>(1)

    function updateAuthenticatedUser(): void {
        requestGraphQLFromVSCode<CurrentAuthStateResult, CurrentAuthStateVariables>(currentAuthStateQuery, {})
            .then(authenticatedUserResult => {
                authenticatedUsers.next(authenticatedUserResult.data ? authenticatedUserResult.data.currentUser : null)
                if (!authenticatedUserResult.data) {
                    throw new Error('Not an authenticated user')
                }
            })
            .catch(error => {
                console.error('core auth error', error)
                // TODO surface error?
                authenticatedUsers.next(null)
            })
    }

    // Initial authenticated user
    updateAuthenticatedUser()

    secretStorage.onDidChange(async event => {
        if (event.key === scretTokenKey) {
            const token = await secretStorage.get(scretTokenKey)
            if (token) {
                updateAuthenticatedUser()
            }
        }
    })

    return authenticatedUsers
}
