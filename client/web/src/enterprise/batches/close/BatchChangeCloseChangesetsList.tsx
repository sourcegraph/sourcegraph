import React, { useCallback } from 'react'

import { repeat } from 'rxjs/operators'

import type { ErrorLike } from '@sourcegraph/common'
import { Container } from '@sourcegraph/wildcard'

import { type FilteredConnectionQueryArguments, FilteredConnection } from '../../../components/FilteredConnection'
import type { Scalars, ChangesetFields, BatchChangeChangesetsResult } from '../../../graphql-operations'
import {
    queryChangesets as _queryChangesets,
    type queryExternalChangesetWithFileDiffs as _queryExternalChangesetWithFileDiffs,
} from '../detail/backend'

import {
    BatchChangeCloseHeaderWillCloseChangesets,
    BatchChangeCloseHeaderWillKeepChangesets,
} from './BatchChangeCloseHeader'
import { type ChangesetCloseNodeProps, ChangesetCloseNode } from './ChangesetCloseNode'
import { CloseChangesetsListEmptyElement } from './CloseChangesetsListEmptyElement'

import styles from './BatchChangeCloseChangesetsList.module.scss'

interface Props {
    batchChangeID: Scalars['ID']
    viewerCanAdminister: boolean
    willClose: boolean
    onUpdate?: (
        connection?: (BatchChangeChangesetsResult['node'] & { __typename: 'BatchChange' })['changesets'] | ErrorLike
    ) => void

    /** For testing only. */
    queryChangesets?: typeof _queryChangesets
    /** For testing only. */
    queryExternalChangesetWithFileDiffs?: typeof _queryExternalChangesetWithFileDiffs
}

/**
 * A list of a batch change's changesets that may be closed.
 */
export const BatchChangeCloseChangesetsList: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    batchChangeID,
    viewerCanAdminister,
    willClose,
    onUpdate,
    queryChangesets = _queryChangesets,
    queryExternalChangesetWithFileDiffs,
}) => {
    const queryChangesetsConnection = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryChangesets({
                state: null,
                onlyClosable: true,
                checkState: null,
                reviewState: null,
                first: args.first ?? null,
                after: args.after ?? null,
                batchChange: batchChangeID,
                onlyPublishedByThisBatchChange: true,
                search: null,
                onlyArchived: false,
            }).pipe(repeat({ delay: 5000 })),
        [batchChangeID, queryChangesets]
    )

    return (
        <div className="list-group position-relative">
            <Container role="region" aria-label="affected changesets">
                <FilteredConnection<
                    ChangesetFields,
                    Omit<ChangesetCloseNodeProps, 'node'>,
                    {},
                    (BatchChangeChangesetsResult['node'] & { __typename: 'BatchChange' })['changesets']
                >
                    nodeComponent={ChangesetCloseNode}
                    nodeComponentProps={{
                        viewerCanAdminister,
                        queryExternalChangesetWithFileDiffs,
                        willClose,
                    }}
                    queryConnection={queryChangesetsConnection}
                    hideSearch={true}
                    defaultFirst={15}
                    noun="open changeset"
                    pluralNoun="open changesets"
                    useURLQuery={true}
                    listClassName={styles.batchChangeCloseChangesetsListGrid}
                    headComponent={
                        willClose ? BatchChangeCloseHeaderWillCloseChangesets : BatchChangeCloseHeaderWillKeepChangesets
                    }
                    noSummaryIfAllNodesVisible={true}
                    onUpdate={onUpdate}
                    emptyElement={<CloseChangesetsListEmptyElement />}
                    withCenteredSummary={true}
                />
            </Container>
        </div>
    )
}
