import { useCallback, useEffect, type FC } from 'react'

import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { H2 } from '@sourcegraph/wildcard'

import { FilteredConnection, type Filter } from '../../components/FilteredConnection'
import type {
    ListNotebooksResult,
    ListNotebooksVariables,
    NotebookFields,
    NotebooksOrderBy,
} from '../../graphql-operations'
import type { fetchNotebooks as _fetchNotebooks } from '../backend'

import { NotebookNode, type NotebookNodeProps } from './NotebookNode'
import { type NotebooksFilterEvents } from './NotebooksListPage'

import styles from './NotebooksList.module.scss'

export interface NotebooksListProps extends TelemetryProps {
    title: string
    logEventName: NotebooksFilterEvents
    orderOptions: Filter[]
    creatorUserID?: string
    starredByUserID?: string
    namespace?: string
    fetchNotebooks: typeof _fetchNotebooks
}

export const NotebooksList: FC<NotebooksListProps> = ({
    title,
    logEventName,
    orderOptions,
    creatorUserID,
    starredByUserID,
    namespace,
    fetchNotebooks,
    telemetryService,
}) => {
    useEffect(() => {
        // No V2 telemetry required, as this is duplicative with the view event logged in NotebooksListPage.tsx.
        telemetryService.logViewEvent(`SearchNotebooksList${logEventName}`)
    }, [logEventName, telemetryService])

    const queryConnection = useCallback(
        (args: Partial<ListNotebooksVariables>) => {
            const { orderBy, descending } = args as {
                orderBy: NotebooksOrderBy | undefined
                descending: boolean | undefined
            }

            return fetchNotebooks({
                first: args.first ?? 10,
                query: args.query ?? undefined,
                after: args.after ?? undefined,
                creatorUserID,
                starredByUserID,
                namespace,
                orderBy,
                descending,
            })
        },
        [creatorUserID, starredByUserID, namespace, fetchNotebooks]
    )

    return (
        <div>
            <H2 className="mb-3">{title}</H2>
            <FilteredConnection<NotebookFields, Omit<NotebookNodeProps, 'node'>, ListNotebooksResult['notebooks']>
                defaultFirst={10}
                compact={false}
                queryConnection={queryConnection}
                filters={orderOptions}
                hideSearch={false}
                nodeComponent={NotebookNode}
                noun="notebook"
                pluralNoun="notebooks"
                noSummaryIfAllNodesVisible={true}
                cursorPaging={true}
                inputClassName={styles.filterInput}
                inputPlaceholder="Search notebooks by title and content"
                useURLQuery={false}
            />
        </div>
    )
}
