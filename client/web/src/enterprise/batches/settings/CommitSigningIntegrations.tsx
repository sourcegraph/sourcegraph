import React from 'react'

import { useLocation } from 'react-router-dom'

import { Container, H3, Link, ProductStatusBadge, Text } from '@sourcegraph/wildcard'

import { DismissibleAlert } from '../../../components/DismissibleAlert'
import type { UseShowMorePaginationResult } from '../../../components/FilteredConnection/hooks/useShowMorePagination'
import {
    ConnectionContainer,
    ConnectionError,
    ConnectionList,
    ConnectionLoading,
    ConnectionSummary,
    ShowMoreButton,
    SummaryContainer,
} from '../../../components/FilteredConnection/ui'
import { GitHubAppFailureAlert } from '../../../components/gitHubApps/GitHubAppFailureAlert'
import {
    type BatchChangesCodeHostFields,
    GitHubAppKind,
    type GlobalBatchChangesCodeHostsResult,
    type Scalars,
    type UserBatchChangesCodeHostsResult,
} from '../../../graphql-operations'

import { useGlobalBatchChangesCodeHostConnection, useUserBatchChangesCodeHostConnection } from './backend'
import { CommitSigningIntegrationNode } from './CommitSigningIntegrationNode'

export const GlobalCommitSigningIntegrations: React.FunctionComponent<React.PropsWithChildren<{}>> = () => (
    <CommitSigningIntegrations connectionResult={useGlobalBatchChangesCodeHostConnection()} readOnly={false} />
)

interface UserCommitSigningIntegrationsProps {
    userID: Scalars['ID']
}

export const UserCommitSigningIntegrations: React.FunctionComponent<
    React.PropsWithChildren<UserCommitSigningIntegrationsProps>
> = ({ userID }) => (
    <CommitSigningIntegrations connectionResult={useUserBatchChangesCodeHostConnection(userID)} readOnly={true} />
)

interface CommitSigningIntegrationsProps {
    readOnly: boolean
    connectionResult: UseShowMorePaginationResult<
        GlobalBatchChangesCodeHostsResult | UserBatchChangesCodeHostsResult,
        BatchChangesCodeHostFields
    >
}

export const CommitSigningIntegrations: React.FunctionComponent<
    React.PropsWithChildren<CommitSigningIntegrationsProps>
> = ({ connectionResult, readOnly }) => {
    const { loading, hasNextPage, fetchMore, connection, error, refetchAll } = connectionResult

    const location = useLocation()
    const kind = new URLSearchParams(location.search).get('kind')
    const success = new URLSearchParams(location.search).get('success') === 'true'
    const appName = new URLSearchParams(location.search).get('app_name')
    const setupError = new URLSearchParams(location.search).get('error')
    const shouldShowError = !success && setupError && !readOnly && kind === GitHubAppKind.COMMIT_SIGNING
    return (
        <Container>
            <H3>
                Commit signing integrations
                <ProductStatusBadge status="beta" className="ml-2" />
            </H3>
            <Text>
                Connect GitHub Apps to enable Batch Changes to sign commits for your changesets.{' '}
                {readOnly ? (
                    'Contact your site admin to manage connections.'
                ) : (
                    <Link to="/help/admin/config/batch_changes#commit-signing-for-github" target="_blank">
                        See how Batch Changes GitHub App configuration works.
                    </Link>
                )}
            </Text>
            <ConnectionContainer className="mb-3">
                {error && <ConnectionError errors={[error.message]} />}
                {loading && !connection && <ConnectionLoading />}
                {success && !readOnly && (
                    <DismissibleAlert
                        className="mb-3"
                        variant="success"
                        partialStorageKey="batch-changes-commit-signing-integration-success"
                    >
                        GitHub App {appName?.length ? `"${appName}" ` : ''}successfully connected.
                    </DismissibleAlert>
                )}
                {shouldShowError && <GitHubAppFailureAlert error={setupError} />}
                <ConnectionList as="ul" className="list-group" aria-label="commit signing integrations">
                    {connection?.nodes?.map(node =>
                        node.supportsCommitSigning ? (
                            <CommitSigningIntegrationNode
                                key={node.externalServiceURL}
                                node={node}
                                readOnly={readOnly}
                                refetch={refetchAll}
                            />
                        ) : null
                    )}
                </ConnectionList>
                {connection && (
                    <SummaryContainer className="mt-2">
                        <ConnectionSummary
                            noSummaryIfAllNodesVisible={true}
                            first={30}
                            centered={true}
                            connection={connection}
                            noun="code host commit signing integration"
                            pluralNoun="code host commit signing integrations"
                            hasNextPage={hasNextPage}
                        />
                        {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                    </SummaryContainer>
                )}
            </ConnectionContainer>
            <Text className="mb-0">
                Code host not present? Batch Changes only supports commit signing on GitHub code hosts today.
            </Text>
        </Container>
    )
}
