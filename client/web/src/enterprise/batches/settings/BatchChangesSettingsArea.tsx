import React from 'react'
import { RouteComponentProps } from 'react-router'
import { PageTitle } from '../../../components/PageTitle'
import { UserAreaUserFields } from '../../../graphql-operations'
import { queryUserBatchChangesCodeHosts } from './backend'
import { CodeHostConnections } from './CodeHostConnections'

export interface BatchChangesSettingsAreaProps extends Pick<RouteComponentProps, 'history' | 'location'> {
    user: Pick<UserAreaUserFields, 'id'>
    queryUserBatchChangesCodeHosts?: typeof queryUserBatchChangesCodeHosts
}

/** The page area for all batch changes settings. It's shown in the user settings sidebar. */
export const BatchChangesSettingsArea: React.FunctionComponent<BatchChangesSettingsAreaProps> = props => (
    <div className="web-content test-batches-settings-page">
        <PageTitle title="Batch changes settings" />
        <CodeHostConnections userID={props.user.id} {...props} />
    </div>
)
