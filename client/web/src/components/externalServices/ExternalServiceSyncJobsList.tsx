import React, { useCallback } from 'react'

import type { Subject } from 'rxjs'
import { repeat, tap } from 'rxjs/operators'

import { H2 } from '@sourcegraph/wildcard'

import {
    type ExternalServiceSyncJobConnectionFields,
    type ExternalServiceSyncJobListFields,
    ExternalServiceSyncJobState,
    type Scalars,
} from '../../graphql-operations'
import { FilteredConnection, type FilteredConnectionQueryArguments } from '../FilteredConnection'

import { queryExternalServiceSyncJobs as _queryExternalServiceSyncJobs } from './backend'
import { EXTERNAL_SERVICE_SYNC_RUNNING_STATUSES } from './externalServices'
import { ExternalServiceSyncJobNode, type ExternalServiceSyncJobNodeProps } from './ExternalServiceSyncJobNode'

interface ExternalServiceSyncJobsListProps {
    externalServiceID: Scalars['ID']
    updates: Subject<void>
    updateSyncInProgress: (syncInProgress: boolean) => void
    updateNumberOfRepos: (numberOfRepos: number) => void

    /** For testing only. */
    queryExternalServiceSyncJobs?: typeof _queryExternalServiceSyncJobs
}

export const ExternalServiceSyncJobsList: React.FunctionComponent<ExternalServiceSyncJobsListProps> = ({
    externalServiceID,
    updates,
    updateSyncInProgress,
    updateNumberOfRepos,
    queryExternalServiceSyncJobs = _queryExternalServiceSyncJobs,
}) => {
    const queryConnection = useCallback(
        (args: FilteredConnectionQueryArguments) =>
            queryExternalServiceSyncJobs({
                first: args.first ?? null,
                externalService: externalServiceID,
            }).pipe(
                tap(({ nodes }) => {
                    if (nodes?.length > 0 && nodes[0]) {
                        const syncJob = nodes[0]
                        const state = syncJob.state
                        updateSyncInProgress(EXTERNAL_SERVICE_SYNC_RUNNING_STATUSES.has(state))
                        if (state === ExternalServiceSyncJobState.COMPLETED) {
                            updateNumberOfRepos(syncJob.reposSynced)
                        }
                    }
                }),
                repeat({ delay: 1500 })
            ),
        [externalServiceID, queryExternalServiceSyncJobs, updateSyncInProgress, updateNumberOfRepos]
    )

    return (
        <>
            <H2 className="mt-3">Recent sync jobs</H2>
            <FilteredConnection<
                ExternalServiceSyncJobListFields,
                Omit<ExternalServiceSyncJobNodeProps, 'node'>,
                {},
                ExternalServiceSyncJobConnectionFields
            >
                className="mb-0 mt-1"
                noun="sync job"
                listClassName="list-group-flush"
                pluralNoun="sync jobs"
                queryConnection={queryConnection}
                nodeComponent={ExternalServiceSyncJobNode}
                nodeComponentProps={{ onUpdate: updates }}
                hideSearch={true}
                noSummaryIfAllNodesVisible={true}
                updates={updates}
            />
        </>
    )
}
