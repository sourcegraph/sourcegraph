import React, { useState, useCallback, useMemo, useEffect, useContext } from 'react'

import { Subject } from 'rxjs'

import { dataOrThrowErrors } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Container } from '@sourcegraph/wildcard'

import { useShowMorePagination } from '../../../../components/FilteredConnection/hooks/useShowMorePagination'
import {
    ConnectionContainer,
    ConnectionError,
    ConnectionList,
    ConnectionLoading,
    ConnectionSummary,
    ShowMoreButton,
    SummaryContainer,
} from '../../../../components/FilteredConnection/ui'
import {
    type ExternalChangesetFields,
    type HiddenExternalChangesetFields,
    type Scalars,
    type BatchChangeChangesetsResult,
    type BatchChangeChangesetsVariables,
    BatchChangeState,
} from '../../../../graphql-operations'
import { MultiSelectContext, MultiSelectContextProvider } from '../../MultiSelectContext'
import {
    type queryExternalChangesetWithFileDiffs as _queryExternalChangesetWithFileDiffs,
    queryAllChangesetIDs as _queryAllChangesetIDs,
    CHANGESETS,
} from '../backend'

import { BatchChangeChangesetsHeader } from './BatchChangeChangesetsHeader'
import { type ChangesetFilters, ChangesetFilterRow } from './ChangesetFilterRow'
import { ChangesetNode } from './ChangesetNode'
import { ChangesetSelectRow } from './ChangesetSelectRow'
import { EmptyArchivedChangesetListElement } from './EmptyArchivedChangesetListElement'
import { EmptyChangesetListElement } from './EmptyChangesetListElement'
import { EmptyChangesetSearchElement } from './EmptyChangesetSearchElement'
import { EmptyDraftChangesetListElement } from './EmptyDraftChangesetListElement'

import styles from './BatchChangeChangesets.module.scss'

interface Props extends TelemetryV2Props {
    batchChangeID: Scalars['ID']
    batchChangeState: BatchChangeState
    isExecutionEnabled: boolean
    viewerCanAdminister: boolean

    hideFilters?: boolean
    onlyArchived?: boolean
    refetchBatchChange: () => void

    /** For testing only. */
    queryExternalChangesetWithFileDiffs?: typeof _queryExternalChangesetWithFileDiffs
    /** For testing only. */
    queryAllChangesetIDs?: typeof _queryAllChangesetIDs
    /** For testing only. */
    expandByDefault?: boolean
}

/**
 * A list of a batch change's changesets.
 */
export const BatchChangeChangesets: React.FunctionComponent<React.PropsWithChildren<Props>> = props => (
    <MultiSelectContextProvider>
        <BatchChangeChangesetsImpl {...props} />
    </MultiSelectContextProvider>
)

const BATCH_COUNT = 15

