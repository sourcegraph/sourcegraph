import React, { type FC, useCallback, useMemo } from 'react'

import { mdiMapSearch } from '@mdi/js'
import classNames from 'classnames'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Container, PageHeader, H3, H5, Icon } from '@sourcegraph/wildcard'

import { FilteredConnection, type FilteredConnectionQueryArguments } from '../../components/FilteredConnection'
import { PageTitle } from '../../components/PageTitle'
import type { BatchSpecListFields, Scalars } from '../../graphql-operations'

import {
    queryBatchSpecs as _queryBatchSpecs,
    queryBatchChangeBatchSpecs as _queryBatchChangeBatchSpecs,
} from './backend'
import { BatchSpecNode, type BatchSpecNodeProps } from './BatchSpecNode'

import styles from './BatchSpecsPage.module.scss'

export interface BatchSpecsPageProps extends TelemetryV2Props {
    queryBatchSpecs?: typeof _queryBatchSpecs

    /** For testing purposes only. Sets the current date */
    now?: () => Date
}

export const BatchSpecsPage: FC<BatchSpecsPageProps> = props => (
    <>
        <PageTitle title="Batch specs" />
        <PageHeader
            headingElement="h2"
            path={[{ text: 'Batch specs' }]}
            description="All batch specs that currently exist."
            className="mb-3"
        />
        <Container>
            <BatchSpecList
                queryBatchSpecs={props.queryBatchSpecs}
                now={props.now}
                telemetryRecorder={props.telemetryRecorder}
            />
        </Container>
    </>
)

export interface BatchChangeBatchSpecListProps extends Omit<BatchSpecListProps, 'queryBatchSpecs'> {
    batchChangeID: Scalars['ID']
    currentSpecID: Scalars['ID']
    queryBatchChangeBatchSpecs?: typeof _queryBatchChangeBatchSpecs
}

export const BatchChangeBatchSpecList: React.FunctionComponent<
    React.PropsWithChildren<BatchChangeBatchSpecListProps>
> = ({
    batchChangeID,
    currentSpecID,
    queryBatchChangeBatchSpecs = _queryBatchChangeBatchSpecs,
    now,
    telemetryRecorder,
}) => {
    const query = useMemo(() => queryBatchChangeBatchSpecs(batchChangeID), [queryBatchChangeBatchSpecs, batchChangeID])

    return (
        <BatchSpecList
            queryBatchSpecs={query}
            currentSpecID={currentSpecID}
            now={now}
            telemetryRecorder={telemetryRecorder}
        />
    )
}

export interface BatchSpecListProps extends TelemetryV2Props {
    currentSpecID?: Scalars['ID']
    queryBatchSpecs?: typeof _queryBatchSpecs
    /** For testing purposes only. Sets the current date */
    now?: () => Date
}

export const BatchSpecList: React.FunctionComponent<React.PropsWithChildren<BatchSpecListProps>> = ({
    currentSpecID,
    queryBatchSpecs = _queryBatchSpecs,
    now,
    telemetryRecorder,
}) => {
    const query = useCallback(
        (args: FilteredConnectionQueryArguments) => {
            const passedArguments = {
                first: args.first ?? null,
                after: args.after ?? null,
                includeLocallyExecutedSpecs: false,
                excludeEmptySpecs: true,
            }
            return queryBatchSpecs(passedArguments)
        },
        [queryBatchSpecs]
    )
    return (
        <FilteredConnection<BatchSpecListFields, Omit<BatchSpecNodeProps, 'node'>>
            nodeComponent={BatchSpecNode}
            nodeComponentProps={{ currentSpecID, now, telemetryRecorder }}
            queryConnection={query}
            hideSearch={true}
            defaultFirst={20}
            noun="batch spec"
            pluralNoun="batch specs"
            listClassName={classNames(styles.specsGrid, 'test-batches-executions')}
            withCenteredSummary={true}
            headComponent={Header}
            cursorPaging={true}
            noSummaryIfAllNodesVisible={true}
            emptyElement={<EmptyList />}
        />
    )
}

const Header: React.FunctionComponent<React.PropsWithChildren<{}>> = () => (
    <>
        <span className="d-none d-md-block" />
        <H5 as={H3} aria-hidden={true} className="p-2 d-none d-md-block text-uppercase text-center text-nowrap">
            State
        </H5>
        <H5 as={H3} aria-hidden={true} className="p-2 d-none d-md-block text-uppercase text-nowrap">
            Batch spec
        </H5>
        <H5 as={H3} aria-hidden={true} className="d-none d-md-block text-uppercase text-center text-nowrap">
            Execution time
        </H5>
    </>
)

const EmptyList: React.FunctionComponent<React.PropsWithChildren<{}>> = () => (
    <div className="text-muted text-center mb-3 w-100">
        <Icon className="icon" svgPath={mdiMapSearch} inline={false} aria-hidden={true} />
        <div className="pt-2">No batch specs have been created so far.</div>
    </div>
)
