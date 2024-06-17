import React from 'react'

import { PageHeader, Text } from '@sourcegraph/wildcard'

import { PageTitle } from '../../../components/PageTitle'
import type { UserAreaUserFields } from '../../../graphql-operations'

import { UserCodeHostConnections } from './CodeHostConnections'
import { UserCommitSigningIntegrations } from './CommitSigningIntegrations'
import { RolloutWindowsConfiguration } from './RolloutWindowsConfiguration'

export interface BatchChangesSettingsAreaProps {
    user: UserAreaUserFields
}

/** The page area for all batch changes settings. It's shown in the user settings sidebar. */
export const BatchChangesSettingsArea: React.FunctionComponent<
    React.PropsWithChildren<BatchChangesSettingsAreaProps>
> = props => (
    <div className="test-batches-settings-page">
        <PageTitle title="Batch changes settings" />
        <PageHeader headingElement="h2" path={[{ text: 'Batch Changes settings' }]} className="mb-3" />
        <RolloutWindowsConfiguration />
        <UserCodeHostConnections
            headerLine={<Text>Add access tokens to enable Batch Changes changeset creation on your code hosts.</Text>}
            user={props.user}
        />
        <UserCommitSigningIntegrations userID={props.user.id} />
    </div>
)
