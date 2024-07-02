import React, { useCallback, useEffect } from 'react'

import { useApolloClient } from '@apollo/client'
import { mdiMapSearch } from '@mdi/js'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { Container, H3, Icon, Link, PageHeader, Text } from '@sourcegraph/wildcard'

import {
    FilteredConnection,
    type Filter,
    type FilteredConnectionQueryArguments,
} from '../../../components/FilteredConnection'
import { PageTitle } from '../../../components/PageTitle'
import type { ExecutorFields } from '../../../graphql-operations'

import { ExecutorNode } from './ExecutorNode'
import { queryExecutors as defaultQueryExecutors } from './useExecutors'

const filters: Filter[] = [
    {
        id: 'filters',
        label: 'State',
        type: 'select',
        options: [
            {
                label: 'All',
                value: 'all',
                tooltip: 'Show all executors',
                args: {},
            },
            {
                label: 'Active',
                value: 'active',
                tooltip: 'Show only active executors',
                args: { active: true },
            },
        ],
    },
]

export interface ExecutorsListPageProps extends TelemetryV2Props {
    queryExecutors?: typeof defaultQueryExecutors
}

export const ExecutorsListPage: React.FC<ExecutorsListPageProps> = ({
    queryExecutors = defaultQueryExecutors,
    telemetryRecorder,
}) => {
    useEffect(() => {
        EVENT_LOGGER.logViewEvent('ExecutorsList')
        telemetryRecorder.recordEvent('admin.executors.list', 'view')
    }, [telemetryRecorder])

    const apolloClient = useApolloClient()
    const queryExecutorsCallback = useCallback(
        (args: FilteredConnectionQueryArguments) => queryExecutors(args, apolloClient),
        [queryExecutors, apolloClient]
    )

    return (
        <>
            <PageTitle title="Executor instances" />
            <PageHeader
                headingElement="h2"
                path={[
                    {
                        text: <>Executor instances</>,
                    },
                ]}
                description="The executor instances attached to your Sourcegraph instance."
                className="mb-3"
            />

            <Container className="mb-3">
                <H3>Setting up executors</H3>
                <Text className="mb-0">
                    Executors enable{' '}
                    <Link to="/help/code_navigation/explanations/auto_indexing" rel="noopener">
                        auto-indexing for code navigation
                    </Link>{' '}
                    and{' '}
                    <Link to="/help/batch_changes/explanations/server_side" rel="noopener">
                        running batch changes server-side
                    </Link>
                    . In order to use those features,{' '}
                    <Link to="/help/admin/executors/deploy_executors" rel="noopener">
                        set them up
                    </Link>
                    .
                </Text>
            </Container>
            <Container className="mb-3">
                <FilteredConnection<ExecutorFields>
                    listComponent="ul"
                    listClassName="list-group mb-2"
                    showMoreClassName="mb-0"
                    noun="executor"
                    pluralNoun="executors"
                    nodeComponent={ExecutorNode}
                    nodeComponentProps={{}}
                    queryConnection={queryExecutorsCallback}
                    cursorPaging={true}
                    filters={filters}
                    emptyElement={<NoExecutors />}
                    noSummaryIfAllNodesVisible={true}
                    withCenteredSummary={true}
                />
            </Container>
        </>
    )
}

export const NoExecutors: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <Text alignment="center" className="text-muted w-100 mb-0 mt-1">
        <Icon className="mb-2" svgPath={mdiMapSearch} inline={false} aria-hidden={true} />
        <br />
        No executors found.
    </Text>
)