const BatchChangeChangesetsImpl: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    batchChangeID,
    viewerCanAdminister,
    hideFilters = false,
    queryAllChangesetIDs = _queryAllChangesetIDs,
    queryExternalChangesetWithFileDiffs,
    expandByDefault,
    onlyArchived,
    refetchBatchChange,
    batchChangeState,
    isExecutionEnabled,
    telemetryRecorder,
}) => {
    // You might look at this destructuring statement and wonder why this isn't
    // just a single context consumer object. The reason is because making it a
    // single object makes it hard to have hooks that depend on individual
    // callbacks and objects within the context. Therefore, we'll have a nice,
    // ugly destructured set of variables here.
    const { selected, deselectAll, areAllVisibleSelected, isSelected, toggleSingle, toggleVisible, setVisible } =
        useContext(MultiSelectContext)

    const [changesetFilters, setChangesetFilters] = useState<ChangesetFilters>({
        checkState: null,
        state: null,
        reviewState: null,
        search: null,
    })

    const setChangesetFiltersAndDeselectAll = useCallback(
        (filters: ChangesetFilters) => {
            deselectAll()
            setChangesetFilters(filters)
        },
        [deselectAll, setChangesetFilters]
    )

    // After selecting and performing a bulk action, deselect all changesets and refetch
    // the batch change to get the actively-running bulk operations.
    const onSubmitBulkAction = useCallback(() => {
        deselectAll()
        refetchBatchChange()
    }, [deselectAll, refetchBatchChange])

    const queryArguments = useMemo(
        () => ({
            state: changesetFilters.state,
            reviewState: changesetFilters.reviewState,
            checkState: changesetFilters.checkState,
            batchChange: batchChangeID,
            onlyPublishedByThisBatchChange: null,
            search: changesetFilters.search,
            onlyArchived: !!onlyArchived,
        }),
        [changesetFilters, batchChangeID, onlyArchived]
    )

    const { connection, error, loading, fetchMore, hasNextPage } = useShowMorePagination<
        BatchChangeChangesetsResult,
        BatchChangeChangesetsVariables,
        ExternalChangesetFields | HiddenExternalChangesetFields
    >({
        query: CHANGESETS,
        variables: {
            ...queryArguments,
            first: BATCH_COUNT,
            after: null,
            onlyClosable: null,
        },
        options: {
            useURL: true,
            fetchPolicy: 'cache-and-network',
            pollInterval: 5000,
        },
        getConnection: result => {
            const data = dataOrThrowErrors(result)

            if (!data.node) {
                throw new Error(`Batch change with ID ${batchChangeID} does not exist`)
            }
            if (data.node.__typename !== 'BatchChange') {
                throw new Error(`The given ID is a ${data.node.__typename as string}, not a BatchChange`)
            }
            return data.node.changesets
        },
    })

    useEffect(() => {
        if (connection?.nodes?.length) {
            // Available changesets are all changesets that the user can view.
            setVisible(
                true,
                connection.nodes.filter(node => node.__typename === 'ExternalChangeset').map(node => node.id)
            )
        }
    }, [connection?.nodes, setVisible])

    const containerElements = useMemo(() => new Subject<HTMLElement | null>(), [])
    const nextContainerElement = useMemo(() => containerElements.next.bind(containerElements), [containerElements])

    const showSelectRow = viewerCanAdminister && (selected === 'all' || selected.size > 0)

    const emptyElement = useMemo(() => {
        if (filtersSelected(changesetFilters)) {
            return <EmptyChangesetSearchElement />
        }

        if (onlyArchived) {
            return <EmptyArchivedChangesetListElement />
        }

        if (batchChangeState === BatchChangeState.DRAFT && isExecutionEnabled) {
            return <EmptyDraftChangesetListElement />
        }

        return <EmptyChangesetListElement />
    }, [changesetFilters, onlyArchived, batchChangeState, isExecutionEnabled])

    return (
        <Container>
            {!hideFilters && !showSelectRow && (
                <ChangesetFilterRow onFiltersChange={setChangesetFiltersAndDeselectAll} />
            )}
            {showSelectRow && queryArguments && (
                <ChangesetSelectRow
                    batchChangeID={batchChangeID}
                    onSubmit={onSubmitBulkAction}
                    queryAllChangesetIDs={queryAllChangesetIDs}
                    queryArguments={queryArguments}
                    telemetryRecorder={telemetryRecorder}
                />
            )}
            <div className="list-group position-relative" ref={nextContainerElement}>
                <ConnectionContainer>
                    {error && <ConnectionError errors={[error.message]} />}
                    <ConnectionList className={styles.batchChangeChangesetsGrid} aria-label="changesets">
                        {connection?.nodes?.length ? (
                            <BatchChangeChangesetsHeader
                                allSelected={showSelectRow && areAllVisibleSelected()}
                                toggleSelectAll={toggleVisible}
                                disabled={!viewerCanAdminister}
                            />
                        ) : null}
                        {connection?.nodes?.map(node => (
                            <ChangesetNode
                                key={node.id}
                                node={node}
                                viewerCanAdminister={viewerCanAdminister}
                                expandByDefault={expandByDefault}
                                queryExternalChangesetWithFileDiffs={queryExternalChangesetWithFileDiffs}
                                selectable={{ onSelect: toggleSingle, isSelected }}
                            />
                        ))}
                    </ConnectionList>
                    {/* TODO: This is to prevent the spinner from flashing as we constantly
                    poll in the background. Once we can distinguish between "loading new data"
                    and "refetching existing data" with the Apollo cache, we should rework to
                    show the spinner whenever we are loading new data. */}
                    {loading && connection?.nodes?.length === 0 && <ConnectionLoading />}
                    {connection && (
                        <SummaryContainer centered={true}>
                            <ConnectionSummary
                                noSummaryIfAllNodesVisible={true}
                                first={BATCH_COUNT}
                                centered={true}
                                connection={connection}
                                noun="changeset"
                                pluralNoun="changesets"
                                hasNextPage={hasNextPage}
                                emptyElement={emptyElement}
                            />
                            {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                        </SummaryContainer>
                    )}
                </ConnectionContainer>
            </div>
        </Container>
    )
}

/**
 * Returns true, if any filter is selected.
 */
function filtersSelected(filters: ChangesetFilters): boolean {
    return filters.checkState !== null || filters.state !== null || filters.reviewState !== null || !!filters.search
}
