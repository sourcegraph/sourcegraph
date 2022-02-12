import React from 'react'
import { RouteComponentProps } from 'react-router'

import { PageHeader, Alert } from '@sourcegraph/wildcard'

import { PageTitle } from '../../../components/PageTitle'

import { CodeHostConnections } from './CodeHostConnections'

/** The page area for all batch changes settings. It's shown in the site admin settings sidebar. */
export const BatchChangesSiteConfigSettingsArea: React.FunctionComponent<
    Pick<RouteComponentProps, 'history' | 'location'>
> = props => (
    <>
        <PageTitle title="Batch changes settings" />
        <PageHeader headingElement="h2" path={[{ text: 'Batch Changes settings' }]} className="mb-3" />
        <CodeHostConnections
            headerLine={
                <>
                    <p>Add access tokens to enable Batch Changes changeset creation for all users.</p>
                    <Alert variant="info">
                        You are configuring <strong>global credentials</strong> for Batch Changes. The credentials on
                        this page can be used by all users of this Sourcegraph instance to create and sync changesets.
                    </Alert>
                </>
            }
            userID={null}
            {...props}
        />
    </>
)
