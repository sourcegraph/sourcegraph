import { storiesOf } from '@storybook/react'
import { boolean } from '@storybook/addon-knobs'
import React from 'react'
import { GlobalBatchChangesArea } from './GlobalBatchChangesArea'
import { AuthenticatedUser } from '../../../auth'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/GlobalBatchChangesArea', module).addDecorator(story => (
    <React.Suspense fallback={<LoadingSpinner />}>
        <div className="p-3 container">{story()}</div>
    </React.Suspense>
))

add('Dotcom', () => (
    <EnterpriseWebStory>
        {props => (
            <GlobalBatchChangesArea
                {...props}
                isSourcegraphDotCom={true}
                platformContext={undefined as any}
                extensionsController={undefined as any}
                authenticatedUser={
                    boolean('isAuthenticated', false) ? ({ username: 'alice' } as AuthenticatedUser) : null
                }
                match={{ isExact: true, path: '/batch-changes', url: 'http://test.test/batch-changes', params: {} }}
            />
        )}
    </EnterpriseWebStory>
))
