import React from 'react'

import { useLocation } from 'react-router-dom'

import { Container, Link, H3, Text } from '@sourcegraph/wildcard'

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
    type UserBatchChangesCodeHostsResult,
    type UserAreaUserFields,
} from '../../../graphql-operations'

import { useGlobalBatchChangesCodeHostConnection, useUserBatchChangesCodeHostConnection } from './backend'
import { CodeHostConnectionNode } from './CodeHostConnectionNode'

export interface GlobalCodeHostConnectionsProps {
    headerLine: JSX.Element
}

export const GlobalCodeHostConnections: React.FunctionComponent<
    React.PropsWithChildren<GlobalCodeHostConnectionsProps>
> = props => <CodeHostConnections user={null} connectionResult={useGlobalBatchChangesCodeHostConnection()} {...props} />

export interface UserCodeHostConnectionsProps extends GlobalCodeHostConnectionsProps {
    user: UserAreaUserFields
}

export const UserCodeHostConnections: React.FunctionComponent<
    React.PropsWithChildren<UserCodeHostConnectionsProps>
> = ({ user, headerLine }) => (
    <CodeHostConnections
        connectionResult={useUserBatchChangesCodeHostConnection(user.id)}
        headerLine={headerLine}
        user={user}
    />
)

interface CodeHostConnectionsProps extends GlobalCodeHostConnectionsProps {
    user: UserAreaUserFields | null
    connectionResult: UseShowMorePaginationResult<
        GlobalBatchChangesCodeHostsResult | UserBatchChangesCodeHostsResult,
        BatchChangesCodeHostFields
    >
}

const CodeHostConnections: React.FunctionComponent<React.PropsWithChildren<CodeHostConnectionsProps>> = ({
    user,
    headerLine,
    connectionResult,
}) => {
    const { loading, hasNextPage, fetchMore, connection, error, refetchAll } = connectionResult
    const location = useLocation()
    const kind = new URLSearchParams(location.search).get('kind')
    const success = new URLSearchParams(location.search).get('success') === 'true'
    const appName = new URLSearchParams(location.search).get('app_name')
    const setupError = new URLSearchParams(location.search).get('error')
    const shouldShowError = !success && setupError && kind !== GitHubAppKind.COMMIT_SIGNING
    return (
        <Container className="mb-3">
            <H3>Code host tokens</H3>
            {headerLine}
            <ConnectionContainer className="mb-3">
                {error && <ConnectionError errors={[error.message]} />}
                {loading && !connection && <ConnectionLoading />}
                {success && (
                    <DismissibleAlert
                        className="mb-3"
                        variant="success"
                        partialStorageKey="batch-changes-github-app-integration-success"
                    >
                        GitHub App {appName?.length ? `"${appName}" ` : ''}successfully connected.
                    </DismissibleAlert>
                )}
                {shouldShowError && <GitHubAppFailureAlert error={setupError} />}
                <ConnectionList as="ul" className="list-group" aria-label="code host connections">
                    {connection?.nodes?.map(node => (
                        <CodeHostConnectionNode
                            key={node.externalServiceURL}
                            node={node}
                            refetchAll={refetchAll}
                            user={user}
                        />
                    ))}
                </ConnectionList>
                {connection && (
                    <SummaryContainer className="mt-2">
                        <ConnectionSummary
                            noSummaryIfAllNodesVisible={true}
                            first={15}
                            centered={true}
                            connection={connection}
                            noun="code host"
                            pluralNoun="code host connections"
                            hasNextPage={hasNextPage}
                        />
                        {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                    </SummaryContainer>
                )}
            </ConnectionContainer>
            <Text className="mb-0">
                Code host not present? Site admins can add a code host in{' '}
                <Link to="/help/admin/external_service" target="_blank" rel="noopener noreferrer">
                    the manage repositories settings
                </Link>
                .
            </Text>
        </Container>
    )
}
