import { storiesOf } from '@storybook/react'
import { subDays } from 'date-fns'
import React from 'react'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'
import { SupersedingBatchSpecAlert } from './SupersedingBatchSpecAlert'

const { add } = storiesOf('web/batches/details/SupersedingBatchSpecAlert', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

add('None published', () => (
    <EnterpriseWebStory>
        {() => (
            <SupersedingBatchSpecAlert
                spec={{
                    applyURL: '/users/alice/batch-changes/preview/123456SAMPLEID',
                    createdAt: subDays(new Date(), 1).toISOString(),
                }}
            />
        )}
    </EnterpriseWebStory>
))
