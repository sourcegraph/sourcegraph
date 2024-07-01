import { useEffect, type FunctionComponent, type MutableRefObject } from 'react'

import { mdiPlus } from '@mdi/js'
import classNames from 'classnames'
import { useLocation } from 'react-router-dom'
import { useCallbackRef } from 'use-callback-ref'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Button,
    Container,
    ErrorAlert,
    H2,
    Icon,
    Link,
    LoadingSpinner,
    PageHeader,
    PageSwitcher,
} from '@sourcegraph/wildcard'

import { usePageSwitcherPagination } from '../components/FilteredConnection/hooks/usePageSwitcherPagination'
import { PageTitle } from '../components/PageTitle'
import type { WorkflowFields, WorkflowsResult, WorkflowsVariables } from '../graphql-operations'
import type { NamespaceProps } from '../namespaces'
import { namespaceTelemetryMetadata } from '../namespaces/telemetry'

import { workflowsQuery } from './backend'
import { WorkflowNameWithOwner } from './WorkflowNameWithOwner'

import styles from './WorkflowListPage.module.scss'

const WorkflowNode: FunctionComponent<
    TelemetryV2Props & {
        workflow: WorkflowFields
        linkRef: MutableRefObject<HTMLAnchorElement | null> | null
    }
> = ({ workflow }) => (
    <div className={classNames(styles.row, 'list-group-item test-workflow-list-page-row')}>
        <div className="flex-1">
            <H2 className="text-base mb-0 font-weight-normal">
                <WorkflowNameWithOwner workflow={workflow} />
            </H2>
        </div>
        <div className="flex-0">
            <Button to={workflow.id} variant="secondary" as={Link}>
                Edit
            </Button>
        </div>
    </div>
)

export const WorkflowListPage: FunctionComponent<NamespaceProps> = ({ namespace, telemetryRecorder }) => {
    useEffect(() => {
        telemetryRecorder.recordEvent('workflows.list', 'view', {
            metadata: namespaceTelemetryMetadata(namespace),
        })
    }, [telemetryRecorder, namespace])

    const { connection, loading, error, refetch, ...paginationProps } = usePageSwitcherPagination<
        WorkflowsResult,
        WorkflowsVariables,
        WorkflowFields
    >({
        query: workflowsQuery,
        variables: { owner: namespace.id, query: null, viewerIsAffiliated: null, includeDrafts: true },
        getConnection: ({ data }) => data?.workflows || undefined,
    })

    return (
        <div data-testid="workflows-list-page" className="p-0">
            <PageHeader
                actions={
                    <Button to="new" className="test-new-workflow-button" variant="primary" as={Link}>
                        <Icon aria-hidden={true} svgPath={mdiPlus} /> New workflow
                    </Button>
                }
                className="mb-3"
            >
                <PageTitle title="Workflows" />
                <PageHeader.Heading as="h3" styleAs="h2">
                    <PageHeader.Breadcrumb>Workflows</PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>
            <WorkflowListPageContent
                namespace={namespace}
                telemetryRecorder={telemetryRecorder}
                workflows={connection?.nodes || []}
                error={error}
                loading={loading}
            />
            <PageSwitcher {...paginationProps} className="mt-4" totalCount={connection?.totalCount || 0} />
        </div>
    )
}

const WorkflowListPageContent: FunctionComponent<
    {
        workflows: WorkflowFields[]
        error: unknown
        loading: boolean
    } & NamespaceProps &
        TelemetryV2Props
> = ({ workflows, error, loading, telemetryRecorder }) => {
    const location = useLocation()
    const callbackReference = useCallbackRef<HTMLAnchorElement>(null, ref => ref?.focus())

    if (loading) {
        return <LoadingSpinner />
    }

    if (error) {
        return <ErrorAlert className="mb-3" error={error} />
    }

    if (workflows.length === 0) {
        return <Container className="text-center text-muted">You haven't created a workflow yet.</Container>
    }

    return (
        <Container>
            <div className="list-group list-group-flush">
                {workflows.map(workflow => (
                    <WorkflowNode
                        key={workflow.id}
                        linkRef={location.state?.description === workflow.description ? callbackReference : null}
                        workflow={workflow}
                        telemetryRecorder={telemetryRecorder}
                    />
                ))}
            </div>
        </Container>
    )
}
