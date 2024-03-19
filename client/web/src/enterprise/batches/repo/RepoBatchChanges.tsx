import React, { useCallback } from 'react'

import { map } from 'rxjs/operators'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Container, H3, H5 } from '@sourcegraph/wildcard'

import { FilteredConnection, type FilteredConnectionQueryArguments } from '../../../components/FilteredConnection'
import type { RepoBatchChange, RepositoryFields } from '../../../graphql-operations'
import { queryExternalChangesetWithFileDiffs as _queryExternalChangesetWithFileDiffs } from '../detail/backend'
import { GettingStarted } from '../list/GettingStarted'

import { queryRepoBatchChanges as _queryRepoBatchChanges } from './backend'
import { BatchChangeNode, type BatchChangeNodeProps } from './BatchChangeNode'

import styles from './RepoBatchChanges.module.scss'

interface Props extends TelemetryV2Props {
    viewerCanAdminister: boolean
    // canCreate indicates whether or not the currently-authenticated user has sufficient
    // permissions to create a batch change. If not, canCreate will be a string reason why
    // the user cannot create.
    canCreate: true | string
    repo: RepositoryFields
    isSourcegraphDotCom: boolean
    onlyArchived?: boolean

    /** For testing only. */
    queryRepoBatchChanges?: typeof _queryRepoBatchChanges
    /** For testing only. */
    queryExternalChangesetWithFileDiffs?: typeof _queryExternalChangesetWithFileDiffs
}

/**
 * A list of batch changes affecting a particular repo.
 */
export const RepoBatchChanges: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    viewerCanAdminister,
    canCreate,
    repo,
    isSourcegraphDotCom,
    queryRepoBatchChanges = _queryRepoBatchChanges,
    queryExternalChangesetWithFileDiffs = _queryExternalChangesetWithFileDiffs,
    telemetryRecorder,
}) => {
    const query = useCallback(
        (args: FilteredConnectionQueryArguments) => {
            const passedArguments = {
                name: repo.name,
                repoID: repo.id,
                first: args.first ?? null,
                after: args.after ?? null,
            }
            return queryRepoBatchChanges(passedArguments).pipe(map(data => data.batchChanges))
        },
        [queryRepoBatchChanges, repo.id, repo.name]
    )

    return (
        <Container role="region" aria-label="batch changes">
            <FilteredConnection<RepoBatchChange, Omit<BatchChangeNodeProps, 'node'>>
                nodeComponent={BatchChangeNode}
                nodeComponentProps={{
                    queryExternalChangesetWithFileDiffs,
                    viewerCanAdminister,
                }}
                queryConnection={query}
                hideSearch={true}
                defaultFirst={15}
                noun="batch change"
                pluralNoun="batch changes"
                listClassName={styles.batchChangesGrid}
                withCenteredSummary={true}
                headComponent={RepoBatchChangesHeader}
                cursorPaging={true}
                noSummaryIfAllNodesVisible={true}
                emptyElement={
                    <GettingStarted
                        isSourcegraphDotCom={isSourcegraphDotCom}
                        canCreate={canCreate}
                        telemetryRecorder={telemetryRecorder}
                    />
                }
            />
        </Container>
    )
}

export const RepoBatchChangesHeader: React.FunctionComponent<React.PropsWithChildren<unknown>> = () => (
    <>
        {/* Empty filler elements for the spaces in the grid that don't need headers */}
        <span />
        <span />
        <H5 as={H3} aria-hidden={true} className="p-2 d-none d-md-block text-uppercase text-center text-nowrap">
            Status
        </H5>
        <H5 as={H3} aria-hidden={true} className="p-2 d-none d-md-block text-uppercase text-nowrap">
            Changeset information
        </H5>
        <H5 as={H3} aria-hidden={true} className="p-2 d-none d-md-block text-uppercase text-center text-nowrap">
            Check state
        </H5>
        <H5 as={H3} aria-hidden={true} className="p-2 d-none d-md-block text-uppercase text-center text-nowrap">
            Review state
        </H5>
        <H5 as={H3} aria-hidden={true} className="p-2 d-none d-md-block text-uppercase text-center text-nowrap">
            Changes
        </H5>
    </>
)
