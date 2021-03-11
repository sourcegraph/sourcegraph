import { storiesOf } from '@storybook/react'
import React from 'react'
import { BatchSpecInfoByline } from './BatchSpecInfoByline'
import { subDays } from 'date-fns'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/preview/BatchSpecInfoByline', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

add('Default', () => (
    <EnterpriseWebStory>
        {() => (
            <BatchSpecInfoByline
                createdAt={subDays(new Date(), 3).toISOString()}
                creator={{ url: 'http://test.test/alice', username: 'alice' }}
            />
        )}
    </EnterpriseWebStory>
))
