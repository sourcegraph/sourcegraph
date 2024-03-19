import React, { type FC, useCallback, useState, useEffect } from 'react'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Button, Container, Link, PageHeader } from '@sourcegraph/wildcard'

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
import {
    type ExecutorSecretFields,
    ExecutorSecretScope,
    type GlobalExecutorSecretsResult,
    type OrgExecutorSecretsResult,
    type Scalars,
    type UserExecutorSecretsResult,
} from '../../../graphql-operations'

import { AddSecretModal } from './AddSecretModal'
import {
    globalExecutorSecretsConnectionFactory,
    userExecutorSecretsConnectionFactory,
    orgExecutorSecretsConnectionFactory,
} from './backend'
import { ExecutorSecretNode } from './ExecutorSecretNode'
import { ExecutorSecretScopeSelector } from './ExecutorSecretScopeSelector'

export interface GlobalExecutorSecretsListPageProps extends TelemetryV2Props {}

export const GlobalExecutorSecretsListPage: FC<GlobalExecutorSecretsListPageProps> = props => {
    useEffect(
        () => props.telemetryRecorder.recordEvent('executors.globalSecretsList', 'view'),
        [props.telemetryRecorder]
    )
    const connectionLoader = useCallback(
        (scope: ExecutorSecretScope) => globalExecutorSecretsConnectionFactory(scope),
        []
    )
    return (
        <ExecutorSecretsListPage
            namespaceID={null}
            headerLine={<>Configure executor secrets that will be available to everyone on the Sourcegraph instance.</>}
            connectionLoader={connectionLoader}
            {...props}
        />
    )
}

export interface UserExecutorSecretsListPageProps extends GlobalExecutorSecretsListPageProps {
    userID: Scalars['ID']
}

export const UserExecutorSecretsListPage: FC<UserExecutorSecretsListPageProps> = props => {
    useEffect(() => props.telemetryRecorder.recordEvent('executors.userSecretsList', 'view'), [props.telemetryRecorder])
    const connectionLoader = useCallback(
        (scope: ExecutorSecretScope) => userExecutorSecretsConnectionFactory(props.userID, scope),
        [props.userID]
    )
    return (
        <ExecutorSecretsListPage
            namespaceID={props.userID}
            headerLine={
                <>
                    Configure executor secrets that will only be available to your executions.
                    <br />
                    Global secrets are available to executions in this namespace. Secrets in this namespace with the
                    same name as a global secret will overwrite the global secret. Site admins can configure global
                    secrets <Link to="/admin/executors/secrets">in site admin settings</Link>.
                </>
            }
            connectionLoader={connectionLoader}
            {...props}
        />
    )
}

export interface OrgExecutorSecretsListPageProps extends GlobalExecutorSecretsListPageProps {
    orgID: Scalars['ID']
}

export const OrgExecutorSecretsListPage: FC<OrgExecutorSecretsListPageProps> = props => {
    useEffect(() => props.telemetryRecorder.recordEvent('executors.orgSecretsList', 'view'), [props.telemetryRecorder])
    const connectionLoader = useCallback(
        (scope: ExecutorSecretScope) => orgExecutorSecretsConnectionFactory(props.orgID, scope),
        [props.orgID]
    )
    return (
        <ExecutorSecretsListPage
            namespaceID={props.orgID}
            headerLine={
                <>
                    Configure executor secrets that will only be available to executions in this organization.
                    <br />
                    Global secrets are available to executions in this namespace. Secrets in this namespace with the
                    same name as a global secret will overwrite the global secret. Site admins can configure global
                    secrets <Link to="/admin/executors/secrets">in site admin settings</Link>.
                </>
            }
            connectionLoader={connectionLoader}
            {...props}
        />
    )
}

export interface ExecutorSecretsListPageProps extends GlobalExecutorSecretsListPageProps {
    namespaceID: Scalars['ID'] | null
    headerLine: JSX.Element
    connectionLoader: (
        scope: ExecutorSecretScope
    ) => UseShowMorePaginationResult<
        OrgExecutorSecretsResult | UserExecutorSecretsResult | GlobalExecutorSecretsResult,
        ExecutorSecretFields
    >
}

const ExecutorSecretsListPage: FC<ExecutorSecretsListPageProps> = ({
    namespaceID,
    headerLine,
    connectionLoader,
    telemetryRecorder,
}) => {
    const [selectedScope, setSelectedScope] = useState<ExecutorSecretScope>(ExecutorSecretScope.BATCHES)
    const { loading, hasNextPage, fetchMore, connection, error, refetchAll } = connectionLoader(selectedScope)

    const [showAddModal, setShowAddModal] = useState<boolean>(false)
    const onClickAdd = useCallback<React.MouseEventHandler>(
        event => {
            event.preventDefault()
            telemetryRecorder.recordEvent('executors.addSecret', 'click')
            setShowAddModal(true)
        },
        [telemetryRecorder]
    )

    const closeModal = useCallback(() => {
        telemetryRecorder.recordEvent('executors.addSecret', 'cancel')
        setShowAddModal(false)
    }, [telemetryRecorder])
    const afterAction = useCallback(() => {
        telemetryRecorder.recordEvent('executors.addSecret', 'submit')
        setShowAddModal(false)
        refetchAll()
    }, [refetchAll, telemetryRecorder])

    return (
        <>
            <PageHeader
                path={[{ text: 'Executor secrets' }]}
                headingElement="h2"
                description={headerLine}
                actions={
                    <>
                        <Button onClick={onClickAdd} variant="primary">
                            Add secret
                        </Button>
                    </>
                }
                className="mb-3"
            />

            {showAddModal && (
                <AddSecretModal
                    onCancel={closeModal}
                    afterCreate={afterAction}
                    scope={selectedScope}
                    namespaceID={namespaceID}
                />
            )}

            <div className="d-flex mb-3">
                {(namespaceID === null ? Object.values(ExecutorSecretScope) : [ExecutorSecretScope.BATCHES]).map(
                    scope => (
                        <ExecutorSecretScopeSelector
                            key={scope}
                            scope={scope}
                            label={executorSecretScopeContext(scope).label}
                            onSelect={() => setSelectedScope(scope)}
                            selected={scope === selectedScope}
                            description={executorSecretScopeContext(scope).description}
                        />
                    )
                )}
            </div>

            <Container>
                <ConnectionContainer className="mb-3">
                    {error && <ConnectionError errors={[error.message]} />}
                    {loading && !connection && <ConnectionLoading />}
                    <ConnectionList as="ul" className="list-group" aria-label="Executor secrets">
                        {connection?.nodes?.map(node => (
                            <ExecutorSecretNode
                                key={node.id}
                                node={node}
                                namespaceID={namespaceID}
                                refetchAll={refetchAll}
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
                                noun="executor secret"
                                pluralNoun="executor secrets"
                                hasNextPage={hasNextPage}
                            />
                            {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                        </SummaryContainer>
                    )}
                </ConnectionContainer>
            </Container>
        </>
    )
}

// When you add a new secret scope, this function will generate a compile error,
// so that this is not forgotten. Add a new label/description tuple for the new
// scope added here and TS will be happy.
function executorSecretScopeContext(scope: ExecutorSecretScope): { label: string; description: string } {
    switch (scope) {
        case ExecutorSecretScope.BATCHES: {
            return { label: 'Batch changes', description: 'Batch change execution secrets' }
        }
        case ExecutorSecretScope.CODEINTEL: {
            return { label: 'Code graph', description: 'Code graph execution secrets' }
        }
    }
}
