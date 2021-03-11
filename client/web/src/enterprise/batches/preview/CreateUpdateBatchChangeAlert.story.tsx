import { storiesOf } from '@storybook/react'
import { boolean } from '@storybook/addon-knobs'
import React from 'react'
import { CreateUpdateBatchChangeAlert } from './CreateUpdateBatchChangeAlert'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/preview/CreateUpdateBatchChangeAlert', module)
    .addDecorator(story => <div className="p-3 container web-content">{story()}</div>)
    .addParameters({
        chromatic: {
            viewports: [320, 576, 978, 1440],
        },
    })

add('Create', () => (
    <EnterpriseWebStory>
        {props => (
            <CreateUpdateBatchChangeAlert
                {...props}
                specID="123"
                batchChange={null}
                viewerCanAdminister={boolean('viewerCanAdminister', true)}
            />
        )}
    </EnterpriseWebStory>
))
add('Update', () => (
    <EnterpriseWebStory>
        {props => (
            <CreateUpdateBatchChangeAlert
                {...props}
                specID="123"
                batchChange={{ id: '123', name: 'awesome-batch-change', url: 'http://test.test/awesome' }}
                viewerCanAdminister={boolean('viewerCanAdminister', true)}
            />
        )}
    </EnterpriseWebStory>
))
