import React, { useCallback, useMemo } from 'react'
import { RouteComponentProps } from 'react-router'
import { Observable, Subject } from 'rxjs'
import { FilteredConnection } from '../../../components/FilteredConnection'
import { PageHeader } from '../../../components/PageHeader'
import {
    BatchChangesCodeHostFields,
    BatchChangesCodeHostsFields,
    Scalars,
    UserBatchChangesCodeHostsVariables,
} from '../../../graphql-operations'
import { BatchChangesIcon } from '../icons'
import { queryUserBatchChangesCodeHosts as _queryUserBatchChangesCodeHosts } from './backend'
import { CodeHostConnectionNode, CodeHostConnectionNodeProps } from './CodeHostConnectionNode'

export interface CodeHostConnectionsProps extends Pick<RouteComponentProps, 'history' | 'location'> {
    userID: Scalars['ID']
    queryUserBatchChangesCodeHosts?: typeof _queryUserBatchChangesCodeHosts
}

export const CodeHostConnections: React.FunctionComponent<CodeHostConnectionsProps> = ({
    userID,
    history,
    location,
    queryUserBatchChangesCodeHosts = _queryUserBatchChangesCodeHosts,
}) => {
    // Subject to fire a reload of the list.
    const updateList = useMemo(() => new Subject<void>(), [])
    const query = useCallback<
        (args: Partial<UserBatchChangesCodeHostsVariables>) => Observable<BatchChangesCodeHostsFields>
    >(
        args =>
            queryUserBatchChangesCodeHosts({
                user: userID,
                first: args.first ?? null,
                after: args.after ?? null,
            }),
        [queryUserBatchChangesCodeHosts, userID]
    )
    return (
        <>
            <PageHeader path={[{ icon: BatchChangesIcon, text: 'Batch Changes' }]} className="mb-3" />
            <h2>Code host tokens</h2>
            <p>Add authentication tokens to enable batch changes changeset creation on your code hosts.</p>
            <FilteredConnection<BatchChangesCodeHostFields, Omit<CodeHostConnectionNodeProps, 'node'>>
                history={history}
                location={location}
                useURLQuery={false}
                nodeComponent={CodeHostConnectionNode}
                nodeComponentProps={{ userID, updateList }}
                queryConnection={query}
                hideSearch={true}
                defaultFirst={15}
                noun="code host"
                pluralNoun="code hosts"
                listClassName="list-group"
                updates={updateList}
                className="mb-3"
                cursorPaging={true}
                noSummaryIfAllNodesVisible={true}
            />
            <p>
                Code host not present? Site admins can add a code host in{' '}
                <a href="https://docs.sourcegraph.com/admin/external_service" target="_blank" rel="noopener noreferrer">
                    the manage repositories settings
                </a>
                .
            </p>
        </>
    )
}
