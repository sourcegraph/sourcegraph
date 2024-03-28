import React, { useCallback, useState } from 'react'

import { timer } from 'rxjs'
import { repeat, retry, tap } from 'rxjs/operators'

import { FilteredConnection, type FilteredConnectionQueryArguments } from '../../../../../components/FilteredConnection'
import { ConnectionError } from '../../../../../components/FilteredConnection/ui'
import type {
    Scalars,
    VisibleBatchSpecWorkspaceListFields,
    HiddenBatchSpecWorkspaceListFields,
} from '../../../../../graphql-operations'
import { Header as WorkspacesListHeader } from '../../../workspaces-list'
import { queryWorkspacesList as _queryWorkspacesList } from '../backend'

import { WorkspaceFilterRow, type WorkspaceFilters } from './WorkspacesFilterRow'
import { WorkspacesListItem, type WorkspacesListItemProps } from './WorkspacesListItem'

import styles from './Workspaces.module.scss'

export interface WorkspacesProps {
    batchSpecID: Scalars['ID']
    /** The currently selected workspace node id. Will be highlighted. */
    selectedNode?: Scalars['ID']
    /** The URL path to the execution page this workspaces list is shown on. */
    executionURL: string
    /** For testing only. */
    queryWorkspacesList?: typeof _queryWorkspacesList
}

export const Workspaces: React.FunctionComponent<React.PropsWithChildren<WorkspacesProps>> = ({
    batchSpecID,
    selectedNode,
    executionURL,
    queryWorkspacesList = _queryWorkspacesList,
}) => {
    const [filters, setFilters] = useState<WorkspaceFilters>({ state: null, search: null })
    const [error, setError] = useState<string>()

    const queryWorkspacesListConnection = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryWorkspacesList({
                batchSpecID,
                first: args.first ?? null,
                after: args.after ?? null,
                search: filters.search ?? null,
                state: filters.state ?? null,
            }).pipe(
                repeat({ delay: 2500 }),
                retry({
                    delay: error => {
                        // Capture the error, but don't throw it so the data in the
                        // connection remains visible.
                        setError(error)
                        return timer(5000)
                    },
                }),
                tap(() => {
                    // Reset the error when the query succeeds.
                    setError(undefined)
                })
            ),
        [batchSpecID, filters.search, filters.state, queryWorkspacesList]
    )

    return (
        <div className="d-flex flex-column w-100 h-100 pr-3">
            <WorkspacesListHeader>Workspaces</WorkspacesListHeader>
            <WorkspaceFilterRow onFiltersChange={setFilters} />
            <div className={styles.listContainer}>
                {error && <ConnectionError errors={[error]} className="mb-2" />}
                {/* We need to use FilteredConnection over the new composable API here because we
                still have to use observables for this connection query. See query docstring for
                more details. */}
                <FilteredConnection<
                    VisibleBatchSpecWorkspaceListFields | HiddenBatchSpecWorkspaceListFields,
                    Omit<WorkspacesListItemProps, 'node'>
                >
                    nodeComponent={WorkspacesListItem}
                    nodeComponentProps={{
                        selectedNode,
                        executionURL,
                    }}
                    queryConnection={queryWorkspacesListConnection}
                    hideSearch={true}
                    defaultFirst={20}
                    noun="workspace"
                    pluralNoun="workspaces"
                    useURLQuery={true}
                    noSummaryIfAllNodesVisible={true}
                    withCenteredSummary={true}
                />
            </div>
        </div>
    )
}
