import { useCallback, type FC } from 'react'

import { capitalize } from 'lodash'
import { useLocation } from 'react-router-dom'

import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { Link } from '@sourcegraph/wildcard'

import { CreateGitHubAppPage } from '../../../components/gitHubApps/CreateGitHubAppPage'
import { GitHubAppDomain, GitHubAppKind } from '../../../graphql-operations'

import { useGlobalBatchChangesCodeHostConnection } from './backend'

const DEFAULT_EVENTS: string[] = []

const DEFAULT_PERMISSIONS = {
    contents: 'write',
    metadata: 'read',
}

interface BatchChangesCreateGitHubAppPageProps {
    authenticatedUser: AuthenticatedUser
    minimizedMode?: boolean
    kind: GitHubAppKind
}

export const BatchChangesCreateGitHubAppPage: FC<BatchChangesCreateGitHubAppPageProps> = ({
    authenticatedUser,
    minimizedMode,
    kind,
}) => {
    const location = useLocation()
    const searchParams = new URLSearchParams(location.search)
    const baseURL = searchParams.get('baseURL')

    const isKindCredential = kind !== GitHubAppKind.COMMIT_SIGNING

    const { connection } = useGlobalBatchChangesCodeHostConnection()
    // validateURL compares a provided URL against the URLs of existing commit signing
    // integrations for code hosts, to ensure the new GH App is for a unique code host.
    const validateURL = useCallback(
        (url: string): true | string => {
            if (!connection) {
                // We don't have a connection yet, so we can't validate that the URL is
                // unique, but we shouldn't block on that.
                return true
            }
            // The default validator already checks that the URL is a valid URL, so we
            // assume this call will succeed.
            const asURL = new URL(url)
            const isDuplicate = connection.nodes.some(node => {
                const existingURL = isKindCredential
                    ? node.externalServiceURL
                    : node.commitSigningConfiguration?.baseURL
                if (!existingURL) {
                    return false
                }

                return new URL(existingURL).hostname === asURL.hostname
            })
            const errorMsg = `A ${
                isKindCredential ? 'GitHub app' : 'commit signing'
            } integration for the code host at this URL already exists.`
            return isDuplicate ? errorMsg : true
        },
        [connection, isKindCredential]
    )
    const pageTitle = isKindCredential
        ? `Create GitHub app for ${
              kind === GitHubAppKind.USER_CREDENTIAL ? authenticatedUser.username : 'Global'
          } Batch Changes credential`
        : 'Create GitHub app for commit signing'
    const defaultAppName = computeAppName(authenticatedUser.username, kind)
    return (
        <CreateGitHubAppPage
            defaultEvents={DEFAULT_EVENTS}
            defaultPermissions={DEFAULT_PERMISSIONS}
            pageTitle={pageTitle}
            minimizedMode={minimizedMode}
            headerDescription={
                <>
                    Register a GitHub App to enable Sourcegraph {isKindCredential ? 'create' : 'sign commits for'} Batch
                    Change changesets on your behalf.
                    {/* TODO (@BolajiOlajide) update link here for credential github app */}
                    <Link to="/help/admin/config/batch_changes#commit-signing-for-github" className="ml-1">
                        See how GitHub App configuration works.
                    </Link>
                </>
            }
            appDomain={GitHubAppDomain.BATCHES}
            appKind={kind}
            defaultAppName={defaultAppName}
            baseURL={baseURL?.length ? baseURL : undefined}
            validateURL={validateURL}
            telemetryRecorder={noOpTelemetryRecorder}
            authenticatedUser={authenticatedUser}
        />
    )
}

const computeAppName = (username: string, kind: GitHubAppKind): string => {
    switch (kind) {
        case GitHubAppKind.COMMIT_SIGNING: {
            return 'Sourcegraph Commit Signing'
        }

        case GitHubAppKind.USER_CREDENTIAL: {
            return `${capitalize(username)}'s Batch Changes GitHub App`
        }

        default: {
            return 'Batch Changes GitHub App'
        }
    }
}
